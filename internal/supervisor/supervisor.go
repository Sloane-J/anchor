// Package supervisor coordinates starting services in dependency order and
// stopping them cleanly on failure or cancellation.
package supervisor

import (
	"context"
	"fmt"
	"sync"

	"github.com/Sloane-J/anchor/internal/dependency"
	"github.com/Sloane-J/anchor/internal/process"
)

// State is a service's lifecycle stage. See ARCHITECTURE.md's service
// lifecycle diagram; v0.1 implements Pending, Starting, Running, Failed,
// and Stopped only.
type State int

const (
	Pending State = iota
	Starting
	Running
	Failed
	Stopped
)

func (s State) String() string {
	switch s {
	case Pending:
		return "pending"
	case Starting:
		return "starting"
	case Running:
		return "running"
	case Failed:
		return "failed"
	case Stopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// ServiceSpec is everything the supervisor needs to start one service.
// The caller (CLI) builds these from validated config.
type ServiceSpec struct {
	Name    string
	Command string
	Args    []string
	Dir     string
	Stdout  process.WriteCloserLike
	Stderr  process.WriteCloserLike
}

// StatusEvent reports a single service's state change, for the caller to
// log or display.
type StatusEvent struct {
	Name  string
	State State
	Err   error // set when State is Failed
}

// Supervisor starts services from a dependency.Plan using a process.Runner,
// and stops everything it started, in reverse order, on failure or
// cancellation.
type Supervisor struct {
	runner process.Runner
	specs  map[string]ServiceSpec

	mu      sync.Mutex
	state   map[string]State
	handles map[string]process.Handle // only services that were started
}

// New builds a Supervisor. specs must contain an entry for every name in
// plan.Startup; New does not validate the plan itself — that is the
// dependency package's job.
func New(runner process.Runner, specs map[string]ServiceSpec) *Supervisor {
	state := make(map[string]State, len(specs))
	for name := range specs {
		state[name] = Pending
	}
	return &Supervisor{
		runner:  runner,
		specs:   specs,
		state:   state,
		handles: make(map[string]process.Handle),
	}
}

// Status returns a snapshot of the current state of every known service.
func (s *Supervisor) Status() map[string]State {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]State, len(s.state))
	for name, st := range s.state {
		out[name] = st
	}
	return out
}

func (s *Supervisor) setState(name string, st State) {
	s.mu.Lock()
	s.state[name] = st
	s.mu.Unlock()
}

// exitReport is sent internally when a running service's process exits on
// its own, whether cleanly or with an error.
type exitReport struct {
	name   string
	result process.ExitResult
	err    error
}

// Run starts services from plan.Startup in order, then blocks until one of:
//   - ctx is cancelled (e.g. Ctrl+C), or
//   - a service fails to start, or
//   - a running service exits on its own (crashes or otherwise stops
//     without Stop being called).
//
// In every case, Run stops every service it started, in reverse startup
// order, before returning.
//
// events, if non-nil, receives a StatusEvent for every state transition.
// Callers should read from events concurrently so Run does not block.
func (s *Supervisor) Run(ctx context.Context, plan dependency.Plan, events chan<- StatusEvent) error {
	emit := func(name string, st State, err error) {
		s.setState(name, st)
		if events != nil {
			events <- StatusEvent{Name: name, State: st, Err: err}
		}
	}

	var startedOrder []string
	var startErr error

startLoop:
	for _, name := range plan.Startup {
		select {
		case <-ctx.Done():
			startErr = ctx.Err()
			break startLoop
		default:
		}

		spec, ok := s.specs[name]
		if !ok {
			startErr = fmt.Errorf("supervisor: no spec provided for service %q", name)
			emit(name, Failed, startErr)
			break startLoop
		}

		emit(name, Starting, nil)

		handle, err := s.runner.Start(ctx, process.Spec{
			Name:    spec.Name,
			Command: spec.Command,
			Args:    spec.Args,
			Dir:     spec.Dir,
			Stdout:  spec.Stdout,
			Stderr:  spec.Stderr,
		})
		if err != nil {
			startErr = fmt.Errorf("start service %q: %w", name, err)
			emit(name, Failed, startErr)
			break startLoop
		}

		s.mu.Lock()
		s.handles[name] = handle
		s.mu.Unlock()

		startedOrder = append(startedOrder, name)
		emit(name, Running, nil)
	}

	if startErr != nil {
		s.stopStarted(startedOrder, emit)
		return startErr
	}

	// All services started successfully. Now watch every one of them
	// concurrently: wait for the first of (a) context cancellation or
	// (b) any service exiting on its own.
	exitCh := make(chan exitReport, len(startedOrder))
	for _, name := range startedOrder {
		s.mu.Lock()
		handle := s.handles[name]
		s.mu.Unlock()

		go func(name string, handle process.Handle) {
			result, err := handle.Wait()
			exitCh <- exitReport{name: name, result: result, err: err}
		}(name, handle)
	}

	var runErr error
	alreadyExited := ""
	select {
	case <-ctx.Done():
		runErr = ctx.Err()
	case report := <-exitCh:
		alreadyExited = report.name
		if report.result.Stopped {
			runErr = fmt.Errorf("service %q stopped", report.name)
		} else if report.err != nil {
			runErr = fmt.Errorf("service %q: %w", report.name, report.err)
			emit(report.name, Failed, runErr)
		} else {
			runErr = fmt.Errorf("service %q exited unexpectedly with code %d", report.name, report.result.Code)
			emit(report.name, Failed, runErr)
		}
	}

	remaining := make([]string, 0, len(startedOrder))
	for _, name := range startedOrder {
		if name != alreadyExited {
			remaining = append(remaining, name)
		}
	}
		s.stopStarted(remaining, emit)
	return runErr
}

// Stop stops every currently-tracked, started service in reverse startup
func (s *Supervisor) Stop() {
	s.mu.Lock()
	order := make([]string, 0, len(s.handles))
	for name := range s.handles {
		order = append(order, name)
	}
	s.mu.Unlock()

	s.stopStarted(order, func(name string, st State, err error) {
		s.setState(name, st)
	})
}

func (s *Supervisor) stopStarted(startedOrder []string, emit func(name string, st State, err error)) {
	for i := len(startedOrder) - 1; i >= 0; i-- {
		name := startedOrder[i]

		s.mu.Lock()
		handle, ok := s.handles[name]
		s.mu.Unlock()
		if !ok {
			continue
		}

		if err := handle.Stop(); err != nil {
			emit(name, Failed, fmt.Errorf("stop service %q: %w", name, err))
			continue
		}
		emit(name, Stopped, nil)
	}
}