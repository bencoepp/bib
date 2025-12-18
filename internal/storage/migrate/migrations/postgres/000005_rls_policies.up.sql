-- Row-Level Security (RLS) policies for enhanced data isolation
-- RLS provides fine-grained access control at the row level based on the current role

-- Enable RLS on all core tables
ALTER TABLE nodes ENABLE ROW LEVEL SECURITY;
ALTER TABLE topics ENABLE ROW LEVEL SECURITY;
ALTER TABLE datasets ENABLE ROW LEVEL SECURITY;
ALTER TABLE dataset_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chunks ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE job_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE blobs ENABLE ROW LEVEL SECURITY;

-- Audit log is special - only audit role can write, admins can read
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;

-- ============================================================================
-- NODES: Read access for all roles, write only for admin
-- ============================================================================

CREATE POLICY nodes_read_all ON nodes
    FOR SELECT
    USING (true);

CREATE POLICY nodes_write_admin ON nodes
    FOR ALL
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs'))
    WITH CHECK (current_user IN ('bibd_admin', 'bibd_admin_jobs'));

-- ============================================================================
-- TOPICS: Read for all, write for transform/admin, owners can manage
-- ============================================================================

CREATE POLICY topics_read_all ON topics
    FOR SELECT
    USING (true);

CREATE POLICY topics_write_admin ON topics
    FOR INSERT
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_transform'));

CREATE POLICY topics_update_owners ON topics
    FOR UPDATE
    USING (
        current_user IN ('bibd_admin', 'bibd_admin_jobs') OR
        current_setting('app.user_id', true) = ANY(owners)
    )
    WITH CHECK (
        current_user IN ('bibd_admin', 'bibd_admin_jobs') OR
        current_setting('app.user_id', true) = ANY(owners)
    );

CREATE POLICY topics_delete_owners ON topics
    FOR DELETE
    USING (
        current_user IN ('bibd_admin', 'bibd_admin_jobs') OR
        current_setting('app.user_id', true) = ANY(owners)
    );

-- ============================================================================
-- DATASETS: Similar to topics - read all, write restricted
-- ============================================================================

CREATE POLICY datasets_read_all ON datasets
    FOR SELECT
    USING (true);

CREATE POLICY datasets_write_scrape ON datasets
    FOR INSERT
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_scrape', 'bibd_transform'));

CREATE POLICY datasets_update_owners ON datasets
    FOR UPDATE
    USING (
        current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_transform') OR
        current_setting('app.user_id', true) = ANY(owners)
    )
    WITH CHECK (
        current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_transform') OR
        current_setting('app.user_id', true) = ANY(owners)
    );

CREATE POLICY datasets_delete_owners ON datasets
    FOR DELETE
    USING (
        current_user IN ('bibd_admin', 'bibd_admin_jobs') OR
        current_setting('app.user_id', true) = ANY(owners)
    );

-- ============================================================================
-- DATASET_VERSIONS: Read all, write for scrape/transform
-- ============================================================================

CREATE POLICY versions_read_all ON dataset_versions
    FOR SELECT
    USING (true);

CREATE POLICY versions_write ON dataset_versions
    FOR INSERT
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_scrape', 'bibd_transform'));

-- Versions are immutable after creation (no UPDATE/DELETE except admin)
CREATE POLICY versions_delete_admin ON dataset_versions
    FOR DELETE
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs'));

-- ============================================================================
-- CHUNKS: Read all, write for scrape/transform
-- ============================================================================

CREATE POLICY chunks_read_all ON chunks
    FOR SELECT
    USING (true);

CREATE POLICY chunks_write ON chunks
    FOR ALL
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_scrape', 'bibd_transform'))
    WITH CHECK (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_scrape', 'bibd_transform'));

-- ============================================================================
-- JOBS: Users can read their own jobs, admins can read all
-- ============================================================================

CREATE POLICY jobs_read_own ON jobs
    FOR SELECT
    USING (
        current_user IN ('bibd_admin', 'bibd_admin_jobs') OR
        current_setting('app.user_id', true) = created_by OR
        current_setting('app.user_id', true) = node_id
    );

CREATE POLICY jobs_write ON jobs
    FOR INSERT
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_transform'));

CREATE POLICY jobs_update_own ON jobs
    FOR UPDATE
    USING (
        current_user IN ('bibd_admin', 'bibd_admin_jobs') OR
        current_setting('app.user_id', true) = created_by OR
        current_setting('app.user_id', true) = node_id
    )
    WITH CHECK (
        current_user IN ('bibd_admin', 'bibd_admin_jobs') OR
        current_setting('app.user_id', true) = created_by OR
        current_setting('app.user_id', true) = node_id
    );

-- Only admins can delete jobs
CREATE POLICY jobs_delete_admin ON jobs
    FOR DELETE
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs'));

-- ============================================================================
-- JOB_RESULTS: Users can read results for their jobs
-- ============================================================================

CREATE POLICY job_results_read ON job_results
    FOR SELECT
    USING (
        current_user IN ('bibd_admin', 'bibd_admin_jobs') OR
        current_setting('app.user_id', true) = node_id OR
        EXISTS (
            SELECT 1 FROM jobs
            WHERE jobs.id = job_results.job_id
            AND jobs.created_by = current_setting('app.user_id', true)
        )
    );

CREATE POLICY job_results_write ON job_results
    FOR INSERT
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_transform'));

-- ============================================================================
-- BLOBS: Read all, write for scrape/transform (stub for Phase 2.7)
-- ============================================================================

CREATE POLICY blobs_read_all ON blobs
    FOR SELECT
    USING (true);

CREATE POLICY blobs_write ON blobs
    FOR ALL
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_scrape', 'bibd_transform'))
    WITH CHECK (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_scrape', 'bibd_transform'));

-- ============================================================================
-- AUDIT_LOG: Only audit role can insert, admins can read
-- ============================================================================

CREATE POLICY audit_log_read_admin ON audit_log
    FOR SELECT
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs', 'bibd_audit'));

CREATE POLICY audit_log_write_audit ON audit_log
    FOR INSERT
    WITH CHECK (current_user IN ('bibd_audit', 'bibd_admin'));

-- Note: UPDATE and DELETE are already blocked by trigger, but RLS adds defense in depth

-- Comments
COMMENT ON POLICY nodes_read_all ON nodes IS 'All roles can read node information';
COMMENT ON POLICY topics_update_owners ON topics IS 'Only topic owners or admins can update topics';
COMMENT ON POLICY datasets_update_owners ON datasets IS 'Only dataset owners or admins can update datasets';
COMMENT ON POLICY jobs_read_own ON jobs IS 'Users can only read their own jobs unless admin';
COMMENT ON POLICY audit_log_write_audit ON audit_log IS 'Only audit role can write to audit log';

