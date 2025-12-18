-- Create indexes for SQLite

-- Nodes indexes
CREATE INDEX IF NOT EXISTS idx_nodes_mode ON nodes(mode);
CREATE INDEX IF NOT EXISTS idx_nodes_seen ON nodes(last_seen);
CREATE INDEX IF NOT EXISTS idx_nodes_cached ON nodes(cached_at);

-- Topics indexes
CREATE INDEX IF NOT EXISTS idx_topics_status ON topics(status);
CREATE INDEX IF NOT EXISTS idx_topics_parent ON topics(parent_id);
CREATE INDEX IF NOT EXISTS idx_topics_name ON topics(name);
CREATE INDEX IF NOT EXISTS idx_topics_created ON topics(created_at);
CREATE INDEX IF NOT EXISTS idx_topics_cached ON topics(cached_at);

-- Datasets indexes
CREATE INDEX IF NOT EXISTS idx_datasets_topic ON datasets(topic_id);
CREATE INDEX IF NOT EXISTS idx_datasets_status ON datasets(status);
CREATE INDEX IF NOT EXISTS idx_datasets_name ON datasets(name);
CREATE INDEX IF NOT EXISTS idx_datasets_created ON datasets(created_at);
CREATE INDEX IF NOT EXISTS idx_datasets_cached ON datasets(cached_at);

-- Dataset versions indexes
CREATE INDEX IF NOT EXISTS idx_versions_dataset ON dataset_versions(dataset_id);
CREATE INDEX IF NOT EXISTS idx_versions_created ON dataset_versions(created_at);
CREATE INDEX IF NOT EXISTS idx_versions_cached ON dataset_versions(cached_at);

-- Chunks indexes
CREATE INDEX IF NOT EXISTS idx_chunks_dataset ON chunks(dataset_id);
CREATE INDEX IF NOT EXISTS idx_chunks_version ON chunks(version_id);
CREATE INDEX IF NOT EXISTS idx_chunks_status ON chunks(status);
CREATE INDEX IF NOT EXISTS idx_chunks_hash ON chunks(hash);
CREATE INDEX IF NOT EXISTS idx_chunks_cached ON chunks(cached_at);

-- Jobs indexes
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type);
CREATE INDEX IF NOT EXISTS idx_jobs_priority ON jobs(priority DESC, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON jobs(created_at);
CREATE INDEX IF NOT EXISTS idx_jobs_topic ON jobs(topic_id);
CREATE INDEX IF NOT EXISTS idx_jobs_dataset ON jobs(dataset_id);
CREATE INDEX IF NOT EXISTS idx_jobs_cached ON jobs(cached_at);

-- Job results indexes
CREATE INDEX IF NOT EXISTS idx_results_job ON job_results(job_id);
CREATE INDEX IF NOT EXISTS idx_results_node ON job_results(node_id);
CREATE INDEX IF NOT EXISTS idx_results_status ON job_results(status);
CREATE INDEX IF NOT EXISTS idx_results_cached ON job_results(cached_at);

-- Cache metadata indexes
CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache_metadata(expires_at);
CREATE INDEX IF NOT EXISTS idx_cache_table ON cache_metadata(table_name);

-- Blobs indexes
CREATE INDEX IF NOT EXISTS idx_blobs_accessed ON blobs(last_accessed);
CREATE INDEX IF NOT EXISTS idx_blobs_cached ON blobs(cached_at);

