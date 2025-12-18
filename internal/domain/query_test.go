package domain

import (
	"testing"
)

func TestQueryType_Constants(t *testing.T) {
	if QueryTypeMetadata != "metadata" {
		t.Errorf("expected 'metadata', got %q", QueryTypeMetadata)
	}
	if QueryTypeSQL != "sql" {
		t.Errorf("expected 'sql', got %q", QueryTypeSQL)
	}
}

func TestQueryRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		query   *QueryRequest
		wantErr error
	}{
		{
			name: "valid metadata query",
			query: &QueryRequest{
				ID:   "query-1",
				Type: QueryTypeMetadata,
			},
			wantErr: nil,
		},
		{
			name: "valid SQL query",
			query: &QueryRequest{
				ID:   "query-1",
				Type: QueryTypeSQL,
				SQL:  "SELECT * FROM data",
				TargetDatasets: []DatasetTarget{
					{DatasetID: "dataset-1"},
				},
			},
			wantErr: nil,
		},
		{
			name: "empty ID",
			query: &QueryRequest{
				ID:   "",
				Type: QueryTypeMetadata,
			},
			wantErr: ErrInvalidQueryID,
		},
		{
			name: "SQL query without SQL",
			query: &QueryRequest{
				ID:   "query-1",
				Type: QueryTypeSQL,
				TargetDatasets: []DatasetTarget{
					{DatasetID: "dataset-1"},
				},
			},
			wantErr: ErrEmptySQLQuery,
		},
		{
			name: "SQL query without targets",
			query: &QueryRequest{
				ID:             "query-1",
				Type:           QueryTypeSQL,
				SQL:            "SELECT * FROM data",
				TargetDatasets: []DatasetTarget{},
			},
			wantErr: ErrNoTargetDatasets,
		},
		{
			name: "invalid query type",
			query: &QueryRequest{
				ID:   "query-1",
				Type: QueryType("invalid"),
			},
			wantErr: ErrInvalidQueryType,
		},
		{
			name: "negative limit",
			query: &QueryRequest{
				ID:    "query-1",
				Type:  QueryTypeMetadata,
				Limit: -1,
			},
			wantErr: ErrInvalidLimit,
		},
		{
			name: "negative offset",
			query: &QueryRequest{
				ID:     "query-1",
				Type:   QueryTypeMetadata,
				Offset: -1,
			},
			wantErr: ErrInvalidOffset,
		},
		{
			name: "empty type is valid (metadata)",
			query: &QueryRequest{
				ID:   "query-1",
				Type: "",
			},
			wantErr: nil,
		},
		{
			name: "SQL with non-SELECT query",
			query: &QueryRequest{
				ID:   "query-1",
				Type: QueryTypeSQL,
				SQL:  "DELETE FROM data",
				TargetDatasets: []DatasetTarget{
					{DatasetID: "dataset-1"},
				},
			},
			wantErr: ErrNonSelectQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			if err != tt.wantErr {
				t.Errorf("QueryRequest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSelectOnly(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr error
	}{
		{"simple select", "SELECT * FROM users", nil},
		{"select with where", "SELECT id, name FROM users WHERE active = true", nil},
		{"select with join", "SELECT u.*, p.name FROM users u JOIN profiles p ON u.id = p.user_id", nil},
		{"with CTE", "WITH cte AS (SELECT * FROM data) SELECT * FROM cte", nil},
		{"select uppercase", "SELECT * FROM DATA", nil},
		{"select lowercase", "select * from data", nil},

		{"insert", "INSERT INTO users (name) VALUES ('test')", ErrNonSelectQuery},
		{"update", "UPDATE users SET name = 'test'", ErrNonSelectQuery},
		{"delete", "DELETE FROM users WHERE id = 1", ErrNonSelectQuery},
		{"drop", "DROP TABLE users", ErrNonSelectQuery},
		{"create", "CREATE TABLE users (id INT)", ErrNonSelectQuery},
		{"alter", "ALTER TABLE users ADD COLUMN age INT", ErrNonSelectQuery},
		{"truncate", "TRUNCATE TABLE users", ErrNonSelectQuery},
		{"grant", "GRANT SELECT ON users TO user1", ErrNonSelectQuery},
		{"revoke", "REVOKE SELECT ON users FROM user1", ErrNonSelectQuery},
		{"exec", "EXEC sp_who", ErrNonSelectQuery},
		{"execute", "EXECUTE sp_help", ErrNonSelectQuery},
		{"select into", "SELECT * INTO backup FROM users", ErrNonSelectQuery},

		// Edge cases
		{"inline insert in select", "SELECT * FROM users; INSERT INTO logs VALUES (1)", ErrNonSelectQuery},
		{"starts with whitespace", "  SELECT * FROM users", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a query to test validateSelectOnly indirectly
			query := &QueryRequest{
				ID:   "test",
				Type: QueryTypeSQL,
				SQL:  tt.sql,
				TargetDatasets: []DatasetTarget{
					{DatasetID: "dataset-1"},
				},
			}
			err := query.Validate()
			if err != tt.wantErr {
				t.Errorf("validate SQL %q: error = %v, wantErr %v", tt.sql, err, tt.wantErr)
			}
		})
	}
}

func TestDatasetTarget(t *testing.T) {
	target := DatasetTarget{
		DatasetID: "dataset-1",
		VersionID: "v1.0.0",
		Alias:     "d1",
	}

	if target.DatasetID != "dataset-1" {
		t.Errorf("expected dataset-1, got %q", target.DatasetID)
	}
	if target.VersionID != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %q", target.VersionID)
	}
	if target.Alias != "d1" {
		t.Errorf("expected d1, got %q", target.Alias)
	}
}

