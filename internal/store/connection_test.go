package store

import (
	"errors"
	"testing"
)

func TestNormalizeConnection(t *testing.T) {
	tests := []struct {
		name    string
		in      ConnectionInput
		wantErr bool
		check   func(t *testing.T, got ConnectionInput)
	}{
		{
			name: "ssh fills in defaults and trims",
			in:   ConnectionInput{Name: " web-1 ", Kind: KindSSH, Target: " 10.0.4.12 ", Username: "deploy"},
			check: func(t *testing.T, got ConnectionInput) {
				if got.Name != "web-1" {
					t.Errorf("Name = %q, want trimmed", got.Name)
				}
				if got.Port != 22 {
					t.Errorf("Port = %d, want the 22 default", got.Port)
				}
				if got.AuthMethod != AuthAgent {
					t.Errorf("AuthMethod = %q, want agent", got.AuthMethod)
				}
				if got.Extra != "{}" {
					t.Errorf("Extra = %q, want {}", got.Extra)
				}
			},
		},
		{
			name:    "ssh without a username is rejected",
			in:      ConnectionInput{Name: "web-1", Kind: KindSSH, Target: "10.0.4.12"},
			wantErr: true,
		},
		{
			name: "key auth without a key path is rejected",
			in: ConnectionInput{Name: "web-1", Kind: KindSSH, Target: "10.0.4.12",
				Username: "deploy", AuthMethod: AuthKey},
			wantErr: true,
		},
		{
			name: "port out of range is rejected",
			in: ConnectionInput{Name: "web-1", Kind: KindSSH, Target: "10.0.4.12",
				Username: "deploy", Port: 70000},
			wantErr: true,
		},
		{
			name:    "unknown kind is rejected",
			in:      ConnectionInput{Name: "x", Kind: "shh", Target: "10.0.4.12"},
			wantErr: true,
		},
		{
			name:    "blank name is rejected",
			in:      ConnectionInput{Name: "   ", Kind: KindSSH, Target: "10.0.4.12", Username: "deploy"},
			wantErr: true,
		},
		{
			name: "ssm clears the ssh-only fields",
			in: ConnectionInput{Name: "api", Kind: KindSSM, Target: "i-0abc12345",
				Username: "deploy", Port: 2222, AuthMethod: AuthKey, KeyPath: "/tmp/k",
				AWSProfile: " prod ", AWSRegion: "ap-southeast-1"},
			check: func(t *testing.T, got ConnectionInput) {
				if got.Username != "" || got.Port != 0 || got.AuthMethod != "" || got.KeyPath != "" {
					t.Errorf("ssh-only fields survived on an ssm row: %+v", got)
				}
				if got.AWSProfile != "prod" {
					t.Errorf("AWSProfile = %q, want trimmed", got.AWSProfile)
				}
			},
		},
		{
			name:    "ssm rejects a target that is not an instance id",
			in:      ConnectionInput{Name: "api", Kind: KindSSM, Target: "10.0.0.1"},
			wantErr: true,
		},
		{
			name:    "ssm-ssh needs both an instance id and a username",
			in:      ConnectionInput{Name: "api", Kind: KindSSMSSH, Target: "i-0abc12345"},
			wantErr: true,
		},
		{
			name: "ssh keeps aws fields out",
			in: ConnectionInput{Name: "web", Kind: KindSSH, Target: "10.0.4.12",
				Username: "deploy", AWSProfile: "prod", AWSRegion: "ap-southeast-1"},
			check: func(t *testing.T, got ConnectionInput) {
				if got.AWSProfile != "" || got.AWSRegion != "" {
					t.Errorf("aws fields survived on an ssh row: %+v", got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.in.normalize()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalize() succeeded, want an error; got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize() failed: %v", err)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}
}

func TestConnectionCRUD(t *testing.T) {
	s := openTest(t)

	ws, err := s.CreateWorkspace(WorkspaceInput{Name: "AWS Prod", AWSProfile: "prod", AWSRegion: "ap-southeast-1"})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	created, err := s.CreateConnection(ws.ID, ConnectionInput{
		Name: "api-1", Kind: KindSSM, Target: "i-0abc12345",
	})
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	if created.ID == "" || created.WorkspaceID != ws.ID {
		t.Fatalf("unexpected row: %+v", created)
	}

	got, err := s.GetConnection(created.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if got.Name != "api-1" || got.Kind != KindSSM {
		t.Errorf("round trip changed the row: %+v", got)
	}

	list, err := s.ListConnections(ws.ID)
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d connections, want 1", len(list))
	}

	updated, err := s.UpdateConnection(created.ID, ConnectionInput{
		Name: "api-1 renamed", Kind: KindSSH, Target: "10.0.4.12", Username: "deploy",
	})
	if err != nil {
		t.Fatalf("UpdateConnection: %v", err)
	}
	if updated.Kind != KindSSH || updated.Port != 22 || updated.Username != "deploy" {
		t.Errorf("update did not stick: %+v", updated)
	}
	if updated.WorkspaceID != ws.ID {
		t.Errorf("update moved the connection to %s", updated.WorkspaceID)
	}

	if err := s.DeleteConnection(created.ID); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}
	if _, err := s.GetConnection(created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: err = %v, want ErrNotFound", err)
	}
}

func TestCreateConnectionRejectsUnknownWorkspace(t *testing.T) {
	s := openTest(t)

	_, err := s.CreateConnection("no-such-workspace", ConnectionInput{
		Name: "x", Kind: KindSSM, Target: "i-0abc12345",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMoveConnection(t *testing.T) {
	s := openTest(t)

	from, _ := s.CreateWorkspace(WorkspaceInput{Name: "From"})
	to, _ := s.CreateWorkspace(WorkspaceInput{Name: "To"})

	c, err := s.CreateConnection(from.ID, ConnectionInput{
		Name: "api", Kind: KindSSM, Target: "i-0abc12345",
	})
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}

	if err := s.MoveConnection(c.ID, to.ID); err != nil {
		t.Fatalf("MoveConnection: %v", err)
	}

	moved, err := s.GetConnection(c.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if moved.WorkspaceID != to.ID {
		t.Errorf("WorkspaceID = %s, want %s", moved.WorkspaceID, to.ID)
	}

	left, _ := s.ListConnections(from.ID)
	if len(left) != 0 {
		t.Errorf("%d connections left in the source workspace, want 0", len(left))
	}
}

func TestResolveAWS(t *testing.T) {
	ws := Workspace{AWSProfile: "ws-profile", AWSRegion: "ws-region"}

	tests := []struct {
		name        string
		conn        Connection
		wantProfile string
		wantRegion  string
	}{
		{"connection wins", Connection{AWSProfile: "c", AWSRegion: "r"}, "c", "r"},
		{"falls back to the workspace", Connection{}, "ws-profile", "ws-region"},
		{"each field falls back on its own", Connection{AWSProfile: "c"}, "c", "ws-region"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveAWS(tc.conn, ws)
			if got.Profile != tc.wantProfile || got.Region != tc.wantRegion {
				t.Errorf("got %+v, want profile %q region %q", got, tc.wantProfile, tc.wantRegion)
			}
		})
	}

	// Neither level set: stay empty so the AWS CLI uses its own default.
	if got := ResolveAWS(Connection{}, Workspace{}); got.Profile != "" || got.Region != "" {
		t.Errorf("got %+v, want both empty", got)
	}
}

func TestParseSSHCommand(t *testing.T) {
	tests := []struct {
		in      string
		want    ParsedSSHCommand
		wantErr bool
	}{
		{in: "ssh deploy@10.0.4.12 -p 22", want: ParsedSSHCommand{Username: "deploy", Host: "10.0.4.12", Port: 22}},
		{in: "ssh -p 2222 ubuntu@example.com", want: ParsedSSHCommand{Username: "ubuntu", Host: "example.com", Port: 2222}},
		{in: "deploy@10.0.4.12", want: ParsedSSHCommand{Username: "deploy", Host: "10.0.4.12", Port: 22}},
		{in: "ssh host-only", want: ParsedSSHCommand{Host: "host-only", Port: 22}},
		{
			in:   "ssh -i ~/.ssh/id_ed25519 -o StrictHostKeyChecking=no ec2-user@1.2.3.4",
			want: ParsedSSHCommand{Username: "ec2-user", Host: "1.2.3.4", Port: 22, KeyPath: "~/.ssh/id_ed25519"},
		},
		{in: "ssh -p notanumber deploy@host", wantErr: true},
		{in: "ssh -p 22", wantErr: true},
		{in: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseSSHCommand(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseSSHCommand(%q) succeeded, want an error; got %+v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSSHCommand(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
