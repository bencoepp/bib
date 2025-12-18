// Package pool provides a role-aware PostgreSQL connection pool.
// It supports per-transaction role switching using SET LOCAL ROLE
// for fine-grained access control.
package pool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"bib/internal/storage"
	"bib/internal/storage/postgres/credentials"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrPoolClosed indicates the pool has been closed.
	ErrPoolClosed = errors.New("connection pool is closed")

	// ErrInvalidRole indicates an invalid or unknown role.
	ErrInvalidRole = errors.New("invalid database role")

	// ErrNoOperationContext indicates OperationContext is required but missing.
	ErrNoOperationContext = errors.New("OperationContext required for database operations")

	// ErrRoleSwitchFailed indicates SET ROLE failed.
	ErrRoleSwitchFailed = errors.New("failed to switch database role")
)

// Config holds pool configuration.
type Config struct {
	// MaxConns is the maximum number of connections in the pool.
	MaxConns int32

	// MinConns is the minimum number of connections to keep open.
	MinConns int32

	// MaxConnLifetime is the maximum lifetime of a connection.
	MaxConnLifetime time.Duration

	// MaxConnIdleTime is the maximum time a connection can be idle.
	MaxConnIdleTime time.Duration

	// HealthCheckPeriod is how often to check connection health.
	HealthCheckPeriod time.Duration

	// ConnectTimeout is the timeout for new connections.
	ConnectTimeout time.Duration
}

// DefaultConfig returns sensible pool defaults.
func DefaultConfig() Config {
	return Config{
		MaxConns:          25,
		MinConns:          5,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		HealthCheckPeriod: time.Minute,
		ConnectTimeout:    5 * time.Second,
	}
}

// RoleAwarePool wraps pgxpool with role-based access control.
// It uses a single connection pool connecting as bibd_admin,
// then uses SET LOCAL ROLE to switch to the appropriate role
// for each transaction.
type RoleAwarePool struct {
	pool        *pgxpool.Pool
	config      Config
	nodeID      string
	defaultRole storage.DBRole
	creds       *credentials.Credentials

	mu     sync.RWMutex
	closed bool

	// Stats
	roleUsage map[storage.DBRole]int64
}

// New creates a new role-aware connection pool.
func New(ctx context.Context, connString string, cfg Config, nodeID string, creds *credentials.Credentials) (*RoleAwarePool, error) {
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Apply pool configuration
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	// Create the pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &RoleAwarePool{
		pool:        pool,
		config:      cfg,
		nodeID:      nodeID,
		defaultRole: storage.RoleReadOnly,
		creds:       creds,
		roleUsage:   make(map[storage.DBRole]int64),
	}, nil
}

// AcquireWithRole acquires a connection and sets up a transaction with the specified role.
func (p *RoleAwarePool) AcquireWithRole(ctx context.Context, role storage.DBRole) (*RoleConnection, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrPoolClosed
	}
	p.mu.RUnlock()

	if !role.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRole, role)
	}

	// Acquire connection from pool
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}

	// Begin transaction
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// SET LOCAL ROLE - only applies to current transaction
	// Using identifier quoting to prevent SQL injection
	_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL ROLE %s", pgx.Identifier{string(role)}.Sanitize()))
	if err != nil {
		tx.Rollback(ctx)
		conn.Release()
		return nil, fmt.Errorf("%w %s: %v", ErrRoleSwitchFailed, role, err)
	}

	// Track role usage
	p.mu.Lock()
	p.roleUsage[role]++
	p.mu.Unlock()

	return &RoleConnection{
		conn:      conn,
		tx:        tx,
		role:      role,
		pool:      p,
		startTime: time.Now(),
	}, nil
}

