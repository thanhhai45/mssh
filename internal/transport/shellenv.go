package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
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

// shellEnvironmentResult is everything worth remembering about one attempt,
// not just the variables: the diagnostics view has to be able to say which
// shell ran, how long it took, and why it failed.
type shellEnvironmentResult struct {
	shell     string
	variables map[string]string
	duration  time.Duration
	err       error
}

// A mutex and a flag rather than sync.Once, because the diagnostics view can
// ask for a fresh reading and Once has no way back.
var (
	shellEnvironmentMutex  sync.Mutex
	shellEnvironmentLoaded bool
	shellEnvironmentCache  shellEnvironmentResult
)

// cachedShellEnvironment reads the environment at most once, unless asked to
// do it again.
//
// The lock is held across the whole read on purpose: a second caller arriving
// mid-read should wait for that answer rather than start another shell.
func cachedShellEnvironment(reload bool) shellEnvironmentResult {
	shellEnvironmentMutex.Lock()
	defer shellEnvironmentMutex.Unlock()

	if shellEnvironmentLoaded && !reload {
		return shellEnvironmentCache
	}

	start := time.Now()
	variables, err := readLoginShellEnvironment()

	shellEnvironmentCache = shellEnvironmentResult{
		shell:     loginShellPath(),
		variables: variables,
		duration:  time.Since(start),
		err:       err,
	}
	shellEnvironmentLoaded = true
	return shellEnvironmentCache
}

// loginShellPath is the shell to run, and the one the report names.
func loginShellPath() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/zsh"
}

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
	result := cachedShellEnvironment(false)
	return result.variables, result.err
}

// WarmShellEnvironment fills that cache from a goroutine, so the second it
// costs is not spent while somebody is waiting to connect.
func WarmShellEnvironment() {
	go func() { _, _ = loginShellEnvironment() }()
}

/* ---------------- diagnostics ---------------- */

// ShellEnvironmentVariable is one row of the report.
type ShellEnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Masked says the value was withheld, so the UI can label it rather than
	// leave the reader wondering why a variable appears to hold dots.
	Masked bool `json:"masked"`
	// Interesting marks the handful that explain a failed connection, so the
	// UI can show those first and keep the other sixty behind a toggle.
	Interesting bool `json:"interesting"`
}

// ShellEnvironmentReport is what the Appearance page shows.
//
// Inheriting an environment silently would make behaviour depend on state
// nobody can see. This is the other half of that bargain: it is automatic, and
// it says what it did.
type ShellEnvironmentReport struct {
	Shell      string                     `json:"shell"`
	DurationMS int64                      `json:"durationMs"`
	Error      string                     `json:"error"`
	Variables  []ShellEnvironmentVariable `json:"variables"`
}

// secretishName matches variables whose value must never leave this process.
//
// AWS_ACCESS_KEY_ID matches on "key" and is only an identifier rather than a
// secret. Masking it costs nothing; the reverse mistake cannot be undone.
var secretishName = regexp.MustCompile(`(?i)secret|token|key|password|passwd|credential`)

// interestingName marks the variables that decide whether a connection works.
func interestingName(name string) bool {
	switch {
	case name == "PATH", name == "SSH_AUTH_SOCK":
		return true
	case strings.HasPrefix(name, "AWS_"):
		return true
	case strings.HasSuffix(strings.ToUpper(name), "_PROXY"):
		return true
	}
	return false
}

// ShellEnvironmentSnapshot builds the report. Passing reload runs the shell
// again, for the button next to it.
//
// Masking happens here rather than in the frontend, so a secret never crosses
// the boundary at all — there is nothing to leak into a log, a screenshot or a
// crash report on the other side.
func ShellEnvironmentSnapshot(reload bool) ShellEnvironmentReport {
	result := cachedShellEnvironment(reload)

	report := ShellEnvironmentReport{
		Shell:      result.shell,
		DurationMS: result.duration.Milliseconds(),
		Variables:  []ShellEnvironmentVariable{},
	}
	if result.err != nil {
		report.Error = result.err.Error()
	}

	for name, value := range result.variables {
		row := ShellEnvironmentVariable{
			Name:        name,
			Value:       value,
			Interesting: interestingName(name),
		}
		if secretishName.MatchString(name) {
			row.Value = ""
			row.Masked = true
		}
		report.Variables = append(report.Variables, row)
	}

	// Interesting ones first, then alphabetical: a map iterates in a different
	// order every time, and a list that reshuffles on every render is unusable.
	sort.Slice(report.Variables, func(first int, second int) bool {
		left, right := report.Variables[first], report.Variables[second]
		if left.Interesting != right.Interesting {
			return left.Interesting
		}
		return left.Name < right.Name
	})

	return report
}

// readLoginShellEnvironment does the work. It is kept apart from the cached
// wrapper so a test can call it more than once in one process.
func readLoginShellEnvironment() (map[string]string, error) {
	shell := loginShellPath()

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
