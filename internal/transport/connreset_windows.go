//go:build windows

package transport

import (
	"errors"
	"syscall"
)

// connectionReset reports whether err is the other end tearing the connection
// down instead of closing it. Winsock has its own numbers for this, and words
// that Windows translates into its display language - only the numbers are
// safe to compare.
func connectionReset(err error) bool {
	return errors.Is(err, syscall.WSAECONNRESET) || errors.Is(err, syscall.WSAECONNABORTED)
}
