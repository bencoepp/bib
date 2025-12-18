-- Drop all RLS policies and disable RLS

-- Drop audit_log policies
DROP POLICY IF EXISTS audit_log_write_audit ON audit_log;
DROP POLICY IF EXISTS audit_log_read_admin ON audit_log;

-- Drop blobs policies
DROP POLICY IF EXISTS blobs_write ON blobs;
DROP POLICY IF EXISTS blobs_read_all ON blobs;

-- Drop job_results policies
DROP POLICY IF EXISTS job_results_write ON job_results;
DROP POLICY IF EXISTS job_results_read ON job_results;

-- Drop jobs policies
DROP POLICY IF EXISTS jobs_delete_admin ON jobs;
DROP POLICY IF EXISTS jobs_update_own ON jobs;
DROP POLICY IF EXISTS jobs_write ON jobs;
DROP POLICY IF EXISTS jobs_read_own ON jobs;

-- Drop chunks policies
DROP POLICY IF EXISTS chunks_write ON chunks;
DROP POLICY IF EXISTS chunks_read_all ON chunks;

-- Drop dataset_versions policies
DROP POLICY IF EXISTS versions_delete_admin ON dataset_versions;
DROP POLICY IF EXISTS versions_write ON dataset_versions;
DROP POLICY IF EXISTS versions_read_all ON dataset_versions;

-- Drop datasets policies
DROP POLICY IF EXISTS datasets_delete_owners ON datasets;
DROP POLICY IF EXISTS datasets_update_owners ON datasets;
DROP POLICY IF EXISTS datasets_write_scrape ON datasets;
DROP POLICY IF EXISTS datasets_read_all ON datasets;

-- Drop topics policies
DROP POLICY IF EXISTS topics_delete_owners ON topics;
DROP POLICY IF EXISTS topics_update_owners ON topics;
DROP POLICY IF EXISTS topics_write_admin ON topics;
DROP POLICY IF EXISTS topics_read_all ON topics;

-- Drop nodes policies
DROP POLICY IF EXISTS nodes_write_admin ON nodes;
DROP POLICY IF EXISTS nodes_read_all ON nodes;

-- Disable RLS on all tables
ALTER TABLE audit_log DISABLE ROW LEVEL SECURITY;
ALTER TABLE blobs DISABLE ROW LEVEL SECURITY;
ALTER TABLE job_results DISABLE ROW LEVEL SECURITY;
ALTER TABLE jobs DISABLE ROW LEVEL SECURITY;
ALTER TABLE chunks DISABLE ROW LEVEL SECURITY;
ALTER TABLE dataset_versions DISABLE ROW LEVEL SECURITY;
ALTER TABLE datasets DISABLE ROW LEVEL SECURITY;
ALTER TABLE topics DISABLE ROW LEVEL SECURITY;
ALTER TABLE nodes DISABLE ROW LEVEL SECURITY;

