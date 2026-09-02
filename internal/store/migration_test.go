package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// writeV1Database builds a database in the pre-password shape by running only
// the first migration — the same file that shipped when v1 was current.
func writeV1Database(t *testing.T, path string) {
	t.Helper()

	body, err := migrationsFS.ReadFile("migrations/001_initial.sql")
	if err != nil {
		t.Fatalf("read migration 001: %v", err)
	}

	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(string(body)); err != nil {
		t.Fatalf("apply migration 001: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO workspaces (id, name, color, aws_profile, aws_region, sort_order, created_at, updated_at)
		 VALUES ('w1', 'Prod', 'violet', 'prod', 'ap-southeast-1', 0, 100, 100)`,
	); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO connections (id, workspace_id, name, kind, target, port, username,
		                          auth_method, key_path, aws_profile, aws_region, extra,
		                          color, sort_order, created_at, updated_at)
		 VALUES ('c1', 'w1', 'Taptanh', 'ssh', '115.73.222.79', 22, 'thinhvu',
		         'agent', '', '', '', '{}', '', 0, 100, 100)`,
	); err != nil {
		t.Fatalf("insert connection: %v", err)
	}
}

func TestLoadMigrations(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(migrations) < 2 {
		t.Fatalf("found %d migrations, want at least 2", len(migrations))
	}
	for i, m := range migrations {
		if m.version != i+1 {
			t.Errorf("migration %s has version %d at position %d", m.name, m.version, i+1)
		}
		if m.sql == "" {
			t.Errorf("migration %s is empty", m.name)
		}
	}
}

func TestMigrateV1ToV2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	writeV1Database(t, path)

	// Opening with the current build must upgrade the file in place.
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open on a v1 database: %v", err)
	}
	defer s.Close()

	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 2 {
		t.Errorf("user_version = %d, want 2", version)
	}

	conn, err := s.GetConnection("c1")
	if err != nil {
		t.Fatalf("connection did not survive the migration: %v", err)
	}
	if conn.Name != "Taptanh" || conn.Username != "thinhvu" || conn.Target != "115.73.222.79" {
		t.Errorf("row changed during migration: %+v", conn)
	}

	// The point of v2.
	if _, err := s.db.Exec(`UPDATE connections SET auth_method = 'password' WHERE id = 'c1'`); err != nil {
		t.Errorf("v2 should accept auth_method 'password': %v", err)
	}
	if _, err := s.db.Exec(`UPDATE connections SET auth_method = 'bogus' WHERE id = 'c1'`); err == nil {
		t.Error("CHECK constraint did not survive the rebuild")
	}

	// DROP TABLE takes the index with it; the migration has to put it back.
	var name string
	if err := s.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_connections_workspace'`,
	).Scan(&name); err != nil {
		t.Errorf("index was not recreated: %v", err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	writeV1Database(t, path)

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

	if _, err := second.GetConnection("c1"); err != nil {
		t.Errorf("connection lost on the second open: %v", err)
	}
}
