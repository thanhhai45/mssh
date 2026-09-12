package transport

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// sshConfigDialer runs the system ssh command against a Host alias.
//
// Everything is OpenSSH's business here: which user, which key, ProxyCommand,
// jump hosts, known_hosts, and its own yes/no prompt for an unknown key. mssh
// stores nothing but the alias, and asks nothing the user has already told
// ~/.ssh/config.
type sshConfigDialer struct{}

var _ Dialer = sshConfigDialer{}

func (sshConfigDialer) Name() string { return "System SSH" }

func (sshConfigDialer) Preflight(config Config) error {
	if strings.TrimSpace(config.Target) == "" {
		return fmt.Errorf("this connection has no ssh host alias")
	}

	ensureUsablePath()
	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("the ssh command is not installed, or not on PATH")
	}

	// Nothing else is checked on purpose. Whether the alias exists, whether the
	// key is right, whether the host answers — ssh reports all of that itself,
	// in the terminal, in the words the user already knows.
	return nil
}

func (sshConfigDialer) Dial(
	dialContext context.Context,
	config Config,
	size Size,
	onOutput func([]byte),
	onExit func(error),
) (Session, error) {
	alias := strings.TrimSpace(config.Target)

	command := exec.CommandContext(dialContext, "ssh", alias)
	command.Env = os.Environ()

	return startPTYProcess(command, size, onOutput, onExit, func(output string) string {
		return explainSSHCommandFailure(alias, output)
	})
}

// explainSSHCommandFailure adds the one piece of context ssh cannot know: that
// its settings came from ~/.ssh/config rather than from this app.
func explainSSHCommandFailure(alias string, output string) string {
	lowered := strings.ToLower(output)

	switch {
	// A ProxyCommand that shells out to the AWS CLI fails here, and the reason
	// is not something the user can guess: they can see the exports in their
	// shell profile, so "no credentials" reads like a bug in this app.
	case strings.Contains(lowered, "unable to locate credentials"),
		strings.Contains(lowered, "credentials not found"),
		strings.Contains(lowered, "the config profile") && strings.Contains(lowered, "could not be found"):
		return fmt.Sprintf(
			"the ProxyCommand for %q could not find AWS credentials.\n\n"+
				"An application started from Finder does not read ~/.zshrc, so "+
				"anything exported there is invisible to it. Run `aws configure` "+
				"to keep the credentials in ~/.aws/credentials instead — every "+
				"process can read that, however it was started.", alias)

	case strings.Contains(lowered, "could not resolve hostname"):
		return fmt.Sprintf(
			"ssh does not know how to reach %q — check that ~/.ssh/config has a "+
				"Host block matching it", alias)

	case strings.Contains(lowered, "permission denied"):
		return fmt.Sprintf(
			"%s refused the credentials ssh offered — check User and IdentityFile "+
				"in its Host block in ~/.ssh/config", alias)

	case strings.Contains(lowered, "closed by remote host"),
		strings.Contains(lowered, "connection reset byt peer"),
		strings.Contains(lowered, "broken pipe"),
		strings.Contains(lowered, "not responding"):
		return fmt.Sprintf("%s closed the connection - usually an idle timeout, a reboot, or the network dropping", alias)
	default:
		// ssh's own message is usually the clearest thing available.
		return lastLines(output, 3)
	}
}
