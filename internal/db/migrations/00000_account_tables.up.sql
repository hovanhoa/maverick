-- Up migration: create account table with id, JSONB payload, and timestamps.
CREATE TABLE IF NOT EXISTS account (
    id         TEXT PRIMARY KEY,
    data       JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
