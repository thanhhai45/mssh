package secrets

import (
	"fmt"
	"strings"
	"sync"
)

type Memory struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewMemory() *Memory {
	return &Memory{data: make(map[string]string)}
}

func (m *Memory) Get(connectionID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	secret, ok := m.data[connectionID]
	if !ok {
		return "", fmt.Errorf("Connection %s: %w", connectionID, ErrNotFound)
	}
	return secret, nil
}

func (m *Memory) Set(connectionID, secret string) error {
	if strings.TrimSpace(connectionID) == "" {
		return fmt.Errorf("Connection id must not be empty")
	}
	if secret == "" {
		return fmt.Errorf("Refusing to store an empty secret")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[connectionID] = secret
	return nil
}

func (m *Memory) Delete(connectionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, connectionID)
	return nil
}

func (m *Memory) Has(connectionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.data[connectionID]
	return ok
}
