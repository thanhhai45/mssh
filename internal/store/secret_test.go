package store

import (
	"encoding/json"
	"strings"
	"testing"

	"mssh/internal/secrets/secretstest"
)

// newConnection makes a workspace and a connection to hang secrets on. The
// foreign key means a secret cannot exist without an owner, which is the point.
func newConnection(t *testing.T, s *Store) (Workspace, Connection) {
	t.Helper()

	workspace, err := s.CreateWorkspace(WorkspaceInput{Name: "Secrets"})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	connection, err := s.CreateConnection(workspace.ID, ConnectionInput{
		Name: "web-1", Kind: KindSSH, Target: "10.0.0.1", Username: "deploy",
	})
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	return workspace, connection
}

// The same contract the keychain and the in-memory vault meet.
func TestConnectionPasswordsMeetTheVaultContract(t *testing.T) {
	s := openTest(t)
	_, connection := newConnection(t, s)

	secretstest.Run(t, s.ConnectionPasswords(), connection.ID)
}

// Cascade only works while foreign_keys(1) is in the DSN. SQLite ships with
// foreign keys off; drop that pragma and this test is what notices.
func TestSecretsGoWhenTheirOwnerDoes(t *testing.T) {
	s := openTest(t)

	t.Run("connection", func(t *testing.T) {
		_, connection := newConnection(t, s)
		if err := s.ConnectionPasswords().Set(connection.ID, "hunter2"); err != nil {
			t.Fatalf("set password: %v", err)
		}
		if err := s.DeleteConnection(connection.ID); err != nil {
			t.Fatalf("DeleteConnection: %v", err)
		}
		if s.ConnectionPasswords().Has(connection.ID) {
			t.Error("the password outlived its connection")
		}
	})

	// Two levels: workspace → connection → password. Nothing in Go walks that
	// chain; the database does it in one DELETE.
	t.Run("workspace", func(t *testing.T) {
		workspace, connection := newConnection(t, s)
		if err := s.ConnectionPasswords().Set(connection.ID, "hunter2"); err != nil {
			t.Fatalf("set password: %v", err)
		}
		if err := s.SetWorkspaceSecret(workspace.ID, SecretAWSSecretAccessKey, "wJalr"); err != nil {
			t.Fatalf("set workspace secret: %v", err)
		}
		if err := s.DeleteWorkspace(workspace.ID); err != nil {
			t.Fatalf("DeleteWorkspace: %v", err)
		}
		if s.HasWorkspaceSecret(workspace.ID, SecretAWSSecretAccessKey) {
			t.Error("the AWS secret outlived its workspace")
		}
		if s.ConnectionPasswords().Has(connection.ID) {
			t.Error("a password outlived the workspace its connection was in")
		}
	})
}

// Workspaces and connections are sent to the frontend whole. A secret must not
// be reachable from either — guaranteed today by keeping secrets in their own
// tables, and this test is what keeps it guaranteed.
func TestSecretsNeverRideAlongToTheFrontend(t *testing.T) {
	s := openTest(t)
	workspace, connection := newConnection(t, s)

	const password = "never-in-json-7f3a"
	const awsSecret = "never-in-json-9c1e"

	if err := s.ConnectionPasswords().Set(connection.ID, password); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if err := s.SetWorkspaceSecret(workspace.ID, SecretAWSSecretAccessKey, awsSecret); err != nil {
		t.Fatalf("set workspace secret: %v", err)
	}

	connections, err := s.ListConnections(workspace.ID)
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	workspaces, err := s.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}

	// Exactly what Wails would send: the structs, marshalled to JSON.
	sent, err := json.Marshal(map[string]any{
		"connections": connections,
		"workspaces":  workspaces,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, secret := range []string{password, awsSecret} {
		if strings.Contains(string(sent), secret) {
			t.Errorf("a secret reached the frontend payload: %s", secret)
		}
	}
}
