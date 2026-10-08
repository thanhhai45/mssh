package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a lookup by id matches no row. Callers detect
// it with errors.Is, never by comparing error strings.
var ErrNotFound = errors.New("not found")

// Where a workspace's AWS credentials come from. Must match
// AWSCredentialsSource in frontend/src/lib/api.ts.
const (
	// The AWS CLI finds its own: ~/.aws, SSO, the profile below. How every
	// workspace worked before stored keys existed, and the default.
	AWSCredentialsFromCLI = "cli"
	// Keys held by mssh: the id here, the secret in workspace_secrets.
	AWSCredentialsStored = "stored"
)

type Workspace struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Color                string `json:"color"`
	AWSProfile           string `json:"awsProfile"`
	AWSRegion            string `json:"awsRegion"`
	AWSCredentialsSource string `json:"awsCredentialsSource"`
	AWSAccessKeyID       string `json:"awsAccessKeyId"`
	SortOrder            int    `json:"sortOrder"`
	CreatedAt            int64  `json:"createdAt"`
	UpdatedAt            int64  `json:"updatedAt"`
}

type WorkspaceInput struct {
	Name                 string `json:"name"`
	Color                string `json:"color"`
	AWSProfile           string `json:"awsProfile"`
	AWSRegion            string `json:"awsRegion"`
	AWSCredentialsSource string `json:"awsCredentialsSource"`
	AWSAccessKeyID       string `json:"awsAccessKeyId"`
}

const workspaceColumns = `id, name, color, aws_profile, aws_region, aws_credentials_source, aws_access_key_id, sort_order, created_at, updated_at`

// workspacePlaceholders s "?, ?, ...", one per column in workspaceColumns -
// derived the same way as connectionPlaceholders, for the same reason
var workspacePlaceholders = strings.TrimSuffix(
	strings.Repeat("?, ", strings.Count(workspaceColumns, ",")+1), ", ")

// workspaceScanTargets returns Scan destinations in the same order as
// workspaceColumns. One definition means List and Get can never drift apart.
func workspaceScanTargets(ws *Workspace) []any {
	return []any{
		&ws.ID, &ws.Name, &ws.Color, &ws.AWSProfile, &ws.AWSRegion,
		&ws.AWSCredentialsSource, &ws.AWSAccessKeyID,
		&ws.SortOrder, &ws.CreatedAt, &ws.UpdatedAt,
	}
}

// workspaceValues returns the column values in the same order as
// workspaceColumns, for INSERT.
func workspaceValues(ws Workspace) []any {
	return []any{
		ws.ID, ws.Name, ws.Color, ws.AWSProfile, ws.AWSRegion,
		ws.AWSCredentialsSource, ws.AWSAccessKeyID,
		ws.SortOrder, ws.CreatedAt, ws.UpdatedAt,
	}
}

// normalizeCredentialsSource maps "" to the CLI default and refuses anything
// else unknow. The column has no CHECK (see migration 005), so this is the
// only thing staging between a typo and a workspace in neither mode.
func normalizeCredentialsSource(source string) (string, error) {
	switch strings.TrimSpace(source) {
	case "", AWSCredentialsFromCLI:
		return AWSCredentialsFromCLI, nil
	case AWSCredentialsStored:
		return AWSCredentialsStored, nil
	default:
		return "", fmt.Errorf("unknown AWS credentials source %q", source)
	}
}

func (s *Store) ListWorkspaces() ([]Workspace, error) {
	rows, err := s.db.Query(
		`SELECT ` + workspaceColumns + ` FROM workspaces ORDER BY sort_order, name`,
	)

	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	defer rows.Close()

	out := []Workspace{}

	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(workspaceScanTargets(&ws)...); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		out = append(out, ws)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}
	return out, nil
}

func (s *Store) GetWorkspace(id string) (Workspace, error) {
	var ws Workspace
	err := s.db.QueryRow(
		`SELECT `+workspaceColumns+` FROM workspaces WHERE id = ?`, id,
	).Scan(workspaceScanTargets(&ws)...)

	if errors.Is(err, sql.ErrNoRows) {
		return Workspace{}, fmt.Errorf("workspace %s: %w", id, ErrNotFound)
	}

	if err != nil {
		return Workspace{}, fmt.Errorf("get workspace: %w", err)
	}

	return ws, nil
}

func (s *Store) CreateWorkspace(input WorkspaceInput) (Workspace, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Workspace{}, fmt.Errorf("workspace name cannot be empty")
	}

	color := strings.TrimSpace(input.Color)
	if color == "" {
		color = "slate"
	}

	source, err := normalizeCredentialsSource(input.AWSCredentialsSource)
	if err != nil {
		return Workspace{}, err
	}

	now := s.now()
	ws := Workspace{
		ID:                   uuid.NewString(),
		Name:                 name,
		Color:                color,
		AWSProfile:           strings.TrimSpace(input.AWSProfile),
		AWSRegion:            strings.TrimSpace(input.AWSRegion),
		AWSCredentialsSource: source,
		AWSAccessKeyID:       strings.TrimSpace(input.AWSAccessKeyID),
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := s.db.QueryRow(
		`SELECT COALESCE(MAX(sort_order) +1 , 0) FROM workspaces`,
	).Scan(&ws.SortOrder); err != nil {
		return Workspace{}, fmt.Errorf("get next sort order: %w", err)
	}

	if _, err := s.db.Exec(
		`INSERT INTO workspaces (`+workspaceColumns+`) VALUES (`+workspacePlaceholders+`)`,
		workspaceValues(ws)...,
	); err != nil {
		return Workspace{}, fmt.Errorf("insert workspace: %w", err)
	}

	return ws, nil
}

func (s *Store) UpdateWorkspace(id string, input WorkspaceInput) (Workspace, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Workspace{}, fmt.Errorf("workspace name cannot be empty")
	}

	color := strings.TrimSpace(input.Color)
	if color == "" {
		color = "slate"
	}

	source, err := normalizeCredentialsSource(input.AWSCredentialsSource)
	if err != nil {
		return Workspace{}, err
	}

	res, err := s.db.Exec(
		`UPDATE workspaces SET name = ?, color = ?, aws_profile = ?, aws_region = ?,
		aws_credentials_source = ?, aws_access_key_id = ?, updated_at = ?
		WHERE id = ?`,
		name, color, strings.TrimSpace(input.AWSProfile), strings.TrimSpace(input.AWSRegion),
		source, strings.TrimSpace(input.AWSAccessKeyID),
		s.now(), id,
	)

	if err != nil {
		return Workspace{}, fmt.Errorf("update workspace: %w", err)
	}

	n, err := res.RowsAffected()

	if err != nil {
		return Workspace{}, fmt.Errorf("check workspace update: %w", err)
	}

	if n == 0 {
		return Workspace{}, fmt.Errorf("workspace %s: %w", id, ErrNotFound)
	}

	return s.GetWorkspace(id)
}

func (s *Store) DeleteWorkspace(id string) error {
	res, err := s.db.Exec(`DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check workspace delete: %w", err)
	}

	if n == 0 {
		return fmt.Errorf("workspace %s: %w", id, ErrNotFound)
	}

	return nil
}

func (s *Store) ReorderWorkspaces(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin reorder: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE workspaces SET sort_order = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare reorder: %w", err)
	}
	defer stmt.Close()
	now := s.now()
	for i, id := range ids {
		if _, err := stmt.Exec(i, now, id); err != nil {
			return fmt.Errorf("reorder workspace %s: %w", id, err)
		}
	}

	return tx.Commit()
}
