-- Drop all triggers and functions created in migration 4

-- Drop triggers
DROP TRIGGER IF EXISTS version_count_trigger ON dataset_versions;
DROP TRIGGER IF EXISTS dataset_count_trigger ON datasets;
DROP TRIGGER IF EXISTS update_chunks_updated_at ON chunks;
DROP TRIGGER IF EXISTS update_nodes_updated_at ON nodes;
DROP TRIGGER IF EXISTS update_datasets_updated_at ON datasets;
DROP TRIGGER IF EXISTS update_topics_updated_at ON topics;

-- Drop functions
DROP FUNCTION IF EXISTS update_dataset_version_count();
DROP FUNCTION IF EXISTS update_topic_dataset_count();
DROP FUNCTION IF EXISTS update_updated_at_column();

