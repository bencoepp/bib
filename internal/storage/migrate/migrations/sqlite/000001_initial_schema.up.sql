-- Initial schema for SQLite (cache/proxy mode only)
-- SQLite stores are non-authoritative and used for caching
-- Note: This migration drops existing tables to ensure clean schema
-- This is safe for SQLite as it's only used for caching (no authoritative data)

-- Drop old tables if they exist (from old migration system)
DROP TABLE IF EXISTS job_results;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS dataset_versions;
DROP TABLE IF EXISTS datasets;
DROP TABLE IF EXISTS topics;
DROP TABLE IF EXISTS nodes;
DROP TABLE IF EXISTS cache_metadata;
DROP TABLE IF EXISTS blobs;

-- Nodes table
CREATE TABLE nodes (
    peer_id TEXT PRIMARY KEY,
    addresses TEXT NOT NULL, -- JSON array
    mode TEXT NOT NULL CHECK (mode IN ('full', 'selective', 'proxy')),
    storage_type TEXT NOT NULL CHECK (storage_type IN ('sqlite', 'postgres')),
    trusted_storage INTEGER NOT NULL DEFAULT 0 CHECK (trusted_storage IN (0, 1)),
    last_seen TEXT NOT NULL,
    metadata TEXT, -- JSON
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    cached_at TEXT -- For TTL expiration
);

-- Topics table
CREATE TABLE topics (
    id TEXT PRIMARY KEY,
    parent_id TEXT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    table_schema TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived', 'deleted')),
    owners TEXT NOT NULL, -- JSON array
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    dataset_count INTEGER NOT NULL DEFAULT 0 CHECK (dataset_count >= 0),
    tags TEXT, -- JSON array
    metadata TEXT, -- JSON
    cached_at TEXT, -- For TTL expiration
    FOREIGN KEY (parent_id) REFERENCES topics(id) ON DELETE CASCADE
);

-- Datasets table
CREATE TABLE datasets (
    id TEXT PRIMARY KEY,
    topic_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived', 'deleted', 'ingesting', 'failed')),
    latest_version_id TEXT,
    version_count INTEGER NOT NULL DEFAULT 0 CHECK (version_count >= 0),
    has_content INTEGER NOT NULL DEFAULT 0 CHECK (has_content IN (0, 1)),
    has_instructions INTEGER NOT NULL DEFAULT 0 CHECK (has_instructions IN (0, 1)),
    owners TEXT NOT NULL, -- JSON array
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    tags TEXT, -- JSON array
    metadata TEXT, -- JSON
    cached_at TEXT, -- For TTL expiration
    FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
    UNIQUE (topic_id, name)
);

-- Dataset versions table
CREATE TABLE dataset_versions (
    id TEXT PRIMARY KEY,
    dataset_id TEXT NOT NULL,
    version TEXT NOT NULL,
    previous_version_id TEXT,
    content TEXT, -- JSON
    instructions TEXT, -- JSON
    table_schema TEXT,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    message TEXT,
    metadata TEXT, -- JSON
    cached_at TEXT,
    FOREIGN KEY (dataset_id) REFERENCES datasets(id) ON DELETE CASCADE,
    FOREIGN KEY (previous_version_id) REFERENCES dataset_versions(id),
    UNIQUE (dataset_id, version)
);

-- Chunks table
CREATE TABLE chunks (
    id TEXT PRIMARY KEY,
    dataset_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL CHECK (chunk_index >= 0),
    hash TEXT NOT NULL,
    size INTEGER NOT NULL CHECK (size >= 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'downloading', 'downloaded', 'verified', 'failed')),
    storage_path TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    cached_at TEXT,
    FOREIGN KEY (dataset_id) REFERENCES datasets(id) ON DELETE CASCADE,
    FOREIGN KEY (version_id) REFERENCES dataset_versions(id) ON DELETE CASCADE,
    UNIQUE (version_id, chunk_index)
);

-- Jobs table
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL CHECK (type IN ('scrape', 'transform', 'clean', 'analyze', 'ml', 'etl', 'ingest', 'export', 'custom')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'queued', 'running', 'completed', 'failed', 'cancelled', 'waiting', 'retrying')),
    task_id TEXT,
    inline_instructions TEXT, -- JSON
    execution_mode TEXT NOT NULL DEFAULT 'goroutine' CHECK (execution_mode IN ('goroutine', 'container', 'pod')),
    schedule TEXT, -- JSON
    inputs TEXT, -- JSON
    outputs TEXT, -- JSON
    dependencies TEXT, -- JSON array of UUIDs
    topic_id TEXT,
    dataset_id TEXT,
    priority INTEGER NOT NULL DEFAULT 0,
    resource_limits TEXT, -- JSON
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    started_at TEXT,
    completed_at TEXT,
    error TEXT,
    result TEXT,
    progress INTEGER NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    current_instruction INTEGER NOT NULL DEFAULT 0 CHECK (current_instruction >= 0),
    node_id TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    metadata TEXT, -- JSON
    cached_at TEXT,
    FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE SET NULL,
    FOREIGN KEY (dataset_id) REFERENCES datasets(id) ON DELETE SET NULL
);

-- Job results table
CREATE TABLE job_results (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    node_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'queued', 'running', 'completed', 'failed', 'cancelled')),
    result TEXT,
    error TEXT,
    started_at TEXT,
    completed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    duration_ms INTEGER NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
    metadata TEXT, -- JSON
    cached_at TEXT,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

-- Cache metadata table (SQLite-specific)
CREATE TABLE cache_metadata (
    table_name TEXT NOT NULL,
    row_id TEXT NOT NULL,
    cached_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    PRIMARY KEY (table_name, row_id)
);

-- Blobs table (stub for Phase 2.7)
CREATE TABLE blobs (
    hash TEXT PRIMARY KEY,
    size INTEGER NOT NULL CHECK (size >= 0),
    storage_path TEXT NOT NULL,
    mime_type TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    last_accessed TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    access_count INTEGER NOT NULL DEFAULT 0 CHECK (access_count >= 0),
    metadata TEXT, -- JSON
    cached_at TEXT
);

-- Add cached_at column to existing tables if they don't have it
-- This handles migration from old schema to new schema
-- SQLite doesn't have "IF NOT EXISTS" for ALTER TABLE, so we need to check first

-- For nodes table
-- Check if cached_at exists, if not add it
-- Note: In SQLite, we can't easily check if column exists, so we use a different approach
-- We'll just try to add it and it will fail silently if it exists

-- Add cached_at to nodes if it doesn't exist
-- SQLite allows adding columns, but we need to handle existing tables
-- The IF NOT EXISTS in CREATE TABLE already handles fresh installs
-- For existing tables from old migrations, they won't have cached_at

-- Workaround: Since we can't conditionally add columns in SQLite easily,
-- we'll rely on the fact that if the table was just created above,
-- it has the column. If it already existed, we need to be careful.

-- This migration assumes either:
-- 1. Fresh install (tables created above with cached_at)
-- 2. Old install (will be handled by next migration that adds columns)

