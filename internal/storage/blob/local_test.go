package blob

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"

	"bib/internal/config"
	"bib/internal/logger"
)

// Helper function to create a test logger
func testLogger(t *testing.T) *logger.Logger {
	log, err := logger.New(config.LogConfig{
		Level:  "debug",
		Format: "text",
		Output: "stdout",
	})
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	return log
}

func TestLocalStore_PutGet(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create logger
	log := testLogger(t)
	defer log.Close()

	// Create encryption key
	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Create local store
	cfg := LocalConfig{
		Enabled: true,
		Path:    filepath.Join(tempDir, "blobs"),
		Encryption: EncryptionConfig{
			Enabled:   false, // Test without encryption first
			Algorithm: "aes256-gcm",
		},
		Compression: CompressionConfig{
			Enabled:   false, // Test without compression first
			Algorithm: "none",
		},
	}

	store, err := NewLocalStore(cfg, tempDir, encKey, log)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Test data
	testData := []byte("Hello, world! This is test data for blob storage.")
	hash := sha256.Sum256(testData)
	hashStr := hex.EncodeToString(hash[:])

	// Put blob
	ctx := context.Background()
	metadata := &Metadata{
		Hash: hashStr,
		Tags: []string{"test"},
	}

	err = store.Put(ctx, hashStr, bytes.NewReader(testData), metadata)
	if err != nil {
		t.Fatalf("failed to put blob: %v", err)
	}

	// Check existence
	exists, err := store.Exists(ctx, hashStr)
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if !exists {
		t.Fatal("blob should exist")
	}

	// Get blob
	reader, err := store.Get(ctx, hashStr)
	if err != nil {
		t.Fatalf("failed to get blob: %v", err)
	}

	// Read and compare
	retrievedData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read blob: %v", err)
	}

	// Close reader explicitly (important on Windows before delete)
	reader.Close()

	if !bytes.Equal(testData, retrievedData) {
		t.Fatalf("data mismatch: expected %s, got %s", testData, retrievedData)
	}

	// Get metadata
	meta, err := store.GetMetadata(ctx, hashStr)
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}

	if len(meta.Tags) != 1 || meta.Tags[0] != "test" {
		t.Fatalf("metadata tags mismatch: %v", meta.Tags)
	}

	// Delete blob
	err = store.Delete(ctx, hashStr)
	if err != nil {
		t.Fatalf("failed to delete blob: %v", err)
	}

	// Verify moved to trash
	exists, err = store.Exists(ctx, hashStr)
	if err != nil {
		t.Fatalf("failed to check existence after delete: %v", err)
	}
	if exists {
		t.Fatal("blob should not exist after delete")
	}

	// Check trash
	trashPath := filepath.Join(tempDir, "blobs", ".trash", hashStr)
	if _, err := os.Stat(trashPath); os.IsNotExist(err) {
		t.Fatal("blob should be in trash")
	}
}

func TestLocalStore_WithEncryption(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create logger
	log := testLogger(t)
	defer log.Close()

	// Create encryption key
	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Create local store with encryption
	cfg := LocalConfig{
		Enabled: true,
		Path:    filepath.Join(tempDir, "blobs"),
		Encryption: EncryptionConfig{
			Enabled:   true,
			Algorithm: "aes256-gcm",
		},
		Compression: CompressionConfig{
			Enabled:   false,
			Algorithm: "none",
		},
	}

	store, err := NewLocalStore(cfg, tempDir, encKey, log)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Test data
	testData := []byte("Encrypted test data")
	hash := sha256.Sum256(testData)
	hashStr := hex.EncodeToString(hash[:])

	// Put blob
	ctx := context.Background()
	err = store.Put(ctx, hashStr, bytes.NewReader(testData), nil)
	if err != nil {
		t.Fatalf("failed to put blob: %v", err)
	}

	// Get blob
	reader, err := store.Get(ctx, hashStr)
	if err != nil {
		t.Fatalf("failed to get blob: %v", err)
	}
	defer reader.Close()

	// Read and compare
	retrievedData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read blob: %v", err)
	}

	if !bytes.Equal(testData, retrievedData) {
		t.Fatalf("data mismatch after encryption/decryption")
	}
}

