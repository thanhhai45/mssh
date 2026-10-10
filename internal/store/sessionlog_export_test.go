package store

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func exportFixture() []SessionLogEntry {
	return []SessionLogEntry{
		{
			ID: "s-closed", ConnectionID: "c1", WorkspaceID: "w1",
			ConnectionName: "web-1", WorkspaceName: "Prod", Kind: "ssm", Target: "i-0abc",
			AWSProfile: "prod", AWSRegion: "ap-southeast-1", AWSCredentialsSource: "cli",
			OpenedAt: 1760083200, ClosedAt: 1760083290, EndReason: SessionFailed,
			EndMessage: "connection reset by peer",
		},
		{
			ID: "s-open", ConnectionID: "c2", WorkspaceID: "w1",
			ConnectionName: "=HYPERLINK(\"http://example.invalid\")", WorkspaceName: "Prod",
			Kind: "ssh", Target: "10.0.0.1", Username: "deploy", OpenedAt: 1760083300,
		},
	}
}

func readCSV(t *testing.T, entries []SessionLogEntry) [][]string {
	t.Helper()
	var buffer bytes.Buffer
	if err := WriteSessionLogCSV(&buffer, entries); err != nil {
		t.Fatalf("WriteSessionLogCSV: %v", err)
	}
	records, err := csv.NewReader(&buffer).ReadAll()
	if err != nil {
		t.Fatalf("the output is not valid CSV: %v", err)
	}
	return records
}

func TestSessionLogCSV(t *testing.T) {
	records := readCSV(t, exportFixture())
	if len(records) != 3 {
		t.Fatalf("got %d rows, want a header and two entries", len(records))
	}

	column := map[string]int{}
	for index, name := range records[0] {
		column[name] = index
	}
	closed, open := records[1], records[2]

	for name, want := range map[string]string{
		"opened_at":        "2025-10-10T08:00:00Z",
		"closed_at":        "2025-10-10T08:01:30Z",
		"duration_seconds": "90",
		"end_message":      "connection reset by peer",
		"aws_region":       "ap-southeast-1",
	} {
		if got := closed[column[name]]; got != want {
			t.Errorf("closed session %s = %q, want %q", name, got, want)
		}
	}

	// Still open: the end is unknown, so it is left empty rather than made up.
	if open[column["closed_at"]] != "" || open[column["duration_seconds"]] != "" {
		t.Errorf("an open session got an end: %q / %q",
			open[column["closed_at"]], open[column["duration_seconds"]])
	}
}

// A name a person typed must stay text when the export is opened in a
// spreadsheet, not become a formula that runs for whoever opens it.
func TestSessionLogCSVKeepsFormulasAsText(t *testing.T) {
	records := readCSV(t, exportFixture())

	name := records[2][5]
	if !strings.HasPrefix(name, "'=") {
		t.Errorf("connection name %q would be evaluated as a formula", name)
	}
	for _, cell := range []string{"=1+1", "+1", "-1", "@SUM(A1)"} {
		if got := spreadsheetSafe(cell); got != "'"+cell {
			t.Errorf("spreadsheetSafe(%q) = %q", cell, got)
		}
	}
	if got := spreadsheetSafe("web-1"); got != "web-1" {
		t.Errorf("an ordinary name was changed to %q", got)
	}
}

func TestSessionLogJSON(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteSessionLogJSON(&buffer, exportFixture()); err != nil {
		t.Fatalf("WriteSessionLogJSON: %v", err)
	}

	var rows []map[string]string
	if err := json.Unmarshal(buffer.Bytes(), &rows); err != nil {
		t.Fatalf("the output is not a JSON array of objects: %v", err)
	}
	if len(rows) != 2 || rows[0]["duration_seconds"] != "90" || rows[1]["closed_at"] != "" {
		t.Errorf("unexpected rows: %v", rows)
	}
	// JSON is not opened by spreadsheets, so names stay exactly as typed.
	if !strings.HasPrefix(rows[1]["connection_name"], "=HYPERLINK") {
		t.Errorf("JSON altered the name: %q", rows[1]["connection_name"])
	}
}

// The CSV header and the JSON keys are two lists of the same names. This keeps
// them from drifting when a field is added to one and not the other.
func TestExportHeaderMatchesTheJSONKeys(t *testing.T) {
	fields := reflect.TypeOf(exportedSession{})
	if fields.NumField() != len(exportHeader) {
		t.Fatalf("%d fields but %d header columns", fields.NumField(), len(exportHeader))
	}
	for index := range exportHeader {
		if tag := fields.Field(index).Tag.Get("json"); tag != exportHeader[index] {
			t.Errorf("column %d: header %q, json key %q", index, exportHeader[index], tag)
		}
	}
	if got := len(exportedSession{}.values()); got != len(exportHeader) {
		t.Errorf("values() returns %d cells for %d columns", got, len(exportHeader))
	}
}

func TestUnknownExportFormat(t *testing.T) {
	var buffer bytes.Buffer
	if err := WriteSessionLog(&buffer, "xlsx", nil); err == nil {
		t.Error("an unknown format was accepted")
	}
}
