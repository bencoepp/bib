-- Rollback audit log (safe down migration)

DO $$
DECLARE
    backup_suffix TEXT := '_backup_' || to_char(NOW(), 'YYYYMMDD_HH24MISS');
BEGIN
    -- Drop triggers and functions first
    DROP TRIGGER IF EXISTS audit_immutable ON audit_log;
    DROP FUNCTION IF EXISTS audit_no_modify();

    -- Rename table for safe keeping
    EXECUTE 'ALTER TABLE IF EXISTS audit_log RENAME TO audit_log' || backup_suffix;

    RAISE NOTICE 'Audit log renamed to: audit_log%', backup_suffix;
    RAISE NOTICE 'To permanently delete, manually DROP the backup table';
    RAISE WARNING 'Audit trail has been preserved but is no longer active';
END $$;

