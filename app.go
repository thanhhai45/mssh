package main

import (
	"context"
	"log"
	"mssh/internal/secrets"
	"mssh/internal/session"
	"mssh/internal/store"
	"mssh/internal/transport"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the bridge between the frontend and the Go packages that do the real
// work. Methods here stay thin on purpose: logic lives in internal/.
type App struct {
	appContext context.Context
	store      *store.Store
	secrets    secrets.Vault
	sessions   *session.Manager
}

// NewApp creates a new App application struct
func NewApp(dataStore *store.Store, vault secrets.Vault, sessions *session.Manager) *App {
	return &App{store: dataStore, secrets: vault, sessions: sessions}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (app *App) startup(startupContext context.Context) {
	app.appContext = startupContext
}
func (app *App) shutdown(shutdownContext context.Context) {
	app.sessions.CloseAll()
}

// / ---------- Workspaces ----------
func (app *App) ListWorkspaces() ([]store.Workspace, error) {
	return app.store.ListWorkspaces()
}

func (app *App) GetWorkspace(id string) (store.Workspace, error) {
	return app.store.GetWorkspace(id)
}

func (app *App) CreateWorkspace(input store.WorkspaceInput) (store.Workspace, error) {
	return app.store.CreateWorkspace(input)
}

func (app *App) UpdateWorkspace(id string, input store.WorkspaceInput) (store.Workspace, error) {
	return app.store.UpdateWorkspace(id, input)
}

func (app *App) DeleteWorkspace(id string) error {
	conns, err := app.store.ListConnections(id)
	if err != nil {
		return err
	}

	if err := app.store.DeleteWorkspace(id); err != nil {
		return err
	}

	for _, c := range conns {
		app.forgetPassword(c.ID)
	}
	return nil
}

func (app *App) ReorderWorkspaces(ids []string) error {
	return app.store.ReorderWorkspaces(ids)
}

// ---------- Connections ----------

// ListConnections returns the connections of one workspace in display order.
func (app *App) ListConnections(workspaceID string) ([]store.Connection, error) {
	return app.store.ListConnections(workspaceID)
}

// GetConnection returns one connection by id.
func (app *App) GetConnection(id string) (store.Connection, error) {
	return app.store.GetConnection(id)
}

// CreateConnection adds a connection to a workspace.
func (app *App) CreateConnection(workspaceID string, input store.ConnectionInput) (store.Connection, error) {
	return app.store.CreateConnection(workspaceID, input)
}

// UpdateConnection overwrites the editable fields of a connection.
func (app *App) UpdateConnection(id string, input store.ConnectionInput) (store.Connection, error) {
	return app.store.UpdateConnection(id, input)
}

// DeleteConnection removes one connection.
func (app *App) DeleteConnection(id string) error {
	if err := app.store.DeleteConnection(id); err != nil {
		return err
	}
	app.forgetPassword(id)
	return nil
}

// MoveConnection puts a connection at the end of another workspace.
func (app *App) MoveConnection(id string, toWorkspaceID string) error {
	return app.store.MoveConnection(id, toWorkspaceID)
}

// ResolveAWSForConnection reports the profile and region a session would use,
// after the connection-then-workspace fallback.
func (app *App) ResolveAWSForConnection(connectionID string) (store.ResolvedAWS, error) {
	return app.store.ResolveAWSForConnection(connectionID)
}

// ParseSSHCommand prefills a form from a pasted ssh command line.
func (app *App) ParseSSHCommand(cmd string) (store.ParsedSSHCommand, error) {
	return store.ParseSSHCommand(cmd)
}

// ---------- Passwords ----------

// SetConnectionPassword stores a password in the OS keychain. It never touches
// the database.
func (app *App) SetConnectionPassword(id string, password string) error {
	if _, err := app.store.GetConnection(id); err != nil {
		return err
	}

	return app.secrets.Set(id, password)
}

// DeleteConnectionPassword forgets a stored password.
func (app *App) DeleteConnectionPassword(id string) error {
	return app.secrets.Delete(id)
}

// HasConnectionPassword reports whether a password is already stored, so the
// UI can show "saved" instead of an empty field.
func (app *App) HasConnectionPassword(id string) bool {
	return app.secrets.Has(id)
}

// forgetPassword removes a stored password as cleanup. A failure leaves an
// unreachable keychain entry and nothing worse, so it is logged rather than
// returned: the deletion the user asked for has already succeeded.
func (app *App) forgetPassword(connectionID string) {
	if err := app.secrets.Delete(connectionID); err != nil {
		log.Printf("mssh: count not remove the keychain entry for %s: %v", connectionID, err)
	}
}

/* ---------- Sessions ---------- */
// ConnectSession opens a live connection.
//
// Pass an empty password to use whatever is in the keychain. If the connection
// authenticates with a password and none can be found, this returns an error
// wrapping transport.ErrPasswordRequired, and the caller is expected to ask the
// user and call again.
func (app *App) ConnectSession(
	connectionID string,
	password string,
	cols uint16,
	rows uint16,
) error {
	connection, err := app.store.GetConnection(connectionID)
	if err != nil {
		return err
	}

	workspace, err := app.store.GetWorkspace(connection.WorkspaceID)
	if err != nil {
		return err
	}

	if password == "" && connection.NeedsPassword() {
		if stored, storedErr := app.secrets.Get(connectionID); storedErr == nil {
			password = stored
		}
	}

	resolvedAWS := store.ResolveAWS(connection, workspace)

	config := transport.Config{
		Kind:       string(connection.Kind),
		Target:     connection.Target,
		Port:       connection.Port,
		Username:   connection.Username,
		AuthMethod: string(connection.AuthMethod),
		KeyPath:    connection.KeyPath,
		Password:   password,
		AWSProfile: resolvedAWS.Profile,
		AWSRegion:  resolvedAWS.Region,
	}

	app.emitSessionStatus(connectionID, "connecting", "")

	err = app.sessions.Open(
		app.appContext,
		connectionID,
		config,
		transport.Size{Cols: cols, Rows: rows},
		func(chunk []byte) {
			runtime.EventsEmit(app.appContext, "session:output:"+connectionID, string(chunk))
		},
		func(exitErr error) {
			// A session that ends with something to say ended badly. The SSM
			// kinds report failures this way: `aws` starts successfully and
			// only then discovers it cannot continue.
			if exitErr != nil {
				app.emitSessionStatus(connectionID, "error", exitErr.Error())
				return
			}
			app.emitSessionStatus(connectionID, "disconnected", "")
		},
	)
	if err != nil {
		app.emitSessionStatus(connectionID, "error", err.Error())
		return err
	}

	app.emitSessionStatus(connectionID, "connected", "")
	return nil
}

func (app *App) WriteToSession(connectionID string, data string) error {
	return app.sessions.Write(connectionID, []byte(data))
}

func (app *App) ResizeSession(connectionID string, cols uint16, rows uint16) error {
	return app.sessions.Resize(connectionID, transport.Size{Cols: cols, Rows: rows})
}

func (app *App) DisconnectSession(connectionID string) error {
	if err := app.sessions.Close(connectionID); err != nil {
		return err
	}
	app.emitSessionStatus(connectionID, "disconnected", "")
	return nil
}

func (app *App) OpenSessionIDs() []string {
	return app.sessions.OpenIDs()
}

func (app *App) emitSessionStatus(connectionID string, state string, message string) {
	runtime.EventsEmit(app.appContext, "session:status", map[string]string{
		"connectionId": connectionID,
		"state":        state,
		"message":      message,
	})
}

// CheckSSMTools reports whether this machine can open SSM sessions at all, so
// the UI can warn before the user configures one.
func (app *App) CheckSSMTools() error {
	return transport.CheckSSMTools()
}
