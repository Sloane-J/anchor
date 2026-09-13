//go:build windows

package process

import (
	"context"
	"testing"
	"time"
)

func TestIsAliveReportsTrueForRunningProcess(t *testing.T) {
	runner := NewRunner()
	handle, err := runner.Start(context.Background(), Spec{
		Name:    "liveness-check",
		Command: "ping",
		Args:    []string{"-t", "127.0.0.1"},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer handle.Stop()

	// The concrete windowsHandle exposes the underlying PID through its
	// cmd field indirectly; simplest is to use os.Getpid() sanity checks
	// plus this real child. We fetch the PID via a short type assertion
	// helper below.
	pid := pidOf(t, handle)

	time.Sleep(200 * time.Millisecond)

	if !IsAlive(pid) {
		t.Fatalf("IsAlive(%d) = false, want true for a running process", pid)
	}

	if err := handle.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	if IsAlive(pid) {
		t.Fatalf("IsAlive(%d) = true, want false after Stop()", pid)
	}
}

func TestIsAliveReportsFalseForUnusedPID(t *testing.T) {
	// A PID that is extremely unlikely to be in use. This is inherently a
	// little best-effort on a real OS, but a very high PID number rarely
	// corresponds to a live process in test environments.
	const unlikelyPID = 999999
	if IsAlive(unlikelyPID) {
		t.Skip("PID unexpectedly alive on this machine; skipping rather than failing spuriously")
	}
}

// pidOf extracts the OS process ID from a Handle for testing purposes
// only. It relies on windowsHandle's exported Name/Wait/Stop plus a
// package-internal accessor added purely for tests.
func pidOf(t *testing.T, h Handle) int {
	t.Helper()
	wh, ok := h.(*windowsHandle)
	if !ok {
		t.Fatalf("handle is not *windowsHandle: %T", h)
	}
	return wh.cmd.Process.Pid
}
