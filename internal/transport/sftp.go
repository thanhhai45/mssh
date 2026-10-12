package transport

import (
	"errors"
	"fmt"
	"sync"

	"github.com/pkg/sftp"
)

// errSessionClosed is what SFTP answers once the session has been closed.
var errSessionClosed = errors.New("the session is closed")

// sharedSFTP is one session's file client: one SFTP channel, opened the first
// time something needs it and shared by every listing and transfer after that.
// The zero value is ready to use.
type sharedSFTP struct {
	mutex  sync.Mutex
	client *sftp.Client
	// closed is set by close, so a client that finishes opening after it is
	// closed at once instead of outliving the connection it runs on.
	closed bool
}

// get returns the client, calling open to make it on the first call.
func (files *sharedSFTP) get(open func() (*sftp.Client, error)) (*sftp.Client, error) {
	files.mutex.Lock()
	closed, existing := files.closed, files.client
	files.mutex.Unlock()
	if closed {
		return nil, errSessionClosed
	}
	if existing != nil {
		return existing, nil
	}

	// Opened without the lock: it waits on the network, and close must never
	// queue behind it - on a dead connection, closing is what makes it return.
	opened, err := open()
	if err != nil {
		return nil, fmt.Errorf("open sftp: %w", err)
	}

	files.mutex.Lock()
	defer files.mutex.Unlock()
	switch {
	case files.closed:
		_ = opened.Close()
		return nil, errSessionClosed
	case files.client != nil:
		// Two first calls raced. Keep the one already handed out.
		_ = opened.Close()
		return files.client, nil
	default:
		files.client = opened
		return opened, nil
	}
}

// close closes the client, if one was opened, and refuses to open another.
// Closing twice is harmless.
func (files *sharedSFTP) close() {
	files.mutex.Lock()
	files.closed = true
	client := files.client
	files.client = nil
	files.mutex.Unlock()

	if client != nil {
		_ = client.Close()
	}
}
