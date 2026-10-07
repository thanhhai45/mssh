package store

import (
	"database/sql"
	"errors"
	"fmt"
	"mssh/internal/secrets"
	"strings"
)

// Names a secret is stored under. Constants rather than string literals at
// each call site: a typo would store a secret that nothing ever reads back,
// with no error anywhere - the 'keydow' bug, in the database this time.
const (
	SecretConnectionPassword = "password"
	SecretAWSSecretAccessKey = "aws_secret_access_key"
	SecretAWSSessionToken    = "aws_session_token"
)

// secretTable names one of the two secret tables and the column saying whose
// secret each row is.
type secretTable struct {
	name        string
	ownerColumn string
}

// These are the only two values secretTable ever takes. The helper below
// paste them into SQL, which is safe precisely because they are constants and
// never come from input.
var (
	connectionSecrets = secretTable{name: "connection_secrets", ownerColumn: "connection_id"}
	workspaceSecrets  = secretTable{name: "workspace_secrets", ownerColumn: "workspace_id"}
)

func (s *Store) getSecret(table secretTable, ownerID string, name string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT value FROM `+table.name+
			` WHERE `+table.ownerColumn+` = ? AND name = ?`,
		ownerID, name,
	).Scan(&value)

	if errors.Is(err, sql.ErrNoRows) {
		// The owner and the name, never the value: an error is the most
		// likely thing in this program to end up in a log.
		return "", fmt.Errorf("%s %q of %s: %w", table.name, name, ownerID, ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("read secret: %w", err)
	}
	return value, nil
}

func (s *Store) setSecret(table secretTable, ownerID string, name string, value string) error {
	if strings.TrimSpace(ownerID) == "" {
		return fmt.Errorf("owner id must not be empty")
	}
	if value == "" {
		// Storing "" would make Has say yes to a secret that is not there.
		// To remove a secret, call deleteSecret.
		return fmt.Errorf("refusing to store an empty secret")
	}

	_, err := s.db.Exec(
		`INSERT INTO `+table.name+` (`+table.ownerColumn+`, name, value, updated_at)
			VALUES (?, ?, ?, ?) ON CONFLICT(`+table.ownerColumn+`,name)
			DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		ownerID, name, value, s.now(),
	)
	if err != nil {
		return fmt.Errorf("write secret: %w", err)
	}
	return nil
}

// deleteSecret is idempotent: removing something already absent is not an
// error, because the caller wanted it gone and it is.
func (s *Store) deleteSecret(table secretTable, ownerID string, name string) error {
	if _, err := s.db.Exec(
		`DELETE FROM `+table.name+` WHERE `+table.ownerColumn+` = ? AND name = ?`,
		ownerID, name,
	); err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	return nil
}

func (s *Store) hasSecret(table secretTable, ownerID string, name string) bool {
	var exists bool
	err := s.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM `+table.name+
			` WHERE `+table.ownerColumn+` = ? AND name = ?)`,
		ownerID, name,
	).Scan(&exists)
	return err == nil && exists
}

/* ---------------- workspace secrets ---------------- */

func (s *Store) GetWorkspaceSecret(workspaceID, name string) (string, error) {
	return s.getSecret(workspaceSecrets, workspaceID, name)
}

func (s *Store) SetWorkspaceSecret(workspaceID, name, value string) error {
	return s.setSecret(workspaceSecrets, workspaceID, name, value)
}

func (s *Store) DeleteWorkspaceSecret(workspaceID, name string) error {
	return s.deleteSecret(workspaceSecrets, workspaceID, name)
}

func (s *Store) HasWorkspaceSecret(workspaceID, name string) bool {
	return s.hasSecret(workspaceSecrets, workspaceID, name)
}

/*-------------------connection passwords, as a secrets.Vault------------------------*/

// ConnectionPasswords present connection_secrets through the secrets.Vault
// interface, so app.go goes on calling Get, Set, Delete and Has exactly as it
// did when the passwords lived in the keychain. Only main.go knows the backend changed
type ConnectionPasswords struct {
	store *Store
}

// Compile-time check, reported here rather than at the call site in main.go.
var _ secrets.Vault = ConnectionPasswords{}

func (s *Store) ConnectionPasswords() ConnectionPasswords {
	return ConnectionPasswords{store: s}
}

func (passwords ConnectionPasswords) Get(connectionID string) (string, error) {
	value, err := passwords.store.getSecret(connectionSecrets, connectionID, SecretConnectionPassword)

	if errors.Is(err, ErrNotFound) {
		// The vault contract promises secrets.ErrNotFound. store.ErrNotFound is
		// a different value, and errors.Is never matches one to the other.
		return "", fmt.Errorf("connection %s: %w", connectionID, secrets.ErrNotFound)
	}
	return value, err
}

func (passwords ConnectionPasswords) Set(connectionID, password string) error {
	return passwords.store.setSecret(
		connectionSecrets, connectionID, SecretConnectionPassword, password)
}

func (passwords ConnectionPasswords) Delete(connectionID string) error {
	return passwords.store.deleteSecret(
		connectionSecrets, connectionID, SecretConnectionPassword)
}

func (passwords ConnectionPasswords) Has(connectionID string) bool {
	return passwords.store.hasSecret(
		connectionSecrets, connectionID, SecretConnectionPassword)
}
