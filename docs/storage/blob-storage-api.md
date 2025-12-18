# Blob Storage API Reference

Quick reference for using the blob storage system in Bib.

## Initialization

```go
import "bib/internal/storage/blob"

// Open blob storage
manager, err := blob.Open(
    cfg.Blob,           // BlobConfig from storage.Config
    dataDir,            // Base data directory
    encryptionKey,      // 32-byte AES-256 key
    dbStore,            // Database store for GC
    s3Client,           // S3 client (or nil for local-only)
    auditLogger,        // Audit logger (or nil)
    logger,             // Logger instance
)
if err != nil {
    return err
}

// Start background processes (GC, tiering)
if err := manager.Start(ctx); err != nil {
    return err
}

// Cleanup
defer manager.Close()
```

## Basic Operations

### Store a Blob

```go
store := manager.Store()

hash := "22af57403105fecf476b9264ca58f0f1f6cad15c00880513ef5376d87649049f"
data := bytes.NewReader([]byte("blob content"))

metadata := &blob.Metadata{
    Hash: hash,
    Tags: []string{"important"},
}

err := store.Put(ctx, hash, data, metadata)
```

### Retrieve a Blob

```go
reader, err := store.Get(ctx, hash)
if err != nil {
    return err
}
defer reader.Close()

content, err := io.ReadAll(reader)
```

### Check Existence

```go
exists, err := store.Exists(ctx, hash)
if err != nil {
    return err
}

if exists {
    fmt.Println("Blob found")
}
```

### Delete a Blob (Move to Trash)

```go
err := store.Delete(ctx, hash)
```

### Get Blob Size

```go
size, err := store.Size(ctx, hash)
```

### List Blobs

```go
// List all blobs
blobs, err := store.List(ctx, "")

// List blobs with prefix
blobs, err := store.List(ctx, "22af")

for _, blob := range blobs {
    fmt.Printf("Hash: %s, Size: %d, Created: %s\n", 
        blob.Hash, blob.Size, blob.CreatedAt)
}
```

### Touch (Update Access Time)

```go
// Update last accessed time for LRU tracking
err := store.Touch(ctx, hash)
```

## Metadata Operations

### Get Metadata

```go
meta, err := store.GetMetadata(ctx, hash)
if err != nil {
    return err
}

fmt.Printf("Size: %d\n", meta.Size)
fmt.Printf("Created: %s\n", meta.CreatedAt)
fmt.Printf("Last Accessed: %s\n", meta.LastAccessed)
fmt.Printf("Access Count: %d\n", meta.AccessCount)
fmt.Printf("References: %d\n", len(meta.References))
fmt.Printf("Compression: %s\n", meta.Compression)
fmt.Printf("Encryption: %s\n", meta.Encryption)
```

### Update Metadata

```go
meta.Tags = append(meta.Tags, "archived")
err := store.UpdateMetadata(ctx, hash, meta)
```

## Tiering Operations (Hybrid Mode Only)

### Move Blob to Another Store

```go
hotStore := manager.Store().(*blob.HybridStore).Hot()
coldStore := manager.Store().(*blob.HybridStore).Cold()

// Move from hot to cold
err := hotStore.Move(ctx, hash, coldStore)
```

### Copy Blob to Another Store

```go
// Copy to cold tier (keep in hot)
err := hotStore.Copy(ctx, hash, coldStore)
```

### Cool Down (Hot → Cold)

```go
hybridStore := manager.Store().(*blob.HybridStore)
err := hybridStore.CoolDown(ctx, hash)
```

### Warm Up (Cold → Hot)

```go
err := hybridStore.WarmUp(ctx, hash)
```

### Apply Tiering Policy

```go
// Manually trigger tiering policy
err := manager.ApplyTieringPolicy(ctx)
```

## Data Ingestion

### Ingest a Chunk

```go
ingestion := manager.Ingestion()

chunk := &domain.Chunk{
    ID:         domain.ChunkID("chunk-123"),
    DatasetID:  domain.DatasetID("dataset-456"),
    VersionID:  domain.DatasetVersionID("version-789"),
    Index:      0,
    Hash:       "22af57403105fecf476b9264ca58f0f1f6cad15c00880513ef5376d87649049f",
}

data := bytes.NewReader(chunkBytes)

err := ingestion.IngestChunk(ctx, chunk, data)
```

### Retrieve a Chunk

```go
reader, err := ingestion.RetrieveChunk(ctx, chunk)
if err != nil {
    return err
}
defer reader.Close()
```

### Delete a Chunk

```go
// Removes reference and deletes blob if no more references
err := ingestion.DeleteChunk(ctx, chunk)
```

### Verify Dataset Integrity

```go
err := ingestion.VerifyDatasetIntegrity(ctx, versionID)
if err != nil {
    log.Error("Integrity check failed", "error", err)
}
```

## Garbage Collection

### Manual GC

