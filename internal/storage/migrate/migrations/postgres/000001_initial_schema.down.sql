-- Rollback initial schema
-- This is a SAFE down migration that renames tables instead of dropping them
-- Tables are renamed with _backup_ prefix and timestamp for potential recovery

-- Drop foreign key constraints first
ALTER TABLE datasets DROP CONSTRAINT IF EXISTS fk_latest_version;

-- Rename tables for safe keeping (instead of DROP)
-- Admin can later DROP these manually if confirmed data loss is acceptable
DO $$
DECLARE
    backup_suffix TEXT := '_backup_' || to_char(NOW(), 'YYYYMMDD_HH24MISS');
BEGIN
    -- Rename in reverse dependency order
    EXECUTE 'ALTER TABLE IF EXISTS job_results RENAME TO job_results' || backup_suffix;
    EXECUTE 'ALTER TABLE IF EXISTS jobs RENAME TO jobs' || backup_suffix;
    EXECUTE 'ALTER TABLE IF EXISTS chunks RENAME TO chunks' || backup_suffix;
    EXECUTE 'ALTER TABLE IF EXISTS dataset_versions RENAME TO dataset_versions' || backup_suffix;
    EXECUTE 'ALTER TABLE IF EXISTS datasets RENAME TO datasets' || backup_suffix;
    EXECUTE 'ALTER TABLE IF EXISTS topics RENAME TO topics' || backup_suffix;
    EXECUTE 'ALTER TABLE IF EXISTS nodes RENAME TO nodes' || backup_suffix;
    EXECUTE 'ALTER TABLE IF EXISTS blobs RENAME TO blobs' || backup_suffix;

    RAISE NOTICE 'Tables renamed with suffix: %', backup_suffix;
    RAISE NOTICE 'To permanently delete data, manually DROP these tables';
END $$;

-- Note: Backup tables can be dropped manually with:
-- DROP TABLE IF EXISTS nodes_backup_YYYYMMDD_HHMMSS CASCADE;

