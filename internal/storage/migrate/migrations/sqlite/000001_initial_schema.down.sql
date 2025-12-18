-- Rollback initial schema for SQLite
-- Renames tables for safe keeping

-- Get current timestamp for backup suffix
-- Note: SQLite doesn't support dynamic table names in DDL, so we use ALTER TABLE directly

ALTER TABLE job_results RENAME TO job_results_backup;
ALTER TABLE jobs RENAME TO jobs_backup;
ALTER TABLE chunks RENAME TO chunks_backup;
ALTER TABLE dataset_versions RENAME TO dataset_versions_backup;
ALTER TABLE datasets RENAME TO datasets_backup;
ALTER TABLE topics RENAME TO topics_backup;
ALTER TABLE nodes RENAME TO nodes_backup;
ALTER TABLE cache_metadata RENAME TO cache_metadata_backup;
ALTER TABLE blobs RENAME TO blobs_backup;

-- Note: To permanently delete, manually DROP tables:
-- DROP TABLE IF EXISTS nodes_backup;
-- DROP TABLE IF EXISTS topics_backup;
-- etc.