```go
gc := manager.GC()

err := gc.Run(ctx)
```

### GC with Storage Pressure Check

```go
err := gc.RunWithPressure(ctx)
// Only runs if storage pressure > threshold
```

### Force Collect Specific Blob

```go
err := gc.ForceCollect(ctx, hash)
```

### Empty Trash

```go
err := gc.EmptyTrash(ctx, true) // permanent=true required
```

## Statistics

### Get Storage Stats

```go
stats, err := store.Stats(ctx)
if err != nil {
    return err
}

fmt.Printf("Total Blobs: %d\n", stats.TotalBlobs)
fmt.Printf("Total Size: %d bytes\n", stats.TotalSize)
fmt.Printf("Oldest Blob: %s\n", stats.OldestBlob)
fmt.Printf("Newest Blob: %s\n", stats.NewestBlob)
fmt.Printf("Backend: %s\n", stats.Backend)
```

### Get Manager Stats

```go
stats, err := manager.Stats(ctx)
```

## Backend Detection

```go
backend := store.Backend()

switch backend {
case blob.BackendLocal:
    fmt.Println("Using local storage")
case blob.BackendS3:
    fmt.Println("Using S3 storage")
default:
    fmt.Println("Using hybrid storage")
}
```

## Error Handling

```go
_, err := store.Get(ctx, hash)
if err != nil {
    if strings.Contains(err.Error(), "not found") {
        // Handle missing blob
    } else if strings.Contains(err.Error(), "decrypt") {
        // Handle decryption error
    } else {
        // Handle other errors
    }
}
```

## Complete Example: Ingest Dataset

```go
func ingestDataset(ctx context.Context, manager *blob.Manager, version *domain.DatasetVersion, rawData []byte) error {
    ingestion := manager.Ingestion()
    
    // Split into chunks
    const chunkSize = 1024 * 1024 // 1MB
    chunks := make([]*domain.Chunk, 0)
    chunkData := make(map[int]io.Reader)
    
    for i := 0; i < len(rawData); i += chunkSize {
        end := i + chunkSize
        if end > len(rawData) {
            end = len(rawData)
        }
        
        data := rawData[i:end]
        hash := sha256.Sum256(data)
        hashStr := hex.EncodeToString(hash[:])
        
        chunk := &domain.Chunk{
            ID:        domain.ChunkID(uuid.New().String()),
            DatasetID: version.DatasetID,
            VersionID: version.ID,
            Index:     len(chunks),
            Hash:      hashStr,
            Size:      int64(len(data)),
        }
        
        chunks = append(chunks, chunk)
        chunkData[chunk.Index] = bytes.NewReader(data)
    }
    
    // Ingest all chunks
    return ingestion.IngestDataset(ctx, version, chunks, chunkData)
}
```

## Configuration Examples

### Local Only

```yaml
storage:
  blob:
    mode: local
    local:
      enabled: true
      compression:
        enabled: true
        algorithm: zstd
        level: 3
```

### S3 Only

```yaml
storage:
  blob:
    mode: s3
    s3:
      enabled: true
      endpoint: https://s3.amazonaws.com
      region: us-east-1
      bucket: my-bib-blobs
      access_key_id: ${AWS_ACCESS_KEY_ID}
      secret_access_key: ${AWS_SECRET_ACCESS_KEY}
```

### Hybrid with Tiering

```yaml
storage:
  blob:
    mode: hybrid
    local:
      enabled: true
      max_size_gb: 100
    s3:
      enabled: true
      bucket: my-bib-blobs
    tiering:
      enabled: true
      strategy: lru
      hot_max_size_gb: 100
      hot_max_age_days: 30
```

### With Encryption

```yaml
storage:
  blob:
    mode: local
    local:
      enabled: true
      encryption:
        enabled: true
        algorithm: aes256-gcm
        key_derivation: node-identity
```

## Performance Tips

1. **Use compression for text data**: Enable zstd compression for logs, CSV, JSON
2. **Batch operations**: Ingest multiple chunks in one call when possible
3. **Hybrid mode for mixed workloads**: Hot tier for active data, cold tier for archives
4. **Tune GC schedule**: Run during low-traffic periods
5. **Enable read audit only when needed**: Reduces logging overhead
6. **Use reference counting for frequent GC**: Faster than mark-and-sweep
7. **Pre-calculate hashes**: Avoid re-hashing large blobs

## Testing

```go
func TestBlobStorage(t *testing.T) {
    // Create test logger
    log, _ := logger.New(config.LogConfig{
        Level: "debug",
        Format: "text",
    })
    defer log.Close()
    
    // Create test config
    cfg := storage.BlobLocalConfig{
        Enabled: true,
        Path: t.TempDir(),
    }
    
    // Create store
    store, err := blob.NewLocalStore(cfg, t.TempDir(), encKey, log)
    require.NoError(t, err)
    defer store.Close()
    
    // Test operations
    // ...
}
```

