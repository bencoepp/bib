-- Cleanup script for old schema_migrations table (SQLite)
-- Run this if you're upgrading from the old migration system to Phase 2.6

-- Check and rename old table if it exists
-- SQLite doesn't support dynamic SQL like PostgreSQL's DO blocks,
-- so this needs to be run conditionally

-- First, check if the old table exists:
-- SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations';

-- If it exists, rename it:
ALTER TABLE schema_migrations RENAME TO schema_migrations_old;

-- The new migration system will automatically create:
-- CREATE TABLE bib_schema_migrations (
--     version INTEGER PRIMARY KEY NOT NULL,
--     dirty INTEGER NOT NULL
-- );

-- Note: You can safely DROP TABLE schema_migrations_old after verifying
-- that the new migration system is working correctly:
-- DROP TABLE IF EXISTS schema_migrations_old;

