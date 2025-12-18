package blob

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"bib/internal/logger"
	"bib/internal/storage/audit"
)

// S3Store implements blob storage on S3-compatible object storage.
type S3Store struct {
	cfg    S3Config
	client audit.S3Client // Reuse the S3Client interface from audit
	encKey []byte         // AES-256 key for client-side encryption
	logger *logger.Logger

	mu    sync.RWMutex
	stats Stats
}

// NewS3Store creates a new S3 blob store.
func NewS3Store(cfg S3Config, client audit.S3Client, encKey []byte, log *logger.Logger) (*S3Store, error) {
	// Validate encryption key if client-side encryption is enabled
	if cfg.ClientSideEncryption.Enabled {
		if len(encKey) != 32 {
			return nil, fmt.Errorf("encryption key must be 32 bytes for AES-256")
		}
	}

	store := &S3Store{
		cfg:    cfg,
		client: client,
		encKey: encKey,
		logger: log,
		stats: Stats{
			Backend: BackendS3,
		},
	}

	return store, nil
}

// Put stores a blob in S3 with the given hash and data.
func (s *S3Store) Put(ctx context.Context, hash string, data io.Reader, metadata *Metadata) error {
	// Validate hash
	if !isValidHash(hash) {
		return fmt.Errorf("invalid hash format")
	}

	// Check if blob already exists
	exists, err := s.Exists(ctx, hash)
	if err != nil {
		return fmt.Errorf("failed to check blob existence: %w", err)
	}
	if exists {
		return fmt.Errorf("blob already exists: %s", hash)
	}

	// Read data into buffer for processing
	buf := new(bytes.Buffer)
	originalSize, err := io.Copy(buf, data)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	dataReader := bytes.NewReader(buf.Bytes())
	var processedData io.Reader = dataReader

	// Apply compression if enabled
	if s.cfg.Compression.Enabled {
		compressed := new(bytes.Buffer)
		compWriter, err := newCompressionWriter(compressed, s.cfg.Compression.Algorithm, s.cfg.Compression.Level)
		if err != nil {
			return fmt.Errorf("failed to create compression writer: %w", err)
		}

		if _, err := io.Copy(compWriter, dataReader); err != nil {
			return fmt.Errorf("failed to compress data: %w", err)
		}

		if err := compWriter.Close(); err != nil {
			return fmt.Errorf("failed to finalize compression: %w", err)
		}

		processedData = bytes.NewReader(compressed.Bytes())
	}

	// Apply client-side encryption if enabled
	if s.cfg.ClientSideEncryption.Enabled {
		encrypted, err := s.encryptData(processedData)
		if err != nil {
			return fmt.Errorf("failed to encrypt data: %w", err)
		}
		processedData = bytes.NewReader(encrypted)
	}

	// Generate S3 key
	key := s.blobKey(hash)

	// Prepare S3 metadata
	s3Metadata := map[string]string{
		"bib-hash":       hash,
		"bib-created-at": time.Now().UTC().Format(time.RFC3339),
	}

	if metadata != nil {
		if len(metadata.Tags) > 0 {
			s3Metadata["bib-tags"] = strings.Join(metadata.Tags, ",")
		}
	}

	// Upload to S3
	contentType := "application/octet-stream"
	if err := s.client.PutObject(ctx, s.cfg.Bucket, key, processedData, contentType, s3Metadata); err != nil {
		return fmt.Errorf("failed to upload blob to S3: %w", err)
	}

	// Initialize metadata if not provided
	if metadata == nil {
		metadata = &Metadata{}
	}
	metadata.Hash = hash
	metadata.Size = originalSize
	metadata.CreatedAt = time.Now().UTC()
	metadata.LastAccessed = metadata.CreatedAt
	metadata.AccessCount = 0

	if s.cfg.Compression.Enabled {
		metadata.Compression = CompressionType(s.cfg.Compression.Algorithm)
	} else {
		metadata.Compression = CompressionNone
	}

	if s.cfg.ClientSideEncryption.Enabled {
		metadata.Encryption = EncryptionAES256GCM
	} else {
		metadata.Encryption = EncryptionNone
	}

	// Store metadata as separate object
	if err := s.putMetadata(ctx, hash, metadata); err != nil {
		s.logger.Warn("Failed to store blob metadata in S3", "hash", hash, "error", err)
	}

	// Update stats
	s.mu.Lock()
	s.stats.TotalBlobs++
	s.stats.TotalSize += originalSize
	s.mu.Unlock()

	return nil
}

