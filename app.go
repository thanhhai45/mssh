package main

import (
	"context"
	"log"

	"mssh/internal/secrets"
	"mssh/internal/store"
)

// App is the bridge between the frontend and the Go packages that do the real
// work. Methods here stay thin on purpose: logic lives in internal/.
type App struct {
	ctx     context.Context
	store   *store.Store
	secrets secrets.Vault
}

// NewApp creates a new App application struct
func NewApp(st *store.Store, vault secrets.Vault) *App {
	return &App{store: st, secrets: vault}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// / ---------- Workspaces ----------
func (a *App) ListWorkspaces() ([]store.Workspace, error) {
	return a.store.ListWorkspaces()
}

func (a *App) GetWorkspace(id string) (store.Workspace, error) {
	return a.store.GetWorkspace(id)
}

func (a *App) CreateWorkspace(input store.WorkspaceInput) (store.Workspace, error) {
	return a.store.CreateWorkspace(input)
}

func (a *App) UpdateWorkspace(id string, input store.WorkspaceInput) (store.Workspace, error) {
	return a.store.UpdateWorkspace(id, input)
}

func (a *App) DeleteWorkspace(id string) error {
	conns, err := a.store.ListConnections(id)
	if err != nil {
		return err
	}

	if err := a.store.DeleteWorkspace(id); err != nil {
		return err
	}

	for _, c := range conns {
		a.forgetPassword(c.ID)
	}
	return nil
}

func (a *App) ReorderWorkspaces(ids []string) error {
	return a.store.ReorderWorkspaces(ids)
}

// ---------- Connections ----------

// ListConnections returns the connections of one workspace in display order.
func (a *App) ListConnections(workspaceID string) ([]store.Connection, error) {
	return a.store.ListConnections(workspaceID)
}

// GetConnection returns one connection by id.
func (a *App) GetConnection(id string) (store.Connection, error) {
	return a.store.GetConnection(id)
}

// CreateConnection adds a connection to a workspace.
func (a *App) CreateConnection(workspaceID string, input store.ConnectionInput) (store.Connection, error) {
	return a.store.CreateConnection(workspaceID, input)
}

// UpdateConnection overwrites the editable fields of a connection.
func (a *App) UpdateConnection(id string, input store.ConnectionInput) (store.Connection, error) {
	return a.store.UpdateConnection(id, input)
}

// DeleteConnection removes one connection.
func (a *App) DeleteConnection(id string) error {
	if err := a.store.DeleteConnection(id); err != nil {
		return err
	}
	a.forgetPassword(id)
	return nil
}

// MoveConnection puts a connection at the end of another workspace.
func (a *App) MoveConnection(id string, toWorkspaceID string) error {
	return a.store.MoveConnection(id, toWorkspaceID)
}

// ResolveAWSForConnection reports the profile and region a session would use,
// after the connection-then-workspace fallback.
func (a *App) ResolveAWSForConnection(connectionID string) (store.ResolvedAWS, error) {
	return a.store.ResolveAWSForConnection(connectionID)
}

// ParseSSHCommand prefills a form from a pasted ssh command line.
func (a *App) ParseSSHCommand(cmd string) (store.ParsedSSHCommand, error) {
	return store.ParseSSHCommand(cmd)
}

// ---------- Passwords ----------

// SetConnectionPassword stores a password in the OS keychain. It never touches
// the database.
func (a *App) SetConnectionPassword(id string, password string) error {
	if _, err := a.store.GetConnection(id); err != nil {
		return err
	}

	return a.secrets.Set(id, password)
}

// DeleteConnectionPassword forgets a stored password.
func (a *App) DeleteConnectionPassword(id string) error {
	return a.secrets.Delete(id)
}

// HasConnectionPassword reports whether a password is already stored, so the
// UI can show "saved" instead of an empty field.
func (a *App) HasConnectionPassword(id string) bool {
	return a.secrets.Has(id)
}

// forgetPassword removes a stored password as cleanup. A failure leaves an
// unreachable keychain entry and nothing worse, so it is logged rather than
// returned: the deletion the user asked for has already succeeded.
func (a *App) forgetPassword(connectionID string) {
	if err := a.secrets.Delete(connectionID); err != nil {
		log.Printf("mssh: count not remove the keychain entry for %s: %v", connectionID, err)
	}
}
