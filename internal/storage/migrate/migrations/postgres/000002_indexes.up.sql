-- Create indexes for common query patterns

-- Nodes indexes
CREATE INDEX IF NOT EXISTS idx_nodes_mode ON nodes(mode);
CREATE INDEX IF NOT EXISTS idx_nodes_seen ON nodes(last_seen);
CREATE INDEX IF NOT EXISTS idx_nodes_trusted ON nodes(trusted_storage) WHERE trusted_storage = true;

-- Topics indexes
CREATE INDEX IF NOT EXISTS idx_topics_status ON topics(status);
CREATE INDEX IF NOT EXISTS idx_topics_parent ON topics(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_topics_name ON topics(name);
CREATE INDEX IF NOT EXISTS idx_topics_owners ON topics USING GIN(owners);
CREATE INDEX IF NOT EXISTS idx_topics_tags ON topics USING GIN(tags) WHERE tags IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_topics_created ON topics(created_at);

-- Datasets indexes
CREATE INDEX IF NOT EXISTS idx_datasets_topic ON datasets(topic_id);
CREATE INDEX IF NOT EXISTS idx_datasets_status ON datasets(status);
CREATE INDEX IF NOT EXISTS idx_datasets_name ON datasets(name);
CREATE INDEX IF NOT EXISTS idx_datasets_owners ON datasets USING GIN(owners);
CREATE INDEX IF NOT EXISTS idx_datasets_tags ON datasets USING GIN(tags) WHERE tags IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_datasets_created ON datasets(created_at);
CREATE INDEX IF NOT EXISTS idx_datasets_latest_version ON datasets(latest_version_id) WHERE latest_version_id IS NOT NULL;

-- Dataset versions indexes
CREATE INDEX IF NOT EXISTS idx_versions_dataset ON dataset_versions(dataset_id);
CREATE INDEX IF NOT EXISTS idx_versions_created ON dataset_versions(created_at);
CREATE INDEX IF NOT EXISTS idx_versions_prev ON dataset_versions(previous_version_id) WHERE previous_version_id IS NOT NULL;

-- Chunks indexes
CREATE INDEX IF NOT EXISTS idx_chunks_dataset ON chunks(dataset_id);
CREATE INDEX IF NOT EXISTS idx_chunks_version ON chunks(version_id);
CREATE INDEX IF NOT EXISTS idx_chunks_status ON chunks(status);
CREATE INDEX IF NOT EXISTS idx_chunks_hash ON chunks(hash);

-- Jobs indexes
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type);
CREATE INDEX IF NOT EXISTS idx_jobs_priority ON jobs(priority DESC, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON jobs(created_at);
CREATE INDEX IF NOT EXISTS idx_jobs_topic ON jobs(topic_id) WHERE topic_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_dataset ON jobs(dataset_id) WHERE dataset_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_node ON jobs(node_id) WHERE node_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_created_by ON jobs(created_by);

-- Job results indexes
CREATE INDEX IF NOT EXISTS idx_results_job ON job_results(job_id);
CREATE INDEX IF NOT EXISTS idx_results_node ON job_results(node_id);
CREATE INDEX IF NOT EXISTS idx_results_status ON job_results(status);
CREATE INDEX IF NOT EXISTS idx_results_completed ON job_results(completed_at);

-- Blobs indexes (for Phase 2.7)
CREATE INDEX IF NOT EXISTS idx_blobs_accessed ON blobs(last_accessed);
CREATE INDEX IF NOT EXISTS idx_blobs_size ON blobs(size);

