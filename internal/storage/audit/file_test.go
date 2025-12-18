package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFileExporter(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := FileExportConfig{
		Enabled:     true,
		Directory:   tmpDir,
		FilePrefix:  "test-audit",
		MaxFileSize: 1024 * 1024,
		Compress:    false,
		BufferSize:  1024,
	}

	exporter, err := NewFileExporter(cfg)
	if err != nil {
		t.Fatalf("NewFileExporter() error = %v", err)
	}
	defer exporter.Close()

	if exporter == nil {
		t.Fatal("NewFileExporter returned nil")
	}
}

func TestFileExporter_Export(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := FileExportConfig{
		Enabled:     true,
		Directory:   tmpDir,
		FilePrefix:  "test-audit",
		MaxFileSize: 1024 * 1024,
		Compress:    false,
		BufferSize:  1024,
	}

	exporter, err := NewFileExporter(cfg)
	if err != nil {
		t.Fatalf("NewFileExporter() error = %v", err)
	}
	defer exporter.Close()

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		entry := NewEntry("node-1", GenerateOperationID(), "bibd_query", "test", ActionSelect)
		entry.TableName = "users"
		entry.RowsAffected = i

		if err := exporter.Export(ctx, entry); err != nil {
			t.Fatalf("Export() error = %v", err)
		}
	}

	if err := exporter.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	stats := exporter.Stats()
	if stats.EntryCount != 10 {
		t.Errorf("EntryCount = %d, want 10", stats.EntryCount)
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	if len(files) == 0 {
		t.Error("No files created")
	}
}

func TestFileExporter_Disabled(t *testing.T) {
	cfg := FileExportConfig{
		Enabled: false,
	}

	exporter, err := NewFileExporter(cfg)
	if err != nil {
		t.Fatalf("NewFileExporter() error = %v", err)
	}

	if exporter != nil {
		t.Error("Disabled exporter should return nil")
	}
}

func TestFileReader(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := FileExportConfig{
		Enabled:     true,
		Directory:   tmpDir,
		FilePrefix:  "test-audit",
		MaxFileSize: 1024 * 1024,
		Compress:    false,
		BufferSize:  1024,
	}

	exporter, err := NewFileExporter(cfg)
	if err != nil {
		t.Fatalf("NewFileExporter() error = %v", err)
	}

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		entry := NewEntry("node-1", GenerateOperationID(), "bibd_query", "test", ActionSelect)
		entry.TableName = "users"
		entry.RowsAffected = i

		if err := exporter.Export(ctx, entry); err != nil {
			t.Fatalf("Export() error = %v", err)
		}
	}

	exporter.Close()

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	if len(files) == 0 {
		t.Fatal("No files to read")
	}

	reader, err := NewFileReader(filepath.Join(tmpDir, files[0].Name()))
	if err != nil {
		t.Fatalf("NewFileReader() error = %v", err)
	}
	defer reader.Close()

	count := 0
	for {
		entry, err := reader.Next()
		if err != nil {
			break
		}
		if entry.Action != ActionSelect {
			t.Errorf("Entry action = %s, want SELECT", entry.Action)
		}
		count++
	}

	if count != 5 {
		t.Errorf("Read %d entries, want 5", count)
	}
}

func TestFileExporter_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := FileExportConfig{
		Enabled:     true,
		Directory:   tmpDir,
		FilePrefix:  "test-audit",
		MaxFileSize: 1024 * 1024,
		MaxAge:      1 * time.Millisecond,
		Compress:    false,
		BufferSize:  1024,
	}

	exporter, err := NewFileExporter(cfg)
	if err != nil {
		t.Fatalf("NewFileExporter() error = %v", err)
	}
	defer exporter.Close()

	ctx := context.Background()

	entry := NewEntry("node-1", GenerateOperationID(), "bibd_query", "test", ActionSelect)
	if err := exporter.Export(ctx, entry); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if err := exporter.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := exporter.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}
