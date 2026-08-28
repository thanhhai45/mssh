package main

import (
	"context"

	"mssh/internal/store"
)

// App is the bridge between the frontend and the Go packages that do the real
// work. Methods here stay thin on purpose: logic lives in internal/.
type App struct {
	ctx   context.Context
	store *store.Store
}

// NewApp creates a new App application struct
func NewApp(st *store.Store) *App {
	return &App{store: st}
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
	return a.store.DeleteWorkspace(id)
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
	return a.store.DeleteConnection(id)
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
