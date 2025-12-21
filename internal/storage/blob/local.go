package blob

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bib/internal/logger"
)

// LocalStore implements blob storage on the local filesystem.
type LocalStore struct {
	cfg      LocalConfig
	basePath string
	encKey   []byte // AES-256 key (32 bytes)
	logger   *logger.Logger

	mu    sync.RWMutex
	stats Stats

	wg sync.WaitGroup // tracks pending async operations
}

// NewLocalStore creates a new local filesystem blob store.
func NewLocalStore(cfg LocalConfig, dataDir string, encKey []byte, log *logger.Logger) (*LocalStore, error) {
	// Determine base path
	basePath := cfg.Path
	if basePath == "" {
		basePath = filepath.Join(dataDir, "blobs")
	}

	// Create directories
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create blob directory: %w", err)
	}

	// Create trash directory
	trashPath := filepath.Join(basePath, ".trash")
	if err := os.MkdirAll(trashPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Validate encryption key if encryption is enabled
	if cfg.Encryption.Enabled {
		if len(encKey) != 32 {
			return nil, fmt.Errorf("encryption key must be 32 bytes for AES-256")
		}
	}

	store := &LocalStore{
		cfg:      cfg,
		basePath: basePath,
		encKey:   encKey,
		logger:   log,
		stats: Stats{
			Backend: BackendLocal,
		},
	}

	// Initialize stats
	if err := store.computeStats(context.Background()); err != nil {
		log.Warn("Failed to compute initial blob stats", "error", err)
	}

	return store, nil
}

// Put stores a blob with the given hash and data.
func (s *LocalStore) Put(ctx context.Context, hash string, data io.Reader, metadata *Metadata) error {
	// Validate hash
	if !isValidHash(hash) {
		return fmt.Errorf("invalid hash format")
	}

	// Check if blob already exists
	blobPath := s.blobPath(hash)
	if _, err := os.Stat(blobPath); err == nil {
		return fmt.Errorf("blob already exists: %s", hash)
	}

	// Create directory structure
	if err := os.MkdirAll(filepath.Dir(blobPath), 0700); err != nil {
		return fmt.Errorf("failed to create blob directory: %w", err)
	}

	// Create temp file
	tempPath := blobPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	var cleanupTemp bool = true
	defer func() {
		if cleanupTemp {
			tempFile.Close()
			os.Remove(tempPath)
		}
	}()

	// Read all data into buffer (needed for proper encryption/compression)
	buf := new(bytes.Buffer)
	size, err := io.Copy(buf, data)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	processedData := buf.Bytes()

	// Apply compression if enabled
	if s.cfg.Compression.Enabled {
		compressed := new(bytes.Buffer)
		compWriter, err := newCompressionWriter(compressed, s.cfg.Compression.Algorithm, s.cfg.Compression.Level)
		if err != nil {
			return fmt.Errorf("failed to create compression writer: %w", err)
		}
		if _, err := compWriter.Write(processedData); err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}
		if err := compWriter.Close(); err != nil {
			return fmt.Errorf("failed to close compression writer: %w", err)
		}
		processedData = compressed.Bytes()
	}

	// Apply encryption if enabled
	if s.cfg.Encryption.Enabled {
		encrypted, err := s.encryptData(processedData)
		if err != nil {
			return fmt.Errorf("failed to encrypt data: %w", err)
		}
		processedData = encrypted
	}

	// Write processed data to file
	if _, err := tempFile.Write(processedData); err != nil {
		return fmt.Errorf("failed to write blob data: %w", err)
	}

	// Close temp file
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Rename temp file to final name (atomic)
	if err := os.Rename(tempPath, blobPath); err != nil {
		return fmt.Errorf("failed to rename blob: %w", err)
	}

	// Mark as successfully renamed (don't delete in defer)
	cleanupTemp = false

	// Initialize metadata if not provided
	if metadata == nil {
		metadata = &Metadata{}
	}
	metadata.Hash = hash
	metadata.Size = size
	metadata.CreatedAt = time.Now().UTC()
	metadata.LastAccessed = metadata.CreatedAt
	metadata.AccessCount = 0

	if s.cfg.Compression.Enabled {
		metadata.Compression = CompressionType(s.cfg.Compression.Algorithm)
	} else {
		metadata.Compression = CompressionNone
	}

	if s.cfg.Encryption.Enabled {
		metadata.Encryption = EncryptionAES256GCM
	} else {
		metadata.Encryption = EncryptionNone
	}

	// Write metadata
	if err := s.writeMetadata(hash, metadata); err != nil {
		// Non-fatal, log error
		s.logger.Warn("Failed to write blob metadata", "hash", hash, "error", err)
	}

	// Update stats
	s.mu.Lock()
	s.stats.TotalBlobs++
	s.stats.TotalSize += size
	s.mu.Unlock()

	return nil
}

