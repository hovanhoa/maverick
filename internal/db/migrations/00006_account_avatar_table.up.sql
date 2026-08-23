-- Up migration: profile picture storage for accounts, stored directly in
-- Postgres rather than an external object store - simple enough for small
-- avatar images and avoids a new infra dependency.
--
-- data is base64-encoded TEXT rather than BYTEA: this connection pool runs
-- pgx in QueryExecModeSimpleProtocol (for pgbouncer compatibility), which
-- inlines parameters as SQL text rather than binding them out-of-band -
-- raw binary bytes fail Postgres's UTF8 validation that way, so everything
-- in this schema stays as valid-UTF8 TEXT/JSONB, avatars included.
CREATE TABLE IF NOT EXISTS account_avatar (
    account_id   TEXT PRIMARY KEY REFERENCES account(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL,
    data         TEXT NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
