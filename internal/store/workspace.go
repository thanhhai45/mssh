package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a lookup by id matches no row. Callers detect
// it with errors.Is, never by comparing error strings.
var ErrNotFound = errors.New("not found")

type Workspace struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	AWSProfile string `json:"awsProfile"`
	AWSRegion  string `json:"awsRegion"`
	SortOrder  int    `json:"sortOrder"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type WorkspaceInput struct {
	Name       string `json:"name"`
	Color      string `json:"color"`
	AWSProfile string `json:"awsProfile"`
	AWSRegion  string `json:"awsRegion"`
}

const workspaceColumns = `id, name, color, aws_profile, aws_region, sort_order, created_at, updated_at`

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
		if err := rows.Scan(
			&ws.ID, &ws.Name, &ws.Color, &ws.AWSProfile, &ws.AWSRegion, &ws.SortOrder, &ws.CreatedAt, &ws.UpdatedAt,
		); err != nil {
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
	).Scan(
		&ws.ID, &ws.Name, &ws.Color, &ws.AWSProfile, &ws.AWSRegion, &ws.SortOrder, &ws.CreatedAt, &ws.UpdatedAt,
	)

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

	now := time.Now().Unix()
	ws := Workspace{
		ID:         uuid.NewString(),
		Name:       name,
		Color:      color,
		AWSProfile: strings.TrimSpace(input.AWSProfile),
		AWSRegion:  strings.TrimSpace(input.AWSRegion),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.db.QueryRow(
		`SELECT COALESCE(MAX(sort_order) +1 , 0) FROM workspaces`,
	).Scan(&ws.SortOrder); err != nil {
		return Workspace{}, fmt.Errorf("get next sort order: %w", err)
	}

	if _, err := s.db.Exec(
		`INSERT INTO workspaces (`+workspaceColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ws.ID, ws.Name, ws.Color, ws.AWSProfile, ws.AWSRegion, ws.SortOrder,
		ws.CreatedAt, ws.UpdatedAt,
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

	res, err := s.db.Exec(
		`UPDATE workspaces SET name = ?, color = ?, aws_profile = ?, aws_region = ?, updated_at = ? WHERE id = ?`,
		name, color, strings.TrimSpace(input.AWSProfile), strings.TrimSpace(input.AWSRegion), time.Now().Unix(), id,
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
	now := time.Now().Unix()
	for i, id := range ids {
		if _, err := stmt.Exec(i, now, id); err != nil {
			return fmt.Errorf("reorder workspace %s: %w", id, err)
		}
	}

	return tx.Commit()
}
