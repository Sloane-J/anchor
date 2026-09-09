//go:build windows

package process

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

// windowsRunner starts each process inside its own Windows Job Object with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, so Stop terminates the whole process
// tree the command spawns, not only the directly-created process.
type windowsRunner struct{}

// NewRunner returns the Windows Runner implementation.
func NewRunner() Runner {
	return windowsRunner{}
}

func (windowsRunner) Start(ctx context.Context, spec Spec) (Handle, error) {
	if spec.Command == "" {
		return nil, fmt.Errorf("process %q: command is empty", spec.Name)
	}

	cmd := exec.Command(spec.Command, spec.Args...)
	cmd.Dir = spec.Dir
	if spec.Stdout != nil {
		cmd.Stdout = spec.Stdout
	}
	if spec.Stderr != nil {
		cmd.Stderr = spec.Stderr
	}

	job, err := createKillOnCloseJob()
	if err != nil {
		return nil, fmt.Errorf("process %q: create job object: %w", spec.Name, err)
	}

	if err := cmd.Start(); err != nil {
		syscall.CloseHandle(job)
		return nil, fmt.Errorf("process %q: start %q: %w", spec.Name, spec.Command, err)
	}

	if err := assignToJob(job, cmd.Process.Pid); err != nil {
		syscall.CloseHandle(job)
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("process %q: assign to job object: %w", spec.Name, err)
	}

	return &windowsHandle{
		name: spec.Name,
		cmd:  cmd,
		job:  job,
	}, nil
}

type windowsHandle struct {
	name string
	cmd  *exec.Cmd
	job  syscall.Handle

	mu      sync.Mutex
	stopped bool
	waited  bool
	result  ExitResult
	waitErr error
}

func (h *windowsHandle) Name() string { return h.name }

func (h *windowsHandle) Wait() (ExitResult, error) {
	h.mu.Lock()
	if h.waited {
		defer h.mu.Unlock()
		return h.result, h.waitErr
	}
	h.mu.Unlock()

	err := h.cmd.Wait()

	h.mu.Lock()
	defer h.mu.Unlock()
	h.waited = true

	if h.stopped {
		h.result = ExitResult{Stopped: true}
		h.waitErr = nil
		syscall.CloseHandle(h.job)
		return h.result, nil
	}

	syscall.CloseHandle(h.job)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			h.result = ExitResult{Code: exitErr.ExitCode()}
			h.waitErr = nil
			return h.result, nil
		}
		h.waitErr = fmt.Errorf("process %q: wait: %w", h.name, err)
		return ExitResult{}, h.waitErr
	}

	h.result = ExitResult{Code: 0}
	return h.result, nil
}

func (h *windowsHandle) Stop() error {
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return nil
	}
	h.stopped = true
	h.mu.Unlock()

	ret, _, callErr := procTerminateJobObject.Call(uintptr(h.job), 1)
	if ret == 0 {
		return fmt.Errorf("process %q: terminate job object: %w", h.name, callErr)
	}
	return nil
}

// --- Win32 Job Object bindings ---

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW      = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObj  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObj = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject    = kernel32.NewProc("TerminateJobObject")
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000
	processAllAccess                  = 0x1F0FFF
)

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformationT struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

func createKillOnCloseJob() (syscall.Handle, error) {
	h, _, err := procCreateJobObjectW.Call(0, 0)
	if h == 0 {
		return 0, fmt.Errorf("CreateJobObjectW: %w", err)
	}
	job := syscall.Handle(h)

	info := jobObjectExtendedLimitInformationT{
		BasicLimitInformation: jobObjectBasicLimitInformation{
			LimitFlags: jobObjectLimitKillOnJobClose,
		},
	}
	ret, _, err := procSetInformationJobObj.Call(
		uintptr(job),
		uintptr(jobObjectExtendedLimitInformation),
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	if ret == 0 {
		syscall.CloseHandle(job)
		return 0, fmt.Errorf("SetInformationJobObject: %w", err)
	}
	return job, nil
}

func assignToJob(job syscall.Handle, pid int) error {
	handle, err := syscall.OpenProcess(processAllAccess, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("OpenProcess: %w", err)
	}
	defer syscall.CloseHandle(handle)

	ret, _, err := procAssignProcessToJobObj.Call(uintptr(job), uintptr(handle))
	if ret == 0 {
		return fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	return nil
}