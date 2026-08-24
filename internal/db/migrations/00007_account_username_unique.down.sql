-- Down migration: drop the unique username index.
DROP INDEX IF EXISTS account_username_unique_idx;