func TestLocalStore_WithCompression(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create logger
	log := testLogger(t)
	defer log.Close()

	// Create encryption key (not used, compression only)
	encKey := make([]byte, 32)

	// Create local store with compression
	cfg := LocalConfig{
		Enabled: true,
		Path:    filepath.Join(tempDir, "blobs"),
		Encryption: EncryptionConfig{
			Enabled: false,
		},
		Compression: CompressionConfig{
			Enabled:   true,
			Algorithm: "gzip",
			Level:     6,
		},
	}

	store, err := NewLocalStore(cfg, tempDir, encKey, log)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Test data (compressible)
	testData := bytes.Repeat([]byte("AAAAAAAAAA"), 100) // Highly compressible
	hash := sha256.Sum256(testData)
	hashStr := hex.EncodeToString(hash[:])

	// Put blob
	ctx := context.Background()
	err = store.Put(ctx, hashStr, bytes.NewReader(testData), nil)
	if err != nil {
		t.Fatalf("failed to put blob: %v", err)
	}

	// Get blob
	reader, err := store.Get(ctx, hashStr)
	if err != nil {
		t.Fatalf("failed to get blob: %v", err)
	}
	defer reader.Close()

	// Read and compare
	retrievedData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read blob: %v", err)
	}

	if !bytes.Equal(testData, retrievedData) {
		t.Fatalf("data mismatch after compression/decompression")
	}
}

func TestLocalStore_List(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create logger
	log := testLogger(t)
	defer log.Close()

	// Create store
	cfg := LocalConfig{
		Enabled: true,
		Path:    filepath.Join(tempDir, "blobs"),
		Encryption: EncryptionConfig{
			Enabled: false,
		},
		Compression: CompressionConfig{
			Enabled: false,
		},
	}

	store, err := NewLocalStore(cfg, tempDir, nil, log)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create multiple blobs
	hashes := make([]string, 0)
	for i := 0; i < 5; i++ {
		testData := []byte{byte(i)}
		hash := sha256.Sum256(testData)
		hashStr := hex.EncodeToString(hash[:])
		hashes = append(hashes, hashStr)

		err = store.Put(ctx, hashStr, bytes.NewReader(testData), nil)
		if err != nil {
			t.Fatalf("failed to put blob %d: %v", i, err)
		}
	}

	// List all blobs
	blobs, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("failed to list blobs: %v", err)
	}

	if len(blobs) != 5 {
		t.Fatalf("expected 5 blobs, got %d", len(blobs))
	}

	// Verify all hashes are present
	found := make(map[string]bool)
	for _, blob := range blobs {
		found[blob.Hash] = true
	}

	for _, hash := range hashes {
		if !found[hash] {
			t.Fatalf("hash %s not found in list", hash)
		}
	}
}

func TestLocalStore_Stats(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create logger
	log := testLogger(t)
	defer log.Close()

	// Create store
	cfg := LocalConfig{
		Enabled: true,
		Path:    filepath.Join(tempDir, "blobs"),
		Encryption: EncryptionConfig{
			Enabled: false,
		},
		Compression: CompressionConfig{
			Enabled: false,
		},
	}

	store, err := NewLocalStore(cfg, tempDir, nil, log)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create blobs
	testData := []byte("test data for stats")
	hash := sha256.Sum256(testData)
	hashStr := hex.EncodeToString(hash[:])

	err = store.Put(ctx, hashStr, bytes.NewReader(testData), nil)
	if err != nil {
		t.Fatalf("failed to put blob: %v", err)
	}

	// Get stats
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.TotalBlobs != 1 {
		t.Fatalf("expected 1 blob, got %d", stats.TotalBlobs)
	}

	if stats.Backend != BackendLocal {
		t.Fatalf("expected local backend, got %s", stats.Backend)
	}
}
