-- Initial schema for PostgreSQL
-- Creates core tables for nodes, topics, datasets, jobs, and audit logging

-- Nodes table - tracks P2P network nodes
CREATE TABLE IF NOT EXISTS nodes (
    peer_id TEXT PRIMARY KEY,
    addresses TEXT[] NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('full', 'selective', 'proxy')),
    storage_type TEXT NOT NULL CHECK (storage_type IN ('sqlite', 'postgres')),
    trusted_storage BOOLEAN NOT NULL DEFAULT false,
    last_seen TIMESTAMPTZ NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Topics table - categories for datasets with hierarchical support
CREATE TABLE IF NOT EXISTS topics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID REFERENCES topics(id) ON DELETE CASCADE,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    table_schema TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived', 'deleted')),
    owners TEXT[] NOT NULL CHECK (array_length(owners, 1) > 0),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dataset_count INTEGER NOT NULL DEFAULT 0 CHECK (dataset_count >= 0),
    tags TEXT[],
    metadata JSONB
);

-- Datasets table - data units within topics
CREATE TABLE IF NOT EXISTS datasets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id UUID NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived', 'deleted', 'ingesting', 'failed')),
    latest_version_id UUID,
    version_count INTEGER NOT NULL DEFAULT 0 CHECK (version_count >= 0),
    has_content BOOLEAN NOT NULL DEFAULT false,
    has_instructions BOOLEAN NOT NULL DEFAULT false,
    owners TEXT[] NOT NULL CHECK (array_length(owners, 1) > 0),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    tags TEXT[],
    metadata JSONB,
    CONSTRAINT dataset_unique_name_per_topic UNIQUE (topic_id, name)
);

-- Dataset versions table - version history for datasets
CREATE TABLE IF NOT EXISTS dataset_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    version TEXT NOT NULL,
    previous_version_id UUID REFERENCES dataset_versions(id),
    content JSONB,
    instructions JSONB,
    table_schema TEXT,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message TEXT,
    metadata JSONB,
    UNIQUE (dataset_id, version)
);

-- Add foreign key constraint for latest_version_id after dataset_versions exists
ALTER TABLE datasets
    ADD CONSTRAINT fk_latest_version
    FOREIGN KEY (latest_version_id) REFERENCES dataset_versions(id) ON DELETE SET NULL;

-- Chunks table - chunked data for transfer and storage
CREATE TABLE IF NOT EXISTS chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    version_id UUID NOT NULL REFERENCES dataset_versions(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL CHECK (chunk_index >= 0),
    hash TEXT NOT NULL,
    size BIGINT NOT NULL CHECK (size >= 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'downloading', 'downloaded', 'verified', 'failed')),
    storage_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (version_id, chunk_index)
);

-- Jobs table - scheduled and on-demand jobs
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type TEXT NOT NULL CHECK (type IN ('scrape', 'transform', 'clean', 'analyze', 'ml', 'etl', 'ingest', 'export', 'custom')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'queued', 'running', 'completed', 'failed', 'cancelled', 'waiting', 'retrying')),
    task_id TEXT,
    inline_instructions JSONB,
    execution_mode TEXT NOT NULL DEFAULT 'goroutine' CHECK (execution_mode IN ('goroutine', 'container', 'pod')),
    schedule JSONB,
    inputs JSONB,
    outputs JSONB,
    dependencies UUID[],
    topic_id UUID REFERENCES topics(id) ON DELETE SET NULL,
    dataset_id UUID REFERENCES datasets(id) ON DELETE SET NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    resource_limits JSONB,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error TEXT,
    result TEXT,
    progress INTEGER NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    current_instruction INTEGER NOT NULL DEFAULT 0 CHECK (current_instruction >= 0),
    node_id TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    metadata JSONB,
    CONSTRAINT job_completion_time_check CHECK (
        (status IN ('completed', 'failed', 'cancelled') AND completed_at IS NOT NULL) OR
        (status NOT IN ('completed', 'failed', 'cancelled') AND completed_at IS NULL)
    )
);

-- Job results table - execution results from nodes
CREATE TABLE IF NOT EXISTS job_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'queued', 'running', 'completed', 'failed', 'cancelled')),
    result TEXT,
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    duration_ms BIGINT NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
    metadata JSONB
);

-- Blob storage tracking table (stub for Phase 2.7)
-- TODO(Phase 2.7): Implement full blob storage with CAS
CREATE TABLE IF NOT EXISTS blobs (
    hash TEXT PRIMARY KEY,
    size BIGINT NOT NULL CHECK (size >= 0),
    storage_path TEXT NOT NULL,
    mime_type TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_accessed TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    access_count INTEGER NOT NULL DEFAULT 0 CHECK (access_count >= 0),
    metadata JSONB
);

-- Comment on blob table
COMMENT ON TABLE blobs IS 'Stub for Phase 2.7 - Content-addressed blob storage';

