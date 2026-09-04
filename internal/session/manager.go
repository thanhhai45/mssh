// Package session keeps live connections alive independently of the user
// interface. A session survives navigating away from its terminal; it ends
// only when the user disconnects or the remote shell exits.
package session

import (
	"context"
	"fmt"
	"sync"

	"mssh/internal/transport"
)

type Manager struct {
	mutex    sync.RWMutex
	sessions map[string]transport.Session
}

func NewManager() *Manager {
	return &Manager{sessions: make(map[string]transport.Session)}
}

func (manager *Manager) Open(
	dialContext context.Context,
	connectionID string,
	config transport.Config,
	size transport.Size,
	onOutput func([]byte),
	onExit func(error),
) error {
	dialer, err := transport.For(config.Kind)
	if err != nil {
		return err
	}
	if err := dialer.Preflight(config); err != nil {
		return err
	}

	if err := manager.reserve(connectionID); err != nil {
		return err
	}

	session, err := dialer.Dial(dialContext, config, size, onOutput,
		func(exitErr error) {
			manager.forget(connectionID)
			onExit(exitErr)
		})
	if err != nil {
		manager.forget(connectionID)
		return err
	}

	manager.mutex.Lock()
	manager.sessions[connectionID] = session
	manager.mutex.Unlock()
	return nil
}

// reserve claims the slot before dialling, so a second Open for the same
// connection cannot start while the first is still in flight.
func (manager *Manager) reserve(connectionID string) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if _, taken := manager.sessions[connectionID]; taken {
		return fmt.Errorf("connection %s is already open", connectionID)
	}

	manager.sessions[connectionID] = nil
	return nil
}

func (manager *Manager) forget(connectionID string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	delete(manager.sessions, connectionID)
}

func (manager *Manager) lookup(connectionID string) transport.Session {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	return manager.sessions[connectionID]
}

// IsOpen reports whether a session exists and has finished dialling.
func (manager *Manager) IsOpen(connectionID string) bool {
	return manager.lookup(connectionID) != nil
}

func (manager *Manager) Write(connectionID string, payload []byte) error {
	session := manager.lookup(connectionID)
	if session == nil {
		return fmt.Errorf("connection %s is not open", connectionID)
	}

	_, err := session.Write(payload)
	return err
}

func (manager *Manager) Resize(connectionID string, size transport.Size) error {
	session := manager.lookup(connectionID)
	if session == nil {
		return fmt.Errorf("connection %s is not open", connectionID)
	}
	return session.Resize(size)
}

// Close ends one session. Closing something that is not open is not an error:
// the caller wanted it gone, and it is.
func (manager *Manager) Close(connectionID string) error {
	session := manager.lookup(connectionID)

	manager.forget(connectionID)

	if session == nil {
		return nil
	}
	return session.Close()
}

func (manager *Manager) CloseAll() {
	manager.mutex.Lock()
	open := manager.sessions
	manager.sessions = make(map[string]transport.Session)
	manager.mutex.Unlock()

	for _, session := range open {
		if session != nil {
			session.Close()
		}
	}
}

func (manager *Manager) OpenIDs() []string {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	ids := []string{}
	for connectionID, session := range manager.sessions {
		if session != nil {
			ids = append(ids, connectionID)
		}
	}
	return ids
}
