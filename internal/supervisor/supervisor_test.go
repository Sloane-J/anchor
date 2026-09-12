package supervisor

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"anchor/internal/dependency"
	"anchor/internal/process"
)

func specsFor(names ...string) map[string]ServiceSpec {
	specs := make(map[string]ServiceSpec, len(names))
	for _, n := range names {
		specs[n] = ServiceSpec{Name: n, Command: "noop"}
	}
	return specs
}

// runWithTimeout runs sup.Run in a goroutine and fails the test if it does
// not return within the given timeout, since Run now blocks until
// cancellation or an exit.
func runWithTimeout(t *testing.T, sup *Supervisor, ctx context.Context, plan dependency.Plan, events chan StatusEvent, timeout time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- sup.Run(ctx, plan, events)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		t.Fatal("Run() did not return within timeout")
		return nil
	}
}

func TestRunStartsServicesInPlanOrderThenWaitsForCancellation(t *testing.T) {
	fake := process.NewFake()
	specs := specsFor("db", "api", "web")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api", "web"}}

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan StatusEvent, 32)

	done := make(chan error, 1)
	go func() {
		done <- sup.Run(ctx, plan, events)
	}()

	// Give the startup loop time to run and reach the watch phase.
	time.Sleep(100 * time.Millisecond)

	want := []string{"db", "api", "web"}
	if got := fake.Started(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Started() = %v, want %v", got, want)
	}

	status := sup.Status()
	for _, name := range want {
		if status[name] != Running {
			t.Errorf("Status()[%q] = %v, want Running", name, status[name])
		}
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of cancellation")
	}
	close(events)

	// Cancellation should have stopped everything, in reverse order.
	wantStopped := []string{"web", "api", "db"}
	if got := fake.Stopped(); !reflect.DeepEqual(got, wantStopped) {
		t.Fatalf("Stopped() = %v, want %v", got, wantStopped)
	}
}

func TestRunStopsStartedServicesOnStartFailure(t *testing.T) {
	fake := process.NewFake()
	fake.FailStart("web", errors.New("boom"))
	specs := specsFor("db", "api", "web")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api", "web"}}

	events := make(chan StatusEvent, 32)
	err := runWithTimeout(t, sup, context.Background(), plan, events, 5*time.Second)
	close(events)

	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}

	// db and api started; web failed to start. Only db and api should have
	// been stopped, in reverse order.
	want := []string{"api", "db"}
	if got := fake.Stopped(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Stopped() = %v, want %v", got, want)
	}

	status := sup.Status()
	if status["web"] != Failed {
		t.Errorf("Status()[web] = %v, want Failed", status["web"])
	}
	if status["db"] != Stopped {
		t.Errorf("Status()[db] = %v, want Stopped", status["db"])
	}
	if status["api"] != Stopped {
		t.Errorf("Status()[api] = %v, want Stopped", status["api"])
	}
}

func TestRunReturnsImmediatelyIfCancelledBeforeStart(t *testing.T) {
	fake := process.NewFake()
	specs := specsFor("db", "api", "web")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api", "web"}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Run even starts the loop

	events := make(chan StatusEvent, 32)
	err := runWithTimeout(t, sup, ctx, plan, events, 5*time.Second)
	close(events)

	if err == nil {
		t.Fatal("Run() error = nil, want context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}

	if got := fake.Started(); len(got) != 0 {
		t.Fatalf("Started() = %v, want none", got)
	}
}

func TestRunStopsRemainingServicesWhenOneExitsUnexpectedly(t *testing.T) {
	fake := process.NewFake()
	specs := specsFor("db", "api", "web")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api", "web"}}

	events := make(chan StatusEvent, 32)
	done := make(chan error, 1)
	go func() {
		done <- sup.Run(context.Background(), plan, events)
	}()

	// Give the startup loop time to finish and reach the watch phase.
	time.Sleep(100 * time.Millisecond)

	// Simulate "api" crashing on its own, unrelated to Stop being called.
	fake.Exit("api", process.ExitResult{Code: 1})

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run() error = nil, want error describing the unexpected exit")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s of the unexpected exit")
	}
	close(events)

	status := sup.Status()
	if status["api"] != Failed {
		t.Errorf("Status()[api] = %v, want Failed", status["api"])
	}

	// db and web should have been stopped as part of cleanup.
	stopped := fake.Stopped()
	stoppedSet := make(map[string]bool, len(stopped))
	for _, name := range stopped {
		stoppedSet[name] = true
	}
	if !stoppedSet["db"] {
		t.Errorf("Stopped() = %v, want it to include db", stopped)
	}
	if !stoppedSet["web"] {
		t.Errorf("Stopped() = %v, want it to include web", stopped)
	}
}

func TestRunFailsFastOnMissingSpec(t *testing.T) {
	fake := process.NewFake()
	// Only "db" has a spec; plan also wants "api", which is missing.
	specs := specsFor("db")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api"}}

	events := make(chan StatusEvent, 32)
	err := runWithTimeout(t, sup, context.Background(), plan, events, 5*time.Second)
	close(events)

	if err == nil {
		t.Fatal("Run() error = nil, want error for missing spec")
	}

	if got := fake.Stopped(); !reflect.DeepEqual(got, []string{"db"}) {
		t.Fatalf("Stopped() = %v, want [db]", got)
	}
}

func TestStopStopsRunningServicesInReverseOrder(t *testing.T) {
	fake := process.NewFake()
	specs := specsFor("db", "api", "web")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api", "web"}}

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan StatusEvent, 32)
	done := make(chan error, 1)
	go func() {
		done <- sup.Run(ctx, plan, events)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s")
	}
	close(events)

	want := []string{"web", "api", "db"}
	if got := fake.Stopped(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Stopped() = %v, want %v", got, want)
	}
}