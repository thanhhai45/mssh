package transport

import (
	"context"
	"fmt"
	"os/exec"
)

// ssmDialer opens a session by running `aws ssm start-session` under a local
// pseudo-terminal.
//
// SSO refresh, MFA and assume-role remain the AWS CLI's business. mssh only
// hands over a key pair when the workspace keeps its own, and then through the
// environment, never the command line
type ssmDialer struct{}

// Compile-time check, reported here rather than wherever For() assigns it.
var _ Dialer = ssmDialer{}

func (ssmDialer) Name() string { return "AWS SSM" }

func (ssmDialer) Preflight(config Config) error {
	if config.Target == "" {
		return fmt.Errorf("this connection has no instance id")
	}
	if err := requireAWSTools(); err != nil {
		return err
	}
	return checkAWSCredentials(config)
}

func (ssmDialer) Dial(
	dialContext context.Context,
	config Config,
	size Size,
	onOutput func([]byte),
	onExit func(error),
) (Session, error) {
	arguments := append(
		[]string{"ssm", "start-session", "--target", config.Target},
		awsFlags(effectiveProfile(config), config.AWSRegion)...,
	)

	command := exec.CommandContext(dialContext, "aws", arguments...)
	command.Env = awsEnvironment(pseudoTerminalEnvironment(), config.AWSCredentials)

	// session-manager-plugin looks at whether its stdin is a terminal to decide
	// on raw mode, and reads the window size from it. Give it a plain pipe and
	// you get a shell with no prompt, a broken Ctrl-C and wrong line wrapping.
	return startPTYProcess(command, size, onOutput, onExit, func(output string) string {
		return explainSSMFailure(config.Target, output)
	})
}
