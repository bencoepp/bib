package audit

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sync"
	"time"
)

// S3Exporter exports audit entries to S3-compatible object storage.
type S3Exporter struct {
	config   S3ExportConfig
	client   S3Client
	buffer   []*Entry
	mu       sync.Mutex
	closed   bool
	lastSync time.Time
}

// S3ExportConfig holds S3 export configuration.
type S3ExportConfig struct {
	// Enabled controls whether S3 export is active.
	Enabled bool `mapstructure:"enabled"`

	// Endpoint is the S3 endpoint URL.
	Endpoint string `mapstructure:"endpoint"`

	// Region is the AWS region.
	Region string `mapstructure:"region"`

	// Bucket is the S3 bucket name.
	Bucket string `mapstructure:"bucket"`

	// Prefix is the key prefix for objects.
	Prefix string `mapstructure:"prefix"`

	// AccessKeyID is the AWS access key.
	AccessKeyID string `mapstructure:"access_key_id"`

	// SecretAccessKey is the AWS secret key.
	SecretAccessKey string `mapstructure:"secret_access_key"`

	// UseIAM uses IAM role for authentication instead of keys.
	UseIAM bool `mapstructure:"use_iam"`

	// BatchSize is the number of entries per batch upload.
	BatchSize int `mapstructure:"batch_size"`

	// FlushInterval is how often to flush pending entries.
	FlushInterval time.Duration `mapstructure:"flush_interval"`

	// Compress enables gzip compression.
	Compress bool `mapstructure:"compress"`

	// PartitionBy determines the key partitioning scheme.
	// Options: "hour", "day", "month"
	PartitionBy string `mapstructure:"partition_by"`
}

// S3Client is the interface for S3 operations.
// This allows for easy mocking in tests.
type S3Client interface {
	// PutObject uploads an object to S3.
	PutObject(ctx context.Context, bucket, key string, body io.Reader, contentType string, metadata map[string]string) error

	// ListObjects lists objects with a given prefix.
	ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]S3Object, error)

	// GetObject retrieves an object from S3.
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)

	// DeleteObject deletes an object from S3.
	DeleteObject(ctx context.Context, bucket, key string) error
}

// S3Object represents an S3 object in a listing.
type S3Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// DefaultS3ExportConfig returns the default S3 export configuration.
func DefaultS3ExportConfig() S3ExportConfig {
	return S3ExportConfig{
		Enabled:       false,
		Region:        "us-east-1",
		Prefix:        "audit/",
		BatchSize:     1000,
		FlushInterval: 5 * time.Minute,
		Compress:      true,
		PartitionBy:   "hour",
	}
}

// NewS3Exporter creates a new S3 exporter.
func NewS3Exporter(cfg S3ExportConfig, client S3Client) (*S3Exporter, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if client == nil {
		return nil, fmt.Errorf("S3 client is required")
	}

	exporter := &S3Exporter{
		config:   cfg,
		client:   client,
		buffer:   make([]*Entry, 0, cfg.BatchSize),
		lastSync: time.Now(),
	}

	return exporter, nil
}

