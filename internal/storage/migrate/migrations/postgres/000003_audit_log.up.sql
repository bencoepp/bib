-- Audit log table with append-only enforcement and hash chain for tamper detection

CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
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
    metadata JSONB,
    prev_hash TEXT,
    entry_hash TEXT NOT NULL,
    CONSTRAINT audit_entry_hash_unique UNIQUE (entry_hash)
);

-- Audit log indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_job ON audit_log(job_id) WHERE job_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_operation ON audit_log(operation_id);
CREATE INDEX IF NOT EXISTS idx_audit_node ON audit_log(node_id);
CREATE INDEX IF NOT EXISTS idx_audit_role ON audit_log(role_used);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_table ON audit_log(table_name) WHERE table_name IS NOT NULL;

-- Audit log hash chain index for verification
CREATE INDEX IF NOT EXISTS idx_audit_prev_hash ON audit_log(prev_hash) WHERE prev_hash IS NOT NULL;

-- Append-only enforcement trigger
CREATE OR REPLACE FUNCTION audit_no_modify() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Audit log is append-only. Modifications are not permitted.';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_immutable ON audit_log;
CREATE TRIGGER audit_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_no_modify();

-- Comment on table
COMMENT ON TABLE audit_log IS 'Append-only audit log with hash chain for tamper detection';
COMMENT ON COLUMN audit_log.prev_hash IS 'Hash of previous entry for chain verification';
COMMENT ON COLUMN audit_log.entry_hash IS 'SHA-256 hash of this entry for tamper detection';

