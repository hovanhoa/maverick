-- Up migration: request_log table, a durable audit trail of every LLM
-- proxy call attempt - including ones that never reached a provider (bad
-- model format, team allowlist block, quota exceeded, policy deny) - with
-- the full raw request/response content. This is the counterpart to
-- usage_event, which only records completed, successful calls.
-- Same plain-relational-table style as usage_event, for the same reason:
-- high-volume, append-only, queried by page rather than looked up by id.
-- account_id/team_id are ON DELETE SET NULL, not CASCADE, matching
-- usage_event's rationale: an audit trail should outlive the entity it
-- references rather than being erased or blocking the delete.
CREATE TABLE IF NOT EXISTS request_log (
    id                TEXT PRIMARY KEY,
    request_id        TEXT NOT NULL,
    account_id        TEXT REFERENCES account(id) ON DELETE SET NULL,
    team_id           TEXT REFERENCES team(id) ON DELETE SET NULL,
    provider          TEXT,
    model             TEXT,
    requested_model   TEXT NOT NULL,
    status            TEXT NOT NULL,
    error_kind        TEXT,
    error_message     TEXT,
    stream            BOOLEAN NOT NULL DEFAULT FALSE,
    request_body      JSONB NOT NULL,
    response_body     JSONB,
    prompt_tokens     INTEGER,
    completion_tokens INTEGER,
    total_tokens      INTEGER,
    cost_usd          NUMERIC(14, 6),
    latency_ms        INTEGER NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS request_log_team_created_idx ON request_log(team_id, created_at);
CREATE INDEX IF NOT EXISTS request_log_account_created_idx ON request_log(account_id, created_at);
