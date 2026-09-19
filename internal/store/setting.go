package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Settings are a flat key-value table rather than one column per setting.
//
// The trade is deliberate and the opposite of the one made for connections:
// adding a setting here costs nothing — no migration, no schema change — but
// the database cannot check anything about it. That is acceptable because no
// setting is ever queried, joined on, or constrained against another row. A
// connection's fields are all three, which is why they are real columns.
//
// Meaning lives in the frontend, which owns the defaults and the types.

// GetSetting returns one stored setting.
//
// A key that was never written returns ErrNotFound rather than an empty
// string, so a caller can tell "set to nothing" from "never set". Callers that
// only want a default should use GetAllSettings and fall back locally.

func (s *Store) GetSetting(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("setting key must not be empty")
	}

	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)

	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("settings %s: %w", key, ErrNotFound)
	}

	if err != nil {
		return "", fmt.Errorf("get setting %s: %w", key, err)
	}

	return value, nil
}

// GetAllSettings returns every stored setting in one call
// This is what the frontend uses at startup: one round trip, and a key that is
// absent simply means the built-in default applies.
func (s *Store) GetAllSettings() (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("query settings: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("Scan settings: %w", err)
		}
		out[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}

	return out, nil
}

// SetSetting writes a setting, replacing any previous value.
func (s *Store) SetSetting(key string, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("Setting key must not be empty")
	}

	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)

	if err != nil {
		return fmt.Errorf("Set setting %s: %w", key, err)
	}
	return nil
}

// DeleteSettings remove a setting
func (s *Store) DeleteSetting(key string) error {
	if _, err := s.db.Exec(`DELETE FROM settings WHERE key = ?`, strings.TrimSpace(key)); err != nil {
		return fmt.Errorf("Delete settings %s: %w", key, err)
	}

	return nil
}
