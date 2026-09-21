package transport

import (
	"os"
	"strings"
	"testing"
)

// TestReadLoginShellEnvironmentAgainstTheRealShell is a smoke test, not a unit
// test: it runs the developer's own shell, so the result depends on whose
// machine it is. Skipped by default, the same way the keychain test is.
func TestReadLoginShellEnvironmentAgainstTheRealShell(t *testing.T) {
	if os.Getenv("MSSH_TEST_REAL_SHELL") == "" {
		t.Skip("set MSSH_TEST_REAL_SHELL=1 to run this against your own shell")
	}

	variables, err := readLoginShellEnvironment()
	if err != nil {
		t.Fatalf("readLoginShellEnvironment: %v", err)
	}

	if variables["PATH"] == "" {
		t.Error("no PATH came back, which cannot be right")
	}
	for _, name := range []string{"PWD", "SHLVL", "TERM"} {
		if _, present := variables[name]; present {
			t.Errorf("%s should have been filtered out", name)
		}
	}

	t.Logf("shell = %s", os.Getenv("SHELL"))
	t.Logf("count = %d variables", len(variables))
	t.Logf("PATH  = %s", variables["PATH"])

	// Names only, never values. This is the first place the "never log" half of
	// the rule in docs/SHELL-ENV.md meets real code, and printing a value here
	// "just while debugging" is exactly how such a rule gets lost.
	for name := range variables {
		if strings.HasPrefix(name, "AWS_") {
			t.Logf("found = %s", name)
		}
	}
}
