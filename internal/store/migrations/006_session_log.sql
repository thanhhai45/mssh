-- One row per session: who reached which machine, when, for how long, and how
-- it ended.
--
-- The only table in this database without foreign keys, and that is the point.
-- Every other table cascades, because deleting a workspace should take its
-- connections with it. Here that would be wrong: deleting a server must not
-- delete the evidence that someone was on it.
--
-- For the same reason the names, target and AWS settings are copied in rather
-- than joined from connections at read time. A record that changes when
-- somebody later renames a machine is not a record. This is the one place in
-- the app where copying data is right.

CREATE TABLE session_log (
    id                     TEXT    PRIMARY KEY,
    connection_id          TEXT    NOT NULL,
    workspace_id           TEXT    NOT NULL,
    connection_name        TEXT    NOT NULL,
    workspace_name         TEXT    NOT NULL,
    kind                   TEXT    NOT NULL,
    target                 TEXT    NOT NULL,
    username               TEXT    NOT NULL DEFAULT '',
    aws_profile            TEXT    NOT NULL DEFAULT '',
    aws_region             TEXT    NOT NULL DEFAULT '',
    aws_credentials_source TEXT    NOT NULL DEFAULT '',
    opened_at              INTEGER NOT NULL,
    -- 0 while the session is open, the same "not yet" convention as
    -- connections.last_used_at.
    closed_at              INTEGER NOT NULL DEFAULT 0,
    end_reason             TEXT    NOT NULL DEFAULT '',
    end_message            TEXT    NOT NULL DEFAULT '',
    -- The session this one replaced after an automatic reconnect (round 18),
    -- so a flaky afternoon does not read as thirty separate logins.
    reconnect_of           TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX idx_session_log_opened ON session_log(opened_at DESC);
CREATE INDEX idx_session_log_connection ON session_log(connection_id, opened_at DESC);
