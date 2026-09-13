//go:build windows

package process

import "syscall"

// IsAlive reports whether a process with the given PID currently exists
// and is running. On Windows, OpenProcess succeeding is a reliable way to
// check this; os.FindProcess alone always succeeds regardless of whether
// the PID is alive, so it cannot be used for this check.
func IsAlive(pid int) bool {
	const processQueryLimitedInformation = 0x1000
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}

	const stillActive = 259
	return exitCode == stillActive
}