package secrets

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const service = "mssh"

var ErrNotFound = errors.New("secret not found")

type Vault interface {
	Get(connectionID string) (string, error)
	Set(connectionID, secret string) error
	Delete(connectionID string) error
	Has(connectionID string) bool
}

type Keyring struct{}

func NewKeyring() *Keyring { return &Keyring{} }

func (k *Keyring) Get(connectionID string) (string, error) {
	secret, err := keyring.Get(service, connectionID)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("connection %s: %w", connectionID, ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("read from keychain: %w", err)
	}
	return secret, nil
}

func (k *Keyring) Set(connectionID, secret string) error {
	if strings.TrimSpace(connectionID) == "" {
		return fmt.Errorf("connection id must not be empty")
	}
	if secret == "" {
		return fmt.Errorf("refusing to store an empty secret")
	}
	if err := keyring.Set(service, connectionID, secret); err != nil {
		return fmt.Errorf("write to keychain: %w", err)
	}
	return nil
}

func (k *Keyring) Delete(connectionID string) error {
	err := keyring.Delete(service, connectionID)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete from keychain: %w", err)
	}

	return nil
}

func (k *Keyring) Has(connectionID string) bool {
	_, err := k.Get(connectionID)
	return err == nil
}
