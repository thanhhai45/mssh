//go:build windows

package transport

import (
	"os"
	"testing"

	"golang.org/x/crypto/ssh/agent"
)

func TestAgentPipe(t *testing.T) {
	tests := []struct {
		authSock string
		want     string
	}{
		{"", openSSHAgentPipe},
		{`\\.\pipe\pageant.me.1234`, `\\.\pipe\pageant.me.1234`},
		// What Git Bash's ssh-agent exports: a path only MSYS understands.
		{"/tmp/ssh-XXXXXXabc/agent.1234", openSSHAgentPipe},
	}
	for _, testCase := range tests {
		if got := agentPipe(testCase.authSock); got != testCase.want {
			t.Errorf("agentPipe(%q) = %q, want %q", testCase.authSock, got, testCase.want)
		}
	}
}

// The agent Windows ships, reached the way a connection reaches it. CI starts
// the service and sets MSSH_TEST_WINDOWS_AGENT; on a machine without the
// service running, this has nothing to talk to.
func TestWindowsAgentAnswers(t *testing.T) {
	if os.Getenv("MSSH_TEST_WINDOWS_AGENT") == "" {
		t.Skip("set MSSH_TEST_WINDOWS_AGENT=1 with the OpenSSH Authentication Agent running")
	}
	t.Setenv("SSH_AUTH_SOCK", "")

	connection, err := dialAgent()
	if err != nil {
		t.Fatalf("dialAgent: %v", err)
	}
	defer func() { _ = connection.Close() }()

	keys, err := agent.NewClient(connection).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	t.Logf("the agent holds %d keys", len(keys))
}
