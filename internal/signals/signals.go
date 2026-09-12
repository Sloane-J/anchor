// Package signals translates OS interruption signals (Ctrl+C, Ctrl+Break
// on Windows) into context cancellation, so the rest of the application
// only ever deals with context.Context.
package signals

import (
	"context"
	"os"
	"os/signal"
)

// WithCancelOnInterrupt returns a context that is cancelled the first time
// the process receives an interrupt signal (Ctrl+C / SIGINT, and
// Ctrl+Break on Windows). The returned stop function releases the signal
// handler; callers should defer it.
//
// A second interrupt signal after the first is not specially handled here
// by design — v0.1 relies on the supervisor's shutdown completing
// promptly. If a future version needs a "force quit on second Ctrl+C"
// behaviour, it is added here without changing callers.
func WithCancelOnInterrupt(parent context.Context) (ctx context.Context, stop func()) {
	return signal.NotifyContext(parent, os.Interrupt)
}