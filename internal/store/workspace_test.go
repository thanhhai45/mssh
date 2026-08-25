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

	created, err := s.CreateWorkspace(WorkspaceInput{Name: "Before", Color: "red"})
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

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

	if updated.CreatedAt != created.CreatedAt {
		t.Errorf("CreatedAt changed on update: %d -> %d", created.CreatedAt, updated.CreatedAt)
	}

	if updated.UpdatedAt == created.UpdatedAt {
		t.Errorf("UpdatedAt did not change on update: %d -> %d", created.UpdatedAt, updated.UpdatedAt)
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
