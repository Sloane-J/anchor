package process

import (
	"context"
	"fmt"
	"sync"
)

// Fake is an in-memory Runner for tests. It never starts a real process.
type Fake struct {
	mu       sync.Mutex
	started  []string // service names, in Start call order
	stopped  []string // service names, in Stop call order
	handles  map[string]*fakeHandle
	startErr map[string]error // optional per-service Start failure
}

// NewFake returns an empty Fake runner.
func NewFake() *Fake {
	return &Fake{
		handles:  make(map[string]*fakeHandle),
		startErr: make(map[string]error),
	}
}

// FailStart makes a future Start call for name return err instead of
// succeeding.
func (f *Fake) FailStart(name string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startErr[name] = err
}

// Started returns service names in the order Start was called, for
// assertions.
func (f *Fake) Started() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.started...)
}

// Stopped returns service names in the order Stop was called, for
// assertions.
func (f *Fake) Stopped() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.stopped...)
}

// Exit makes the handle for name finish Wait with result, unblocking any
// caller currently waiting on it. Exit must be called at most once per
// handle.
func (f *Fake) Exit(name string, result ExitResult) {
	f.mu.Lock()
	h, ok := f.handles[name]
	f.mu.Unlock()
	if !ok {
		return
	}
	h.exit(result, nil)
}

func (f *Fake) Start(ctx context.Context, spec Spec) (Handle, error) {
	f.mu.Lock()
	if err, ok := f.startErr[spec.Name]; ok {
		f.mu.Unlock()
		return nil, err
	}
	f.started = append(f.started, spec.Name)
	h := &fakeHandle{
		name: spec.Name,
		done: make(chan struct{}),
	}
	h.onStop = func(name string) {
		f.mu.Lock()
		f.stopped = append(f.stopped, name)
		f.mu.Unlock()
	}
	f.handles[spec.Name] = h
	f.mu.Unlock()

	return h, nil
}

// stopRecorder lets fakeHandle report Stop calls back to the owning Fake
// without a circular import problem; Fake sets this on each handle it hands
// out.
type fakeHandle struct {
	name string

	mu     sync.Mutex
	done   chan struct{}
	result ExitResult
	err    error
	fired  bool

	onStop func(name string)
}

func (h *fakeHandle) Name() string { return h.name }

func (h *fakeHandle) Wait() (ExitResult, error) {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.result, h.err
}

func (h *fakeHandle) Stop() error {
	h.mu.Lock()
	alreadyFired := h.fired
	h.mu.Unlock()

	if h.onStop != nil {
		h.onStop(h.name)
	}
	if alreadyFired {
		return nil
	}
	h.exit(ExitResult{Stopped: true}, nil)
	return nil
}

func (h *fakeHandle) exit(result ExitResult, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.fired {
		return
	}
	h.fired = true
	h.result = result
	h.err = err
	close(h.done)
}

var _ Runner = (*Fake)(nil)
var _ = fmt.Sprintf // placeholder to keep fmt import if unused later