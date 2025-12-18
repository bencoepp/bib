-- Rollback audit log for SQLite

DROP TRIGGER IF EXISTS audit_no_delete;
DROP TRIGGER IF EXISTS audit_no_modify;

ALTER TABLE audit_log RENAME TO audit_log_backup;

-- Note: To permanently delete, manually DROP the backup table:
-- DROP TABLE IF EXISTS audit_log_backup;

