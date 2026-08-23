-- Down migration: drop password_hash from account.
ALTER TABLE account DROP COLUMN IF EXISTS password_hash;
