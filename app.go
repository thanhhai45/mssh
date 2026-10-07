package main

import (
	"context"
	"mssh/internal/secrets"
	"mssh/internal/session"
	"mssh/internal/store"
	"mssh/internal/transport"

	"github.com/google/uuid"
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

	// Running the user's login shell costs most of a second. Do it now, in the
	// background, rather than when somebody is waiting for a connection.
	transport.WarmShellEnvironment()
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
		// Every tab of every machine in here is about to point at nothing.
		app.sessions.CloseConnection(c.ID)
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
	// Closes a gap that predates multi-tab: the session used to keep running
	// with nothing in the UI able to reach it.
	app.sessions.CloseConnection(id)
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

// SetConnectionPassword saves a password in the local database, in a table of
// its own so that it never travels with the connection row to the frontend.
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

/* ---------- Sessions ---------- */
// ConnectSession opens a live connection.
//
// Pass an empty password to use the saved one, if any. If the connection
// authenticates with a password and none can be found, this returns an error
// wrapping transport.ErrPasswordRequired, and the caller is expected to ask the
// user and call again.
func (app *App) ConnectSession(
	connectionID string,
	password string,
	cols uint16,
	rows uint16,
) (string, error) {
	connection, err := app.store.GetConnection(connectionID)
	if err != nil {
		return "", err
	}

	workspace, err := app.store.GetWorkspace(connection.WorkspaceID)
	if err != nil {
		return "", err
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
		Extra:      connection.Extra,
	}

	// The id is made here rather than inside the manager because it is needed
	// before Open returns: it names the channel the output events arrive on,
	// and those start during the dial
	sessionID := uuid.NewString()

	app.emitSessionStatus(sessionID, connectionID, "connecting", "")

	err = app.sessions.Open(
		app.appContext,
		sessionID,
		connectionID,
		config,
		transport.Size{Cols: cols, Rows: rows},
		func(chunk []byte) {
			runtime.EventsEmit(app.appContext, "session:output:"+sessionID, string(chunk))
		},
		func(exitErr error) {
			// A session that ends with something to say ended badly. The SSM
			// kinds report failures this way: `aws` starts successfully and
			// only then discovers it cannot continue.
			if exitErr != nil {
				app.emitSessionStatus(sessionID, connectionID, "error", exitErr.Error())
				return
			}
			app.emitSessionStatus(sessionID, connectionID, "disconnected", "")
		},
	)
	if err != nil {
		app.emitSessionStatus(sessionID, connectionID, "error", err.Error())
		return "", err
	}

	// Best effort, and after the session is up: a bookkeeping failure must not
	// turn a working connection into a reported error.
	_ = app.store.MarkConnectionUsed(connectionID)

	app.emitSessionStatus(sessionID, connectionID, "connected", "")
	return sessionID, nil
}

func (app *App) WriteToSession(sessionID string, data string) error {
	return app.sessions.Write(sessionID, []byte(data))
}

func (app *App) ResizeSession(sessionID string, cols uint16, rows uint16) error {
	return app.sessions.Resize(sessionID, transport.Size{Cols: cols, Rows: rows})
}

func (app *App) DisconnectSession(sessionID string) error {
	// Ask before closing. Close forgets the session, and then there is
	// nothing left to ask
	connectionID := app.sessions.ConnectionOf(sessionID)
	if err := app.sessions.Close(sessionID); err != nil {
		return err
	}
	app.emitSessionStatus(sessionID, connectionID, "disconnected", "")
	return nil
}

// OpenSessionIDs lets the frontend rebuild its tabs after a reload.
func (app *App) OpenSessions() []session.Info {
	return app.sessions.OpenSessions()
}

// emitSessionStatus carries  both ids on purpose. The terminal tab cares
// which session changed; the sidebar, the home page and the breadscrumbs all
// still draw a dot per machine, and they cannot work that out from a session id alone.
func (app *App) emitSessionStatus(
	sessionID string,
	connectionID string,
	state string,
	message string,
) {
	runtime.EventsEmit(app.appContext, "session:status", map[string]string{
		"sessionId":    sessionID,
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

/*------------------ Settings ----------------------------------*/
// GetSetting
func (app *App) GetAllSettings() (map[string]string, error) {
	return app.store.GetAllSettings()
}

// SetSetting
func (app *App) SetSetting(key string, value string) error {
	return app.store.SetSetting(key, value)
}

// DeleteSetting
func (app *App) DeleteSetting(key string) error {
	return app.store.DeleteSetting(key)
}
