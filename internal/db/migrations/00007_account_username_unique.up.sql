-- Up migration: enforce unique usernames. /login looks an account up by
-- username alone (see internal/http/login.go), so two accounts sharing a
-- username make that lookup resolve to an arbitrary one of them.
CREATE UNIQUE INDEX IF NOT EXISTS account_username_unique_idx
    ON account ((account->>'username'));
