package session

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"mssh/internal/store"
	"mssh/internal/transport"
)

// fakeJournal remembers what it was told, in order, as short strings:
// "opened s1", "closed s1 closed", "closed s1 failed: <message>".
type fakeJournal struct {
	mutex  sync.Mutex
	events []string
}

func (journal *fakeJournal) RecordSessionOpened(sessionID string, _ string) error {
	journal.add("opened " + sessionID)
	return nil
}

func (journal *fakeJournal) RecordSessionClosed(sessionID string, reason string, message string) error {
	event := "closed " + sessionID + " " + reason
	if message != "" {
		event += ": " + message
	}
	journal.add(event)
	return nil
}

func (journal *fakeJournal) add(event string) {
	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	journal.events = append(journal.events, event)
}

func (journal *fakeJournal) got() string {
	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	return strings.Join(journal.events, " | ")
}

// *store.Store is the real journal. Checked here so a renamed store method
// fails this package's tests rather than only main's build.
var _ Journal = (*store.Store)(nil)

func journalled(dialer *fakeDialer) (*Manager, *fakeJournal) {
	journal := &fakeJournal{}
	manager := NewManager(journal)
	manager.dialerFor = func(string) (transport.Dialer, error) { return dialer, nil }
	return manager, journal
}

func TestJournalHearsHowEachSessionEnded(t *testing.T) {
	dialer := &fakeDialer{}
	manager, journal := journalled(dialer)

	open(t, manager, "user", "c1")
	open(t, manager, "exited", "c1")
	open(t, manager, "dropped", "c1")

	if err := manager.Close("user"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	dialer.exits[1](nil)
	dialer.exits[2](errors.New("connection reset by peer"))

	want := "opened user | opened exited | opened dropped | " +
		"closed user closed | closed exited ended | closed dropped failed: connection reset by peer"
	if got := journal.got(); got != want {
		t.Errorf("journal:\n got  %s\n want %s", got, want)
	}
}

// Closing a session makes its transport report an ending of its own a moment
// later. The user's choice goes in first, so it is the one the store keeps.
func TestUserCloseIsRecordedBeforeTheTransportReportsIt(t *testing.T) {
	manager, journal := journalled(&fakeDialer{exitOnClose: true})

	open(t, manager, "s1", "c1")
	if err := manager.Close("s1"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := journal.got()
	if !strings.HasPrefix(got, "opened s1 | closed s1 closed") {
		t.Errorf("the user's close was not first: %s", got)
	}
}

func TestDeletingAConnectionAndQuittingAreTheirOwnReasons(t *testing.T) {
	dialer := &fakeDialer{}
	manager, journal := journalled(dialer)

	open(t, manager, "a1", "a")
	open(t, manager, "b1", "b")
	manager.CloseConnection("a")
	manager.CloseAll()

	want := "opened a1 | opened b1 | closed a1 connection-deleted | closed b1 app-quit"
	if got := journal.got(); got != want {
		t.Errorf("journal:\n got  %s\n want %s", got, want)
	}
}

// A session that dies while it is still opening: its onExit runs before there
// is a row to close. Without the extra close in Open, the row would read as
// open until the next start called it interrupted.
func TestASessionThatEndsWhileOpeningIsNotLeftOpen(t *testing.T) {
	manager, journal := journalled(&fakeDialer{exitDuringDial: true})

	if err := manager.Open(context.Background(), "s1", "c1",
		transport.Config{}, transport.Size{Cols: 80, Rows: 24},
		func([]byte) {}, func(error) {}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	got := journal.got()
	if !strings.Contains(got, "opened s1") || !strings.HasSuffix(got, "closed s1 ended: ended while it was opening") {
		t.Errorf("journal: %s", got)
	}
}

// The log is of sessions, not of attempts: a dial that never connected
// leaves nothing in it.
func TestAFailedDialIsNotRecorded(t *testing.T) {
	manager, journal := journalled(&fakeDialer{dialErr: errors.New("connection refused")})

	err := manager.Open(context.Background(), "s1", "c1",
		transport.Config{}, transport.Size{Cols: 80, Rows: 24},
		func([]byte) {}, func(error) {})
	if err == nil {
		t.Fatal("Open succeeded with a failing dialer")
	}
	if got := journal.got(); got != "" {
		t.Errorf("a failed dial was recorded: %s", got)
	}
}
