CREATE TABLE workspaces (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT 'slate',
    aws_profile TEXT NOT NULL DEFAULT '',
    aws_region  TEXT NOT NULL DEFAULT '',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE connections (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,

    kind         TEXT NOT NULL DEFAULT 'ssh'
                 CHECK (kind IN ('ssh', 'ssm', 'ssm-ssh')),

    target       TEXT NOT NULL,

    port         INTEGER NOT NULL DEFAULT 22,
    username     TEXT NOT NULL DEFAULT '',
    auth_method  TEXT NOT NULL DEFAULT ''
                 CHECK (auth_method IN ('', 'agent', 'key')),
    key_path     TEXT NOT NULL DEFAULT '',

    aws_profile  TEXT NOT NULL DEFAULT '',
    aws_region   TEXT NOT NULL DEFAULT '',

    extra        TEXT NOT NULL DEFAULT '{}',

    color        TEXT NOT NULL DEFAULT '',
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE INDEX idx_connections_workspace ON connections(workspace_id, sort_order);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);