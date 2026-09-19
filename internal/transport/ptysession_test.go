package transport

import (
	"strings"
	"testing"
)

func TestPseudoTerminalEnvironmentForcesTERM(t *testing.T) {
	// t.Setenv restores the old value when the test ends, which is the only
	// safe way to touch process-wide state from a test.
	t.Setenv("TERM", "dumb")
	t.Setenv("MSSH_TEST_MARKER", "kept")

	environment := pseudoTerminalEnvironment()

	terminalEntries := 0
	markerFound := false
	for _, variable := range environment {
		if strings.HasPrefix(variable, "TERM=") {
			terminalEntries++
			if variable != "TERM=xterm-256color" {
				t.Errorf("got %q, want TERM=xterm-256color", variable)
			}
		}
		if variable == "MSSH_TEST_MARKER=kept" {
			markerFound = true
		}
	}

	// Exactly one: an inherited TERM must be replaced, not shadowed.
	if terminalEntries != 1 {
		t.Errorf("found %d TERM entries, want exactly 1", terminalEntries)
	}
	if !markerFound {
		t.Error("the rest of the environment was not carried through")
	}
}
