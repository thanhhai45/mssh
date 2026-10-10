package store

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Export formats for the session log.
const (
	ExportCSV  = "csv"
	ExportJSON = "json"
)

// exportedSession is one row as it leaves the app. Both formats carry the same
// fields under the same names, in the same order, so a spreadsheet column and a
// JSON key never disagree about what they are called.
//
// Times are RFC 3339 in UTC rather than epoch seconds: an export is read by
// people and by tools that are not mssh, and neither should have to convert.
// Empty means unknown — still open, or interrupted.
type exportedSession struct {
	ID                   string `json:"id"`
	OpenedAt             string `json:"opened_at"`
	ClosedAt             string `json:"closed_at"`
	DurationSeconds      string `json:"duration_seconds"`
	WorkspaceName        string `json:"workspace_name"`
	ConnectionName       string `json:"connection_name"`
	Kind                 string `json:"kind"`
	Target               string `json:"target"`
	Username             string `json:"username"`
	AWSProfile           string `json:"aws_profile"`
	AWSRegion            string `json:"aws_region"`
	AWSCredentialsSource string `json:"aws_credentials_source"`
	EndReason            string `json:"end_reason"`
	EndMessage           string `json:"end_message"`
	ReconnectOf          string `json:"reconnect_of"`
	ConnectionID         string `json:"connection_id"`
	WorkspaceID          string `json:"workspace_id"`
}

var exportHeader = []string{
	"id", "opened_at", "closed_at", "duration_seconds", "workspace_name",
	"connection_name", "kind", "target", "username", "aws_profile", "aws_region",
	"aws_credentials_source", "end_reason", "end_message", "reconnect_of",
	"connection_id", "workspace_id",
}

func (row exportedSession) values() []string {
	return []string{
		row.ID, row.OpenedAt, row.ClosedAt, row.DurationSeconds, row.WorkspaceName,
		row.ConnectionName, row.Kind, row.Target, row.Username, row.AWSProfile, row.AWSRegion,
		row.AWSCredentialsSource, row.EndReason, row.EndMessage, row.ReconnectOf,
		row.ConnectionID, row.WorkspaceID,
	}
}

func exportTime(epoch int64) string {
	if epoch == 0 {
		return ""
	}
	return time.Unix(epoch, 0).UTC().Format(time.RFC3339)
}

func exported(entry SessionLogEntry) exportedSession {
	duration := ""
	if entry.ClosedAt > 0 {
		duration = strconv.FormatInt(entry.ClosedAt-entry.OpenedAt, 10)
	}
	return exportedSession{
		ID:                   entry.ID,
		OpenedAt:             exportTime(entry.OpenedAt),
		ClosedAt:             exportTime(entry.ClosedAt),
		DurationSeconds:      duration,
		WorkspaceName:        entry.WorkspaceName,
		ConnectionName:       entry.ConnectionName,
		Kind:                 entry.Kind,
		Target:               entry.Target,
		Username:             entry.Username,
		AWSProfile:           entry.AWSProfile,
		AWSRegion:            entry.AWSRegion,
		AWSCredentialsSource: entry.AWSCredentialsSource,
		EndReason:            entry.EndReason,
		EndMessage:           entry.EndMessage,
		ReconnectOf:          entry.ReconnectOf,
		ConnectionID:         entry.ConnectionID,
		WorkspaceID:          entry.WorkspaceID,
	}
}

// spreadsheetSafe stops a cell from being read as a formula.
//
// A CSV is opened in Excel or Numbers, which treat a cell starting with =, +,
// - or @ as something to evaluate. Connection names are typed by people, so a
// name like =HYPERLINK(...) would run when somebody else opened the export.
// Prefixing a quote is the OWASP remedy: the spreadsheet shows the text as is.
func spreadsheetSafe(cell string) string {
	if cell == "" {
		return cell
	}
	switch cell[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + cell
	}
	return cell
}

// WriteSessionLogCSV writes entries as CSV with a header row.
func WriteSessionLogCSV(out io.Writer, entries []SessionLogEntry) error {
	writer := csv.NewWriter(out)
	if err := writer.Write(exportHeader); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}
	for _, entry := range entries {
		cells := exported(entry).values()
		for index := range cells {
			cells[index] = spreadsheetSafe(cells[index])
		}
		if err := writer.Write(cells); err != nil {
			return fmt.Errorf("write csv: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("write csv: %w", err)
	}
	return nil
}

// WriteSessionLogJSON writes entries as a JSON array, indented for reading.
// No formula guard here: JSON is not opened by a spreadsheet.
func WriteSessionLogJSON(out io.Writer, entries []SessionLogEntry) error {
	rows := make([]exportedSession, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, exported(entry))
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(rows); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

// WriteSessionLog writes in the named format.
func WriteSessionLog(out io.Writer, format string, entries []SessionLogEntry) error {
	switch strings.ToLower(format) {
	case ExportCSV:
		return WriteSessionLogCSV(out, entries)
	case ExportJSON:
		return WriteSessionLogJSON(out, entries)
	default:
		return fmt.Errorf("unknown export format %q", format)
	}
}
