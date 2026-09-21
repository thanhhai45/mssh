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

func TestMergePath(t *testing.T) {
	tests := []struct {
		name    string
		primary string
		extra   string
		want    string
	}{
		{
			name:    "the first argument keeps its order",
			primary: "/opt/homebrew/bin:/usr/bin",
			extra:   "/bin",
			want:    "/opt/homebrew/bin:/usr/bin:/bin",
		},
		{
			name:    "a directory in both appears once",
			primary: "/usr/bin:/bin",
			extra:   "/bin:/sbin",
			want:    "/usr/bin:/bin:/sbin",
		},
		{
			// The old code asked strings.Contains(path, "/usr/bin"), which is
			// true of "/opt/usr/bin" — so a real /usr/bin was never added.
			// Whole entries are compared now.
			name:    "a directory is not confused with one it is a substring of",
			primary: "/opt/usr/bin:/usr/bin-old",
			extra:   "/usr/bin",
			want:    "/opt/usr/bin:/usr/bin-old:/usr/bin",
		},
		{
			name:    "empty entries are dropped",
			primary: "/usr/bin::",
			extra:   ":/bin",
			want:    "/usr/bin:/bin",
		},
		{
			name:    "an empty side changes nothing",
			primary: "/usr/bin",
			extra:   "",
			want:    "/usr/bin",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := mergePath(testCase.primary, testCase.extra); got != testCase.want {
				t.Errorf("mergePath = %q, want %q", got, testCase.want)
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
			// Not "workspace": that word survived the rewrite of this message
			// and so proved nothing. The claim worth pinning is the one that
			// would be a lie if adoptLoginShellEnvironment were ever removed.
			name:    "missing credentials say the shell environment was already read",
			profile: "",
			output:  "Unable to locate credentials. You can configure credentials by running...",
			mustSay: "login shell",
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
