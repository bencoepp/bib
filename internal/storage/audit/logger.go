package audit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Logger is the main audit logger that coordinates all audit components.
type Logger struct {
	config       Config
	nodeID       string
	redactor     *Redactor
	streamer     *Streamer
	detector     *AlertDetector
	rateLimiter  *RateLimiter
	syslog       *SyslogExporter
	fileExporter *FileExporter
	s3Exporter   *S3Exporter
	repository   Repository

	hashChain  bool
	lastHash   string
	mu         sync.Mutex
	closed     bool
	entryCount int64
}

// Config holds the complete audit configuration.
type Config struct {
	// Enabled controls whether audit logging is active.
	Enabled bool `mapstructure:"enabled"`

	// HashChain enables hash chain for tamper detection.
	HashChain bool `mapstructure:"hash_chain"`

	// RetentionDays is how long to keep audit logs in the database.
	RetentionDays int `mapstructure:"retention_days"`

	// Redact holds redaction configuration.
	Redact RedactorConfig `mapstructure:"redact"`

	// Streaming holds streamer configuration.
	Streaming StreamerConfig `mapstructure:"streaming"`

	// Alerts holds alert detection configuration.
	Alerts AlertConfig `mapstructure:"alerts"`

	// RateLimit holds rate limiter configuration.
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`

	// Syslog holds syslog export configuration.
	Syslog SyslogConfig `mapstructure:"syslog"`

	// File holds file export configuration.
	File FileExportConfig `mapstructure:"file"`

	// S3 holds S3 export configuration.
	S3 S3ExportConfig `mapstructure:"s3"`
}

// Repository is the interface for persisting audit entries.
type Repository interface {
	// Log persists an audit entry.
	Log(ctx context.Context, entry *Entry) error

	// Query retrieves entries matching a filter.
	Query(ctx context.Context, filter Filter) ([]*Entry, error)

	// Count returns the number of matching entries.
	Count(ctx context.Context, filter Filter) (int64, error)

	// Purge removes entries older than the given time.
	Purge(ctx context.Context, before time.Time) (int64, error)

	// VerifyChain verifies hash chain integrity.
	VerifyChain(ctx context.Context, from, to int64) (bool, error)

	// GetLastHash returns the hash of the last entry.
	GetLastHash(ctx context.Context) (string, error)
}

// DefaultConfig returns the default audit configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		HashChain:     true,
		RetentionDays: 90,
		Redact:        DefaultRedactorConfig(),
		Streaming:     DefaultStreamerConfig(),
		Alerts:        DefaultAlertConfig(),
		RateLimit:     DefaultRateLimitConfig(),
		Syslog:        DefaultSyslogConfig(),
		File:          DefaultFileExportConfig(),
		S3:            DefaultS3ExportConfig(),
	}
}

// NewLogger creates a new audit logger.
func NewLogger(cfg Config, nodeID string, repo Repository, s3Client S3Client) (*Logger, error) {
	if !cfg.Enabled {
		return &Logger{closed: true}, nil
	}

	logger := &Logger{
		config:     cfg,
		nodeID:     nodeID,
		hashChain:  cfg.HashChain,
		repository: repo,
	}

	// Initialize redactor
	logger.redactor = NewRedactor(cfg.Redact)

	// Initialize streamer
	logger.streamer = NewStreamer(cfg.Streaming)

	// Initialize alert detector
	if cfg.Alerts.Enabled {
		detector, err := NewAlertDetector(cfg.Alerts)
		if err != nil {
			return nil, fmt.Errorf("failed to create alert detector: %w", err)
		}
		logger.detector = detector

		// Wire up rate limiting from alerts
		if cfg.RateLimit.Enabled {
			logger.rateLimiter = NewRateLimiter(cfg.RateLimit)
			detector.OnAlert(func(ctx context.Context, alert *Alert) {
				if alert.TriggerRateLimit {
					key := "actor:" + alert.Entry.Actor
					logger.rateLimiter.TriggerBlock(key, cfg.RateLimit.BlockDuration)
				}
			})
		}
	}

	// Initialize syslog exporter
	if cfg.Syslog.Enabled {
		syslog, err := NewSyslogExporter(cfg.Syslog)
		if err != nil {
			return nil, fmt.Errorf("failed to create syslog exporter: %w", err)
		}
		logger.syslog = syslog
	}

	// Initialize file exporter
	if cfg.File.Enabled {
		fileExp, err := NewFileExporter(cfg.File)
		if err != nil {
			return nil, fmt.Errorf("failed to create file exporter: %w", err)
		}
		logger.fileExporter = fileExp
	}

	// Initialize S3 exporter
	if cfg.S3.Enabled && s3Client != nil {
		s3Exp, err := NewS3Exporter(cfg.S3, s3Client)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 exporter: %w", err)
		}
		logger.s3Exporter = s3Exp
	}

	// Load last hash for chain continuity
	if cfg.HashChain && repo != nil {
		lastHash, err := repo.GetLastHash(context.Background())
		if err == nil {
			logger.lastHash = lastHash
		}
	}

	return logger, nil
}

// Log records an audit entry from query information.
func (l *Logger) Log(ctx context.Context, info QueryInfo, role, source, actor string) error {
	if l == nil || l.closed || !l.config.Enabled {
		return nil
	}

	// Create entry
	entry := l.createEntry(info, role, source, actor)

	// Check rate limit
	if l.rateLimiter != nil {
		allowed, reason := l.rateLimiter.Check(ctx, entry)
		if !allowed {
			entry.Flags.RateLimited = true
			entry.Metadata["rate_limit_reason"] = reason
		}
	}

	// Check for alerts
	if l.detector != nil {
		alerts := l.detector.Check(ctx, entry)
		if len(alerts) > 0 {
			entry.Flags.AlertTriggered = true
			entry.Flags.Suspicious = true
			entry.Metadata["alerts"] = alerts
		}
	}

	// Set hash chain
	l.mu.Lock()
	if l.hashChain {
		entry.SetHashChain(l.lastHash)
		l.lastHash = entry.EntryHash
	} else {
		entry.EntryHash = entry.CalculateHash()
	}
	l.entryCount++
	l.mu.Unlock()

	// Persist to repository
	if l.repository != nil {
		if err := l.repository.Log(ctx, entry); err != nil {
			return fmt.Errorf("failed to persist audit entry: %w", err)
		}
	}

	// Publish to stream
	if l.streamer != nil {
		l.streamer.Publish(entry)
	}

	// Export to syslog
	if l.syslog != nil {
		if err := l.syslog.Export(ctx, entry); err != nil {
			// Log error but don't fail
			_ = err
		}
	}

	// Export to file
	if l.fileExporter != nil {
		if err := l.fileExporter.Export(ctx, entry); err != nil {
			_ = err
		}
	}

	// Export to S3
	if l.s3Exporter != nil {
		if err := l.s3Exporter.Export(ctx, entry); err != nil {
			_ = err
		}
	}

	return nil
}

// LogEntry records a pre-built audit entry.
func (l *Logger) LogEntry(ctx context.Context, entry *Entry) error {
	if l == nil || l.closed || !l.config.Enabled {
		return nil
	}

	// Ensure required fields
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	if entry.NodeID == "" {
		entry.NodeID = l.nodeID
	}
	if entry.OperationID == "" {
		entry.OperationID = GenerateOperationID()
	}

	// Redact metadata
	if entry.Metadata != nil {
		entry.Metadata = l.redactor.RedactMetadata(entry.Metadata)
	}

	// Set hash chain
	l.mu.Lock()
	if l.hashChain {
		entry.SetHashChain(l.lastHash)
		l.lastHash = entry.EntryHash
	} else {
		entry.EntryHash = entry.CalculateHash()
	}
	l.entryCount++
	l.mu.Unlock()

	// Persist to repository
	if l.repository != nil {
		if err := l.repository.Log(ctx, entry); err != nil {
			return fmt.Errorf("failed to persist audit entry: %w", err)
		}
	}

	// Publish to stream
	if l.streamer != nil {
		l.streamer.Publish(entry)
	}

	// Export to syslog
	if l.syslog != nil {
		if err := l.syslog.Export(ctx, entry); err != nil {
			_ = err
		}
	}

	// Export to file
	if l.fileExporter != nil {
		if err := l.fileExporter.Export(ctx, entry); err != nil {
			_ = err
		}
	}

	// Export to S3
	if l.s3Exporter != nil {
		if err := l.s3Exporter.Export(ctx, entry); err != nil {
			_ = err
		}
	}

	return nil
}

// createEntry creates an audit entry from query info.
func (l *Logger) createEntry(info QueryInfo, role, source, actor string) *Entry {
	// Redact query and arguments
	redactedQuery, redactedArgs := l.redactor.RedactQuery(info.Query, info.Args)

	entry := &Entry{
		Timestamp:       info.StartTime,
		NodeID:          l.nodeID,
		OperationID:     GenerateOperationID(),
		RoleUsed:        role,
		Action:          ParseAction(info.Query),
		TableName:       ExtractTableName(info.Query),
		Query:           redactedQuery,
		QueryHash:       l.redactor.HashQuery(info.Query),
		RowsAffected:    int(info.RowsAffected),
		DurationMS:      int(info.Duration.Milliseconds()),
		SourceComponent: source,
		Actor:           actor,
		Metadata:        make(map[string]any),
	}

	// Add redacted args to metadata if not empty
	if len(redactedArgs) > 0 {
		entry.Metadata["args"] = redactedArgs
	}

	// Add error if present
	if info.Error != nil {
		entry.Metadata["error"] = info.Error.Error()
	}

	return entry
}

// Query retrieves audit entries matching the filter.
func (l *Logger) Query(ctx context.Context, filter Filter) ([]*Entry, error) {
	if l == nil || l.repository == nil {
		return nil, nil
	}
	return l.repository.Query(ctx, filter)
}

// Count returns the number of entries matching the filter.
func (l *Logger) Count(ctx context.Context, filter Filter) (int64, error) {
	if l == nil || l.repository == nil {
		return 0, nil
	}
	return l.repository.Count(ctx, filter)
}

// Purge removes entries older than the retention period.
func (l *Logger) Purge(ctx context.Context) (int64, error) {
	if l == nil || l.repository == nil {
		return 0, nil
	}

	before := time.Now().AddDate(0, 0, -l.config.RetentionDays)
	return l.repository.Purge(ctx, before)
}

// VerifyChain verifies the hash chain integrity.
func (l *Logger) VerifyChain(ctx context.Context, from, to int64) (bool, error) {
	if l == nil || l.repository == nil {
		return true, nil
	}
	return l.repository.VerifyChain(ctx, from, to)
}

// Streamer returns the internal streamer for subscribing to events.
func (l *Logger) Streamer() *Streamer {
	if l == nil {
		return nil
	}
	return l.streamer
}

// Detector returns the alert detector.
func (l *Logger) Detector() *AlertDetector {
	if l == nil {
		return nil
	}
	return l.detector
}

// RateLimiter returns the rate limiter.
func (l *Logger) RateLimiter() *RateLimiter {
	if l == nil {
		return nil
	}
	return l.rateLimiter
}

// IsRateLimited checks if an operation should be rate limited.
func (l *Logger) IsRateLimited(ctx context.Context, entry *Entry) (bool, string) {
	if l == nil || l.rateLimiter == nil {
		return false, ""
	}
	allowed, reason := l.rateLimiter.Check(ctx, entry)
	return !allowed, reason
}

// Flush flushes all buffered data.
func (l *Logger) Flush(ctx context.Context) error {
	if l == nil {
		return nil
	}

	if l.fileExporter != nil {
		if err := l.fileExporter.Flush(); err != nil {
			return err
		}
	}

	if l.s3Exporter != nil {
		if err := l.s3Exporter.Flush(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the audit logger.
func (l *Logger) Close(ctx context.Context) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()

	// Flush pending data
	if err := l.Flush(ctx); err != nil {
		return err
	}

	// Close exporters
	if l.streamer != nil {
		l.streamer.Close()
	}

	if l.syslog != nil {
		l.syslog.Close()
	}

	if l.fileExporter != nil {
		l.fileExporter.Close()
	}

	if l.s3Exporter != nil {
		l.s3Exporter.Close(ctx)
	}

	return nil
}

// Stats returns audit logger statistics.
func (l *Logger) Stats() LoggerStats {
	if l == nil {
		return LoggerStats{}
	}

	l.mu.Lock()
	entryCount := l.entryCount
	l.mu.Unlock()

	stats := LoggerStats{
		Enabled:    l.config.Enabled,
		EntryCount: entryCount,
		Closed:     l.closed,
	}

	if l.streamer != nil {
		stats.Subscribers = l.streamer.SubscriberCount()
	}

	if l.detector != nil {
		stats.Alerts = l.detector.GetStats()
	}

	if l.rateLimiter != nil {
		stats.RateLimit = l.rateLimiter.GetStats()
	}

	if l.fileExporter != nil {
		stats.FileExport = l.fileExporter.Stats()
	}

	if l.s3Exporter != nil {
		stats.S3Export = l.s3Exporter.Stats()
	}

	return stats
}

// LoggerStats contains audit logger statistics.
type LoggerStats struct {
	Enabled     bool            `json:"enabled"`
	EntryCount  int64           `json:"entry_count"`
	Subscribers int             `json:"subscribers"`
	Closed      bool            `json:"closed"`
	Alerts      AlertStats      `json:"alerts"`
	RateLimit   RateLimitStats  `json:"rate_limit"`
	FileExport  FileExportStats `json:"file_export"`
	S3Export    S3ExportStats   `json:"s3_export"`
}
