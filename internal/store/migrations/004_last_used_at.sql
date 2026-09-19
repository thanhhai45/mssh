-- Records when a connection was last opened, so the home page can put the
-- machines you actually use at the top.
--
-- No table rebuild this time: ADD COLUMN is cheap, and nothing here touches a
-- CHECK constraint. A NOT NULL column needs a constant default, and 0 reads
-- naturally as "never opened" — the same epoch-seconds scale as created_at.

ALTER TABLE connections ADD COLUMN last_used_at INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_connections_last_used
    ON connections(last_used_at DESC);
