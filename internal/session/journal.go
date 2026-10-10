package session

import (
	"log"

	"mssh/internal/store"
)

// Journal is where the manager reports sessions opening and closing.
//
// The manager knows ids, not names: which machine a connection is, and what
// it was called that day, is the journal's business. *store.Store implements
// this with the same two methods it already has, and copies the rest in.
type Journal interface {
	RecordSessionOpened(sessionID string, connectionID string) error
	RecordSessionClosed(sessionID string, reason string, message string) error
}

// recordOpened and recordClosed are best effort. A log that cannot be written
// must not take down a session that works; it is reported, and the session
// carries on. A nil journal records nothing, which is what tests want.
func (manager *Manager) recordOpened(sessionID string, connectionID string) {
	if manager.journal == nil {
		return
	}
	if err := manager.journal.RecordSessionOpened(sessionID, connectionID); err != nil {
		log.Printf("mssh: session log: %v", err)
	}
}

func (manager *Manager) recordClosed(sessionID string, reason string, message string) {
	if manager.journal == nil {
		return
	}
	if err := manager.journal.RecordSessionClosed(sessionID, reason, message); err != nil {
		log.Printf("mssh: session log: %v", err)
	}
}

// recordExit turns how a transport ended into a reason. nil is the remote side
// finishing normally — `exit`, or the program completing; anything else is
// a failure, kept with its message.
func (manager *Manager) recordExit(sessionID string, exitErr error) {
	if exitErr == nil {
		manager.recordClosed(sessionID, store.SessionEnded, "")
		return
	}
	manager.recordClosed(sessionID, store.SessionFailed, exitErr.Error())
}
