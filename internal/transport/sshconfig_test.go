package transport

import (
	"strings"
	"testing"
)

func TestExplainSSHCommandFailure(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		mustSay []string
	}{
		{
			// The advice used to be "Finder never reads ~/.zshrc". It does now,
			// so the test asserts the current claim rather than a phrase that
			// happens to survive both versions.
			name: "a ProxyCommand with no AWS credentials points at the shell environment",
			output: "Unable to locate credentials. You can configure credentials by " +
				"running \"aws configure\".",
			mustSay: []string{
				"login shell",
				"aws sts get-caller-identity",
				"~/.aws/credentials",
			},
		},
		{
			name:    "a missing profile is the same problem",
			output:  "The config profile (itviec) could not be found",
			mustSay: []string{"aws configure"},
		},
		{
			name:    "an alias ssh does not know about points at the config file",
			output:  "ssh: Could not resolve hostname prod-web-1: nodename nor servname provided",
			mustSay: []string{"~/.ssh/config", "prod-web-1"},
		},
		{
			name:    "rejected credentials point at User and IdentityFile",
			output:  "itviec-admin@prod-web-1: Permission denied (publickey).",
			mustSay: []string{"User", "IdentityFile"},
		},
		{
			name:    "anything else is passed through untouched",
			output:  "kex_exchange_identification: read: Connection reset by peer",
			mustSay: []string{"kex_exchange_identification"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := explainSSHCommandFailure("prod-web-1", testCase.output)
			for _, phrase := range testCase.mustSay {
				if !strings.Contains(got, phrase) {
					t.Errorf("explanation does not mention %q:\n%s", phrase, got)
				}
			}
		})
	}
}