// Get retrieves a blob by hash.
func (s *LocalStore) Get(ctx context.Context, hash string) (io.ReadCloser, error) {
	if !isValidHash(hash) {
		return nil, fmt.Errorf("invalid hash format")
	}

	blobPath := s.blobPath(hash)

	// Read entire file
	fileData, err := os.ReadFile(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("blob not found: %s", hash)
		}
		return nil, fmt.Errorf("failed to read blob: %w", err)
	}

	// Update access time asynchronously
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.Touch(context.Background(), hash)
	}()

	processedData := fileData

	// Apply decryption if enabled
	if s.cfg.Encryption.Enabled {
		decrypted, err := s.decryptData(processedData)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt data: %w", err)
		}
		processedData = decrypted
	}

	// Apply decompression if enabled
	if s.cfg.Compression.Enabled {
		decompReader, err := newDecompressionReader(bytes.NewReader(processedData), s.cfg.Compression.Algorithm)
		if err != nil {
			return nil, fmt.Errorf("failed to create decompression reader: %w", err)
		}
		return decompReader, nil
	}

	return io.NopCloser(bytes.NewReader(processedData)), nil
}

// Delete removes a blob by hash (moves to trash).
func (s *LocalStore) Delete(ctx context.Context, hash string) error {
	if !isValidHash(hash) {
		return fmt.Errorf("invalid hash format")
	}

	blobPath := s.blobPath(hash)
	if _, err := os.Stat(blobPath); os.IsNotExist(err) {
		return fmt.Errorf("blob not found: %s", hash)
	}

	// Move to trash instead of immediate deletion
	trashPath := filepath.Join(s.basePath, ".trash", hash)
	if err := os.MkdirAll(filepath.Dir(trashPath), 0700); err != nil {
		return fmt.Errorf("failed to create trash directory: %w", err)
	}

	if err := os.Rename(blobPath, trashPath); err != nil {
		return fmt.Errorf("failed to move blob to trash: %w", err)
	}

	// Move metadata to trash
	metaPath := s.metadataPath(hash)
	trashMetaPath := filepath.Join(s.basePath, ".trash", hash+".meta")
	os.Rename(metaPath, trashMetaPath) // Non-fatal if fails

	// Update stats
	meta, _ := s.GetMetadata(ctx, hash)
	if meta != nil {
		s.mu.Lock()
		s.stats.TotalBlobs--
		s.stats.TotalSize -= meta.Size
		s.mu.Unlock()
	}

	return nil
}

// Exists checks if a blob exists.
func (s *LocalStore) Exists(ctx context.Context, hash string) (bool, error) {
	if !isValidHash(hash) {
		return false, fmt.Errorf("invalid hash format")
	}

	blobPath := s.blobPath(hash)
	_, err := os.Stat(blobPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Size returns the size of a blob in bytes.
func (s *LocalStore) Size(ctx context.Context, hash string) (int64, error) {
	meta, err := s.GetMetadata(ctx, hash)
	if err != nil {
		return 0, err
	}
	return meta.Size, nil
}

// List lists blobs with the given prefix.
func (s *LocalStore) List(ctx context.Context, prefix string) ([]BlobInfo, error) {
	var blobs []BlobInfo

	// Walk the blob directory
	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and trash
		if info.IsDir() || strings.Contains(path, ".trash") {
			return nil
		}

		// Skip metadata files
		if strings.HasSuffix(path, ".meta") {
			return nil
		}

		// Extract hash from path
		relPath, _ := filepath.Rel(s.basePath, path)
		hash := filepath.Base(relPath)

		// Check prefix
		if prefix != "" && !strings.HasPrefix(hash, prefix) {
			return nil
		}

		// Load metadata
		meta, err := s.GetMetadata(ctx, hash)
		if err != nil {
			// Skip if metadata can't be loaded
			return nil
		}

		blobs = append(blobs, BlobInfo{
			Hash:         hash,
			Size:         meta.Size,
			CreatedAt:    meta.CreatedAt,
			LastAccessed: meta.LastAccessed,
			Tags:         meta.Tags,
		})

		return nil
	})

	return blobs, err
}

// Touch updates the last accessed time for LRU tracking.
func (s *LocalStore) Touch(ctx context.Context, hash string) error {
	meta, err := s.GetMetadata(ctx, hash)
	if err != nil {
		return err
	}

	meta.LastAccessed = time.Now().UTC()
	meta.AccessCount++

	return s.UpdateMetadata(ctx, hash, meta)
}

// GetMetadata retrieves blob metadata.
func (s *LocalStore) GetMetadata(ctx context.Context, hash string) (*Metadata, error) {
	if !isValidHash(hash) {
		return nil, fmt.Errorf("invalid hash format")
	}

	metaPath := s.metadataPath(hash)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("metadata not found: %s", hash)
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &meta, nil
}

