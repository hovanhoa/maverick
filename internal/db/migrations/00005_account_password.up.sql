-- Up migration: password-based login for accounts, alongside API keys.
ALTER TABLE account ADD COLUMN IF NOT EXISTS password_hash TEXT;
