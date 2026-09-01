import * as App from '../../wailsjs/go/main/App';
import type {store} from '../../wailsjs/go/models';

export type Workspace = store.Workspace;
export type WorkspaceInput = store.WorkspaceInput;
export type Connection = store.Connection;
export type ConnectionInput = store.ConnectionInput;
export type ParsedSSHCommand = store.ParsedSSHCommand;
export type ResolvedAWS = store.ResolvedAWS;

export type ConnectionKind = 'ssh' | 'ssm' | 'ssm-ssh';
export type AuthMethod = 'agent' | 'key';

export const CONNECTION_KINDS: ConnectionKind[] = ['ssh', 'ssm', 'ssm-ssh'];

export const KIND_META: Record<ConnectionKind, { label: string; hint: string }> = {
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
    hint: 'Your own user, tunnelled through Session Manager.',
  }
}

export function describeConnection(c: Connection): string {
  if (c.kind === 'ssm') return c.target;
  return `${c.username}@${c.target}:${c.port}`;
}

/*--------------------CALLS------------------------*/

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
}

export function errorMessage(err: unknown): string {
  if (typeof err === 'string') return err;
  if (err instanceof Error) return err.message;
  return String(err);
}