package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type ConnectionKind string

const (
	KindSSH    ConnectionKind = "ssh"
	KindSSM    ConnectionKind = "ssm"
	KindSSMSSH ConnectionKind = "ssm-ssh"
)

type AuthMethod string

const (
	AuthAgent    AuthMethod = "agent"
	AuthKey      AuthMethod = "key"
	AuthPassword AuthMethod = "password"
)

func (k ConnectionKind) Valid() bool {
	switch k {
	case KindSSH, KindSSM, KindSSMSSH:
		return true
	default:
		return false
	}
}

func (k ConnectionKind) UsesSSH() bool {
	return k == KindSSH || k == KindSSMSSH
}

func (k ConnectionKind) UsesAWS() bool {
	return k == KindSSM || k == KindSSMSSH
}

// NeedsPassword reports whether opening a session for this connection requires
// a password, from the keychain or from the user.
func (c Connection) NeedsPassword() bool {
	return c.Kind.UsesSSH() && c.AuthMethod == AuthPassword
}

type Connection struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspaceId"`
	Name        string         `json:"name"`
	Kind        ConnectionKind `json:"kind"`
	Target      string         `json:"target"`
	Port        int            `json:"port"`
	Username    string         `json:"username"`
	AuthMethod  AuthMethod     `json:"authMethod"`
	KeyPath     string         `json:"keyPath"`
	AWSProfile  string         `json:"awsProfile"`
	AWSRegion   string         `json:"awsRegion"`
	Extra       string         `json:"extra"`
	Color       string         `json:"color"`
	SortOrder   int            `json:"sortOrder"`
	CreatedAt   int64          `json:"createdAt"`
	UpdatedAt   int64          `json:"updatedAt"`
}

type ConnectionInput struct {
	Name       string         `json:"name"`
	Kind       ConnectionKind `json:"kind"`
	Target     string         `json:"target"`
	Port       int            `json:"port"`
	Username   string         `json:"username"`
	AuthMethod AuthMethod     `json:"authMethod"`
	KeyPath    string         `json:"keyPath"`
	AWSProfile string         `json:"awsProfile"`
	AWSRegion  string         `json:"awsRegion"`
	Extra      string         `json:"extra"`
	Color      string         `json:"color"`
}

const connectionColumns = `id, workspace_id, name, kind, target, port, username,` +
	`auth_method, key_path, aws_profile, aws_region, extra,` +
	`color, sort_order, created_at, updated_at`

// instanceIDPattern matches EC2 (i-…) and managed (mi-…) instance ids.
// Compiled once at package load, not on every validation.
var instanceIDPattern = regexp.MustCompile(`^(i|mi)-[0-9a-f]{8,}$`)

func (input ConnectionInput) normalize() (ConnectionInput, error) {
	out := input

	out.Name = strings.TrimSpace(out.Name)
	out.Target = strings.TrimSpace(out.Target)
	out.Color = strings.TrimSpace(out.Color)

	if out.Extra == "" {
		out.Extra = "{}"
	}

	if !out.Kind.Valid() {
		return out, fmt.Errorf("unknown connection kind %q", out.Kind)
	}
	if out.Name == "" {
		return out, fmt.Errorf("connection name is required")
	}
	if out.Target == "" {
		return out, fmt.Errorf("connection target is required")
	}

	if out.Kind.UsesSSH() {
		out.Username = strings.TrimSpace(out.Username)
		out.KeyPath = strings.TrimSpace(out.KeyPath)

		if out.Port == 0 {
			out.Port = 22
		}
		if out.Port < 1 || out.Port > 65535 {
			return out, fmt.Errorf("connection port %d must be between 1 and 65535", out.Port)
		}
		if out.Username == "" {
			return out, fmt.Errorf("%s connections need a username", out.Kind)
		}
		if out.AuthMethod == "" {
			out.AuthMethod = AuthAgent
		}

		switch out.AuthMethod {
		case AuthAgent:
			// Nothing else to check: ssh-agent either has a usable key or it
			// does not, and that only shows up at connect time.
			out.KeyPath = ""
		case AuthKey:
			if out.KeyPath == "" {
				return out, fmt.Errorf("key authentication needs a key path")
			}
		case AuthPassword:
			// The password itself is never stored here. Nothing to validate.
			out.KeyPath = ""
		default:
			return out, fmt.Errorf("unknown auth method %q", out.AuthMethod)
		}
	} else {
		// Not an SSH kind: keep the row honest instead of storing values that
		// mean nothing for it.
		out.Port = 0
		out.Username = ""
		out.AuthMethod = ""
		out.KeyPath = ""
	}

	if out.Kind.UsesAWS() {
		out.AWSProfile = strings.TrimSpace(out.AWSProfile)
		out.AWSRegion = strings.TrimSpace(out.AWSRegion)

		if !instanceIDPattern.MatchString(out.Target) {
			return out, fmt.Errorf(
				"%s needs an instance id like i-0abc12345, got %q", out.Kind, out.Target)
		}
	} else {
		out.AWSProfile = ""
		out.AWSRegion = ""
	}

	return out, nil
}

