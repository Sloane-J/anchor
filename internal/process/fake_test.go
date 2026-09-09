package process

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestFakeRecordsStartOrder(t *testing.T) {
	f := NewFake()
	ctx := context.Background()

	if _, err := f.Start(ctx, Spec{Name: "db"}); err != nil {
		t.Fatalf("Start(db) error = %v", err)
	}
	if _, err := f.Start(ctx, Spec{Name: "api"}); err != nil {
		t.Fatalf("Start(api) error = %v", err)
	}

	want := []string{"db", "api"}
	if got := f.Started(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Started() = %v, want %v", got, want)
	}
}

func TestFakeStartFailure(t *testing.T) {
	f := NewFake()
	wantErr := errors.New("boom")
	f.FailStart("api", wantErr)

	_, err := f.Start(context.Background(), Spec{Name: "api"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Start(api) error = %v, want %v", err, wantErr)
	}
}

func TestFakeStopRecordsOrderAndUnblocksWait(t *testing.T) {
	f := NewFake()
	ctx := context.Background()

	h, err := f.Start(ctx, Spec{Name: "web"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	done := make(chan ExitResult, 1)
	go func() {
		result, _ := h.Wait()
		done <- result
	}()

	if err := h.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	result := <-done
	if !result.Stopped {
		t.Fatalf("ExitResult.Stopped = false, want true")
	}

	if got := f.Stopped(); !reflect.DeepEqual(got, []string{"web"}) {
		t.Fatalf("Stopped() = %v, want [web]", got)
	}
}

func TestFakeStopIsIdempotent(t *testing.T) {
	f := NewFake()
	h, err := f.Start(context.Background(), Spec{Name: "web"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := h.Stop(); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	if err := h.Stop(); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}

	if got := f.Stopped(); len(got) != 2 {
		t.Fatalf("Stopped() = %v, want 2 recorded calls (idempotent at handle level, Fake records both calls)", got)
	}
}

func TestFakeExitReportsNaturalExit(t *testing.T) {
	f := NewFake()
	h, err := f.Start(context.Background(), Spec{Name: "web"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	f.Exit("web", ExitResult{Code: 1})

	result, err := h.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if result.Stopped {
		t.Fatal("ExitResult.Stopped = true, want false")
	}
	if result.Code != 1 {
		t.Fatalf("ExitResult.Code = %d, want 1", result.Code)
	}
}

var _ Runner = (*Fake)(nil)