// AcquireWithContext acquires a connection using the role from OperationContext.
func (p *RoleAwarePool) AcquireWithContext(ctx context.Context) (*RoleConnection, error) {
	opCtx := storage.GetOperationContext(ctx)
	if opCtx == nil {
		return nil, ErrNoOperationContext
	}

	role := opCtx.Role
	if !role.IsValid() {
		role = p.defaultRole
	}

	return p.AcquireWithRole(ctx, role)
}

// Execute runs a function within a transaction using the role from OperationContext.
func (p *RoleAwarePool) Execute(ctx context.Context, fn func(tx pgx.Tx) error) error {
	conn, err := p.AcquireWithContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if err := fn(conn.Tx()); err != nil {
		conn.Rollback(ctx)
		return err
	}

	return conn.Commit(ctx)
}

// ExecuteWithRole runs a function within a transaction using the specified role.
func (p *RoleAwarePool) ExecuteWithRole(ctx context.Context, role storage.DBRole, fn func(tx pgx.Tx) error) error {
	conn, err := p.AcquireWithRole(ctx, role)
	if err != nil {
		return err
	}
	defer conn.Release()

	if err := fn(conn.Tx()); err != nil {
		conn.Rollback(ctx)
		return err
	}

	return conn.Commit(ctx)
}

// Query executes a query with the role from OperationContext.
func (p *RoleAwarePool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	conn, err := p.AcquireWithContext(ctx)
	if err != nil {
		return nil, err
	}
	// Note: caller must handle connection release via rows.Close()

	rows, err := conn.tx.Query(ctx, sql, args...)
	if err != nil {
		conn.Rollback(ctx)
		conn.Release()
		return nil, err
	}

	return &trackedRows{
		Rows: rows,
		conn: conn,
		ctx:  ctx,
	}, nil
}

// QueryRow executes a query that returns a single row.
func (p *RoleAwarePool) QueryRow(ctx context.Context, sql string, args ...any) (pgx.Row, func(), error) {
	conn, err := p.AcquireWithContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	row := conn.tx.QueryRow(ctx, sql, args...)
	cleanup := func() {
		conn.Commit(ctx)
		conn.Release()
	}

	return row, cleanup, nil
}

// Exec executes a statement that doesn't return rows.
func (p *RoleAwarePool) Exec(ctx context.Context, sql string, args ...any) error {
	return p.Execute(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, sql, args...)
		return err
	})
}

// UpdateCredentials updates the pool's credentials for rotation.
func (p *RoleAwarePool) UpdateCredentials(creds *credentials.Credentials) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.creds = creds
}

// Stats returns pool statistics.
func (p *RoleAwarePool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	poolStats := p.pool.Stat()

	usage := make(map[storage.DBRole]int64, len(p.roleUsage))
	for role, count := range p.roleUsage {
		usage[role] = count
	}

	return PoolStats{
		TotalConns:        poolStats.TotalConns(),
		AcquiredConns:     poolStats.AcquiredConns(),
		IdleConns:         poolStats.IdleConns(),
		MaxConns:          poolStats.MaxConns(),
		AcquireCount:      poolStats.AcquireCount(),
		AcquireDuration:   poolStats.AcquireDuration(),
		EmptyAcquireCount: poolStats.EmptyAcquireCount(),
		RoleUsage:         usage,
	}
}

// Close closes the connection pool.
func (p *RoleAwarePool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	p.pool.Close()
}

// Ping checks database connectivity.
func (p *RoleAwarePool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// PoolStats contains pool statistics.
type PoolStats struct {
	TotalConns        int32
	AcquiredConns     int32
	IdleConns         int32
	MaxConns          int32
	AcquireCount      int64
	AcquireDuration   time.Duration
	EmptyAcquireCount int64
	RoleUsage         map[storage.DBRole]int64
}

// trackedRows wraps pgx.Rows to handle connection cleanup.
type trackedRows struct {
	pgx.Rows
	conn *RoleConnection
	ctx  context.Context
}

func (r *trackedRows) Close() {
	r.Rows.Close()
	r.conn.Commit(r.ctx)
	r.conn.Release()
}
