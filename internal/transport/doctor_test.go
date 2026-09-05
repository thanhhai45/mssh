package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAWSFlags(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		region  string
		want    []string
	}{
		{"both empty means let the CLI decide", "", "", []string{}},
		{"profile only", "prod", "", []string{"--profile", "prod"}},
		{"region only", "", "ap-southeast-1", []string{"--region", "ap-southeast-1"}},
		{"both", "prod", "ap-southeast-1",
			[]string{"--profile", "prod", "--region", "ap-southeast-1"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := awsFlags(testCase.profile, testCase.region)
			if len(got) != len(testCase.want) {
				t.Fatalf("got %v, want %v", got, testCase.want)
			}
			for index := range got {
				if got[index] != testCase.want[index] {
					t.Errorf("got %v, want %v", got, testCase.want)
					return
				}
			}
		})
	}
}

func TestExplainAWSFailure(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		output  string
		mustSay string
	}{
		{
			name:    "expired sso says how to log in again",
			profile: "prod",
			output:  "Error loading SSO Token: Token has expired and refresh failed",
			mustSay: "aws sso login --profile prod",
		},
		{
			name:    "missing credentials mentions the workspace",
			profile: "",
			output:  "Unable to locate credentials. You can configure credentials by running...",
			mustSay: "workspace",
		},
		{
			name:    "unknown profile names it",
			profile: "staging",
			output:  "The config profile (staging) could not be found",
			mustSay: `"staging"`,
		},
		{
			name:    "anything else is passed through",
			profile: "",
			output:  "some brand new error nobody has seen",
			mustSay: "some brand new error nobody has seen",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := explainAWSFailure(testCase.profile, testCase.output)
			if !strings.Contains(got, testCase.mustSay) {
				t.Errorf("explanation %q does not mention %q", got, testCase.mustSay)
			}
		})
	}
}

func TestExplainSSMFailure(t *testing.T) {
	got := explainSSMFailure("i-0abc12345",
		"An error occurred (TargetNotConnected) when calling the StartSession operation")
	if !strings.Contains(got, "SSM Agent") {
		t.Errorf("explanation %q does not mention the agent", got)
	}

	got = explainSSMFailure("i-0abc12345",
		"An error occurred (AccessDeniedException) when calling the StartSession operation")
	if !strings.Contains(got, "ssm:StartSession") {
		t.Errorf("explanation %q does not mention the missing permission", got)
	}
}

func TestExpandHome(t *testing.T) {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory on this machine")
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"tilde is expanded", "~/.ssh/id_ed25519", filepath.Join(homeDirectory, ".ssh/id_ed25519")},
		{"absolute paths are left alone", "/etc/ssh/key", "/etc/ssh/key"},
		{"relative paths are left alone", "keys/id_ed25519", "keys/id_ed25519"},
		{"a bare tilde is not a home path", "~weird", "~weird"},
		{"empty stays empty", "", ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := expandHome(testCase.in); got != testCase.want {
				t.Errorf("expandHome(%q) = %q, want %q", testCase.in, got, testCase.want)
			}
		})
	}
}
