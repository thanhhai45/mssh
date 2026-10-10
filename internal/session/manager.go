// Package session keeps live connections alive independently of the user
// interface. A session survives navigating away from its terminal; it ends
// only when the user disconnects or the remote shell exits.
package session

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"mssh/internal/store"
	"mssh/internal/transport"
)

// liveSession is one open terminal: the transport, plus which connection it belongs to
// The manager keys these by session id rather than connection id, because one
// machine can now have several terminals at once. connectionID rides along so
// the UI can still ask "is this server connected?" - a question that used to
// be a map lookup and is now a search
type liveSession struct {
	connectionID string
	transport    transport.Session
}

type Info struct {
	SessionID    string `json:"sessionId"`
	ConnectionID string `json:"connectionId"`
}

type Manager struct {
	mutex     sync.RWMutex
	sessions  map[string]*liveSession
	dialerFor func(kind string) (transport.Dialer, error)
	// journal hears every session open and close. Kept here rather than in
	// app.go so that every way a session can end goes through one place.
	journal Journal
}

// NewManager makes a manager that reports to journal. nil records nothing.
func NewManager(journal Journal) *Manager {
	return &Manager{
		sessions:  make(map[string]*liveSession),
		dialerFor: transport.For,
		journal:   journal,
	}
}

// Open dials and registers one session.
//
// The session id comes from the caller rather than being generated here,
// because the caller needs it before this returns: it is the channel name the
// output events are published on, and those start arriving during the dial.
func (manager *Manager) Open(
	dialContext context.Context,
	sessionID string,
	connectionID string,
	config transport.Config,
	size transport.Size,
	onOutput func([]byte),
	onExit func(error),
) error {
	dialer, err := manager.dialerFor(config.Kind)
	if err != nil {
		return err
	}
	if err := dialer.Preflight(config); err != nil {
		return err
	}

	if err := manager.reserve(sessionID, connectionID); err != nil {
		return err
	}

	session, err := dialer.Dial(dialContext, config, size, onOutput,
		func(exitErr error) {
			manager.forget(sessionID)
			manager.recordExit(sessionID, exitErr)
			onExit(exitErr)
		})
	if err != nil {
		// Never opened, so never logged: the log is of sessions, not of
		// attempts.
		manager.forget(sessionID)
		return err
	}

	// Recorded before anything can learn the session ended, so the row exists
	// by the time an ending is written to it — whichever goroutine gets there.
	manager.recordOpened(sessionID, connectionID)

	manager.mutex.Lock()
	live := manager.sessions[sessionID]
	if live == nil {
		manager.mutex.Unlock()
		// It ended while it was opening. Its onExit ran before the row above
		// existed and so wrote nothing; close the row here instead, or it
		// would read as open until the next start called it interrupted.
		manager.recordClosed(sessionID, store.SessionEnded, "ended while it was opening")
		return session.Close()
	}
	live.transport = session
	manager.mutex.Unlock()

	return nil
}

// reserve claims the slot before dialling
// It used to also reject a second session for the same connection. Multi-tab
// is precisely that, so the rejection is gone - but the race it was really
// protecting against is not: Dial can call onExit before Open has stored
// anything, and with no placeholder under the key, forget would delete
// nothing. A duplicate session id is the only thing left to refuse, and that
// would be a bug in the caller rather than something a user can do.
func (manager *Manager) reserve(sessionID string, connectionID string) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if _, taken := manager.sessions[sessionID]; taken {
		return fmt.Errorf("session %s is already open", sessionID)
	}

	manager.sessions[sessionID] = &liveSession{connectionID: connectionID}
	return nil
}

func (manager *Manager) forget(sessionID string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	delete(manager.sessions, sessionID)
}

func (manager *Manager) lookup(sessionID string) transport.Session {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	live := manager.sessions[sessionID]
	if live == nil {
		return nil
	}
	return live.transport
}

// IsOpen reports whether a session exists and has finished dialling.
func (manager *Manager) IsOpen(sessionID string) bool {
	return manager.lookup(sessionID) != nil
}

// ConnectionOf says which connection a session belongs to, or "" when there
// is no such session
//
// Callers that are about to close a session must ask first: Close forgets
// the session, and then there is nothing left to ask.
func (manager *Manager) ConnectionOf(sessionID string) string {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	live := manager.sessions[sessionID]
	if live == nil {
		return ""
	}
	return live.connectionID
}

func (manager *Manager) Write(sessionID string, payload []byte) error {
	session := manager.lookup(sessionID)
	if session == nil {
		return fmt.Errorf("session %s is not open", sessionID)
	}

	_, err := session.Write(payload)
	return err
}

func (manager *Manager) Resize(sessionID string, size transport.Size) error {
	session := manager.lookup(sessionID)
	if session == nil {
		return fmt.Errorf("session %s is not open", sessionID)
	}
	return session.Resize(size)
}

// Close ends one session. Closing something that is not open is not an error:
// the caller wanted it gone, and it is.
func (manager *Manager) Close(sessionID string) error {
	session := manager.lookup(sessionID)

	manager.forget(sessionID)

	if session == nil {
		return nil
	}
	// Before Close, not after: closing makes the transport report its own
	// ending a moment later, and the user's choice has to be the one on record.
	manager.recordClosed(sessionID, store.SessionClosedByUser, "")
	return session.Close()
}

// CloseConnection ends every session belonging to one connection.
//
// Needed because a connection can now have several. It also closes a gap that
// predates multi-tab: deleting a connection left its session running. With
// nothing in the UI able to reach it any more.
func (manager *Manager) CloseConnection(connectionID string) {
	manager.mutex.Lock()
	doomed := []transport.Session{}

	doomedIDs := []string{}

	for sessionID, live := range manager.sessions {
		if live.connectionID != connectionID {
			continue
		}
		if live.transport != nil {
			doomed = append(doomed, live.transport)
			doomedIDs = append(doomedIDs, sessionID)
		}
		delete(manager.sessions, sessionID)
	}
	manager.mutex.Unlock()

	for _, sessionID := range doomedIDs {
		manager.recordClosed(sessionID, store.SessionConnectionDeleted, "")
	}
	for _, session := range doomed {
		_ = session.Close()
	}
}

func (manager *Manager) CloseAll() {
	manager.mutex.Lock()
	open := manager.sessions
	manager.sessions = make(map[string]*liveSession)
	manager.mutex.Unlock()

	// This runs on the way out of the application. One session refusing to
	// close must not stop the others from being closed, and there is nobody
	// left to tell about it either way.
	for sessionID, live := range open {
		if live.transport != nil {
			manager.recordClosed(sessionID, store.SessionAppQuit, "")
			_ = live.transport.Close()
		}
	}
}

// OpenSessions lists what is live, so the UI can rebuild its tabs after a
// reload. Sessions still dialling are left out: there is nothing to attach to yet
// Sorted because a map iterates in a different order every time, and a tab strip that
// reshuffles on every read is unusable. The order is only stable, not meaningful - the real
// tab order belongs to the frontend, which knows which tab was opened first
func (manager *Manager) OpenSessions() []Info {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	open := []Info{}
	for sessionID, live := range manager.sessions {
		if live.transport == nil {
			continue
		}
		open = append(open, Info{SessionID: sessionID, ConnectionID: live.connectionID})
	}

	sort.Slice(open, func(first int, second int) bool {
		return open[first].SessionID < open[second].SessionID
	})
	return open
}