// UpdateMetadata updates blob metadata.
func (s *LocalStore) UpdateMetadata(ctx context.Context, hash string, meta *Metadata) error {
	return s.writeMetadata(hash, meta)
}

// Move moves a blob to another store.
func (s *LocalStore) Move(ctx context.Context, hash string, to Store) error {
	// Copy to destination
	if err := s.Copy(ctx, hash, to); err != nil {
		return err
	}

	// Delete from source
	return s.Delete(ctx, hash)
}

// Copy copies a blob to another store.
func (s *LocalStore) Copy(ctx context.Context, hash string, to Store) error {
	// Get blob data
	reader, err := s.Get(ctx, hash)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Get metadata
	meta, err := s.GetMetadata(ctx, hash)
	if err != nil {
		return err
	}

	// Put to destination
	return to.Put(ctx, hash, reader, meta)
}

// Backend returns the storage backend type.
func (s *LocalStore) Backend() BackendType {
	return BackendLocal
}

// Stats returns storage statistics.
func (s *LocalStore) Stats(ctx context.Context) (*Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statsCopy := s.stats
	return &statsCopy, nil
}

// Close closes the store and waits for pending async operations.
func (s *LocalStore) Close() error {
	s.wg.Wait()
	return nil
}

// Helper methods

func (s *LocalStore) blobPath(hash string) string {
	// Structure: <basePath>/<hash[0:2]>/<hash[2:4]>/<hash>
	return filepath.Join(s.basePath, hash[0:2], hash[2:4], hash)
}

func (s *LocalStore) metadataPath(hash string) string {
	// Metadata stored alongside blob: <blobPath>.meta
	return s.blobPath(hash) + ".meta"
}

func (s *LocalStore) writeMetadata(hash string, meta *Metadata) error {
	metaPath := s.metadataPath(hash)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0700); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

func (s *LocalStore) computeStats(ctx context.Context) error {
	var totalBlobs int64
	var totalSize int64
	var oldest, newest time.Time

	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || strings.Contains(path, ".trash") || strings.HasSuffix(path, ".meta") {
			return nil
		}

		totalBlobs++
		totalSize += info.Size()

		modTime := info.ModTime()
		if oldest.IsZero() || modTime.Before(oldest) {
			oldest = modTime
		}
		if newest.IsZero() || modTime.After(newest) {
			newest = modTime
		}

		return nil
	})

	if err != nil {
		return err
	}

	s.mu.Lock()
	s.stats.TotalBlobs = totalBlobs
	s.stats.TotalSize = totalSize
	s.stats.OldestBlob = oldest
	s.stats.NewestBlob = newest
	s.mu.Unlock()

	return nil
}

func (s *LocalStore) encryptData(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt: nonce || ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (s *LocalStore) decryptData(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (s *LocalStore) newEncryptWriter(w io.Writer) (io.Writer, error) {
	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Write nonce first
	if _, err := w.Write(nonce); err != nil {
		return nil, err
	}

	return &gcmWriter{
		writer: w,
		gcm:    gcm,
		nonce:  nonce,
	}, nil
}

func (s *LocalStore) newDecryptReader(r io.Reader) (io.Reader, error) {
	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Read nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(r, nonce); err != nil {
		return nil, err
	}

	return &gcmReader{
		reader: r,
		gcm:    gcm,
		nonce:  nonce,
	}, nil
}

// gcmWriter wraps a writer with AES-GCM encryption.
type gcmWriter struct {
	writer io.Writer
	gcm    cipher.AEAD
	nonce  []byte
}

func (w *gcmWriter) Write(p []byte) (n int, err error) {
	encrypted := w.gcm.Seal(nil, w.nonce, p, nil)
	_, err = w.writer.Write(encrypted)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// gcmReader wraps a reader with AES-GCM decryption.
type gcmReader struct {
	reader io.Reader
	gcm    cipher.AEAD
	nonce  []byte
}

func (r *gcmReader) Read(p []byte) (n int, err error) {
	// Read encrypted data
	buf := make([]byte, len(p)+r.gcm.Overhead())
	n, err = r.reader.Read(buf)
	if err != nil && err != io.EOF {
		return 0, err
	}

	if n == 0 {
		return 0, io.EOF
	}

	// Decrypt
	decrypted, err := r.gcm.Open(nil, r.nonce, buf[:n], nil)
	if err != nil {
		return 0, err
	}

	copy(p, decrypted)
	return len(decrypted), nil
}

// multiCloser closes multiple closers.
type multiCloser struct {
	io.Reader
	closers []io.Closer
}

func (mc *multiCloser) Close() error {
	for _, c := range mc.closers {
		c.Close()
	}
	return nil
}

func isValidHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}
