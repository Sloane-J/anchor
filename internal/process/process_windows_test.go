//go:build windows

package process

import (
	"context"
	"testing"
	"time"
)

// TestWindowsRunnerStartAndStop launches a real, harmless long-running
// child (ping with an effectively infinite loop) and confirms Stop
// terminates it within a reasonable time. This is the only test in this
// package that touches a real OS process; everything else uses Fake.
func TestWindowsRunnerStartAndStop(t *testing.T) {
	runner := NewRunner()

	handle, err := runner.Start(context.Background(), Spec{
		Name:    "spike-child",
		Command: "ping",
		Args:    []string{"-t", "127.0.0.1"},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	done := make(chan ExitResult, 1)
	waitErr := make(chan error, 1)
	go func() {
		result, err := handle.Wait()
		waitErr <- err
		done <- result
	}()

	// Give the process a moment to actually be running before we stop it.
	time.Sleep(500 * time.Millisecond)

	if err := handle.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	select {
	case result := <-done:
		if err := <-waitErr; err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
		if !result.Stopped {
			t.Fatalf("ExitResult.Stopped = false, want true (result = %+v)", result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Wait() did not return within 10s of Stop()")
	}
}

// TestWindowsRunnerNaturalExit confirms a process that exits on its own
// (not stopped) reports Stopped=false and its real exit code.
func TestWindowsRunnerNaturalExit(t *testing.T) {
	runner := NewRunner()

	// "cmd /c exit 3" starts, exits immediately with code 3, no Stop call.
	handle, err := runner.Start(context.Background(), Spec{
		Name:    "spike-exit",
		Command: "cmd",
		Args:    []string{"/c", "exit", "3"},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := handle.Wait()
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if result.Stopped {
		t.Fatal("ExitResult.Stopped = true, want false")
	}
	if result.Code != 3 {
		t.Fatalf("ExitResult.Code = %d, want 3", result.Code)
	}
}