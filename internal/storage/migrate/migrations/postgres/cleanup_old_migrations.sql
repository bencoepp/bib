-- Cleanup script for old schema_migrations table
-- Run this if you're upgrading from the old migration system to Phase 2.6

-- Check if old table exists with wrong structure
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_name = 'schema_migrations'
    ) THEN
        -- Check if it has the old structure (applied_at column)
        IF EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_name = 'schema_migrations'
            AND column_name = 'applied_at'
        ) THEN
            RAISE NOTICE 'Found old schema_migrations table with legacy structure';
            RAISE NOTICE 'Renaming to schema_migrations_old for backup';

            -- Rename the old table
            ALTER TABLE schema_migrations RENAME TO schema_migrations_old;

            RAISE NOTICE 'Old migration history preserved in schema_migrations_old';
            RAISE NOTICE 'New migration system will use bib_schema_migrations table';
        ELSE
            RAISE NOTICE 'schema_migrations table exists but structure is unknown';
            RAISE NOTICE 'Please review manually before proceeding';
        END IF;
    ELSE
        RAISE NOTICE 'No old schema_migrations table found - clean start';
    END IF;
END $$;

-- The new migration system will automatically create:
-- CREATE TABLE bib_schema_migrations (
--     version bigint NOT NULL PRIMARY KEY,
--     dirty boolean NOT NULL
-- );

-- Note: You can safely DROP TABLE schema_migrations_old after verifying
-- that the new migration system is working correctly