// Export adds an entry to the buffer.
func (e *S3Exporter) Export(ctx context.Context, entry *Entry) error {
	if e == nil || e.closed {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	e.buffer = append(e.buffer, entry)

	// Check if we should flush
	if len(e.buffer) >= e.config.BatchSize {
		return e.flushLocked(ctx)
	}

	return nil
}

// ExportBatch adds multiple entries to the buffer.
func (e *S3Exporter) ExportBatch(ctx context.Context, entries []*Entry) error {
	if e == nil || e.closed {
		return nil
	}

	for _, entry := range entries {
		if err := e.Export(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

// Flush flushes buffered entries to S3.
func (e *S3Exporter) Flush(ctx context.Context) error {
	if e == nil || e.closed {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return e.flushLocked(ctx)
}

// flushLocked flushes entries (caller must hold lock).
func (e *S3Exporter) flushLocked(ctx context.Context) error {
	if len(e.buffer) == 0 {
		return nil
	}

	entries := e.buffer
	e.buffer = make([]*Entry, 0, e.config.BatchSize)
	e.lastSync = time.Now()

	// Generate key
	key := e.generateKey(entries[0].Timestamp)

	// Marshal entries to JSON-lines
	var buf bytes.Buffer
	var writer io.Writer = &buf

	if e.config.Compress {
		gzw := gzip.NewWriter(&buf)
		writer = gzw
		defer gzw.Close()
	}

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal entry: %w", err)
		}
		if _, err := writer.Write(data); err != nil {
			return err
		}
		if _, err := writer.Write([]byte("\n")); err != nil {
			return err
		}
	}

	if e.config.Compress {
		// Flush gzip writer
		if gzw, ok := writer.(*gzip.Writer); ok {
			if err := gzw.Close(); err != nil {
				return err
			}
		}
	}

	// Upload to S3
	contentType := "application/x-ndjson"
	if e.config.Compress {
		contentType = "application/gzip"
	}

	metadata := map[string]string{
		"entry_count": fmt.Sprintf("%d", len(entries)),
		"first_entry": entries[0].Timestamp.Format(time.RFC3339),
		"last_entry":  entries[len(entries)-1].Timestamp.Format(time.RFC3339),
		"node_id":     entries[0].NodeID,
	}

	if err := e.client.PutObject(ctx, e.config.Bucket, key, &buf, contentType, metadata); err != nil {
		// Re-add entries to buffer on failure
		e.buffer = append(entries, e.buffer...)
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// generateKey generates the S3 object key for a batch.
func (e *S3Exporter) generateKey(t time.Time) string {
	var partition string
	switch e.config.PartitionBy {
	case "hour":
		partition = t.Format("2006/01/02/15")
	case "day":
		partition = t.Format("2006/01/02")
	case "month":
		partition = t.Format("2006/01")
	default:
		partition = t.Format("2006/01/02/15")
	}

	timestamp := t.Format("20060102T150405Z")
	extension := ".jsonl"
	if e.config.Compress {
		extension = ".jsonl.gz"
	}

	return path.Join(e.config.Prefix, partition, fmt.Sprintf("audit-%s%s", timestamp, extension))
}

// Close closes the S3 exporter, flushing any remaining entries.
func (e *S3Exporter) Close(ctx context.Context) error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	e.mu.Unlock()

	return e.Flush(ctx)
}

// Stats returns S3 exporter statistics.
func (e *S3Exporter) Stats() S3ExportStats {
	if e == nil {
		return S3ExportStats{}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return S3ExportStats{
		BufferedEntries: len(e.buffer),
		LastSyncTime:    e.lastSync,
		Closed:          e.closed,
	}
}

// S3ExportStats contains S3 export statistics.
type S3ExportStats struct {
	BufferedEntries int       `json:"buffered_entries"`
	LastSyncTime    time.Time `json:"last_sync_time"`
	Closed          bool      `json:"closed"`
}

// S3Reader reads audit entries from S3.
type S3Reader struct {
	client S3Client
	config S3ExportConfig
}

// NewS3Reader creates a reader for S3 audit data.
func NewS3Reader(cfg S3ExportConfig, client S3Client) *S3Reader {
	return &S3Reader{
		client: client,
		config: cfg,
	}
}

// ListBatches lists available audit batches in a time range.
func (r *S3Reader) ListBatches(ctx context.Context, after, before time.Time) ([]S3Object, error) {
	// Generate prefix based on time range
	prefix := r.config.Prefix

	objects, err := r.client.ListObjects(ctx, r.config.Bucket, prefix, 10000)
	if err != nil {
		return nil, err
	}

	// Filter by time range
	var filtered []S3Object
	for _, obj := range objects {
		if obj.LastModified.After(after) && obj.LastModified.Before(before) {
			filtered = append(filtered, obj)
		}
	}

	return filtered, nil
}

// ReadBatch reads entries from a specific batch.
func (r *S3Reader) ReadBatch(ctx context.Context, key string) ([]*Entry, error) {
	reader, err := r.client.GetObject(ctx, r.config.Bucket, key)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var dataReader io.Reader = reader

	// Check if compressed
	if path.Ext(key) == ".gz" {
		gzr, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer gzr.Close()
		dataReader = gzr
	}

	var entries []*Entry
	decoder := json.NewDecoder(dataReader)
	for decoder.More() {
		var entry Entry
		if err := decoder.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}
