package session

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/pkg/sftp"

	"mssh/internal/transport"
)

/* ---------------- a dialer that needs no network ---------------- */
type fakeSession struct {
	mutex   sync.Mutex
	closed  bool
	written []byte
	// onClose, when set, runs as Close finishes — the way a real transport
	// reports its own ending once it has been closed.
	onClose func()
}

func (s *fakeSession) Write(payload []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.written = append(s.written, payload...)
	return len(payload), nil
}

func (s *fakeSession) Resize(size transport.Size) error {
	return nil
}

func (s *fakeSession) Close() error {
	s.mutex.Lock()
	s.closed = true
	onClose := s.onClose
	s.onClose = nil
	s.mutex.Unlock()

	if onClose != nil {
		onClose()
	}
	return nil
}

func (s *fakeSession) isClosed() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.closed
}

type fakeDialer struct {
	preflightErr   error
	dialErr        error
	exitDuringDial bool
	opened         []*fakeSession
	// exits holds each session's onExit, so a test can end one from the
	// remote side, the way a dropped network or a typed `exit` would.
	exits []func(error)
	// exitOnClose makes each session report an ending of its own when closed.
	exitOnClose bool
}

func (d *fakeDialer) Name() string { return "fake" }

func (d *fakeDialer) Preflight(transport.Config) error {
	return d.preflightErr
}

func (d *fakeDialer) Dial(
	_ context.Context,
	_ transport.Config,
	_ transport.Size,
	_ func([]byte), onExit func(error),
) (transport.Session, error) {
	if d.dialErr != nil {
		return nil, d.dialErr
	}

	session := &fakeSession{}
	d.opened = append(d.opened, session)
	d.exits = append(d.exits, onExit)
	if d.exitOnClose {
		session.onClose = func() { onExit(errors.New("use of closed network connection")) }
	}

	if d.exitDuringDial {
		onExit(nil)
	}

	return session, nil
}

func newTestManager(dialer transport.Dialer) *Manager {
	manager := NewManager(nil)
	manager.dialerFor = func(string) (transport.Dialer, error) { return dialer, nil }
	return manager
}

func open(t *testing.T, manager *Manager, sessionID string, connectID string) {
	t.Helper()
	err := manager.Open(context.Background(), sessionID, connectID,
		transport.Config{}, transport.Size{Cols: 80, Rows: 24},
		func([]byte) {}, func(error) {})
	if err != nil {
		t.Fatalf("Open(%s): %v", sessionID, err)
	}
}

/* ---------------- the tests ---------------- */

// The whole point of the change: one machine, two terminals.
func TestTwoSessionsToTheSameConnection(t *testing.T) {
	manager := newTestManager(&fakeDialer{})
	open(t, manager, "session1", "connect1")
	open(t, manager, "session2", "connect1")

	if !manager.IsOpen("session1") || !manager.IsOpen("session2") {
		t.Fatal("both sessions should be open")
	}

	// Closing one must leave the other alone
	if err := manager.Close("session1"); err != nil {
		t.Fatalf("Close(session1): %v", err)
	}
	if manager.IsOpen("session1") {
		t.Errorf("session1 is still open after Close")
	}
	if !manager.IsOpen("session2") {
		t.Errorf("session2 is closed after closing session1")
	}
}

