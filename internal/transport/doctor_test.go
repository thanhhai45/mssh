package transport

import (
	"os"
	"path/filepath"
	"slices"
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
			// The claim worth pinning is the one that would become a lie if the
			// feature behind it were removed: that the workspace can import
			// keys from the shell.
			name:    "missing credentials point at importing from the shell",
			profile: "",
			output:  "Unable to locate credentials. You can configure credentials by running...",
			mustSay: "Import from shell",
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

func TestAWSEnvironment(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"AWS_PROFILE=inherited",
		"AWS_DEFAULT_PROFILE=inherited",
		"AWS_ACCESS_KEY_ID=OLDKEY",
		"AWS_SECURITY_TOKEN=stale",
		"AWS_REGION=eu-west-1",
	}

	if got := awsEnvironment(base, nil, ""); !slices.Equal(got, base) {
		t.Errorf("no credentials and no region changed the environment: %v", got)
	}

	got := awsEnvironment(base, &AWSCredentials{
		AccessKeyID: "AKIANEW", SecretAccessKey: "wJalrNEW",
	}, "")

	count := map[string]int{}
	values := map[string]string{}
	for _, variable := range got {
		name, value, _ := strings.Cut(variable, "=")
		count[name]++
		values[name] = value
	}

	// Replaced, not shadowed: exactly one of each, and the new one.
	if count["AWS_ACCESS_KEY_ID"] != 1 || values["AWS_ACCESS_KEY_ID"] != "AKIANEW" {
		t.Errorf("AWS_ACCESS_KEY_ID: %d of them, value %q",
			count["AWS_ACCESS_KEY_ID"], values["AWS_ACCESS_KEY_ID"])
	}
	// Each of these would make the CLI use something other than the stored keys.
	for _, name := range []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_SECURITY_TOKEN", "AWS_SESSION_TOKEN"} {
		if count[name] != 0 {
			t.Errorf("%s survived; it could outrank or taint the stored keys", name)
		}
	}
	// Everything unrelated to credentials is untouched, and with no region
	// given, the inherited one stays.
	if values["PATH"] != "/usr/bin" || values["AWS_REGION"] != "eu-west-1" {
		t.Errorf("unrelated variables were lost: %v", got)
	}

	withToken := awsEnvironment(nil, &AWSCredentials{
		AccessKeyID: "A", SecretAccessKey: "S", SessionToken: "T",
	}, "")
	if !slices.Contains(withToken, "AWS_SESSION_TOKEN=T") {
		t.Errorf("the session token was not passed on: %v", withToken)
	}
}

func TestEffectiveProfile(t *testing.T) {
	cli := Config{AWSProfile: "prod"}
	stored := Config{AWSProfile: "prod", AWSCredentials: &AWSCredentials{AccessKeyID: "AKIA"}}

	if got := effectiveProfile(cli); got != "prod" {
		t.Errorf("without stored keys: %q, want the profile to pass through", got)
	}
	if got := effectiveProfile(stored); got != "" {
		t.Errorf("with stored keys: %q, want none — --profile would outrank them", got)
	}
}

func TestExplainStoredKeysFailure(t *testing.T) {
	tests := []struct {
		output  string
		mustSay string
	}{
		{"An error occurred (ExpiredToken) when calling the GetCallerIdentity operation: The security token included in the request is expired", "expired"},
		{`You must specify a region. You can also configure your region by running "aws configure".`, "no region"},
		{"An error occurred (InvalidClientTokenId) when calling the GetCallerIdentity operation: The security token included in the request is invalid.", "InvalidClientTokenId"},
	}
	for _, testCase := range tests {
		got := explainStoredKeysFailure(testCase.output)
		if !strings.Contains(got, testCase.mustSay) {
			t.Errorf("explanation %q does not mention %q", got, testCase.mustSay)
		}
		if strings.Contains(got, "sso login") {
			t.Errorf("explanation %q sends a stored-keys user to aws sso login", got)
		}
	}
}

// The ssh-config kind cannot pass --region: its aws runs inside the user's
// ProxyCommand. The environment is the only way the workspace's region gets
// there, with stored keys or without.
func TestAWSEnvironmentCarriesTheRegion(t *testing.T) {
	base := []string{"PATH=/usr/bin", "AWS_REGION=inherited", "AWS_DEFAULT_REGION=inherited"}

	for _, testCase := range []struct {
		name        string
		credentials *AWSCredentials
	}{
		{"with stored keys", &AWSCredentials{AccessKeyID: "AKIA", SecretAccessKey: "S"}},
		{"with the CLI's own credentials", nil},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := awsEnvironment(base, testCase.credentials, "ap-southeast-1")

			count := map[string]int{}
			values := map[string]string{}
			for _, variable := range got {
				name, value, _ := strings.Cut(variable, "=")
				count[name]++
				values[name] = value
			}
			for _, name := range []string{"AWS_REGION", "AWS_DEFAULT_REGION"} {
				if count[name] != 1 || values[name] != "ap-southeast-1" {
					t.Errorf("%s: %d of them, value %q; want exactly one, ap-southeast-1",
						name, count[name], values[name])
				}
			}
			if values["PATH"] != "/usr/bin" {
				t.Error("PATH was lost")
			}
			if (testCase.credentials == nil) != (count["AWS_ACCESS_KEY_ID"] == 0) {
				t.Errorf("keys present = %v, want %v", count["AWS_ACCESS_KEY_ID"] != 0, testCase.credentials != nil)
			}
		})
	}
}
