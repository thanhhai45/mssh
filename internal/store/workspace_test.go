package store

import (
	"errors"
	"testing"
)

func TestCreateAndListWorkspaces(t *testing.T) {
	s := openTest(t)

	ws, err := s.CreateWorkspace(WorkspaceInput{
		Name:       "Test Workspace",
		AWSProfile: "prod",
		AWSRegion:  "ap-southeast-1",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	if ws.ID == "" {
		t.Fatalf("CreateWorkspace returned empty ID")
	}

	if ws.Name != "Test Workspace" {
		t.Errorf("CreateWorkspace returned wrong name: got %q, want %q", ws.Name, "Test Workspace")
	}

	if ws.Color != "slate" {
		t.Errorf("CreateWorkspace returned wrong color: got %q, want %q", ws.Color, "blue")
	}

	if ws.AWSProfile != "prod" {
		t.Errorf("CreateWorkspace returned wrong AWSProfile: got %q, want %q", ws.AWSProfile, "prod")
	}

	if ws.AWSRegion != "ap-southeast-1" {
		t.Errorf("CreateWorkspace returned wrong AWSRegion: got %q, want %q", ws.AWSRegion, "ap-southeast-1")
	}

	workspaces, err := s.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces failed: %v", err)
	}

	if len(workspaces) != 2 {
		t.Fatalf("got %d workspaces, want 2 (Default plus the new one)", len(workspaces))
	}
}

func TestCreateWorkspaceRejectsBlankName(t *testing.T) {
	s := openTest(t)

	if _, err := s.CreateWorkspace(WorkspaceInput{Name: "   "}); err == nil {
		t.Fatal("blank name was accepted, want an error")
	}
}

func TestGetWorkspaceNotFound(t *testing.T) {
	s := openTest(t)

	_, err := s.GetWorkspace("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("expected error for nonexistent workspace")
	}
}

func TestUpdateWorkspace(t *testing.T) {
	s := openTest(t)

	clock := int64(1_000)
	s.now = func() int64 { return clock }

	created, err := s.CreateWorkspace(WorkspaceInput{Name: "Before", Color: "red"})
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	clock = 2_000

	updated, err := s.UpdateWorkspace(created.ID, WorkspaceInput{
		Name:       "After",
		Color:      "blue",
		AWSProfile: "dev",
		AWSRegion:  "us-west-2",
	})
	if err != nil {
		t.Fatalf("UpdateWorkspace failed: %v", err)
	}

	if updated.Name != "After" || updated.Color != "blue" || updated.AWSProfile != "dev" || updated.AWSRegion != "us-west-2" {
		t.Errorf("update did not stick: %+v", updated)
	}

	if created.CreatedAt != 1_000 {
		t.Errorf("CreatedAt = %d, want 1000", created.CreatedAt)
	}
	if updated.CreatedAt != 1_000 {
		t.Errorf("CreatedAt changed on update: %d, want 1000", updated.CreatedAt)
	}
	if updated.UpdatedAt != 2_000 {
		t.Errorf("UpdatedAt = %d, want 2000", updated.UpdatedAt)
	}

	if updated.SortOrder != created.SortOrder {
		t.Errorf("SortOrder changed on update: %d -> %d", created.SortOrder, updated.SortOrder)
	}

	if _, err := s.UpdateWorkspace("nope", WorkspaceInput{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("updating a missing id: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteWorkspace(t *testing.T) {
	s := openTest(t)

	ws, err := s.CreateWorkspace(WorkspaceInput{Name: "Doomed"})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	if err := s.DeleteWorkspace(ws.ID); err != nil {
		t.Fatalf("DeleteWorkspace: %v", err)
	}
	if _, err := s.GetWorkspace(ws.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteWorkspace(ws.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting twice: err = %v, want ErrNotFound", err)
	}
}

func TestReorderWorkspaces(t *testing.T) {
	s := openTest(t)

	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		if _, err := s.CreateWorkspace(WorkspaceInput{Name: name}); err != nil {
			t.Fatalf("CreateWorkspace %s: %v", name, err)
		}
	}

	before, err := s.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}

	// Reverse the current order, seeded workspace included.
	want := make([]string, 0, len(before))
	for i := len(before) - 1; i >= 0; i-- {
		want = append(want, before[i].ID)
	}

	if err := s.ReorderWorkspaces(want); err != nil {
		t.Fatalf("ReorderWorkspaces: %v", err)
	}

	after, err := s.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(after) != len(want) {
		t.Fatalf("got %d workspaces, want %d", len(after), len(want))
	}
	for i := range want {
		if after[i].ID != want[i] {
			t.Fatalf("position %d = %s, want %s", i, after[i].ID, want[i])
		}
	}
}

func TestWorkspaceCredentialsSource(t *testing.T) {
	s := openTest(t)

	// Nothing said means the AWS CLI decides, exactly as before stored keys.
	plain, err := s.CreateWorkspace(WorkspaceInput{Name: "Plain"})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if plain.AWSCredentialsSource != AWSCredentialsFromCLI {
		t.Errorf("default source = %q, want %q", plain.AWSCredentialsSource, AWSCredentialsFromCLI)
	}

	stored, err := s.CreateWorkspace(WorkspaceInput{
		Name:                 "Stored",
		AWSCredentialsSource: AWSCredentialsStored,
		AWSAccessKeyID:       "  AKIAEXAMPLE  ",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace stored: %v", err)
	}

	// Read back through both paths: Get and List share workspaceScanTargets,
	// and this is what proves the new columns are in it, in the right place.
	got, err := s.GetWorkspace(stored.ID)
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if got.AWSCredentialsSource != AWSCredentialsStored || got.AWSAccessKeyID != "AKIAEXAMPLE" {
		t.Errorf("Get = %q / %q, want stored / AKIAEXAMPLE",
			got.AWSCredentialsSource, got.AWSAccessKeyID)
	}
	list, err := s.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	found := false
	for _, ws := range list {
		if ws.ID == stored.ID {
			found = ws.AWSCredentialsSource == AWSCredentialsStored && ws.AWSAccessKeyID == "AKIAEXAMPLE"
		}
	}
	if !found {
		t.Error("List did not return the stored source and key id")
	}

	// Back to the CLI through Update.
	updated, err := s.UpdateWorkspace(stored.ID, WorkspaceInput{
		Name: "Stored", AWSCredentialsSource: AWSCredentialsFromCLI,
	})
	if err != nil {
		t.Fatalf("UpdateWorkspace: %v", err)
	}
	if updated.AWSCredentialsSource != AWSCredentialsFromCLI || updated.AWSAccessKeyID != "" {
		t.Errorf("after update = %q / %q, want cli / empty",
			updated.AWSCredentialsSource, updated.AWSAccessKeyID)
	}

	// A value that is neither is refused on both paths, not stored.
	if _, err := s.CreateWorkspace(WorkspaceInput{Name: "Typo", AWSCredentialsSource: "stord"}); err == nil {
		t.Error("CreateWorkspace accepted an unknown source")
	}
	if _, err := s.UpdateWorkspace(stored.ID, WorkspaceInput{Name: "Typo", AWSCredentialsSource: "stord"}); err == nil {
		t.Error("UpdateWorkspace accepted an unknown source")
	}
}
