package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// envMarker separates whatever the startup files printed from the environment
// dump that follows. Shell startup files print banners, update notices and
// motd; without a marker those get read as variables.
const envMarker = "__MSSH_ENV_BEGINS__"

// shellEnvironmentLimit bounds the whole thing. A startup file that loads nvm
// or conda takes seconds; one that ends in `exec tmux` never returns at all.
//
// A var rather than a const so the test for it can run in a fraction of a
// second instead of five of them. Same reason Store takes its clock as a field.
var shellEnvironmentLimit = 5 * time.Second

// notInherited are variables that belong to the shell which produced them
// rather than to this process. TERM is set deliberately in
// pseudoTerminalEnvironment, and the rest describe a shell session that does
// not exist here.
var notInherited = map[string]bool{
	"_":       true,
	"OLDPWD":  true,
	"PWD":     true,
	"SHLVL":   true,
	"TERM":    true,
	"HOME":    true,
	"USER":    true,
	"LOGNAME": true,
}

var (
	shellEnvironmentOnce sync.Once
	shellEnvironmentVars map[string]string
	shellEnvironmentErr  error
)

// loginShellEnvironment returns the variables the user's own terminal would
// have, by running their login shell and asking it.
//
// An application launched from Finder inherits a minimal environment: a bare
// PATH, and none of the exports in ~/.zshrc. Reading those files instead of
// running them does not work — they are programs, and values routinely come
// from $(…), from `source`, or from a branch.
//
// The goal is to match what the user's terminal would have, not to hunt for
// variables wherever they might hide. If their terminal cannot see it, neither
// should mssh.
//
// Computed once and kept: it costs most of a second, and it cannot change
// while the app is running.
func loginShellEnvironment() (map[string]string, error) {
	shellEnvironmentOnce.Do(func() {
		shellEnvironmentVars, shellEnvironmentErr = readLoginShellEnvironment()
	})
	return shellEnvironmentVars, shellEnvironmentErr
}

// WarmShellEnvironment fills that cache from a goroutine, so the second it
// costs is not spent while somebody is waiting to connect.
func WarmShellEnvironment() {
	go func() { _, _ = loginShellEnvironment() }()
}

// readLoginShellEnvironment does the work. It is kept apart from the cached
// wrapper so a test can call it more than once in one process.
func readLoginShellEnvironment() (map[string]string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}

	runContext, cancel := context.WithTimeout(
		context.Background(), shellEnvironmentLimit)
	defer cancel()

	// -l reads the login files (.zprofile, .bash_profile); -i reads .zshrc and
	// .bashrc. Both are needed: which of them holds the exports is a matter of
	// personal habit.
	//
	// env -0 rather than plain env: entries are separated by NUL, so a value
	// containing a newline survives intact.
	command := exec.CommandContext(runContext, shell, "-l", "-i", "-c",
		"printf '%s' '"+envMarker+"'; env -0")

	// Without this the timeout above is decorative. CommandContext kills the
	// shell, but anything the shell started keeps the output pipe open, and
	// Wait blocks until that pipe closes — so a startup file that launches a
	// daemon, or ends in `exec tmux`, hangs here for as long as the child
	// lives. WaitDelay gives the pipes a second to drain and then closes them.
	command.WaitDelay = time.Second

	var output bytes.Buffer
	command.Stdout = &output

	// stderr is kept but never mixed in. An interactive shell with no terminal
	// complains, and that noise must not be parsed as variables — it is only
	// ever used to explain a failure.
	complaints := &tailBuffer{limit: 2048}
	command.Stderr = complaints

	if err := command.Run(); err != nil {
		if errors.Is(runContext.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf(
				"%s did not finish starting up within %s — something in your "+
					"shell startup files is slow, or waiting for input",
				shell, shellEnvironmentLimit)
		}
		if message := lastLines(complaints.string(), 3); message != "" {
			return nil, fmt.Errorf("run %s: %w: %s", shell, err, message)
		}
		return nil, fmt.Errorf("run %s: %w", shell, err)
	}

	_, dump, found := strings.Cut(output.String(), envMarker)
	if !found {
		return nil, fmt.Errorf("%s printed no environment", shell)
	}

	variables := map[string]string{}
	for _, entry := range strings.Split(dump, "\x00") {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || notInherited[name] {
			continue
		}
		variables[name] = value
	}
	return variables, nil
}
