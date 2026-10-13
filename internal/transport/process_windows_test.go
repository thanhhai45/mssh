//go:build windows

package transport

import "golang.org/x/sys/windows"

// stillActive is what GetExitCodeProcess reports for a process that has not
// exited. x/sys/windows has no name for it.
const stillActive = 259

// processRunning reports whether the process with this pid has not exited yet.
func processRunning(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code == stillActive
}
