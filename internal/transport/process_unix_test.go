//go:build unix

package transport

import "syscall"

// processRunning reports whether a process with this pid still exists.
// Signal 0 delivers nothing: the kernel only checks that the process is there.
func processRunning(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
