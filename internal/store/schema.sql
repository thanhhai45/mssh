CREATE TABLE workspaces (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT 'slate',
    aws_profile TEXT NOT NULL DEFAULT '',   -- mặc định cho connection bên trong
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

    -- ssh: hostname hoặc IP.  ssm / ssm-ssh: instance-id (i-0abc...)
    target       TEXT NOT NULL,

    -- chỉ dùng cho ssh và ssm-ssh
    port         INTEGER NOT NULL DEFAULT 22,
    username     TEXT NOT NULL DEFAULT '',
    auth_method  TEXT NOT NULL DEFAULT 'agent'
                 CHECK (auth_method IN ('agent', 'key')),
    key_path     TEXT NOT NULL DEFAULT '',

    -- chỉ dùng cho ssm và ssm-ssh;  '' = kế thừa workspace
    aws_profile  TEXT NOT NULL DEFAULT '',
    aws_region   TEXT NOT NULL DEFAULT '',

    -- lối thoát hiểm cho tuỳ chọn hiếm (vd document_name khác mặc định)
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