-- Up migration: usage_event table for LLM proxy metering (Phase 4).
-- A normal relational table, not the JSONB payload pattern used for
-- account/team, since these rows are high-volume, append-only, and queried
-- via aggregate (SUM/COUNT) rather than looked up by id.
-- account_id/team_id are ON DELETE SET NULL, not CASCADE: usage_event is a
-- durable metering/billing trail, so deleting an account or team should
-- orphan its past usage rows rather than either erasing billing history or
-- blocking the delete outright with a raw FK violation.
CREATE TABLE IF NOT EXISTS usage_event (
    id                TEXT PRIMARY KEY,
    request_id        TEXT NOT NULL,
    account_id        TEXT REFERENCES account(id) ON DELETE SET NULL,
    team_id           TEXT REFERENCES team(id) ON DELETE SET NULL,
    provider          TEXT NOT NULL,
    model             TEXT NOT NULL,
    prompt_tokens     INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    total_tokens      INTEGER NOT NULL,
    cost_usd          NUMERIC(14, 6) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS usage_event_team_created_idx ON usage_event(team_id, created_at);
CREATE INDEX IF NOT EXISTS usage_event_account_created_idx ON usage_event(account_id, created_at);
