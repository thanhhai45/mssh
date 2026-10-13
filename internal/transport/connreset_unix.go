//go:build unix

package transport

import (
	"errors"
	"syscall"
)

// connectionReset reports whether err is the other end tearing the connection
// down instead of closing it: a reset, or a write into a connection already gone.
func connectionReset(err error) bool {
	return errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}
