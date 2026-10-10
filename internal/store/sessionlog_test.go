package store

import "testing"

// clockAt makes s.now return each given time in turn, then keep returning the
// last one — so a test can say exactly when each row opened and closed.
func clockAt(s *Store, times ...int64) {
	next := 0
	s.now = func() int64 {
		current := times[next]
		if next < len(times)-1 {
			next++
		}
		return current
	}
}

func onlyEntry(t *testing.T, s *Store, filter SessionLogFilter) SessionLogEntry {
	t.Helper()
	entries, err := s.ListSessionLog(filter)
	if err != nil {
		t.Fatalf("ListSessionLog: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	return entries[0]
}

// A record must read the same later as it did when it was written, however
// the machine has been renamed or moved since.
func TestSessionLogKeepsWhatWasTrueAtTheTime(t *testing.T) {
	s := openTest(t)
	workspace, connection := newConnection(t, s)

	if err := s.RecordSessionOpened("session-1", connection.ID); err != nil {
		t.Fatalf("RecordSessionOpened: %v", err)
	}

	if _, err := s.UpdateConnection(connection.ID, ConnectionInput{
		Name: "renamed", Kind: KindSSH, Target: "10.9.9.9", Username: "someone-else",
	}); err != nil {
		t.Fatalf("UpdateConnection: %v", err)
	}
	if _, err := s.UpdateWorkspace(workspace.ID, WorkspaceInput{Name: "Renamed too"}); err != nil {
		t.Fatalf("UpdateWorkspace: %v", err)
	}

	entry := onlyEntry(t, s, SessionLogFilter{})
	if entry.ConnectionName != "web-1" || entry.Target != "10.0.0.1" ||
		entry.Username != "deploy" || entry.WorkspaceName != "Secrets" {
		t.Errorf("the record changed with the connection: %+v", entry)
	}
}

// Every other table cascades. This one must not: deleting a server must not
// delete the evidence that someone was on it.
func TestSessionLogOutlivesWhatItDescribes(t *testing.T) {
	s := openTest(t)
	workspace, connection := newConnection(t, s)

	if err := s.RecordSessionOpened("session-1", connection.ID); err != nil {
		t.Fatalf("RecordSessionOpened: %v", err)
	}
	if err := s.DeleteWorkspace(workspace.ID); err != nil {
		t.Fatalf("DeleteWorkspace: %v", err)
	}

	entry := onlyEntry(t, s, SessionLogFilter{})
	if entry.ConnectionID != connection.ID || entry.ConnectionName != "web-1" {
		t.Errorf("the record lost what it described: %+v", entry)
	}
}

// Disconnecting a session is reported twice: by the manager, as the user's
// choice, and moments later by the transport it closed. The first is the truth.
func TestFirstReportOfAnEndingWins(t *testing.T) {
	s := openTest(t)
	_, connection := newConnection(t, s)
	clockAt(s, 100, 160, 170)

	if err := s.RecordSessionOpened("session-1", connection.ID); err != nil {
		t.Fatalf("RecordSessionOpened: %v", err)
	}
	if err := s.RecordSessionClosed("session-1", SessionClosedByUser, ""); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := s.RecordSessionClosed("session-1", SessionFailed, "connection reset"); err != nil {
		t.Fatalf("second close: %v", err)
	}

	entry := onlyEntry(t, s, SessionLogFilter{})
	if entry.EndReason != SessionClosedByUser || entry.EndMessage != "" || entry.ClosedAt != 160 {
		t.Errorf("got %q %q at %d, want closed by the user at 160",
			entry.EndReason, entry.EndMessage, entry.ClosedAt)
	}
	if entry.OpenedAt != 100 {
		t.Errorf("opened at %d, want 100", entry.OpenedAt)
	}
}

// A session open when mssh died never got its ending written. The next start
// says so, without inventing an end time.
func TestInterruptedSessionsAreSettledAtStartup(t *testing.T) {
	s := openTest(t)
	_, connection := newConnection(t, s)

	for _, id := range []string{"left-open", "closed-properly"} {
		if err := s.RecordSessionOpened(id, connection.ID); err != nil {
			t.Fatalf("RecordSessionOpened %s: %v", id, err)
		}
	}
	if err := s.RecordSessionClosed("closed-properly", SessionEnded, ""); err != nil {
		t.Fatalf("RecordSessionClosed: %v", err)
	}

	settled, err := s.MarkInterruptedSessions()
	if err != nil {
		t.Fatalf("MarkInterruptedSessions: %v", err)
	}
	if settled != 1 {
		t.Errorf("settled %d rows, want 1", settled)
	}

	entries, err := s.ListSessionLog(SessionLogFilter{})
	if err != nil {
		t.Fatalf("ListSessionLog: %v", err)
	}
	for _, entry := range entries {
		switch entry.ID {
		case "left-open":
			if entry.EndReason != SessionInterrupted || entry.ClosedAt != 0 {
				t.Errorf("left-open: %q at %d, want interrupted with no end time", entry.EndReason, entry.ClosedAt)
			}
		case "closed-properly":
			if entry.EndReason != SessionEnded {
				t.Errorf("closed-properly was rewritten to %q", entry.EndReason)
			}
		}
	}
}

func TestSessionLogFilters(t *testing.T) {
	s := openTest(t)
	first, alpha := newConnection(t, s)
	second, beta := newConnection(t, s)
	clockAt(s, 100, 200, 300)

	for _, row := range []struct{ id, connection string }{
		{"a-100", alpha.ID}, {"b-200", beta.ID}, {"a-300", alpha.ID},
	} {
		if err := s.RecordSessionOpened(row.id, row.connection); err != nil {
			t.Fatalf("RecordSessionOpened %s: %v", row.id, err)
		}
	}

	ids := func(filter SessionLogFilter) []string {
		entries, err := s.ListSessionLog(filter)
		if err != nil {
			t.Fatalf("ListSessionLog: %v", err)
		}
		out := []string{}
		for _, entry := range entries {
			out = append(out, entry.ID)
		}
		return out
	}

	for _, testCase := range []struct {
		name   string
		filter SessionLogFilter
		want   string
	}{
		{"everything, newest first", SessionLogFilter{}, "a-300 b-200 a-100"},
		{"one workspace", SessionLogFilter{WorkspaceID: second.ID}, "b-200"},
		{"one connection", SessionLogFilter{ConnectionID: alpha.ID}, "a-300 a-100"},
		{"from is inclusive, to exclusive", SessionLogFilter{From: 200, To: 300}, "b-200"},
		{"limit", SessionLogFilter{Limit: 2}, "a-300 b-200"},
		{"workspace and range together", SessionLogFilter{WorkspaceID: first.ID, From: 150}, "a-300"},
	} {
		got := ""
		for index, id := range ids(testCase.filter) {
			if index > 0 {
				got += " "
			}
			got += id
		}
		if got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestRecordingAnUnknownConnectionFails(t *testing.T) {
	s := openTest(t)
	if err := s.RecordSessionOpened("session-1", "no-such-connection"); err == nil {
		t.Error("a session for a connection that does not exist was recorded")
	}
}
