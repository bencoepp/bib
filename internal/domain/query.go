package domain

// QueryRequest represents a data query.
type QueryRequest struct {
	// ID is a unique identifier for this query.
	ID string `json:"id"`

	// TopicID filters by topic (optional).
	TopicID TopicID `json:"topic_id,omitempty"`

	// DatasetID filters by dataset (optional).
	DatasetID DatasetID `json:"dataset_id,omitempty"`

	// NamePattern filters by name pattern with wildcards (optional).
	NamePattern string `json:"name_pattern,omitempty"`

	// Expression is a CEL expression for filtering (optional).
	// TODO: Will be implemented in Phase 3 (Scheduler).
	Expression string `json:"expression,omitempty"`

	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`

	// Offset is the starting position for pagination.
	Offset int `json:"offset,omitempty"`
}

// QueryResult represents the result of a query.
type QueryResult struct {
	// QueryID is the original query ID.
	QueryID string `json:"query_id"`

	// Entries are the matching catalog entries.
	Entries []CatalogEntry `json:"entries"`

	// TotalCount is the total number of matches (before pagination).
	TotalCount int `json:"total_count"`

	// FromCache indicates if this result was served from cache.
	FromCache bool `json:"from_cache"`

	// SourcePeer is the peer that provided this result.
	SourcePeer string `json:"source_peer,omitempty"`
}