func TestWriteGoesToTheRightSession(t *testing.T) {
	dialer := &fakeDialer{}
	manager := newTestManager(dialer)

	open(t, manager, "s1", "conn-a")
	open(t, manager, "s2", "conn-a")

	if err := manager.Write("s2", []byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if string(dialer.opened[0].written) != "" {
		t.Error("the first session received input meant for the second")
	}
	if string(dialer.opened[1].written) != "hello" {
		t.Errorf("second session got %q, want hello", dialer.opened[1].written)
	}
}

// The bug this restructure fixes. A session that ends before Dial returns used
// to be stored anyway, leaving a dead entry nothing would ever remove.
func TestASessionThatEndsDuringDialIsNotRegistered(t *testing.T) {
	dialer := &fakeDialer{exitDuringDial: true}
	manager := newTestManager(dialer)

	open(t, manager, "s1", "conn-a")

	if manager.IsOpen("s1") {
		t.Error("a session that already ended is still registered")
	}
	if got := len(manager.OpenSessions()); got != 0 {
		t.Errorf("OpenSessions returned %d, want 0", got)
	}
	if !dialer.opened[0].isClosed() {
		t.Error("the abandoned session was left unclosed")
	}
}

func TestFailuresLeaveNothingBehind(t *testing.T) {
	t.Run("preflight", func(t *testing.T) {
		manager := newTestManager(&fakeDialer{preflightErr: errors.New("no")})

		err := manager.Open(context.Background(), "s1", "conn-a",
			transport.Config{}, transport.Size{}, func([]byte) {}, func(error) {})
		if err == nil {
			t.Fatal("expected an error")
		}
		if len(manager.OpenSessions()) != 0 {
			t.Error("a failed preflight left a slot behind")
		}
	})

	t.Run("dial", func(t *testing.T) {
		manager := newTestManager(&fakeDialer{dialErr: errors.New("no")})

		err := manager.Open(context.Background(), "s1", "conn-a",
			transport.Config{}, transport.Size{}, func([]byte) {}, func(error) {})
		if err == nil {
			t.Fatal("expected an error")
		}
		// The reserved placeholder must go, or the id can never be used again.
		if len(manager.OpenSessions()) != 0 {
			t.Error("a failed dial left a slot behind")
		}
		if err := manager.Open(context.Background(), "s1", "conn-a",
			transport.Config{}, transport.Size{}, func([]byte) {}, func(error) {},
		); err == nil {
			t.Log("retrying the same id works, as it should")
		}
	})
}

func TestCloseConnectionTakesEveryTabOfThatMachine(t *testing.T) {
	dialer := &fakeDialer{}
	manager := newTestManager(dialer)

	open(t, manager, "s1", "conn-a")
	open(t, manager, "s2", "conn-a")
	open(t, manager, "s3", "conn-b")

	manager.CloseConnection("conn-a")

	if manager.IsOpen("s1") || manager.IsOpen("s2") {
		t.Error("a session of conn-a survived")
	}
	if !manager.IsOpen("s3") {
		t.Error("conn-b was closed too")
	}
	if !dialer.opened[0].isClosed() || !dialer.opened[1].isClosed() {
		t.Error("the transports were dropped without being closed")
	}
}

func TestCloseAll(t *testing.T) {
	dialer := &fakeDialer{}
	manager := newTestManager(dialer)

	open(t, manager, "s1", "conn-a")
	open(t, manager, "s2", "conn-b")

	manager.CloseAll()

	if len(manager.OpenSessions()) != 0 {
		t.Error("sessions survived CloseAll")
	}
	for index, session := range dialer.opened {
		if !session.isClosed() {
			t.Errorf("session %d was not closed", index)
		}
	}
}

/* ---------------- files ---------------- */

// fakeBrowser is a session that can move files, the way ssh and ssm-ssh can.
type fakeBrowser struct {
	fakeSession
	calls int
	err   error
}

func (s *fakeBrowser) SFTP() (*sftp.Client, error) {
	s.calls++
	return nil, s.err
}

func TestSFTPOfASessionThatIsNotOpen(t *testing.T) {
	manager := newTestManager(&fakeDialer{})

	_, err := manager.SFTP("missing")
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("SFTP(missing) = %v, want an error naming the session", err)
	}
}

// An ssm or ssh-config session runs a program and has no files to offer.
func TestSFTPOfASessionWithoutFiles(t *testing.T) {
	manager := newTestManager(&fakeDialer{})
	open(t, manager, "session1", "connect1")

	if _, err := manager.SFTP("session1"); !errors.Is(err, transport.ErrNoFileBrowsing) {
		t.Errorf("SFTP = %v, want ErrNoFileBrowsing", err)
	}
}

// The manager asks the session each time; sharing one client is the
// session's job, not something the manager caches on top.
func TestSFTPAsksTheSession(t *testing.T) {
	manager := newTestManager(&fakeDialer{})
	open(t, manager, "session1", "connect1")
	refused := errors.New("subsystem request failed")
	browser := &fakeBrowser{err: refused}
	manager.sessions["session1"].transport = browser

	for range 2 {
		if _, err := manager.SFTP("session1"); !errors.Is(err, refused) {
			t.Errorf("SFTP = %v, want the session's own error", err)
		}
	}
	if browser.calls != 2 {
		t.Errorf("the session was asked %d times, want 2", browser.calls)
	}
}
