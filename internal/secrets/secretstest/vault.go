// Package secretstest holds the behaviour every secrets.Vault must have, so
// each implementation is checked against the same promises.
//
// A package of its own, the way the standard library has testing/fstest:
// production code must not import "testing", and a helper living in a _test.go
// file cannot be reached from another package's tests.
package secretstest

import (
	"errors"
	"testing"

	"mssh/internal/secrets"
)

// Run checks a vault against the contract.
//
// ownerID must be an id the vault will accept. The keychain and the in-memory
// vault take any string; the database vault stores each secret under a foreign
// key, so it needs the id of a connection that really exists.
func Run(t *testing.T, vault secrets.Vault, ownerID string) {
	t.Helper()
	t.Cleanup(func() { _ = vault.Delete(ownerID) })

	if vault.Has(ownerID) {
		t.Fatalf("Has reported a secret before anything was stored")
	}
	if _, err := vault.Get(ownerID); !errors.Is(err, secrets.ErrNotFound) {
		t.Errorf("Get on a missing secret: err = %v, want secrets.ErrNotFound", err)
	}

	if err := vault.Set(ownerID, "hunter2"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !vault.Has(ownerID) {
		t.Error("Has reported nothing after Set")
	}
	got, err := vault.Get(ownerID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "hunter2" {
		t.Errorf("Get = %q, want %q", got, "hunter2")
	}

	// Set on an existing entry overwrites rather than failing.
	if err := vault.Set(ownerID, "correct horse"); err != nil {
		t.Fatalf("Set over an existing secret: %v", err)
	}
	if got, _ := vault.Get(ownerID); got != "correct horse" {
		t.Errorf("Get after overwrite = %q, want %q", got, "correct horse")
	}

	if err := vault.Delete(ownerID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if vault.Has(ownerID) {
		t.Error("Has still reported a secret after Delete")
	}
	// Deleting twice is not an error: the caller wanted it gone, and it is.
	if err := vault.Delete(ownerID); err != nil {
		t.Errorf("second Delete: %v", err)
	}

	// Empty input is refused by every implementation, not only the one that
	// happened to have a test for it.
	if err := vault.Set("", "secret"); err == nil {
		t.Error("an empty owner id was accepted")
	}
	if err := vault.Set(ownerID, ""); err == nil {
		t.Error("an empty secret was accepted")
	}
}
