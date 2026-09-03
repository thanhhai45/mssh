package secrets

import (
	"errors"
	"os"
	"testing"
)

func vaultBehaviour(t *testing.T, v Vault) {
	t.Helper()

	const id = "test-connection-id"
	t.Cleanup(func() { v.Delete(id) })

	if v.Has(id) {
		t.Fatalf("Has reported a secret before anything was stored")
	}

	if _, err := v.Get(id); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get on missing secret: err = %v, want ErrorNotFound", err)
	}

	if err := v.Set(id, "hunter2"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if !v.Has(id) {
		t.Error("Has reported noting after Set")
	}

	got, err := v.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "hunter2" {
		t.Errorf("Get = %q, want %q", got, "hunter2")
	}

	// Set on an existing entry overwrites rather than failing.
	if err := v.Set(id, "correct horse"); err != nil {
		t.Fatalf("Set over an existing secret: %v", err)
	}

	if got, _ := v.Get(id); got != "correct horse" {
		t.Errorf("Get after overwrite = %q, want %q", got, "correct horse")
	}

	if err := v.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if v.Has(id) {
		t.Error("Has still reported a secret after Delete")
	}

	// Deleting twice is not an error: the caller wanted it gone, and it is.
	if err := v.Delete(id); err != nil {
		t.Errorf("second Delete: %v", err)
	}
}

func TestMemoryVault(t *testing.T) {
	vaultBehaviour(t, NewMemory())
}

// TestKeyringVault touches the real OS credential store, so it only runs when
// asked for: MSSH_TEST_KEYCHAIN=1 go test ./internal/secrets/
func TestKeyringVault(t *testing.T) {
	if os.Getenv("MSSH_TEST_KEYCHAIN") != "1" {
		t.Skip("set MSSH_TEST_KEYCHAIN=1 to exercise the real keychain")
	}
	vaultBehaviour(t, NewKeyring())
}

func TestSetRejectsEmptyInput(t *testing.T) {
	v := NewMemory()

	if err := v.Set("", "secret"); err == nil {
		t.Error("empty connection id was accepted")
	}
	if err := v.Set("some-id", ""); err == nil {
		t.Error("empty secret was accepted")
	}
}
