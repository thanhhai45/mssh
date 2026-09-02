-- SQLite cannot ALTER a CHECK constraint, so the table has to be rebuilt.
-- Create, copy, drop, rename, then put the indexes back.

CREATE TABLE connections_new (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,

    kind         TEXT NOT NULL DEFAULT 'ssh'
                 CHECK (kind IN ('ssh', 'ssm', 'ssm-ssh')),

    target       TEXT NOT NULL,

    port         INTEGER NOT NULL DEFAULT 22,
    username     TEXT NOT NULL DEFAULT '',
    auth_method  TEXT NOT NULL DEFAULT ''
                 CHECK (auth_method IN ('', 'agent', 'key', 'password')),
    key_path     TEXT NOT NULL DEFAULT '',

    aws_profile  TEXT NOT NULL DEFAULT '',
    aws_region   TEXT NOT NULL DEFAULT '',

    extra        TEXT NOT NULL DEFAULT '{}',

    color        TEXT NOT NULL DEFAULT '',
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

INSERT INTO connections_new
SELECT id, workspace_id, name, kind, target, port, username, auth_method,
       key_path, aws_profile, aws_region, extra, color, sort_order,
       created_at, updated_at
FROM connections;

DROP TABLE connections;

ALTER TABLE connections_new RENAME TO connections;

-- Dropping the old table dropped its index with it.
CREATE INDEX idx_connections_workspace ON connections(workspace_id, sort_order);