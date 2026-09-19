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
			// The case that started this. sshd answers a connection it will not
			// take with the words "Exceeded MaxStartups" where the version
			// banner belongs; x/crypto is reading for a line starting "SSH-",
			// so all that survives is the word EOF.
			name:    "bare EOF",
			err:     fmt.Errorf("ssh: handshake failed: %w", io.EOF),
			wantAll: []string{"MaxStartups", "before the SSH handshake started"},
		},
		{
			name:    "connection reset is the same story",
			err:     errors.New("read tcp 1.2.3.4:5->10.0.0.9:2298: read: connection reset by peer"),
			wantAll: []string{"MaxStartups", "2298"},
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

func TestDroppedBeforeAuth(t *testing.T) {
	// The whole point of this function is deciding what may be retried, so the
	// two halves are asserted together: anything mistakenly called a pre-auth
	// drop gets the credentials replayed.
	retryable := []error{
		io.EOF,
		fmt.Errorf("ssh: handshake failed: %w", io.EOF),
		errors.New("read: connection reset by peer"),
		errors.New("write: broken pipe"),
		errors.New("Exceeded MaxStartups"),
	}
	for _, err := range retryable {
		if !droppedBeforeAuth(err) {
			t.Errorf("droppedBeforeAuth(%v) = false, want true", err)
		}
	}

	notRetryable := []error{
		nil,
		errors.New("ssh: unable to authenticate, attempted methods [none password]"),
		errors.New("ssh: no common algorithm for key exchange"),
		errors.New("knownhosts: key mismatch"),
		errors.New("read tcp: i/o timeout"),
	}
	for _, err := range notRetryable {
		if droppedBeforeAuth(err) {
			t.Errorf("droppedBeforeAuth(%v) = true, want false", err)
		}
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
