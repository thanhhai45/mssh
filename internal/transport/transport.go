package transport

import (
	"context"
	"errors"
	"fmt"
)

const (
	KindSSH    = "ssh"
	KindSSM    = "ssm"
	KindSSMSSH = "ssm-ssh"
)

const (
	AuthAgent    = "agent"
	AuthKey      = "key"
	AuthPassword = "password"
)

var ErrPasswordRequired = errors.New("password required")

// Size is the terminal geometry a session starts with, in character cells.
type Size struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// Config is everything a dialer needs, already resolved. No database lookups,
// no inheritance rules and no keychain access happen inside this package.
type Config struct {
	Kind       string
	Target     string // Target is a hostname or IP for KindSSH, an instance id for the SSM kinds.
	Port       int
	Username   string
	AuthMethod string
	KeyPath    string
	Password   string
	AWSProfile string
	AWSRegion  string
	// Extra carries rarely-used options as JSON, straight from the
	// connection's extra column. Nothing here is ever queried or validated.
	Extra string
	// KnownHostsPath is where host keys are verified against. Empty means
	// ~/.ssh/known_hosts.
	KnownHostsPath string
}

type Session interface {
	Write(payload []byte) (int, error)
	Resize(size Size) error
	Close() error
}

type Dialer interface {
	// Name is used in error messages.
	Name() string
	// Preflight fails before any network traffic when the machine is missing
	// something the dial needs. Its errors should say what to do about it.
	Preflight(config Config) error
	// Dial opens a session. onOutput is called with each chunk the remote
	// sends; onExit is called once, when the session ends for any reason.
	Dial(
		dialContext context.Context,
		config Config,
		size Size,
		onOutput func([]byte),
		onExit func(error),
	) (Session, error)
}

func For(kind string) (Dialer, error) {
	switch kind {
	case KindSSH:
		return sshDialer{}, nil
	case KindSSM:
		return ssmDialer{}, nil
	case KindSSMSSH:
		return ssmSSHDialer{}, nil
	default:
		return nil, fmt.Errorf("unknown connection kind %q", kind)
	}
}
