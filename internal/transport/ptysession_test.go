package transport

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
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

// ptyTestShell is a shell this system has, and the line that makes it print
// 42: the answer, unlike the line typed, is not echoed back by the terminal.
func ptyTestShell() (command *exec.Cmd, sum string) {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd.exe"), "set /a 6*7\r\n"
	}
	return exec.Command("sh"), "echo $((6*7))\n"
}

// A program started under the pty prints into the session, and its exit
// reaches onExit. On Windows this is ConPTY, on Unix creack/pty.
func TestPTYRunsAProgram(t *testing.T) {
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.Command("cmd.exe", "/c", "echo pty-ok")
	} else {
		command = exec.Command("sh", "-c", "echo pty-ok")
	}

	log := newSessionLog()
	session, err := startPTYProcess(command, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit, nil)
	if err != nil {
		t.Fatalf("startPTYProcess: %v", err)
	}
	defer func() { _ = session.Close() }()

	log.waitFor(t, "pty-ok")
	select {
	case err := <-log.exited:
		if err != nil {
			t.Errorf("onExit(%v), want nil for a program that succeeded", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the program finished but onExit was never called")
	}
}

// What the user types reaches the program, the terminal can be resized while
// it runs, and Close ends a program that would otherwise wait for ever.
func TestPTYTakesKeystrokes(t *testing.T) {
	command, sum := ptyTestShell()

	log := newSessionLog()
	session, err := startPTYProcess(command, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit, nil)
	if err != nil {
		t.Fatalf("startPTYProcess: %v", err)
	}

	if err := session.Resize(Size{Cols: 120, Rows: 40}); err != nil {
		t.Errorf("Resize: %v", err)
	}
	if _, err := session.Write([]byte(sum)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	log.waitFor(t, "42")

	if err := session.Close(); err != nil {
		t.Logf("Close: %v", err)
	}
	select {
	case err := <-log.exited:
		if err != nil {
			t.Errorf("onExit(%v) after Close, want nil: the user ended it", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not end the shell")
	}
}
