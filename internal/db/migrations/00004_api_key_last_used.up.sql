-- Up migration: track when an API key was last used to authenticate a request.
ALTER TABLE api_key ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;
