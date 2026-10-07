// package secrets_test, not package secrets: this file imports secretstest,
// which imports secrets, and an in-package test doing that would be an import
// cycle. An external test package is the way out.
package secrets_test

import (
	"os"
	"testing"

	"mssh/internal/secrets"
	"mssh/internal/secrets/secretstest"
)

func TestMemoryVault(t *testing.T) {
	secretstest.Run(t, secrets.NewMemory(), "test-connection-id")
}

// TestKeyringVault touches the real OS credential store, so it only runs when
// asked for: MSSH_TEST_KEYCHAIN=1 go test ./internal/secrets/
func TestKeyringVault(t *testing.T) {
	if os.Getenv("MSSH_TEST_KEYCHAIN") != "1" {
		t.Skip("set MSSH_TEST_KEYCHAIN=1 to exercise the real keychain")
	}
	secretstest.Run(t, secrets.NewKeyring(), "test-connection-id")
}
