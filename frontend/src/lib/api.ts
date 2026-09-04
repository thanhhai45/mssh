import * as App from '../../wailsjs/go/main/App';
import type {store} from '../../wailsjs/go/models';

/* ---------------- Types ---------------- */

export type Workspace = store.Workspace;
export type WorkspaceInput = store.WorkspaceInput;
export type Connection = store.Connection;
export type ConnectionInput = store.ConnectionInput;
export type ParsedSSHCommand = store.ParsedSSHCommand;
export type ResolvedAWS = store.ResolvedAWS;

/** Go sends these as plain strings. These are the sets it actually accepts. */
export type ConnectionKind = 'ssh' | 'ssm' | 'ssm-ssh';
export type AuthMethod = 'agent' | 'key' | 'password';

export const CONNECTION_KINDS: ConnectionKind[] = ['ssh', 'ssm', 'ssm-ssh'];
export const AUTH_METHODS: AuthMethod[] = ['agent', 'key', 'password'];

export const KIND_META: Record<ConnectionKind, {label: string; hint: string}> = {
  'ssh': {
    label: 'Direct SSH',
    hint: 'Straight to host:port, using your key or ssh-agent',
  },
  'ssm': {
    label: 'AWS SSM',
    hint: 'Through Session Manager. No key, no open port, lands as ssm-user',
  },
  'ssm-ssh': {
    label: 'SSH over SSM',
    hint: 'Your own user, tunnelled through Session Manager',
  },
};

export const AUTH_META: Record<AuthMethod, {label: string; hint: string}> = {
  agent: {
    label: 'SSH agent',
    hint: 'Use a key already loaded in ssh-agent',
  },
  key: {
    label: 'Key file',
    hint: 'Point at a private key on disk',
  },
  password: {
    label: 'Password',
    hint: 'Kept in your login keychain, never in the mssh database',
  },
};

/* ---------------- Kind helpers ---------------- */

export function isConnectionKind(value: string): value is ConnectionKind {
  return (CONNECTION_KINDS as readonly string[]).includes(value);
}

/** Falls back to ssh so a kind the UI does not know about still renders. */
export function kindMeta(kind: string) {
  return isConnectionKind(kind) ? KIND_META[kind] : KIND_META.ssh;
}

/** Mirrors ConnectionKind.UsesSSH in Go: needs a username, port and credentials. */
export function usesSSH(kind: string): boolean {
  return kind === 'ssh' || kind === 'ssm-ssh';
}

/** Mirrors ConnectionKind.UsesAWS in Go: needs an AWS profile and region. */
export function usesAWS(kind: string): boolean {
  return kind === 'ssm' || kind === 'ssm-ssh';
}

/** One-line subtitle, the way the sidebar shows it. */
export function describeConnection(c: Connection): string {
  switch (c.kind) {
    case 'ssm':
      return c.awsRegion ? `${c.target} · ${c.awsRegion}` : c.target;
    case 'ssm-ssh':
      return `${c.username}@${c.target}`;
    default:
      return `${c.username}@${c.target}:${c.port}`;
  }
}

/* ---------------- Form helpers ---------------- */

/** A blank form. Every field is present so the form state never has holes. */
export function emptyConnectionInput(kind: ConnectionKind = 'ssh'): ConnectionInput {
  return {
    name: '',
    kind,
    target: '',
    port: 22,
    username: '',
    authMethod: 'agent',
    keyPath: '',
    awsProfile: '',
    awsRegion: '',
    extra: '',
    color: '',
  };
}

/** Turns a stored connection back into something the form can edit. */
export function toConnectionInput(c: Connection): ConnectionInput {
  return {
    name: c.name,
    kind: c.kind,
    target: c.target,
    port: c.port,
    username: c.username,
    authMethod: c.authMethod,
    keyPath: c.keyPath,
    awsProfile: c.awsProfile,
    awsRegion: c.awsRegion,
    extra: c.extra,
    color: c.color,
  };
}

/* ---------------- Calls ---------------- */

export const api = {
  listWorkspaces: (): Promise<Workspace[]> => App.ListWorkspaces(),
  getWorkspace: (id: string): Promise<Workspace> => App.GetWorkspace(id),
  createWorkspace: (input: WorkspaceInput): Promise<Workspace> => App.CreateWorkspace(input),
  updateWorkspace: (id: string, input: WorkspaceInput): Promise<Workspace> => App.UpdateWorkspace(id, input),
  deleteWorkspace: (id: string): Promise<void> => App.DeleteWorkspace(id),
  reorderWorkspaces: (ids: string[]): Promise<void> => App.ReorderWorkspaces(ids),

  listConnections: (workspaceId: string): Promise<Connection[]> => App.ListConnections(workspaceId),
  getConnection: (id: string): Promise<Connection> => App.GetConnection(id),
  createConnection: (workspaceId: string, input: ConnectionInput): Promise<Connection> => App.CreateConnection(workspaceId, input),
  updateConnection: (id: string, input: ConnectionInput): Promise<Connection> => App.UpdateConnection(id, input),
  deleteConnection: (id: string): Promise<void> => App.DeleteConnection(id),
  moveConnection: (id: string, toWorkspaceId: string): Promise<void> => App.MoveConnection(id, toWorkspaceId),
  parseSSHCommand: (command: string): Promise<ParsedSSHCommand> => App.ParseSSHCommand(command),
  resolveAWS: (connectionId: string): Promise<ResolvedAWS> => App.ResolveAWSForConnection(connectionId),

  setConnectionPassword: (id: string, password: string): Promise<void> => App.SetConnectionPassword(id, password),
  deleteConnectionPassword: (id: string): Promise<void> => App.DeleteConnectionPassword(id),
  hasConnectionPassword: (id: string): Promise<boolean> => App.HasConnectionPassword(id),
};

/** Wails rejects with a bare string, not an Error. Normalise it for the UI. */
export function errorMessage(err: unknown): string {
  if (typeof err === 'string') return err;
  if (err instanceof Error) return err.message;
  return String(err);
}