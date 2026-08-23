-- Down migration: drop last_used_at from api_key.
ALTER TABLE api_key DROP COLUMN IF EXISTS last_used_at;
