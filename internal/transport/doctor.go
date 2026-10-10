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

var adoptPathOnce sync.Once

// ensureUsableEnvironment makes sure exec.LookPath can find aws, ssh and
// gcloud, whether mssh was started from a terminal or from Finder.
func ensureUsableEnvironment() {
	adoptPathOnce.Do(adoptLoginShellPath)
}

// adoptLoginShellPath takes PATH from the user's login shell, and nothing else.
//
// launchd hands an app opened from Finder a PATH of four system directories,
// so aws and ssh installed through Homebrew, asdf or mise are not found. The
// shell's PATH is merged in front of it rather than replacing it.
//
// An earlier version adopted the whole environment. That reached further than
// mssh needs: it has to find programs, not carry everything a shell profile
// happens to export. AWS credentials now come from the workspace instead — see
// docs/CREDENTIALS.md.
func adoptLoginShellPath() {
	variables, err := loginShellEnvironment()
	if err != nil {
		// Fall back to the old guess rather than to nothing at all.
		extendPathWithCommonDirectories()
		return
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

// awsCredentialVariables are every variable that can make the AWS CLI use
// credentials other than the ones mssh hands it. Measured against awws-cli 2.22;
// a profile named by AWS_PROFILE or AWS_DEFAULT_PROFILE outranks keys in the
// environment, and AWS_SECURITY_TOKEN - the old name - is still read as the session token.
var awsCredentialVariables = map[string]bool{
	"AWS_PROFILE":           true,
	"AWS_DEFAULT_PROFILE":   true,
	"AWS_ACCESS_KEY_ID":     true,
	"AWS_SECRET_ACCESS_KEY": true,
	"AWS_SESSION_TOKEN":     true,
	"AWS_SECURITY_TOKEN":    true,
}

// awsRegionVariables are the two names the AWS CLI reads a region from. Both
// are set, to the same value: AWS_REGION is the current one, and an older CLI
// or SDK inside a ProxyCommand may still read only AWS_DEFAULT_REGION.
var awsRegionVariables = map[string]bool{
	"AWS_REGION":         true,
	"AWS_DEFAULT_REGION": true,
}

// awsEnvironment is the environment for a child that will run aws, directly or
// through an ssh ProxyCommand
//
// With stored keys, every credential variable above is dropped first. Two
// sets of credentials in one environment is not a configuration anybody
// chose, and which one the CLI picks is not something to leave to chance.
//
// The region goes in the environment too. The ssm kinds also pass --region,
// which outranks it, but the ssh-config kind cannot: its aws command sits in
// the user's ProxyCommand, where mssh adds no flags. Without this, a workspace
// on stored keys — usually a machine with no ~/.aws/config to fall back on —
// failed there with "You must specify a region".
func awsEnvironment(base []string, credentials *AWSCredentials, region string) []string {
	if credentials == nil && region == "" {
		return base
	}

	kept := make([]string, 0, len(base)+5)
	for _, variable := range base {
		name, _, _ := strings.Cut(variable, "=")
		if credentials != nil && awsCredentialVariables[name] {
			continue
		}
		if region != "" && awsRegionVariables[name] {
			continue
		}
		kept = append(kept, variable)
	}

	if credentials != nil {
		kept = append(kept,
			"AWS_ACCESS_KEY_ID="+credentials.AccessKeyID,
			"AWS_SECRET_ACCESS_KEY="+credentials.SecretAccessKey,
		)
		if credentials.SessionToken != "" {
			kept = append(kept, "AWS_SESSION_TOKEN="+credentials.SessionToken)
		}
	}
	if region != "" {
		kept = append(kept, "AWS_REGION="+region, "AWS_DEFAULT_REGION="+region)
	}
	return kept
}

// effectiveProfile is the profile to put on the command line. With stored keys
// there is none: --profile outranks the environment, so a profile would
// quietly win over the keys this connection was configured with.
func effectiveProfile(config Config) string {
	if config.AWSCredentials != nil {
		return ""
	}
	return config.AWSProfile
}

// checkAWSCredentials asks STS who we are. It is the cheapest call that proves
// the profile exists, are valid, and have not expired.
func checkAWSCredentials(config Config) error {
	checkContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	arguments := append(
		[]string{"sts", "get-caller-identity"},
		awsFlags(effectiveProfile(config), config.AWSRegion)...)
	command := exec.CommandContext(checkContext, "aws", arguments...)
	command.Env = awsEnvironment(os.Environ(), config.AWSCredentials, config.AWSRegion)

	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	if errors.Is(checkContext.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("`aws sts get-caller-identity` did not answer within 15 seconds")
	}
	if config.AWSCredentials != nil {
		return errors.New(explainStoredKeysFailure(string(output)))
	}
	return errors.New(explainAWSFailure(config.AWSProfile, string(output)))
}

// explainStoredKeysFailure is explainAWSFailure for a workspace that keeps its
// own keys. The advice is different: `aws sso login` and ~/.aws are beside the
// point when the keys came from the workspace settings.
func explainStoredKeysFailure(output string) string {
	lowered := strings.ToLower(output)

	switch {
	case strings.Contains(lowered, "expiredtoken"),
		strings.Contains(lowered, "token has expired"),
		strings.Contains(lowered, "is expired"):
		return "the temporary AWS keys stored on this workspace have expired - " +
			"import or paste fresh ones in the workspace setting"

	case strings.Contains(lowered, "you must specify a region"):
		return "this workspace uses stored AWS keys but has no region - " +
			"set one in the workspace setting"

	default:
		return "AWS did not accept the keys stored on this workspace: " +
			strings.TrimSpace(output)
	}
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
				"mssh does not take AWS keys from your shell environment. Either let "+
				"the AWS CLI find them — run `%s` if you use SSO, or `aws configure` to "+
				"keep them in ~/.aws/credentials, then set the profile on the "+
				"workspace — or keep keys on the workspace itself: edit the workspace, "+
				"choose \"Use keys stored in mssh\", and \"Import from shell\" copies "+
				"the AWS_* variables your shell profile exports.", loginCommand)

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
