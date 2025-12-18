package audit

import (
	"context"
	"time"
)

// PoolMiddleware wraps database operations to automatically log queries.
type PoolMiddleware struct {
	logger *Logger
	source string
}

// NewPoolMiddleware creates a new pool middleware for automatic query logging.
func NewPoolMiddleware(logger *Logger, source string) *PoolMiddleware {
	return &PoolMiddleware{
		logger: logger,
		source: source,
	}
}

// LogQuery logs a query execution.
func (m *PoolMiddleware) LogQuery(ctx context.Context, query string, args []any, startTime time.Time, rowsAffected int64, err error, role, actor string) {
	if m == nil || m.logger == nil {
		return
	}

	info := QueryInfo{
		Query:        query,
		Args:         args,
		StartTime:    startTime,
		Duration:     time.Since(startTime),
		RowsAffected: rowsAffected,
		Error:        err,
	}

	// Log asynchronously to not block the query
	go func() {
		_ = m.logger.Log(context.Background(), info, role, m.source, actor)
	}()
}

// WrapQuery wraps a query function with logging.
func (m *PoolMiddleware) WrapQuery(ctx context.Context, role, actor string, query string, args []any, fn func() (int64, error)) (int64, error) {
	startTime := time.Now()
	rowsAffected, err := fn()
	m.LogQuery(ctx, query, args, startTime, rowsAffected, err, role, actor)
	return rowsAffected, err
}

// QueryTracer provides tracing hooks for database operations.
type QueryTracer struct {
	middleware *PoolMiddleware
}

// NewQueryTracer creates a new query tracer.
func NewQueryTracer(middleware *PoolMiddleware) *QueryTracer {
	return &QueryTracer{
		middleware: middleware,
	}
}

// TraceQueryStart is called when a query starts.
func (t *QueryTracer) TraceQueryStart(ctx context.Context, query string, args []any) context.Context {
	return context.WithValue(ctx, queryStartKey, queryStartData{
		startTime: time.Now(),
		query:     query,
		args:      args,
	})
}

// TraceQueryEnd is called when a query ends.
func (t *QueryTracer) TraceQueryEnd(ctx context.Context, rowsAffected int64, err error, role, actor string) {
	data, ok := ctx.Value(queryStartKey).(queryStartData)
	if !ok {
		return
	}

	t.middleware.LogQuery(ctx, data.query, data.args, data.startTime, rowsAffected, err, role, actor)
}

type queryStartKeyType int

const queryStartKey queryStartKeyType = 0

type queryStartData struct {
	startTime time.Time
	query     string
	args      []any
}

// BatchLogger accumulates query logs and flushes them periodically.
type BatchLogger struct {
	logger    *Logger
	source    string
	entries   chan *Entry
	batchSize int
	done      chan struct{}
}

// NewBatchLogger creates a new batch logger.
func NewBatchLogger(logger *Logger, source string, batchSize int, bufferSize int) *BatchLogger {
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	bl := &BatchLogger{
		logger:    logger,
		source:    source,
		entries:   make(chan *Entry, bufferSize),
		batchSize: batchSize,
		done:      make(chan struct{}),
	}

	return bl
}

// Start begins processing entries.
func (bl *BatchLogger) Start(ctx context.Context) {
	go bl.processLoop(ctx)
}

// processLoop processes entries in batches.
func (bl *BatchLogger) processLoop(ctx context.Context) {
	batch := make([]*Entry, 0, bl.batchSize)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			bl.flushBatch(batch)
			return
		case <-bl.done:
			bl.flushBatch(batch)
			return
		case entry := <-bl.entries:
			batch = append(batch, entry)
			if len(batch) >= bl.batchSize {
				bl.flushBatch(batch)
				batch = make([]*Entry, 0, bl.batchSize)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				bl.flushBatch(batch)
				batch = make([]*Entry, 0, bl.batchSize)
			}
		}
	}
}

// flushBatch persists a batch of entries.
func (bl *BatchLogger) flushBatch(batch []*Entry) {
	for _, entry := range batch {
		_ = bl.logger.LogEntry(context.Background(), entry)
	}
}

// Log queues an entry for logging.
func (bl *BatchLogger) Log(info QueryInfo, role, actor string) {
	if bl.logger == nil {
		return
	}

	// Create entry
	redactedQuery, redactedArgs := bl.logger.redactor.RedactQuery(info.Query, info.Args)

	entry := &Entry{
		Timestamp:       info.StartTime,
		NodeID:          bl.logger.nodeID,
		OperationID:     GenerateOperationID(),
		RoleUsed:        role,
		Action:          ParseAction(info.Query),
		TableName:       ExtractTableName(info.Query),
		Query:           redactedQuery,
		QueryHash:       bl.logger.redactor.HashQuery(info.Query),
		RowsAffected:    int(info.RowsAffected),
		DurationMS:      int(info.Duration.Milliseconds()),
		SourceComponent: bl.source,
		Actor:           actor,
		Metadata:        make(map[string]any),
	}

	if len(redactedArgs) > 0 {
		entry.Metadata["args"] = redactedArgs
	}

	if info.Error != nil {
		entry.Metadata["error"] = info.Error.Error()
	}

	select {
	case bl.entries <- entry:
	default:
		// Channel full, drop entry (or could log to error)
	}
}

// Stop stops the batch logger.
func (bl *BatchLogger) Stop() {
	close(bl.done)
}
