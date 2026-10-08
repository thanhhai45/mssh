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

// loginShellPath is the shell to run.
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
// Computed once and kept: it costs most of a second, and nothing that uses it
// needs a fresher answer. sync.OnceValues also makes a second caller arriving
// mid-read wait for that answer rather than start another shell.
//
// A variable holding a function rather than a function, so a test can put a
// fixed answer in its place without running a shell — the same seam Store has
// for its clock.
var loginShellEnvironment = sync.OnceValues(readLoginShellEnvironment)

// WarmShellEnvironment fills that cache from a goroutine, so the second it
// costs is not spent while somebody is waiting to connect.
func WarmShellEnvironment() {
	go func() { _, _ = loginShellEnvironment() }()
}

// readLoginShellEnvironment does the work, uncached. Tests call it directly,
// and so will importing AWS keys from the shell, which has to see an edit made
// to ~/.zshrc a minute ago.
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

// AWSShellPreview is what the import button shows before anything is saved:
// enough to recognise the keys, never the secret itself
type AWSShellPreview struct {
	AccessKeyID        string `json:"accessKeyId"`
	HasSecretAccessKey bool   `json:"hasSecretAccessKey"`
	HasSessionToken    bool   `json:"hasSessionToken"`
	Region             string `json:"region"`
	Profile            string `json:"profile"`
}

// AWSFromShell runs the login shell once and picks out the AWS variables, and
// only those: six names, looked up one by one, never a walk over everything
// the shell exported. It reads fresh rather than from the startup cache,
// because the usual reason to press the button is having just edited ~/.zshrc.
//
// The preview is safe to send to the frontend. The credentials are not: the
// caller keeps them in Go. They are nil unless both halves of a key pair were
// found.
func AWSFromShell() (AWSShellPreview, *AWSCredentials, error) {
	variables, err := readLoginShellEnvironment()
	if err != nil {
		return AWSShellPreview{}, nil, err
	}

	region := variables["AWS_REGION"]
	if region == "" {
		region = variables["AWS_DEFAULT_REGION"]
	}

	preview := AWSShellPreview{
		AccessKeyID:        variables["AWS_ACCESS_KEY_ID"],
		HasSecretAccessKey: variables["AWS_SECRET_ACCESS_KEY"] != "",
		HasSessionToken:    variables["AWS_SESSION_TOKEN"] != "",
		Region:             region,
		Profile:            variables["AWS_PROFILE"],
	}

	// A key id without its secret, or the reverse, is not a usable pair.
	if preview.AccessKeyID == "" || !preview.HasSecretAccessKey {
		return preview, nil, nil
	}

	return preview, &AWSCredentials{
		AccessKeyID:     variables["AWS_ACCESS_KEY_ID"],
		SecretAccessKey: variables["AWS_SECRET_ACCESS_KEY"],
		SessionToken:    variables["AWS_SESSION_TOKEN"],
	}, nil
}
