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

// commonBinDirectories are where the AWS CLI usually lives. They are only the
// fallback for when the login shell could not be read at all — a guess, and a
// poor one for anybody using asdf, mise, nvm or a custom prefix.
var commonBinDirectories = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
	"/usr/bin",
	"/bin",
}

var adoptEnvironmentOnce sync.Once

// ensureUsableEnvironment makes this process's environment resemble the one the
// user's own terminal would have, once.
//
// Every caller wants the same thing in the end: that exec.LookPath finds `aws`
// and `ssh`, and that whatever those programs read from the environment is
// there when they look.
func ensureUsableEnvironment() {
	adoptEnvironmentOnce.Do(adoptLoginShellEnvironment)
}

// adoptLoginShellEnvironment copies the login shell's environment into this
// process.
//
// Values already set here win. Someone who ran `AWS_PROFILE=other mssh` from a
// terminal meant it, and a default from their shell profile must not overrule
// them. The case this exists for — launched from Finder — has almost nothing
// set, so almost everything gets filled in.
//
// PATH is the exception. launchd does hand this process a PATH, just a useless
// one, so skipping it on the grounds that it is "already set" would defeat the
// whole exercise. It is merged instead, the shell's entries first.
func adoptLoginShellEnvironment() {
	variables, err := loginShellEnvironment()
	if err != nil {
		// Fall back to the old guess rather than to nothing at all.
		extendPathWithCommonDirectories()
		return
	}

	for name, value := range variables {
		if name == "PATH" {
			continue
		}
		if _, alreadySet := os.LookupEnv(name); alreadySet {
			continue
		}
		_ = os.Setenv(name, value)
	}

	if shellPath := variables["PATH"]; shellPath != "" {
		_ = os.Setenv("PATH", mergePath(shellPath, os.Getenv("PATH")))
	}
}

// mergePath joins two PATH values, keeping the order of the first and adding
// only the directories the second contributes.
//
// Comparing whole entries rather than running strings.Contains over the joined
// value, which the previous version did: "/usr/bin" is a substring of
// "/opt/usr/bin" and of "/usr/bin-old", and that test quietly answered yes.
func mergePath(primary string, extra string) string {
	seen := map[string]bool{}
	merged := make([]string, 0, 16)

	for _, group := range []string{primary, extra} {
		for _, directory := range strings.Split(group, ":") {
			if directory == "" || seen[directory] {
				continue
			}
			seen[directory] = true
			merged = append(merged, directory)
		}
	}
	return strings.Join(merged, ":")
}

// extendPathWithCommonDirectories is the fallback for a login shell that could
// not be run: better than nothing, and wrong for anybody with a custom prefix.
func extendPathWithCommonDirectories() {
	// The only documented failure of Setenv is an invalid variable name, which
	// "PATH" is not. If it somehow failed, LookPath would report the real
	// consequence anyway.
	_ = os.Setenv("PATH",
		mergePath(os.Getenv("PATH"), strings.Join(commonBinDirectories, ":")))
}

// requireAWSTools checks the two binaries an SSM session cannot run without.
func requireAWSTools() error {
	ensureUsableEnvironment()

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
			"no AWS credentials this app can see.\n\n"+
				"mssh runs your login shell at startup and takes its environment, "+
				"so anything exported in ~/.zshrc or ~/.zprofile should already be "+
				"here. Check `echo $AWS_PROFILE` and `aws sts get-caller-identity` "+
				"in a terminal: if they work there but not here, the export is "+
				"probably in a file your login shell does not read.\n\n"+
				"If you use SSO, run `%s`. Otherwise run `aws configure` to keep "+
				"the credentials in ~/.aws/credentials, or set a profile and "+
				"region on the workspace.", loginCommand)

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
	case strings.Contains(lowered, "terminated"),
		strings.Contains(lowered, "session is not in a valid state"):
		return fmt.Sprintf(
			"Session Manager ended the session with %s - most often the idle "+
				"timeout on the account, which defaults to 20 minutes", target)
	default:
		return lastLines(output, 3)
	}
}
