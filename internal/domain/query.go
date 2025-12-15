package domain

import (
	"regexp"
	"strings"
	"time"
)

// QueryType represents the type of query.
type QueryType string

const (
	// QueryTypeMetadata queries dataset/topic metadata.
	QueryTypeMetadata QueryType = "metadata"

	// QueryTypeSQL queries actual dataset data via SQL SELECT.
	QueryTypeSQL QueryType = "sql"
)

// QueryRequest represents a data query.
type QueryRequest struct {
	// ID is a unique identifier for this query.
	ID string `json:"id"`

	// Type is the query type.
	Type QueryType `json:"type"`

	// TopicID filters by topic (optional, for metadata queries).
	TopicID TopicID `json:"topic_id,omitempty"`

	// DatasetID filters by dataset (optional, for metadata queries).
	DatasetID DatasetID `json:"dataset_id,omitempty"`

	// TargetDatasets are the datasets to query (for SQL queries).
	// SQL queries can span multiple datasets.
	TargetDatasets []DatasetTarget `json:"target_datasets,omitempty"`

	// SQL is the SQL SELECT query (for QueryTypeSQL).
	// Only SELECT statements are allowed.
	SQL string `json:"sql,omitempty"`

	// NamePattern filters by name pattern with wildcards (optional).
	NamePattern string `json:"name_pattern,omitempty"`

	// Expression is a CEL expression for filtering (optional).
	Expression string `json:"expression,omitempty"`

	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`

	// Offset is the starting position for pagination.
	Offset int `json:"offset,omitempty"`

	// StreamResults indicates if results should be streamed.
	StreamResults bool `json:"stream_results,omitempty"`

	// Timeout is the query timeout duration.
	Timeout time.Duration `json:"timeout,omitempty"`

	// RequestedBy is the user making the query.
	RequestedBy UserID `json:"requested_by,omitempty"`

	// RequestedAt is when the query was made.
	RequestedAt time.Time `json:"requested_at"`
}

// DatasetTarget specifies a dataset to include in a SQL query.
type DatasetTarget struct {
	// DatasetID is the dataset to query.
	DatasetID DatasetID `json:"dataset_id"`

	// VersionID is the specific version to query (optional, defaults to latest).
	VersionID DatasetVersionID `json:"version_id,omitempty"`

	// Alias is the table alias to use in the SQL query.
	Alias string `json:"alias,omitempty"`
}

// Validate validates the query request.
func (q *QueryRequest) Validate() error {
	if q.ID == "" {
		return ErrInvalidQueryID
	}

	switch q.Type {
	case QueryTypeSQL:
		if q.SQL == "" {
			return ErrEmptySQLQuery
		}
		if err := validateSelectOnly(q.SQL); err != nil {
			return err
		}
		if len(q.TargetDatasets) == 0 {
			return ErrNoTargetDatasets
		}
	case QueryTypeMetadata, "":
		// Metadata query - at least one filter should be set
		// or NamePattern/Expression
	default:
		return ErrInvalidQueryType
	}

	if q.Limit < 0 {
		return ErrInvalidLimit
	}
	if q.Offset < 0 {
		return ErrInvalidOffset
	}

	return nil
}

// validateSelectOnly ensures the SQL query is a SELECT statement only.
// Rejects INSERT, UPDATE, DELETE, DROP, CREATE, ALTER, etc.
func validateSelectOnly(sql string) error {
	// Normalize whitespace and case
	normalized := strings.TrimSpace(strings.ToUpper(sql))

	// Must start with SELECT or WITH (for CTEs)
	if !strings.HasPrefix(normalized, "SELECT") && !strings.HasPrefix(normalized, "WITH") {
		return ErrNonSelectQuery
	}

	// Check for dangerous keywords
	dangerousPatterns := []string{
		`\bINSERT\b`,
		`\bUPDATE\b`,
		`\bDELETE\b`,
		`\bDROP\b`,
		`\bCREATE\b`,
		`\bALTER\b`,
		`\bTRUNCATE\b`,
		`\bGRANT\b`,
		`\bREVOKE\b`,
		`\bEXEC\b`,
		`\bEXECUTE\b`,
		`\bINTO\b`, // SELECT INTO
	}

	for _, pattern := range dangerousPatterns {
		matched, _ := regexp.MatchString(pattern, normalized)
		if matched {
			return ErrNonSelectQuery
		}
	}

	return nil
}

// QueryResult represents the result of a query.
type QueryResult struct {
	// QueryID is the original query ID.
	QueryID string `json:"query_id"`

	// Type is the result type matching the query type.
	Type QueryType `json:"type"`

	// Entries are the matching catalog entries (for metadata queries).
	Entries []CatalogEntry `json:"entries,omitempty"`

	// Columns are the column definitions (for SQL queries).
	Columns []QueryColumn `json:"columns,omitempty"`

	// Rows are the result rows (for SQL queries).
	// Each row is a slice of values matching Columns order.
	Rows [][]any `json:"rows,omitempty"`

	// TotalCount is the total number of matches (before pagination).
	TotalCount int `json:"total_count"`

	// FromCache indicates if this result was served from cache.
	FromCache bool `json:"from_cache"`

	// SourcePeer is the peer that provided this result.
	SourcePeer string `json:"source_peer,omitempty"`

	// ExecutionTimeMs is the query execution time in milliseconds.
	ExecutionTimeMs int64 `json:"execution_time_ms,omitempty"`

	// Truncated indicates if results were truncated due to limits.
	Truncated bool `json:"truncated,omitempty"`

	// Error is set if the query failed.
	Error string `json:"error,omitempty"`
}

// QueryColumn describes a column in SQL query results.
type QueryColumn struct {
	// Name is the column name.
	Name string `json:"name"`

	// Type is the column data type.
	Type string `json:"type"`

	// Nullable indicates if the column can be null.
	Nullable bool `json:"nullable"`
}

// QueryResultRow is a convenience type for a single result row.
type QueryResultRow struct {
	// Values are the row values in column order.
	Values []any `json:"values"`
}

// StreamedQueryResult represents a single chunk of streamed results.
type StreamedQueryResult struct {
	// QueryID is the query this result belongs to.
	QueryID string `json:"query_id"`

	// ChunkIndex is the index of this chunk (0-based).
	ChunkIndex int `json:"chunk_index"`

	// Columns are only included in the first chunk.
	Columns []QueryColumn `json:"columns,omitempty"`

	// Rows are the rows in this chunk.
	Rows [][]any `json:"rows"`

	// IsLast indicates if this is the final chunk.
	IsLast bool `json:"is_last"`

	// TotalRows is the total row count (only in last chunk).
	TotalRows int `json:"total_rows,omitempty"`

	// Error is set if streaming failed.
	Error string `json:"error,omitempty"`
}
