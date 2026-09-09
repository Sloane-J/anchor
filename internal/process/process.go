// Package process starts and stops child processes as an owned group.
//
// A Handle represents one started process together with the OS resources
// needed to guarantee that stopping it also stops anything it spawned.
// Callers never target processes by PID guess or image name — only by the
// Handle returned from Start.
package process

import "context"

// Spec describes how to start one process.
type Spec struct {
	// Name identifies the process for logs and errors. It is not passed to
	// the OS; it is normally the service name from dev.yaml.
	Name string

	// Command is the executable to run. It is never passed through a shell.
	Command string

	// Args are passed directly to the executable.
	Args []string

	// Dir is the working directory. It must already be resolved and
	// validated by the caller; Runner does not fall back to the current
	// directory if Dir is invalid.
	Dir string

	// Stdout and Stderr, if set, receive the process's output streams.
	Stdout, Stderr WriteCloserLike
}

// WriteCloserLike is the minimal sink Runner writes process output to.
// io.Writer is intentionally sufficient; this alias documents intent.
type WriteCloserLike interface {
	Write(p []byte) (n int, err error)
}

// Handle represents one running process owned by a Runner.
type Handle interface {
	// Name returns the Spec.Name this handle was started with.
	Name() string

	// Wait blocks until the process exits and returns its outcome.
	// Wait must be safe to call from a single goroutine per Handle.
	Wait() (ExitResult, error)

	// Stop terminates the process and everything it spawned. Stop is
	// idempotent: calling it after the process has already exited is not
	// an error.
	Stop() error
}

// ExitResult describes how a process finished.
type ExitResult struct {
	// Code is the process exit code. It is meaningless if Stopped is true.
	Code int

	// Stopped is true if the process was terminated by Stop rather than
	// exiting on its own.
	Stopped bool
}

// Runner starts processes. Implementations must guarantee that Handle.Stop
// terminates every process the started command spawns, not only the
// directly-created process.
type Runner interface {
	Start(ctx context.Context, spec Spec) (Handle, error)
}