package pool

import (
	"context"
	"time"

	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RoleConnection wraps a pooled connection with role context.
// It provides a transaction that is automatically configured with
// the appropriate database role using SET LOCAL ROLE.
type RoleConnection struct {
	conn       *pgxpool.Conn
	tx         pgx.Tx
	role       storage.DBRole
	pool       *RoleAwarePool
	startTime  time.Time
	committed  bool
	rolledBack bool
}

// Tx returns the underlying transaction.
func (rc *RoleConnection) Tx() pgx.Tx {
	return rc.tx
}

// Role returns the role this connection is operating as.
func (rc *RoleConnection) Role() storage.DBRole {
	return rc.role
}

// StartTime returns when this connection was acquired.
func (rc *RoleConnection) StartTime() time.Time {
	return rc.startTime
}

// Duration returns how long this connection has been held.
func (rc *RoleConnection) Duration() time.Duration {
	return time.Since(rc.startTime)
}

// Commit commits the transaction.
// The role automatically resets when the transaction ends.
func (rc *RoleConnection) Commit(ctx context.Context) error {
	if rc.committed || rc.rolledBack {
		return nil
	}
	rc.committed = true
	return rc.tx.Commit(ctx)
}

// Rollback rolls back the transaction.
// The role automatically resets when the transaction ends.
func (rc *RoleConnection) Rollback(ctx context.Context) error {
	if rc.committed || rc.rolledBack {
		return nil
	}
	rc.rolledBack = true
	return rc.tx.Rollback(ctx)
}

// Release returns the connection to the pool.
// If the transaction hasn't been committed or rolled back,
// it will be rolled back automatically.
func (rc *RoleConnection) Release() {
	if !rc.committed && !rc.rolledBack {
		rc.tx.Rollback(context.Background())
		rc.rolledBack = true
	}
	rc.conn.Release()
}

// Exec executes a query that doesn't return rows.
func (rc *RoleConnection) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return rc.tx.Exec(ctx, sql, args...)
}

// Query executes a query that returns rows.
func (rc *RoleConnection) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return rc.tx.Query(ctx, sql, args...)
}

// QueryRow executes a query that returns at most one row.
func (rc *RoleConnection) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return rc.tx.QueryRow(ctx, sql, args...)
}

// CopyFrom uses the PostgreSQL COPY protocol to bulk insert rows.
func (rc *RoleConnection) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return rc.tx.CopyFrom(ctx, tableName, columnNames, rowSrc)
}

// Prepare creates a prepared statement.
func (rc *RoleConnection) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	// Note: Prepared statements are connection-scoped, not transaction-scoped
	return rc.conn.Conn().Prepare(ctx, name, sql)
}

// SendBatch sends a batch of queries.
func (rc *RoleConnection) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	return rc.tx.SendBatch(ctx, batch)
}

// LargeObjects returns a LargeObjects instance for working with large objects.
func (rc *RoleConnection) LargeObjects() pgx.LargeObjects {
	return rc.tx.LargeObjects()
}

// Conn returns the underlying *pgx.Conn.
// Use with caution - direct access bypasses role restrictions.
func (rc *RoleConnection) Conn() *pgx.Conn {
	return rc.conn.Conn()
}

// IsCommitted returns true if the transaction has been committed.
func (rc *RoleConnection) IsCommitted() bool {
	return rc.committed
}

// IsRolledBack returns true if the transaction has been rolled back.
func (rc *RoleConnection) IsRolledBack() bool {
	return rc.rolledBack
}

// IsClosed returns true if the transaction has ended (committed or rolled back).
func (rc *RoleConnection) IsClosed() bool {
	return rc.committed || rc.rolledBack
}

// Savepoint creates a savepoint within the transaction.
func (rc *RoleConnection) Savepoint(ctx context.Context, name string) error {
	_, err := rc.tx.Exec(ctx, "SAVEPOINT "+pgx.Identifier{name}.Sanitize())
	return err
}

// RollbackToSavepoint rolls back to a savepoint.
func (rc *RoleConnection) RollbackToSavepoint(ctx context.Context, name string) error {
	_, err := rc.tx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+pgx.Identifier{name}.Sanitize())
	return err
}

// ReleaseSavepoint releases a savepoint.
func (rc *RoleConnection) ReleaseSavepoint(ctx context.Context, name string) error {
	_, err := rc.tx.Exec(ctx, "RELEASE SAVEPOINT "+pgx.Identifier{name}.Sanitize())
	return err
}
