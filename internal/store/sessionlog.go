package store

import (
	"fmt"
	"strings"
)

// Why a session ended. Stored as text so the log stays readable in any SQLite
// browser, and exported as is.
const (
	// The user pressed Disconnect or closed the tab.
	SessionClosedByUser = "closed"
	// The remote side ended it normally: `exit`, or the program finished.
	SessionEnded = "ended"
	// It ended with an error: the network dropped, a timeout, a server reboot.
	SessionFailed = "failed"
	// Its connection was deleted while it was open.
	SessionConnectionDeleted = "connection-deleted"
	// mssh was quit with it still open.
	SessionAppQuit = "app-quit"
	// It was open when mssh last stopped without a chance to say so: a crash,
	// a force quit, the machine losing power. When it really ended is unknown.
	SessionInterrupted = "interrupted"
)

// SessionLogEntry is one row of the log. Everything in it is a copy taken
// when the session opened, so it reads the same however the connection has changed
// since - or whether it still exists at all.
type SessionLogEntry struct {
	ID                   string `json:"id"`
	ConnectionID         string `json:"connectionId"`
	WorkspaceID          string `json:"workspaceId"`
	ConnectionName       string `json:"connectionName"`
	WorkspaceName        string `json:"workspaceName"`
	Kind                 string `json:"kind"`
	Target               string `json:"target"`
	Username             string `json:"username"`
	AWSProfile           string `json:"awsProfile"`
	AWSRegion            string `json:"awsRegion"`
	AWSCredentialsSource string `json:"awsCredentialsSource"`
	OpenedAt             int64  `json:"openedAt"`
	ClosedAt             int64  `json:"closedAt"`
	EndReason            string `json:"endReason"`
	EndMessage           string `json:"endMessage"`
	ReconnectOf          string `json:"reconnectOf"`
}

const sessionLogColumns = `id, connection_id, workspace_id, connection_name, ` +
	`workspace_name, kind, target, username, aws_profile, aws_region, ` +
	`aws_credentials_source, opened_at, closed_at, end_reason, end_message, reconnect_of`

var sessionLogPlaceholders = strings.TrimSuffix(
	strings.Repeat("?, ", strings.Count(sessionLogColumns, ",")+1), ", ")

func sessionLogScanTargets(entry *SessionLogEntry) []any {
	return []any{
		&entry.ID, &entry.ConnectionID, &entry.WorkspaceID, &entry.ConnectionName,
		&entry.WorkspaceName, &entry.Kind, &entry.Target, &entry.Username,
		&entry.AWSProfile, &entry.AWSRegion, &entry.AWSCredentialsSource, &entry.OpenedAt,
		&entry.ClosedAt, &entry.EndReason, &entry.EndMessage, &entry.ReconnectOf,
	}
}

func sessionLogValues(entry SessionLogEntry) []any {
	return []any{
		entry.ID, entry.ConnectionID, entry.WorkspaceID, entry.ConnectionName,
		entry.WorkspaceName, entry.Kind, entry.Target, entry.Username,
		entry.AWSProfile, entry.AWSRegion, entry.AWSCredentialsSource,
		entry.OpenedAt, entry.ClosedAt, entry.EndReason, entry.EndMessage,
		entry.ReconnectOf,
	}
}

// RecordSessionOpened writes a new row, copying the connection and workspace
// as they are right now. The AWS profile and region are the resolved ones -
// what the session actually used, after the connection-then-workspace rule.
// Never a secret: only where the credentials came from.
func (s *Store) RecordSessionOpened(sessionID string, connectionID string) error {
	connection, err := s.GetConnection(connectionID)
	if err != nil {
		return err
	}
	workspace, err := s.GetWorkspace(connection.WorkspaceID)
	if err != nil {
		return err
	}
	resolved := ResolveAWS(connection, workspace)

	entry := SessionLogEntry{
		ID:             sessionID,
		ConnectionID:   connection.ID,
		WorkspaceID:    workspace.ID,
		ConnectionName: connection.Name,
		WorkspaceName:  workspace.Name,
		Kind:           string(connection.Kind),
		Target:         connection.Target,
		Username:       connection.Username,
		OpenedAt:       s.now(),
	}
	if connection.Kind.UsesAWS() {
		entry.AWSProfile = resolved.Profile
		entry.AWSRegion = resolved.Region
		entry.AWSCredentialsSource = workspace.AWSCredentialsSource
	}

	if _, err := s.db.Exec(
		`INSERT INTO session_log (`+sessionLogColumns+`) VALUES (`+sessionLogPlaceholders+`)`,
		sessionLogValues(entry)...,
	); err != nil {
		return fmt.Errorf("record session opened: %w", err)
	}
	return nil
}

// RecordSessionClosed marks a session ended. The first call wins: a session
// the user disconnects is also reported moments later by its transport, and
// "closed by the user" must not be overwritten by the error that closing it caused.
func (s *Store) RecordSessionClosed(sessionID string, reason string, message string) error {
	if _, err := s.db.Exec(
		`UPDATE session_log SET closed_at = ?, end_reason = ?, end_message = ? WHERE id = ? AND end_reason = ''`,
		s.now(), reason, message, sessionID,
	); err != nil {
		return fmt.Errorf("record session closed: %w", err)
	}
	return nil
}

// MarkInterruptedSessions settles rows left open by a previous run that never
// got to close them. Run once at startup, before any session can open.
func (s *Store) MarkInterruptedSessions() (int64, error) {
	result, err := s.db.Exec(
		`UPDATE session_log SET end_reason = ? WHERE end_reason = ''`,
		SessionInterrupted,
	)
	if err != nil {
		return 0, fmt.Errorf("mark interrupted sessions: %w", err)
	}
	return result.RowsAffected()
}

// SessionLogFilter narrows ListSessionLog. Zero values mean "any".
type SessionLogFilter struct {
	WorkspaceID  string `json:"workspaceId"`
	ConnectionID string `json:"connectionId"`
	// From and To bound opened_at, in epoch seconds, inclusive of From and
	// exclusive of To - so "from the start of Monday to the start of Tuesday"
	// is exactly Monday.
	From  int64 `json:"from"`
	To    int64 `json:"to"`
	Limit int   `json:"limit"`
}

// ListSessionLog returns matching rows, newest first.
func (s *Store) ListSessionLog(filter SessionLogFilter) ([]SessionLogEntry, error) {
	var conditions []string
	var arguments []any

	if filter.WorkspaceID != "" {
		conditions = append(conditions, "workspace_id = ?")
		arguments = append(arguments, filter.WorkspaceID)
	}
	if filter.ConnectionID != "" {
		conditions = append(conditions, "connection_id = ?")
		arguments = append(arguments, filter.ConnectionID)
	}
	if filter.From > 0 {
		conditions = append(conditions, "opened_at >= ?")
		arguments = append(arguments, filter.From)
	}
	if filter.To > 0 {
		conditions = append(conditions, "opened_at < ?")
		arguments = append(arguments, filter.To)
	}

	query := `SELECT ` + sessionLogColumns + ` FROM session_log`
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, " AND ")
	}
	query += ` ORDER BY opened_at DESC, id`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		arguments = append(arguments, filter.Limit)
	}

	rows, err := s.db.Query(query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list session log: %w", err)
	}
	defer rows.Close()

	entries := []SessionLogEntry{}
	for rows.Next() {
		var entry SessionLogEntry
		if err := rows.Scan(sessionLogScanTargets(&entry)...); err != nil {
			return nil, fmt.Errorf("list session log: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list session log: %w", err)
	}
	return entries, nil
}