func TestQueryResult(t *testing.T) {
	result := QueryResult{
		QueryID:    "query-1",
		Type:       QueryTypeSQL,
		TotalCount: 100,
		Truncated:  true,
		Columns: []QueryColumn{
			{Name: "id", Type: "integer", Nullable: false},
			{Name: "name", Type: "text", Nullable: true},
		},
		Rows: [][]any{
			{1, "Alice"},
			{2, "Bob"},
		},
	}

	if result.QueryID != "query-1" {
		t.Errorf("expected query-1, got %q", result.QueryID)
	}
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(result.Columns))
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}
	if result.TotalCount != 100 {
		t.Errorf("expected 100, got %d", result.TotalCount)
	}
	if !result.Truncated {
		t.Error("expected Truncated to be true")
	}
}

func TestQueryColumn(t *testing.T) {
	col := QueryColumn{
		Name:     "user_id",
		Type:     "integer",
		Nullable: false,
	}

	if col.Name != "user_id" {
		t.Errorf("expected user_id, got %q", col.Name)
	}
	if col.Type != "integer" {
		t.Errorf("expected integer, got %q", col.Type)
	}
	if col.Nullable {
		t.Error("expected Nullable to be false")
	}
}

func TestStreamedQueryResult(t *testing.T) {
	result := StreamedQueryResult{
		QueryID:    "query-1",
		ChunkIndex: 0,
		Columns: []QueryColumn{
			{Name: "id", Type: "integer"},
		},
		Rows:      [][]any{{1}, {2}},
		IsLast:    false,
		TotalRows: 0,
	}

	if result.ChunkIndex != 0 {
		t.Errorf("expected chunk index 0, got %d", result.ChunkIndex)
	}
	if result.IsLast {
		t.Error("expected IsLast to be false")
	}

	// Test last chunk
	lastChunk := StreamedQueryResult{
		QueryID:    "query-1",
		ChunkIndex: 5,
		Rows:       [][]any{{11}, {12}},
		IsLast:     true,
		TotalRows:  12,
	}

	if !lastChunk.IsLast {
		t.Error("expected IsLast to be true")
	}
	if lastChunk.TotalRows != 12 {
		t.Errorf("expected TotalRows 12, got %d", lastChunk.TotalRows)
	}
}
