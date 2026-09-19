package transport

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestExplainHandshakeFailure(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantAll []string
	}{
		{
			// The case that started this: x/crypto reports a server hanging up
			// as the bare word EOF, which tells the reader nothing.
			name:    "bare EOF",
			err:     fmt.Errorf("ssh: handshake failed: %w", io.EOF),
			wantAll: []string{"hung up", "rate limiting", "ssh -p 2298 10.0.0.9"},
		},
		{
			name:    "connection reset is the same story",
			err:     errors.New("read tcp 1.2.3.4:5->10.0.0.9:2298: read: connection reset by peer"),
			wantAll: []string{"hung up", "fail2ban"},
		},
		{
			name: "credentials refused",
			err: errors.New(
				"ssh: unable to authenticate, attempted methods [none password], " +
					"no supported methods remain"),
			wantAll: []string{"refused the credentials", "capitalise", "attempted methods"},
		},
		{
			name:    "algorithm mismatch points at the other kind",
			err:     errors.New("ssh: no common algorithm for key exchange"),
			wantAll: []string{"System SSH"},
		},
		{
			// Anything unrecognised must still say what went wrong rather than
			// being swallowed by a friendly sentence that fits nothing.
			name:    "unknown errors keep their text",
			err:     errors.New("something nobody has seen before"),
			wantAll: []string{"something nobody has seen before", "10.0.0.9:2298"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := explainHandshakeFailure("10.0.0.9:2298", test.err)

			for _, want := range test.wantAll {
				if !strings.Contains(got, want) {
					t.Errorf("explanation is missing %q:\n%s", want, got)
				}
			}
		})
	}
}

func TestPortOf(t *testing.T) {
	if got := portOf("10.0.0.9:2298"); got != "2298" {
		t.Errorf("portOf = %q, want 2298", got)
	}
	// A malformed address must not produce a command that cannot be pasted.
	if got := portOf("10.0.0.9"); got != "22" {
		t.Errorf("portOf without a port = %q, want the 22 fallback", got)
	}
}
