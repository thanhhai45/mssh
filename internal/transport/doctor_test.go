package transport

import (
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
