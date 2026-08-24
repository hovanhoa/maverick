-- Down migration: drop the per-key monthly token budget column.
ALTER TABLE api_key DROP COLUMN IF EXISTS monthly_token_budget;
