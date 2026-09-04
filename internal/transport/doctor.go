package transport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// commonBinDirectories are added to PATH because an application launched from
// Finder does not inherit the shell's PATH, and the AWS CLI almost always
// lives in one of these.
var commonBinDirectories = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
	"/usr/bin",
	"/bin",
}

var extendPathOnce sync.Once

// ensureUsablePath widens the process PATH once, so exec.LookPath finds the
// AWS tools whether mssh was started from a terminal or double-clicked.
func ensureUsablePath() {
	extendPathOnce.Do(func() {
		currentPath := os.Getenv("PATH")

		var missing []string
		for _, directory := range commonBinDirectories {
			if !strings.Contains(currentPath, directory) {
				missing = append(missing, directory)
			}
		}
		if len(missing) > 0 {
			os.Setenv("PATH", currentPath+":"+strings.Join(missing, ":"))
		}
	})
}

// requireAWSTools checks the two binaries an SSM session cannot run without.
func requireAWSTools() error {
	ensureUsablePath()

	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf(
			"the AWS CLI is not installed, or not on PATH — install it with " +
				"`brew install awscli`")
	}
	if _, err := exec.LookPath("session-manager-plugin"); err != nil {
		return fmt.Errorf(
			"the Session Manager plugin is not installed — install it with " +
				"`brew install --cask session-manager-plugin`")
	}
	return nil
}

// CheckSSMTools reports whether this machine has what the SSM kinds need. It
// is exported so the UI can warn before the user configures such a connection,
// instead of failing at connect time.
func CheckSSMTools() error {
	return requireAWSTools()
}

// awsFlags turns a resolved profile and region into command-line flags.
//
// Empty values are left out entirely rather than passed as "", so the AWS CLI
// falls back to its own configuration. That is the third tier of the
// inheritance rule from the store package.
func awsFlags(profile string, region string) []string {
	flags := []string{}
	if profile != "" {
		flags = append(flags, "--profile", profile)
	}
	if region != "" {
		flags = append(flags, "--region", region)
	}
	return flags
}

// checkAWSCredentials asks STS who we are. It is the cheapest call that proves
// the profile exists, its credentials are valid, and they have not expired.
func checkAWSCredentials(profile string, region string) error {
	checkContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	arguments := append([]string{"sts", "get-caller-identity"}, awsFlags(profile, region)...)
	command := exec.CommandContext(checkContext, "aws", arguments...)
	command.Env = os.Environ()

	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	if errors.Is(checkContext.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("`aws sts get-caller-identity` did not answer within 15 seconds")
	}
	return errors.New(explainAWSFailure(profile, string(output)))
}

// explainAWSFailure turns the AWS CLI's output into something that says what to
// do next. It is a pure function so the mapping can be tested without AWS.
func explainAWSFailure(profile string, output string) string {
	lowered := strings.ToLower(output)

	loginCommand := "aws sso login"
	if profile != "" {
		loginCommand += " --profile " + profile
	}

	switch {
	case strings.Contains(lowered, "sso session associated with this profile has expired"),
		strings.Contains(lowered, "token has expired"),
		strings.Contains(lowered, "expiredtoken"):
		return fmt.Sprintf("your AWS session has expired — run `%s`", loginCommand)

	case strings.Contains(lowered, "unable to locate credentials"),
		strings.Contains(lowered, "you must specify a region"):
		return fmt.Sprintf(
			"no usable AWS credentials for this connection — run `%s`, "+
				"or set a profile and region on the workspace", loginCommand)

	case strings.Contains(lowered, "could not be found") && strings.Contains(lowered, "profile"):
		return fmt.Sprintf("AWS profile %q is not configured in ~/.aws/config", profile)

	default:
		return "aws sts get-caller-identity failed: " + strings.TrimSpace(output)
	}
}

// explainSSMFailure turns start-session output into something actionable.
func explainSSMFailure(target string, output string) string {
	lowered := strings.ToLower(output)

	switch {
	case strings.Contains(lowered, "targetnotconnected"):
		return fmt.Sprintf(
			"%s is not reachable through Session Manager — check that the "+
				"instance is running, has the SSM Agent, and has an IAM role "+
				"with AmazonSSMManagedInstanceCore", target)

	case strings.Contains(lowered, "accessdenied"):
		return fmt.Sprintf(
			"this AWS profile is not allowed to run ssm:StartSession on %s", target)

	case strings.Contains(lowered, "invalidinstanceid"):
		return fmt.Sprintf("%s is not an instance this account can see", target)

	default:
		return strings.TrimSpace(output)
	}
}