// Get retrieves a blob from S3 by hash.
func (s *S3Store) Get(ctx context.Context, hash string) (io.ReadCloser, error) {
	if !isValidHash(hash) {
		return nil, fmt.Errorf("invalid hash format")
	}

	key := s.blobKey(hash)

	// Download from S3
	reader, err := s.client.GetObject(ctx, s.cfg.Bucket, key)
	if err != nil {
		return nil, fmt.Errorf("failed to download blob from S3: %w", err)
	}

	// Update access time asynchronously
	go s.Touch(context.Background(), hash)

	// Read entire object for decryption/decompression
	// Note: For large objects, this could be optimized with streaming
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object: %w", err)
	}

	processedData := bytes.NewReader(data)

	// Apply decryption if enabled
	if s.cfg.ClientSideEncryption.Enabled {
		decrypted, err := s.decryptData(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt data: %w", err)
		}
		processedData = bytes.NewReader(decrypted)
	}

	// Apply decompression if enabled
	if s.cfg.Compression.Enabled {
		decompReader, err := newDecompressionReader(processedData, s.cfg.Compression.Algorithm)
		if err != nil {
			return nil, fmt.Errorf("failed to create decompression reader: %w", err)
		}
		return decompReader, nil
	}

	return io.NopCloser(processedData), nil
}

