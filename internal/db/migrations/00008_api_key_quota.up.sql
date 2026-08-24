-- Up migration: add a per-key monthly token budget, alongside the existing
-- per-team and per-account ones.
ALTER TABLE api_key ADD COLUMN IF NOT EXISTS monthly_token_budget INTEGER;
