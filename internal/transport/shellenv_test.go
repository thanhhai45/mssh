package transport

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

/* ---------------- helpers ---------------- */

// fakeShellScript is written once for the whole test binary, and does whatever
// the environment tells it to.
//
// Once, because macOS scans a newly written executable the first time it runs:
// measured here, 200ms for a fresh file against 4ms for one that has run
// before. A script per subtest would spend an order of magnitude more time in
// syspolicyd than in the code under test, and would read as though
// readLoginShellEnvironment were slow.
var fakeShellScript string

const fakeShellBody = `#!/bin/sh
# Behaviour comes from the environment, so this file never has to change.
if [ -n "$MSSH_FAKE_SHELL_SLEEP" ]; then
	sleep "$MSSH_FAKE_SHELL_SLEEP"
fi
exec cat "$MSSH_FAKE_SHELL_OUTPUT"
`

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "mssh-fake-shell")
	if err != nil {
		panic(err)
	}

	fakeShellScript = filepath.Join(directory, "shell")
	if err := os.WriteFile(fakeShellScript, []byte(fakeShellBody), 0o700); err != nil {
		panic(err)
	}

	code := m.Run()
	_ = os.RemoveAll(directory)
	os.Exit(code)
}

// fakeShell points SHELL at that script and hands it exactly the bytes to
// print.
//
// The bytes come from Go rather than from a printf inside the script. NUL is
// the separator under test, and whether a particular /bin/sh emits \000
// faithfully is the sort of thing that differs between machines — the test
// would then be measuring the machine rather than the code.
func fakeShell(t *testing.T, output []byte) {
	t.Helper()

	fixture := filepath.Join(t.TempDir(), "output")
	if err := os.WriteFile(fixture, output, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	t.Setenv("SHELL", fakeShellScript)
	t.Setenv("MSSH_FAKE_SHELL_OUTPUT", fixture)
}

// envDump builds what a real shell would print: anything the startup files
// said, then the marker, then NUL-separated variables.
func envDump(banner string, entries ...string) []byte {
	var buffer bytes.Buffer
	buffer.WriteString(banner)
	buffer.WriteString(envMarker)
	for _, entry := range entries {
		buffer.WriteString(entry)
		buffer.WriteByte(0)
	}
	return buffer.Bytes()
}

/* ---------------- the eight cases ---------------- */

func TestReadLoginShellEnvironment(t *testing.T) {
	t.Run("the plain case", func(t *testing.T) {
		fakeShell(t, envDump("", "PATH=/x/y", "FOO=bar"))

		variables, err := readLoginShellEnvironment()
		if err != nil {
			t.Fatalf("readLoginShellEnvironment: %v", err)
		}
		if variables["PATH"] != "/x/y" || variables["FOO"] != "bar" {
			t.Errorf("got %v", variables)
		}
	})

	t.Run("a banner before the marker is discarded", func(t *testing.T) {
		// This is why the marker exists. Startup files print motd, update
		// notices, version banners — and some of that looks exactly like a
		// variable assignment.
		fakeShell(t, envDump("Welcome!\nNOT_A_VAR=nope\n", "FOO=bar"))

		variables, err := readLoginShellEnvironment()
		if err != nil {
			t.Fatalf("readLoginShellEnvironment: %v", err)
		}
		if _, leaked := variables["NOT_A_VAR"]; leaked {
			t.Error("a line the banner printed was parsed as a variable")
		}
		if variables["FOO"] != "bar" {
			t.Errorf("FOO = %q, want bar", variables["FOO"])
		}
	})

	t.Run("a value containing a newline survives", func(t *testing.T) {
		// This is why env -0 is used instead of plain env. Splitting on "\n"
		// would turn this into one variable and one piece of nonsense.
		fakeShell(t, envDump("", "MULTI=first\nsecond", "AFTER=yes"))

		variables, err := readLoginShellEnvironment()
		if err != nil {
			t.Fatalf("readLoginShellEnvironment: %v", err)
		}
		if variables["MULTI"] != "first\nsecond" {
			t.Errorf("MULTI = %q, want the newline kept", variables["MULTI"])
		}
		if variables["AFTER"] != "yes" {
			t.Error("the variable after a multi-line one was lost")
		}
	})

	t.Run("a value containing an equals sign is not split twice", func(t *testing.T) {
		fakeShell(t, envDump("", "QUERY=a=b&c=d"))

		variables, err := readLoginShellEnvironment()
		if err != nil {
			t.Fatalf("readLoginShellEnvironment: %v", err)
		}
		if variables["QUERY"] != "a=b&c=d" {
			t.Errorf("QUERY = %q, want a=b&c=d", variables["QUERY"])
		}
	})

	t.Run("variables belonging to that shell are dropped", func(t *testing.T) {
		fakeShell(t, envDump("",
			"PWD=/tmp", "SHLVL=3", "TERM=dumb", "OLDPWD=/", "_=/bin/env",
			"HOME=/somewhere/else", "KEEP=yes"))

		variables, err := readLoginShellEnvironment()
		if err != nil {
			t.Fatalf("readLoginShellEnvironment: %v", err)
		}
		for _, name := range []string{"PWD", "SHLVL", "TERM", "OLDPWD", "_", "HOME"} {
			if _, present := variables[name]; present {
				t.Errorf("%s should have been filtered out", name)
			}
		}
		if variables["KEEP"] != "yes" {
			t.Error("a normal variable was filtered out too")
		}
	})

	t.Run("no marker is an error, not an empty environment", func(t *testing.T) {
		// Silently returning nothing here would look like "your shell has no
		// variables", which sends the reader in the wrong direction.
		fakeShell(t, []byte("hello, no marker here\n"))

		if _, err := readLoginShellEnvironment(); err == nil {
			t.Fatal("expected an error")
		} else if !strings.Contains(err.Error(), "printed no environment") {
			t.Errorf("unhelpful error: %v", err)
		}
	})

	t.Run("a shell that does not exist is an error, not a panic", func(t *testing.T) {
		t.Setenv("SHELL", "/nope/not/a/shell")

		if _, err := readLoginShellEnvironment(); err == nil {
			t.Fatal("expected an error")
		} else if !strings.Contains(err.Error(), "/nope/not/a/shell") {
			t.Errorf("the error does not name the shell: %v", err)
		}
	})

	t.Run("a shell that hangs is abandoned", func(t *testing.T) {
		previousLimit := shellEnvironmentLimit
		shellEnvironmentLimit = 200 * time.Millisecond
		t.Cleanup(func() { shellEnvironmentLimit = previousLimit })

		// sleep is a child of the fake shell, so killing the shell leaves the
		// output pipe held open by a grandchild. Without command.WaitDelay this
		// returns after the full five seconds rather than at the deadline — the
		// bug this case exists to keep fixed.
		fakeShell(t, envDump("", "NEVER=printed"))
		t.Setenv("MSSH_FAKE_SHELL_SLEEP", "5")

		start := time.Now()
		_, err := readLoginShellEnvironment()
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected a timeout error")
		}
		if !strings.Contains(err.Error(), "did not finish starting up") {
			t.Errorf("unhelpful error: %v", err)
		}
		if elapsed > 3*time.Second {
			t.Errorf("took %s: the deadline fired but Wait blocked on the "+
				"grandchild, so WaitDelay is not doing its job", elapsed)
		}
	})
}

/* ---------------- smoke tests against the real shell ---------------- */

// These run the developer's own shell, so the result depends on whose machine
// it is. Skipped by default, the same way the keychain test is.

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

// TestAdoptLoginShellEnvironmentRepairsAFinderLikePATH proves the point of the
// whole exercise, so it starts from the environment launchd actually hands a
// double-clicked app.
//
// adoptLoginShellEnvironment calls os.Setenv for variables this process lacks,
// and those are not restored afterwards — acceptable for a test that only runs
// on demand.
func TestAdoptLoginShellEnvironmentRepairsAFinderLikePATH(t *testing.T) {
	if os.Getenv("MSSH_TEST_REAL_SHELL") == "" {
		t.Skip("set MSSH_TEST_REAL_SHELL=1 to run this against your own shell")
	}

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
