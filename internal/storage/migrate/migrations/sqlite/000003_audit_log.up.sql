-- Audit log for SQLite (simplified, append-only)

CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    node_id TEXT NOT NULL,
    job_id TEXT,
    operation_id TEXT NOT NULL,
    role_used TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('SELECT', 'INSERT', 'UPDATE', 'DELETE', 'DDL', 'AUTH', 'ADMIN')),
    table_name TEXT,
    query_hash TEXT,
    rows_affected INTEGER CHECK (rows_affected >= 0),
    duration_ms INTEGER CHECK (duration_ms >= 0),
    source_component TEXT,
    metadata TEXT, -- JSON
    prev_hash TEXT,
    entry_hash TEXT NOT NULL UNIQUE
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_job ON audit_log(job_id);
CREATE INDEX IF NOT EXISTS idx_audit_operation ON audit_log(operation_id);
CREATE INDEX IF NOT EXISTS idx_audit_node ON audit_log(node_id);
CREATE INDEX IF NOT EXISTS idx_audit_role ON audit_log(role_used);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);

-- Append-only enforcement trigger
CREATE TRIGGER IF NOT EXISTS audit_no_modify
BEFORE UPDATE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'Audit log is append-only. Modifications are not permitted.');
END;

CREATE TRIGGER IF NOT EXISTS audit_no_delete
BEFORE DELETE ON audit_log
BEGIN
    SELECT RAISE(ABORT, 'Audit log is append-only. Deletions are not permitted.');
END;

