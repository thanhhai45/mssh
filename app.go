package main

import (
	"context"
	"errors"
	"fmt"
	"mssh/internal/secrets"
	"mssh/internal/session"
	"mssh/internal/store"
	"mssh/internal/transport"
	"os"
	"strings"
	"sync"
	"time"

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

	// Host keys waiting for the user to say whether to trust them, by a token
	// the frontend holds. The key itself never crosses into JavaScript: the
	// frontend can only answer yes, and Go records exactly what it was shown.
	hostKeysMutex   sync.Mutex
	pendingHostKeys map[string]*transport.UnknownHostKeyError
}

// NewApp creates a new App application struct
func NewApp(dataStore *store.Store, vault secrets.Vault, sessions *session.Manager) *App {
	return &App{
		store:           dataStore,
		secrets:         vault,
		sessions:        sessions,
		pendingHostKeys: make(map[string]*transport.UnknownHostKeyError),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (app *App) startup(startupContext context.Context) {
	app.appContext = startupContext

	// Running the user's login shell costs most of a second. Do it now, in the
	// background, rather than when somebody is waiting for a connection.
	transport.WarmShellEnvironment()
}

// shutdown runs before wails.Run returns, and so before main's deferred
// st.Close: CloseAll can still write "app-quit" for every open session.
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

// ---------- AWS keys kept on a workspace ----------

// storedAWSCredentials returns the workspace's own keys, or nil when it lets
// the AWS CLI find credentials itself.
//
// A workspace set to stored keys that has none is an error for the kinds that
// run aws — falling back to the CLI would connect with whatever the machine
// happens to have configured, which may be another account. A plain ssh
// connection in that workspace needs no keys and is left alone.
func (app *App) storedAWSCredentials(
	connection store.Connection,
	workspace store.Workspace,
) (*transport.AWSCredentials, error) {
	if workspace.AWSCredentialsSource != store.AWSCredentialsStored {
		return nil, nil
	}

	secretAccessKey, err := app.store.GetWorkspaceSecret(workspace.ID, store.SecretAWSSecretAccessKey)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	if secretAccessKey == "" || workspace.AWSAccessKeyID == "" {
		if connection.Kind.UsesAWS() {
			return nil, fmt.Errorf(
				"workspace %q is set to use AWS keys stored in mssh, but none are "+
					"saved - add them in the workspace settings", workspace.Name)
		}
		return nil, nil
	}

	sessionToken, err := app.store.GetWorkspaceSecret(workspace.ID, store.SecretAWSSessionToken)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	return &transport.AWSCredentials{
		AccessKeyID:     workspace.AWSAccessKeyID,
		SecretAccessKey: secretAccessKey,
		SessionToken:    sessionToken,
	}, nil
}

// SetWorkspaceAWSSecret stores a workspace's secret access key, and its session
// token when there is one. An empty token removes any stored before: moving
// from temporary credentials to long-lived ones must not leave a stale token
// behind that the CLI would still send.
//
// Secrets come in through here and never go back out. There is no
// GetWorkspaceAWSSecret, on purpose; the frontend learns only whether one
// exists, from HasWorkspaceAWSSecret.
func (app *App) SetWorkspaceAWSSecret(workspaceID string, secretAccessKey string, sessionToken string) error {
	if _, err := app.store.GetWorkspace(workspaceID); err != nil {
		return err
	}
	if err := app.store.SetWorkspaceSecret(
		workspaceID, store.SecretAWSSecretAccessKey, secretAccessKey); err != nil {
		return err
	}
	if sessionToken == "" {
		return app.store.DeleteWorkspaceSecret(workspaceID, store.SecretAWSSessionToken)
	}
	return app.store.SetWorkspaceSecret(workspaceID, store.SecretAWSSessionToken, sessionToken)
}

// HasWorkspaceAWSSecret reports whether a secret key is saved, so the UI can say
// "saved" rather than show an empty field.
func (app *App) HasWorkspaceAWSSecret(workspaceID string) bool {
	return app.store.HasWorkspaceSecret(workspaceID, store.SecretAWSSecretAccessKey)
}

// ClearWorkspaceAWSSecret forgets the secret key and any session token.
func (app *App) ClearWorkspaceAWSSecret(workspaceID string) error {
	if err := app.store.DeleteWorkspaceSecret(
		workspaceID, store.SecretAWSSecretAccessKey); err != nil {
		return err
	}
	return app.store.DeleteWorkspaceSecret(workspaceID, store.SecretAWSSessionToken)
}

// PreviewAWSFromShell shows what Import would save, without saving it and
// without the secret.
func (app *App) PreviewAWSFromShell() (transport.AWSShellPreview, error) {
	preview, _, err := transport.AWSFromShell()
	return preview, err
}

// ImportAWSFromShell saves the keys found in the login shell to a workspace and
// switches it to stored credentials.
//
// It runs the shell again rather than taking anything from the frontend: the
// secret never crossed into JavaScript, so there is nothing there to take.
func (app *App) ImportAWSFromShell(workspaceID string) error {
	preview, credentials, err := transport.AWSFromShell()
	if err != nil {
		return err
	}
	if credentials == nil {
		return fmt.Errorf("no AWS key pair was found in your login shell")
	}

	workspace, err := app.store.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	region := workspace.AWSRegion
	if region == "" {
		// Fill the region only when the workspace has none; never overwrite
		// a choice made by hand.
		region = preview.Region
	}
	// Checked before anything is saved: the store would refuse the switch to
	// stored keys anyway, but only after the secret had already been written.
	if region == "" {
		return fmt.Errorf("%w — your shell exports no AWS_REGION either, so "+
			"set one on the workspace first", store.ErrStoredKeysNeedRegion)
	}

	// The secret first. If switching the workspace over then fails, it is
	// left on the CLI with an unused secret, which is harmless; the other
	// order could leave it set to stored keys with none saved.
	if err := app.SetWorkspaceAWSSecret(
		workspaceID, credentials.SecretAccessKey, credentials.SessionToken); err != nil {
		return err
	}

	_, err = app.store.UpdateWorkspace(workspaceID, store.WorkspaceInput{
		Name:                 workspace.Name,
		Color:                workspace.Color,
		AWSProfile:           workspace.AWSProfile,
		AWSRegion:            region,
		AWSCredentialsSource: store.AWSCredentialsStored,
		AWSAccessKeyID:       credentials.AccessKeyID,
	})
	return err
}

// ---------- Host keys seen for the first time ----------

// hostKeyQuestionMarker opens the error ConnectSession returns for a host it
// has never seen, followed by the token for HostKeyQuestion and TrustHostKey.
// Must match HOST_KEY_QUESTION in frontend/src/lib/api.ts.
const hostKeyQuestionMarker = "mssh:host-key:"

// askAboutHostKey parks an unknown key until the user answers, and returns
// the token that names it. Entries are a few hundred bytes and only appear
// when someone presses Connect, so an unanswered one is simply left behind.
func (app *App) askAboutHostKey(unknown *transport.UnknownHostKeyError) string {
	token := uuid.NewString()
	app.hostKeysMutex.Lock()
	app.pendingHostKeys[token] = unknown
	app.hostKeysMutex.Unlock()
	return token
}

// HostKeyQuestion returns what the trust dialog shows: the host, the key type
// and its SHA256 fingerprint — the same line ssh prints when it asks.
func (app *App) HostKeyQuestion(token string) (transport.HostKeyPrompt, error) {
	app.hostKeysMutex.Lock()
	unknown := app.pendingHostKeys[token]
	app.hostKeysMutex.Unlock()

	if unknown == nil {
		return transport.HostKeyPrompt{}, fmt.Errorf("this host key question has expired; connect again")
	}
	return unknown.Prompt(), nil
}

// TrustHostKey records the key the user was shown in known_hosts — the file
// the ssh command reads too — so the next connection goes straight through.
func (app *App) TrustHostKey(token string) error {
	app.hostKeysMutex.Lock()
	unknown := app.pendingHostKeys[token]
	delete(app.pendingHostKeys, token)
	app.hostKeysMutex.Unlock()

	if unknown == nil {
		return fmt.Errorf("this host key question has expired; connect again")
	}
	return unknown.Remember()
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

	credentials, err := app.storedAWSCredentials(connection, workspace)
	if err != nil {
		return "", err
	}

	config := transport.Config{
		Kind:           string(connection.Kind),
		Target:         connection.Target,
		Port:           connection.Port,
		Username:       connection.Username,
		AuthMethod:     string(connection.AuthMethod),
		KeyPath:        connection.KeyPath,
		Password:       password,
		AWSProfile:     resolvedAWS.Profile,
		AWSRegion:      resolvedAWS.Region,
		Extra:          connection.Extra,
		AWSCredentials: credentials,
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
		var unknown *transport.UnknownHostKeyError
		if errors.As(err, &unknown) {
			// Not a failure to report but a question to ask, the way ssh asks
			// yes/no. Nothing started, so the session simply never existed.
			app.emitSessionStatus(sessionID, connectionID, "disconnected", "")
			return "", fmt.Errorf("%s%s %w", hostKeyQuestionMarker, app.askAboutHostKey(unknown), err)
		}
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

// ---------- Session log ----------

// ListSessionLog returns the session log, newest first, narrowed by filter.
// Zero values in the filter mean "any".
func (app *App) ListSessionLog(filter store.SessionLogFilter) ([]store.SessionLogEntry, error) {
	return app.store.ListSessionLog(filter)
}

// ExportSessionLog asks where to save, then writes the matching part of the
// log there as CSV or JSON. It returns the path written, or "" when the user
// cancelled the dialog — not an error.
//
// The file is written here rather than in JavaScript: the log never has to be
// held by the page, and a new file gets the same owner-only permissions as the
// database it came from.
func (app *App) ExportSessionLog(filter store.SessionLogFilter, format string) (string, error) {
	format = strings.ToLower(format)
	if format != store.ExportCSV && format != store.ExportJSON {
		return "", fmt.Errorf("unknown export format %q", format)
	}

	path, err := runtime.SaveFileDialog(app.appContext, runtime.SaveDialogOptions{
		Title:           "Export session log",
		DefaultFilename: fmt.Sprintf("mssh-sessions-%s.%s", time.Now().Format("2006-01-02"), format),
		Filters: []runtime.FileFilter{
			{DisplayName: strings.ToUpper(format), Pattern: "*." + format},
		},
	})
	if err != nil || path == "" {
		return "", err
	}

	entries, err := app.store.ListSessionLog(filter)
	if err != nil {
		return "", err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", path, err)
	}
	if err := store.WriteSessionLog(file, format, entries); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
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
