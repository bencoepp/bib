-- Helper functions and triggers for common operations

-- Function to automatically update updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply updated_at trigger to tables
DROP TRIGGER IF EXISTS update_topics_updated_at ON topics;
CREATE TRIGGER update_topics_updated_at
    BEFORE UPDATE ON topics
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_datasets_updated_at ON datasets;
CREATE TRIGGER update_datasets_updated_at
    BEFORE UPDATE ON datasets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_nodes_updated_at ON nodes;
CREATE TRIGGER update_nodes_updated_at
    BEFORE UPDATE ON nodes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_chunks_updated_at ON chunks;
CREATE TRIGGER update_chunks_updated_at
    BEFORE UPDATE ON chunks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to increment dataset_count on topics
CREATE OR REPLACE FUNCTION update_topic_dataset_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE topics SET dataset_count = dataset_count + 1 WHERE id = NEW.topic_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE topics SET dataset_count = GREATEST(0, dataset_count - 1) WHERE id = OLD.topic_id;
    ELSIF TG_OP = 'UPDATE' AND NEW.topic_id != OLD.topic_id THEN
        UPDATE topics SET dataset_count = GREATEST(0, dataset_count - 1) WHERE id = OLD.topic_id;
        UPDATE topics SET dataset_count = dataset_count + 1 WHERE id = NEW.topic_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS dataset_count_trigger ON datasets;
CREATE TRIGGER dataset_count_trigger
    AFTER INSERT OR DELETE OR UPDATE OF topic_id ON datasets
    FOR EACH ROW EXECUTE FUNCTION update_topic_dataset_count();

-- Function to increment version_count on datasets
CREATE OR REPLACE FUNCTION update_dataset_version_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE datasets SET version_count = version_count + 1 WHERE id = NEW.dataset_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE datasets SET version_count = GREATEST(0, version_count - 1) WHERE id = OLD.dataset_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS version_count_trigger ON dataset_versions;
CREATE TRIGGER version_count_trigger
    AFTER INSERT OR DELETE ON dataset_versions
    FOR EACH ROW EXECUTE FUNCTION update_dataset_version_count();

-- Comments
COMMENT ON FUNCTION update_updated_at_column() IS 'Automatically updates updated_at timestamp on row modification';
COMMENT ON FUNCTION update_topic_dataset_count() IS 'Maintains dataset_count on topics table';
COMMENT ON FUNCTION update_dataset_version_count() IS 'Maintains version_count on datasets table';

