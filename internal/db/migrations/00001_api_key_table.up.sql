-- Up migration: api_key table for API key issuance and lookup.
CREATE TABLE IF NOT EXISTS api_key (
    id         TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES account(id),
    key_hash   TEXT NOT NULL UNIQUE,
    prefix     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS api_key_account_id_idx ON api_key(account_id);
