package audit

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileExporter exports audit entries to JSON-lines files.
type FileExporter struct {
	config   FileExportConfig
	file     *os.File
	writer   *bufio.Writer
	gzWriter *gzip.Writer
	mu       sync.Mutex
	closed   bool
	count    int64
	size     int64
}

// FileExportConfig holds file export configuration.
type FileExportConfig struct {
	// Enabled controls whether file export is active.
	Enabled bool `mapstructure:"enabled"`

	// Directory is the base directory for export files.
	Directory string `mapstructure:"directory"`

	// FilePrefix is the prefix for export files.
	FilePrefix string `mapstructure:"file_prefix"`

	// MaxFileSize is the maximum file size before rotation (bytes).
	MaxFileSize int64 `mapstructure:"max_file_size"`

	// MaxAge is the maximum age of files before cleanup.
	MaxAge time.Duration `mapstructure:"max_age"`

	// Compress enables gzip compression.
	Compress bool `mapstructure:"compress"`

	// RotationInterval is how often to rotate files.
	RotationInterval time.Duration `mapstructure:"rotation_interval"`

	// BufferSize is the write buffer size.
	BufferSize int `mapstructure:"buffer_size"`
}

// DefaultFileExportConfig returns the default file export configuration.
func DefaultFileExportConfig() FileExportConfig {
	return FileExportConfig{
		Enabled:          false,
		Directory:        "./audit-logs",
		FilePrefix:       "audit",
		MaxFileSize:      100 * 1024 * 1024, // 100MB
		MaxAge:           90 * 24 * time.Hour,
		Compress:         true,
		RotationInterval: 24 * time.Hour,
		BufferSize:       64 * 1024, // 64KB
	}
}

// NewFileExporter creates a new file exporter.
func NewFileExporter(cfg FileExportConfig) (*FileExporter, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(cfg.Directory, 0750); err != nil {
		return nil, fmt.Errorf("failed to create audit directory: %w", err)
	}

	exporter := &FileExporter{
		config: cfg,
	}

	if err := exporter.openNewFile(); err != nil {
		return nil, err
	}

	return exporter, nil
}

// openNewFile opens a new audit file.
func (e *FileExporter) openNewFile() error {
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05")
	extension := ".jsonl"
	if e.config.Compress {
		extension = ".jsonl.gz"
	}

	filename := fmt.Sprintf("%s-%s%s", e.config.FilePrefix, timestamp, extension)
	path := filepath.Join(e.config.Directory, filename)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return fmt.Errorf("failed to open audit file: %w", err)
	}

	bufSize := e.config.BufferSize
	if bufSize <= 0 {
		bufSize = 64 * 1024
	}

	e.file = file
	e.size = 0
	e.count = 0

	if e.config.Compress {
		e.gzWriter = gzip.NewWriter(file)
		e.writer = bufio.NewWriterSize(e.gzWriter, bufSize)
	} else {
		e.writer = bufio.NewWriterSize(file, bufSize)
	}

	return nil
}

// Export writes an audit entry to the file.
func (e *FileExporter) Export(ctx context.Context, entry *Entry) error {
	if e == nil || e.closed {
		return nil
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	// Check if rotation is needed
	if e.size+int64(len(data)+1) > e.config.MaxFileSize {
		if err := e.rotate(); err != nil {
			return fmt.Errorf("failed to rotate audit file: %w", err)
		}
	}

	// Write entry
	n, err := e.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}
	e.size += int64(n)

	// Write newline
	if _, err := e.writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}
	e.size++
	e.count++

	return nil
}

// ExportBatch writes multiple entries to the file.
func (e *FileExporter) ExportBatch(ctx context.Context, entries []*Entry) error {
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

// rotate closes the current file and opens a new one.
func (e *FileExporter) rotate() error {
	if err := e.closeCurrentFile(); err != nil {
		return err
	}
	return e.openNewFile()
}

// closeCurrentFile closes the current file.
func (e *FileExporter) closeCurrentFile() error {
	if e.writer != nil {
		if err := e.writer.Flush(); err != nil {
			return err
		}
	}
	if e.gzWriter != nil {
		if err := e.gzWriter.Close(); err != nil {
			return err
		}
	}
	if e.file != nil {
		if err := e.file.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Flush flushes buffered data to disk.
func (e *FileExporter) Flush() error {
	if e == nil || e.closed {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.writer != nil {
		if err := e.writer.Flush(); err != nil {
			return err
		}
	}
	if e.gzWriter != nil {
		if err := e.gzWriter.Flush(); err != nil {
			return err
		}
	}
	if e.file != nil {
		if err := e.file.Sync(); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the file exporter.
func (e *FileExporter) Close() error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	return e.closeCurrentFile()
}

// Cleanup removes old audit files.
func (e *FileExporter) Cleanup() error {
	if e == nil || e.config.MaxAge <= 0 {
		return nil
	}

	cutoff := time.Now().Add(-e.config.MaxAge)

	entries, err := os.ReadDir(e.config.Directory)
	if err != nil {
		return fmt.Errorf("failed to read audit directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(e.config.Directory, entry.Name())
			if err := os.Remove(path); err != nil {
				// Log error but continue
				continue
			}
		}
	}

	return nil
}

// Stats returns file exporter statistics.
func (e *FileExporter) Stats() FileExportStats {
	if e == nil {
		return FileExportStats{}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return FileExportStats{
		CurrentFileSize: e.size,
		EntryCount:      e.count,
		Closed:          e.closed,
	}
}

// FileExportStats contains file export statistics.
type FileExportStats struct {
	CurrentFileSize int64 `json:"current_file_size"`
	EntryCount      int64 `json:"entry_count"`
	Closed          bool  `json:"closed"`
}

// FileReader reads audit entries from exported files.
type FileReader struct {
	path       string
	file       *os.File
	gzReader   *gzip.Reader
	scanner    *bufio.Scanner
	compressed bool
}

// NewFileReader creates a reader for an audit file.
func NewFileReader(path string) (*FileReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	reader := &FileReader{
		path:       path,
		file:       file,
		compressed: filepath.Ext(path) == ".gz",
	}

	var r io.Reader = file
	if reader.compressed {
		gzr, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		reader.gzReader = gzr
		r = gzr
	}

	reader.scanner = bufio.NewScanner(r)
	reader.scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line

	return reader, nil
}

// Next reads the next audit entry.
func (r *FileReader) Next() (*Entry, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	var entry Entry
	if err := json.Unmarshal(r.scanner.Bytes(), &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entry: %w", err)
	}

	return &entry, nil
}

// Close closes the file reader.
func (r *FileReader) Close() error {
	if r.gzReader != nil {
		r.gzReader.Close()
	}
	return r.file.Close()
}