// connectionScanTargets returns Scan destinations in the same order as
// connectionColumns. One definition means List and Get can never drift apart.
func connectionScanTargets(c *Connection) []any {
	return []any{
		&c.ID, &c.WorkspaceID, &c.Name, &c.Kind, &c.Target,
		&c.Port, &c.Username, &c.AuthMethod, &c.KeyPath,
		&c.AWSProfile, &c.AWSRegion, &c.Extra, &c.Color,
		&c.SortOrder, &c.CreatedAt, &c.UpdatedAt,
	}
}

// connectionValues returns the column values in the same order as
// connectionColumns, for INSERT.
func connectionValues(c Connection) []any {
	return []any{
		c.ID, c.WorkspaceID, c.Name, c.Kind, c.Target,
		c.Port, c.Username, c.AuthMethod, c.KeyPath,
		c.AWSProfile, c.AWSRegion, c.Extra, c.Color,
		c.SortOrder, c.CreatedAt, c.UpdatedAt,
	}
}

func (s *Store) ListConnections(workspaceID string) ([]Connection, error) {
	rows, err := s.db.Query(
		`SELECT `+connectionColumns+`
		FROM connections WHERE workspace_id = ?
		ORDER BY sort_order, name`, workspaceID)

	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		var c Connection
		if err := rows.Scan(connectionScanTargets(&c)...); err != nil {
			return nil, fmt.Errorf("list connections: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	return out, nil
}

func (s *Store) GetConnection(id string) (Connection, error) {
	var c Connection
	err := s.db.QueryRow(
		`SELECT `+connectionColumns+` FROM connections WHERE id = ?`, id,
	).Scan(connectionScanTargets(&c)...)

	if errors.Is(err, sql.ErrNoRows) {
		return Connection{}, fmt.Errorf("connection %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return Connection{}, fmt.Errorf("get connection: %w", err)
	}
	return c, nil
}

func (s *Store) CreateConnection(workspaceID string, input ConnectionInput) (Connection, error) {
	in, err := input.normalize()
	if err != nil {
		return Connection{}, fmt.Errorf("create connection: %w", err)
	}

	if _, err := s.GetWorkspace(workspaceID); err != nil {
		return Connection{}, fmt.Errorf("create connection: %w", err)
	}

	now := s.now()
	c := Connection{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        in.Name,
		Kind:        in.Kind,
		Target:      in.Target,
		Port:        in.Port,
		Username:    in.Username,
		AuthMethod:  in.AuthMethod,
		KeyPath:     in.KeyPath,
		AWSProfile:  in.AWSProfile,
		AWSRegion:   in.AWSRegion,
		Extra:       in.Extra,
		Color:       in.Color,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.db.QueryRow(
		`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM connections WHERE workspace_id = ?`,
		workspaceID,
	).Scan(&c.SortOrder); err != nil {
		return Connection{}, fmt.Errorf("create connection: %w", err)
	}

	if _, err := s.db.Exec(
		`INSERT INTO connections (`+connectionColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		connectionValues(c)...,
	); err != nil {
		return Connection{}, fmt.Errorf("create connection: %w", err)
	}

	return c, nil
}

func (s *Store) UpdateConnection(id string, input ConnectionInput) (Connection, error) {
	in, err := input.normalize()
	if err != nil {
		return Connection{}, fmt.Errorf("update connection: %w", err)
	}

	res, err := s.db.Exec(
		`Update connections
			SET name = ?, kind = ?, target = ?, port = ?, username = ?,
					auth_method = ?, key_path = ?, aws_profile = ?, aws_region = ?,
					extra = ?, color = ?, updated_at = ?
			WHERE id = ?`,
		in.Name, in.Kind, in.Target, in.Port, in.Username,
		in.AuthMethod, in.KeyPath, in.AWSProfile, in.AWSRegion,
		in.Extra, in.Color, s.now(), id,
	)
	if err != nil {
		return Connection{}, fmt.Errorf("update connection: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return Connection{}, fmt.Errorf("update connection: %w", err)
	}
	if n == 0 {
		return Connection{}, fmt.Errorf("connection %s: %w", id, ErrNotFound)
	}

	return s.GetConnection(id)
}

func (s *Store) DeleteConnection(id string) error {
	res, err := s.db.Exec(`DELETE FROM connections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("connection %s: %w", id, ErrNotFound)
	}

	return nil
}

func (s *Store) MoveConnection(id string, toWorkspaceID string) error {
	if _, err := s.GetWorkspace(toWorkspaceID); err != nil {
		return fmt.Errorf("move connection: %w", err)
	}

	var sortOrder int
	if err := s.db.QueryRow(
		`SELECT COALESCE(MAX(sort_order) + 1, 0) FROM connections WHERE workspace_id = ?`, toWorkspaceID,
	).Scan(&sortOrder); err != nil {
		return fmt.Errorf("move connection: %w", err)
	}

	res, err := s.db.Exec(
		`UPDATE connections SET workspace_id = ?, sort_order = ?, updated_at = ? WHERE id = ?`,
		toWorkspaceID, sortOrder, s.now(), id,
	)
	if err != nil {
		return fmt.Errorf("move connection: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("move connection: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("connection %s: %w", id, ErrNotFound)
	}

	return nil
}

// ResolvedAWS is the profile/region pair a transport should actually use.
// Empty fields mean "let the AWS CLI fall back to its own configuration".
type ResolvedAWS struct {
	Profile string `json:"profile"`
	Region  string `json:"region"`
}

// ResolveAWS applies the three-tier rule: the connection wins, then the
// workspace, then nothing at all.
func ResolveAWS(c Connection, w Workspace) ResolvedAWS {
	out := ResolvedAWS{Profile: c.AWSProfile, Region: c.AWSRegion}
	if out.Profile == "" {
		out.Profile = w.AWSProfile
	}
	if out.Region == "" {
		out.Region = w.AWSRegion
	}
	return out
}

// ResolveAWSForConnection looks the two rows up and applies ResolveAWS.
func (s *Store) ResolveAWSForConnection(connectionID string) (ResolvedAWS, error) {
	c, err := s.GetConnection(connectionID)
	if err != nil {
		return ResolvedAWS{}, err
	}
	w, err := s.GetWorkspace(c.WorkspaceID)
	if err != nil {
		return ResolvedAWS{}, err
	}
	return ResolveAWS(c, w), nil
}

// ParsedSSHCommand holds what could be recovered from a command line. Fields
// that were not present keep their zero value.
type ParsedSSHCommand struct {
	Username string `json:"username"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	KeyPath  string `json:"keyPath"`
}

// ParseSSHCommand reads a command such as `ssh -p 2222 deploy@10.0.4.12` so the
// UI can prefill a form. It is a convenience only: what gets stored is always
// the individual fields, never the command string.
func ParseSSHCommand(cmd string) (ParsedSSHCommand, error) {
	var out ParsedSSHCommand

	fields := strings.Fields(cmd)
	if len(fields) > 0 && fields[0] == "ssh" {
		fields = fields[1:]
	}

	// Flags that swallow the token after them.
	takesValue := map[string]bool{
		"-p": true, "-i": true, "-l": true,
		"-o": true, "-J": true, "-F": true, "-b": true,
	}

	for i := 0; i < len(fields); i++ {
		token := fields[i]

		if takesValue[token] {
			if i+1 >= len(fields) {
				return out, fmt.Errorf("flag %s has no value", token)
			}
			value := fields[i+1]
			i++

			switch token {
			case "-p":
				port, err := strconv.Atoi(value)
				if err != nil {
					return out, fmt.Errorf("invalid port %q", value)
				}
				out.Port = port
			case "-i":
				out.KeyPath = value
			case "-l":
				out.Username = value
			}
			continue
		}

		if strings.HasPrefix(token, "-") {
			continue // a flag we do not care about
		}

		if out.Host != "" {
			continue // the first bare token is the destination; the rest is a remote command
		}
		if user, host, found := strings.Cut(token, "@"); found {
			out.Username, out.Host = user, host
		} else {
			out.Host = token
		}
	}

	if out.Host == "" {
		return out, fmt.Errorf("no host found in %q", cmd)
	}
	if out.Port == 0 {
		out.Port = 22
	}
	return out, nil
}
