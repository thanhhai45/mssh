-- Credentials move out of the OS keychain and into this database.
--
-- The keychain differs on every platform and is sometimes missing entirely on
-- Linux, and each of those cases was a separate way to fail. One table behaves
-- the same everywhere. The AWS CLI keeps this very same secret in plain text in
-- ~/.aws/credentials; this directory is created 0700, which is the same bar.
--
-- Secrets live in tables of their own, never as columns of connections or
-- workspaces. Those two rows travel to the frontend whole; a secret column
-- would be one edit to the column list away from doing the same.
--
-- From here on, connections and workspaces both have children that cascade.
-- Never rebuild either the way 003 rebuilt connections: with foreign keys on,
-- DROP TABLE deletes every row first, the cascade follows, and every saved
-- password goes with it. TestNoMigrationDropsAParentTable is there to stop it.

CREATE TABLE connection_secrets (
    connection_id TEXT    NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
    name          TEXT    NOT NULL,
    value         TEXT    NOT NULL,
    updated_at    INTEGER NOT NULL,
    PRIMARY KEY (connection_id, name)
);

CREATE TABLE workspace_secrets (
    workspace_id TEXT    NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT    NOT NULL,
    value        TEXT    NOT NULL,
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (workspace_id, name)
);

-- Whether a workspace lets the AWS CLI find its own credentials ('cli', as
-- every workspace did before this) or uses keys held here ('stored').
-- Validated in Go rather than with a CHECK: adding a column carrying a CHECK
-- constraint is the one part of ALTER TABLE that varies between SQLite builds.
ALTER TABLE workspaces ADD COLUMN aws_credentials_source TEXT NOT NULL DEFAULT 'cli';

-- An access key id is an identifier, not a secret: it appears in CloudTrail and
-- in the AWS console. It sits in the ordinary table; its secret does not.
ALTER TABLE workspaces ADD COLUMN aws_access_key_id TEXT NOT NULL DEFAULT '';
