package store

import (
	"path/filepath"
	"testing"
	"time"
)

// openTest opens a throwaway database inside the test's temp dir.
func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestDefaultPath(t *testing.T) {
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("DefaultPath() = %q, want an absolute path", got)
	}
	if base := filepath.Base(got); base != "mssh.db" {
		t.Errorf("filename = %q, want mssh.db", base)
	}
	if dir := filepath.Base(filepath.Dir(got)); dir != "mssh" {
		t.Errorf("parent dir = %q, want mssh", dir)
	}
	t.Logf("database path on this machine: %s", got)
}

func TestOpenCreatesSchema(t *testing.T) {
	s := openTest(t)

	for _, table := range []string{"workspaces", "connections", "settings"} {
		var name string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}

	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != schemaVersion {
		t.Errorf("user_version = %d, want %d", version, schemaVersion)
	}
}

// TestForeignKeysEnabled guards the trap that SQLite disables foreign keys by
// default, per connection. database/sql hands out pooled connections, so we
// check several in a row rather than trusting the first one.
func TestForeignKeysEnabled(t *testing.T) {
	s := openTest(t)

	for i := 0; i < 5; i++ {
		var on int
		if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&on); err != nil {
			t.Fatalf("read pragma: %v", err)
		}
		if on != 1 {
			t.Fatalf("foreign_keys = %d on connection %d, want 1", on, i)
		}
	}
}

func TestDeleteWorkspaceCascades(t *testing.T) {
	s := openTest(t)
	now := time.Now().Unix()

	if _, err := s.db.Exec(
		`INSERT INTO workspaces (id, name, color, aws_profile, aws_region, sort_order, created_at, updated_at)
		 VALUES ('w1', 'Test', 'slate', '', '', 0, ?, ?)`, now, now,
	); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	for _, c := range []struct{ id, kind, target string }{
		{"c1", "ssm", "i-0abc12345"},
		{"c2", "ssh", "10.0.0.9"},
	} {
		if _, err := s.db.Exec(
			`INSERT INTO connections (id, workspace_id, name, kind, target, created_at, updated_at)
			 VALUES (?, 'w1', ?, ?, ?, ?, ?)`, c.id, c.id, c.kind, c.target, now, now,
		); err != nil {
			t.Fatalf("insert connection %s: %v", c.id, err)
		}
	}

	if _, err := s.db.Exec(`DELETE FROM workspaces WHERE id = 'w1'`); err != nil {
		t.Fatalf("delete workspace: %v", err)
	}

	var orphans int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM connections`).Scan(&orphans); err != nil {
		t.Fatalf("count connections: %v", err)
	}
	if orphans != 0 {
		t.Errorf("%d orphaned connections left behind, want 0", orphans)
	}
}

func TestKindCheckConstraint(t *testing.T) {
	s := openTest(t)
	now := time.Now().Unix()

	if _, err := s.db.Exec(
		`INSERT INTO workspaces (id, name, color, aws_profile, aws_region, sort_order, created_at, updated_at)
		 VALUES ('w1', 'Test', 'slate', '', '', 0, ?, ?)`, now, now,
	); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	_, err := s.db.Exec(
		`INSERT INTO connections (id, workspace_id, name, kind, target, created_at, updated_at)
		 VALUES ('c1', 'w1', 'Typo', 'shh', '10.0.0.1', ?, ?)`, now, now,
	)
	if err == nil {
		t.Fatal("inserted kind 'shh', want CHECK constraint failure")
	}
}

// TestOpenIsIdempotent reopens an existing database: migrations must not run
// twice and the seed must not duplicate.
func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer second.Close()

	var count int
	if err := second.db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&count); err != nil {
		t.Fatalf("count workspaces: %v", err)
	}
	if count != 1 {
		t.Errorf("workspace count = %d after reopen, want 1", count)
	}
}
