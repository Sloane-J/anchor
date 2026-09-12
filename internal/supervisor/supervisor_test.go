package supervisor

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"dev-orchestrator/internal/dependency"
	"dev-orchestrator/internal/process"
)

func specsFor(names ...string) map[string]ServiceSpec {
	specs := make(map[string]ServiceSpec, len(names))
	for _, n := range names {
		specs[n] = ServiceSpec{Name: n, Command: "noop"}
	}
	return specs
}

func TestRunStartsServicesInPlanOrder(t *testing.T) {
	fake := process.NewFake()
	specs := specsFor("db", "api", "web")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api", "web"}}

	events := make(chan StatusEvent, 32)
	err := sup.Run(context.Background(), plan, events)
	close(events)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

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
}

func TestRunStopsStartedServicesOnStartFailure(t *testing.T) {
	fake := process.NewFake()
	fake.FailStart("web", errors.New("boom"))
	specs := specsFor("db", "api", "web")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api", "web"}}

	events := make(chan StatusEvent, 32)
	err := sup.Run(context.Background(), plan, events)
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

func TestRunStopsStartedServicesOnCancellation(t *testing.T) {
	fake := process.NewFake()
	specs := specsFor("db", "api", "web")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api", "web"}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Run even starts the loop

	events := make(chan StatusEvent, 32)
	err := sup.Run(ctx, plan, events)
	close(events)

	if err == nil {
		t.Fatal("Run() error = nil, want context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}

	// Nothing should have been started since the context was already
	// cancelled before the loop began.
	if got := fake.Started(); len(got) != 0 {
		t.Fatalf("Started() = %v, want none", got)
	}
}

func TestStopStopsRunningServicesInReverseOrder(t *testing.T) {
	fake := process.NewFake()
	specs := specsFor("db", "api", "web")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api", "web"}}

	events := make(chan StatusEvent, 32)
	if err := sup.Run(context.Background(), plan, events); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	close(events)

	sup.Stop()

	want := []string{"web", "api", "db"}
	if got := fake.Stopped(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Stopped() = %v, want %v", got, want)
	}

	status := sup.Status()
	for _, name := range want {
		if status[name] != Stopped {
			t.Errorf("Status()[%q] = %v, want Stopped", name, status[name])
		}
	}
}

func TestRunEmitsStatusEventsInOrder(t *testing.T) {
	fake := process.NewFake()
	specs := specsFor("db")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db"}}

	events := make(chan StatusEvent, 32)
	if err := sup.Run(context.Background(), plan, events); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	close(events)

	var got []State
	for ev := range events {
		got = append(got, ev.State)
	}

	want := []State{Starting, Running}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestRunFailsFastOnMissingSpec(t *testing.T) {
	fake := process.NewFake()
	// Only "db" has a spec; plan also wants "api", which is missing.
	specs := specsFor("db")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db", "api"}}

	events := make(chan StatusEvent, 32)
	err := sup.Run(context.Background(), plan, events)
	close(events)

	if err == nil {
		t.Fatal("Run() error = nil, want error for missing spec")
	}

	// db started, then api had no spec and failed immediately: db should
	// be stopped.
	if got := fake.Stopped(); !reflect.DeepEqual(got, []string{"db"}) {
		t.Fatalf("Stopped() = %v, want [db]", got)
	}
}

// TestRunWithNilEventsChannel confirms Run works fine when the caller
// doesn't care about status events.
func TestRunWithNilEventsChannel(t *testing.T) {
	fake := process.NewFake()
	specs := specsFor("db")
	sup := New(fake, specs)

	plan := dependency.Plan{Startup: []string{"db"}}

	done := make(chan error, 1)
	go func() {
		done <- sup.Run(context.Background(), plan, nil)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return within 5s")
	}
}