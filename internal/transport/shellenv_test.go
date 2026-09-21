package transport

import (
	"os"
	"slices"
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

// TestAdoptLoginShellEnvironmentRepairsAFinderLikePATH is the one that proves
// the point of the whole exercise, so it starts from the environment launchd
// actually hands a double-clicked app.
//
// Also gated: it needs the real shell. Note that adoptLoginShellEnvironment
// calls os.Setenv for variables this process lacks, and those are not restored
// afterwards — acceptable for a test that only runs on demand.
func TestAdoptLoginShellEnvironmentRepairsAFinderLikePATH(t *testing.T) {
	if os.Getenv("MSSH_TEST_REAL_SHELL") == "" {
		t.Skip("set MSSH_TEST_REAL_SHELL=1 to run this against your own shell")
	}

	// What an application launched from Finder starts with.
	const launchdPath = "/usr/bin:/bin:/usr/sbin:/sbin"
	t.Setenv("PATH", launchdPath)

	adoptLoginShellEnvironment()

	repaired := strings.Split(os.Getenv("PATH"), ":")
	if len(repaired) <= 4 {
		t.Fatalf("PATH is still %d entries, the shell added nothing: %v",
			len(repaired), repaired)
	}

	// Nothing launchd gave us may be lost on the way.
	for _, required := range strings.Split(launchdPath, ":") {
		if !slices.Contains(repaired, required) {
			t.Errorf("%s disappeared from PATH", required)
		}
	}

	t.Logf("4 entries -> %d", len(repaired))
}
