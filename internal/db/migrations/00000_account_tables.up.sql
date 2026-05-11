-- Up migration: account and team tables with JSONB payloads and timestamps.
CREATE TABLE IF NOT EXISTS account (
    id         TEXT PRIMARY KEY,
    account    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS team (
    id         TEXT PRIMARY KEY,
    team       JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