// Delete removes a blob from S3 (moves to trash prefix).
func (s *S3Store) Delete(ctx context.Context, hash string) error {
	if !isValidHash(hash) {
		return fmt.Errorf("invalid hash format")
	}

	key := s.blobKey(hash)
	trashKey := s.trashKey(hash)

	// Get object to copy to trash
	reader, err := s.client.GetObject(ctx, s.cfg.Bucket, key)
	if err != nil {
		return fmt.Errorf("failed to get blob for deletion: %w", err)
	}

	// Copy to trash
	if err := s.client.PutObject(ctx, s.cfg.Bucket, trashKey, reader, "application/octet-stream", nil); err != nil {
		reader.Close()
		return fmt.Errorf("failed to move blob to trash: %w", err)
	}
	reader.Close()

	// Delete original
	if err := s.client.DeleteObject(ctx, s.cfg.Bucket, key); err != nil {
		return fmt.Errorf("failed to delete original blob: %w", err)
	}

	// Move metadata to trash
	metaKey := s.metadataKey(hash)
	trashMetaKey := s.trashKey(hash) + ".meta"
	if metaReader, err := s.client.GetObject(ctx, s.cfg.Bucket, metaKey); err == nil {
		s.client.PutObject(ctx, s.cfg.Bucket, trashMetaKey, metaReader, "application/json", nil)
		metaReader.Close()
		s.client.DeleteObject(ctx, s.cfg.Bucket, metaKey)
	}

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

// Exists checks if a blob exists in S3.
func (s *S3Store) Exists(ctx context.Context, hash string) (bool, error) {
	if !isValidHash(hash) {
		return false, fmt.Errorf("invalid hash format")
	}

	key := s.blobKey(hash)

	// Try to get object metadata
	objects, err := s.client.ListObjects(ctx, s.cfg.Bucket, key, 1)
	if err != nil {
		return false, err
	}

	for _, obj := range objects {
		if obj.Key == key {
			return true, nil
		}
	}

	return false, nil
}

// Size returns the size of a blob in bytes.
func (s *S3Store) Size(ctx context.Context, hash string) (int64, error) {
	meta, err := s.GetMetadata(ctx, hash)
	if err != nil {
		return 0, err
	}
	return meta.Size, nil
}

// List lists blobs with the given prefix in S3.
func (s *S3Store) List(ctx context.Context, prefix string) ([]BlobInfo, error) {
	var blobs []BlobInfo

	// List objects with prefix
	searchPrefix := s.cfg.Prefix
	if prefix != "" {
		searchPrefix = path.Join(s.cfg.Prefix, "chunks", prefix[0:2], prefix[2:4], prefix)
	} else {
		searchPrefix = path.Join(s.cfg.Prefix, "chunks")
	}

	objects, err := s.client.ListObjects(ctx, s.cfg.Bucket, searchPrefix, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 objects: %w", err)
	}

	for _, obj := range objects {
		// Skip metadata files and trash
		if strings.HasSuffix(obj.Key, ".meta") || strings.Contains(obj.Key, ".trash") {
			continue
		}

		// Extract hash from key
		hash := path.Base(obj.Key)
		if !isValidHash(hash) {
			continue
		}

		// Try to load metadata
		meta, err := s.GetMetadata(ctx, hash)
		if err != nil {
			// Use object info if metadata not available
			blobs = append(blobs, BlobInfo{
				Hash:         hash,
				Size:         obj.Size,
				CreatedAt:    obj.LastModified,
				LastAccessed: obj.LastModified,
			})
			continue
		}

		blobs = append(blobs, BlobInfo{
			Hash:         hash,
			Size:         meta.Size,
			CreatedAt:    meta.CreatedAt,
			LastAccessed: meta.LastAccessed,
			Tags:         meta.Tags,
		})
	}

	return blobs, nil
}

// Touch updates the last accessed time in metadata.
func (s *S3Store) Touch(ctx context.Context, hash string) error {
	meta, err := s.GetMetadata(ctx, hash)
	if err != nil {
		return err
	}

	meta.LastAccessed = time.Now().UTC()
	meta.AccessCount++

	return s.UpdateMetadata(ctx, hash, meta)
}

// GetMetadata retrieves blob metadata from S3.
func (s *S3Store) GetMetadata(ctx context.Context, hash string) (*Metadata, error) {
	if !isValidHash(hash) {
		return nil, fmt.Errorf("invalid hash format")
	}

	key := s.metadataKey(hash)

	reader, err := s.client.GetObject(ctx, s.cfg.Bucket, key)
	if err != nil {
		return nil, fmt.Errorf("metadata not found: %s", hash)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &meta, nil
}

// UpdateMetadata updates blob metadata in S3.
func (s *S3Store) UpdateMetadata(ctx context.Context, hash string, meta *Metadata) error {
	return s.putMetadata(ctx, hash, meta)
}

// Move moves a blob to another store.
func (s *S3Store) Move(ctx context.Context, hash string, to Store) error {
	// Copy to destination
	if err := s.Copy(ctx, hash, to); err != nil {
		return err
	}

	// Delete from source
	return s.Delete(ctx, hash)
}

// Copy copies a blob to another store.
func (s *S3Store) Copy(ctx context.Context, hash string, to Store) error {
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
func (s *S3Store) Backend() BackendType {
	return BackendS3
}

// Stats returns storage statistics.
func (s *S3Store) Stats(ctx context.Context) (*Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statsCopy := s.stats
	return &statsCopy, nil
}

// Close closes the store.
func (s *S3Store) Close() error {
	return nil
}

// Helper methods

func (s *S3Store) blobKey(hash string) string {
	// Structure: <prefix>/chunks/<hash[0:2]>/<hash[2:4]>/<hash>
	return path.Join(s.cfg.Prefix, "chunks", hash[0:2], hash[2:4], hash)
}

func (s *S3Store) metadataKey(hash string) string {
	return s.blobKey(hash) + ".meta"
}

func (s *S3Store) trashKey(hash string) string {
	return path.Join(s.cfg.Prefix, ".trash", hash)
}

func (s *S3Store) putMetadata(ctx context.Context, hash string, meta *Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	key := s.metadataKey(hash)
	reader := bytes.NewReader(data)

	if err := s.client.PutObject(ctx, s.cfg.Bucket, key, reader, "application/json", nil); err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	return nil
}

func (s *S3Store) encryptData(r io.Reader) ([]byte, error) {
	// Read all data
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

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

func (s *S3Store) decryptData(ciphertext []byte) ([]byte, error) {
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
