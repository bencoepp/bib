# Phase 2.7: Blob Storage Implementation

**Status**: ✅ COMPLETE  
**Date**: December 18, 2025  
**Tasks**: DB-020 (Local Blob Storage), DB-021 (S3-Compatible Storage)

## Overview

Implemented a comprehensive blob storage system for Bib with support for local filesystem and S3-compatible object storage, featuring content-addressed storage (CAS), encryption, compression, garbage collection, and tiering capabilities.

## Architecture

### Core Components

1. **Store Interface** (`internal/storage/blob/types.go`)
   - Unified interface for all blob storage backends
   - Operations: Put, Get, Delete, Exists, Size, List, Touch, Move, Copy
   - Metadata management and statistics

2. **Local Storage** (`internal/storage/blob/local.go`)
   - Content-addressed filesystem storage
   - Directory structure: `<data_dir>/blobs/<hash[0:2]>/<hash[2:4]>/<hash>`
   - AES-256-GCM encryption at rest
   - Compression support (gzip, zstd)
   - Trash directory for soft deletes

3. **S3 Storage** (`internal/storage/blob/s3.go`)
   - S3-compatible object storage (MinIO, AWS S3, etc.)
   - Client-side encryption before upload
   - Server-side encryption support (SSE-S3, SSE-KMS)
   - Compression before upload
   - Reuses audit S3Client interface

4. **Hybrid Storage** (`internal/storage/blob/hybrid.go`)
   - Tiered hot (local) / cold (S3) storage
   - Three tiering strategies:
     - **LRU**: Least Recently Used eviction
     - **Age**: Time-based eviction
     - **Manual**: User-controlled tiering
   - Automatic warm-up on cold blob access
   - Configurable hot tier size limits

5. **Garbage Collection** (`internal/storage/blob/gc.go`)
   - Mark-and-sweep algorithm (default)
   - Reference counting algorithm (alternative)
   - Scheduled GC via cron expressions
   - Storage pressure-triggered GC
   - Trash retention with configurable period
   - Orphaned blob detection via database scan

6. **Data Ingestion** (`internal/storage/blob/ingestion.go`)
   - Integration layer between blob storage and datasets
   - Atomic chunk ingestion (blob + database)
   - Content deduplication via hashing
   - Reference tracking in blob metadata
   - Integrity verification

7. **Manager** (`internal/storage/blob/manager.go`)
   - Lifecycle management for blob storage
   - Background process coordination (GC, tiering)
   - Configuration-based initialization

## Features

### Content-Addressed Storage (CAS)

- **SHA-256 hashing** for all blobs
- **Automatic deduplication** - identical content stored once
- **Integrity verification** on read
- **Two-level directory sharding** reduces filesystem pressure

### Encryption

- **Local Storage**: AES-256-GCM per-blob encryption
- **S3 Storage**: Client-side encryption before upload
- **Key Derivation**: From node identity or custom key file
- **Metadata Protection**: Encryption state tracked in blob metadata

### Compression

- **Algorithms**: gzip, zstd (default)
- **Configurable Levels**: Balance speed vs. compression ratio
- **Transparent**: Automatic compression/decompression
- **Per-Backend**: Different compression for local vs. S3

### Garbage Collection

**Mark-and-Sweep** (Default):
1. Scan database for all chunk references
2. Mark referenced blobs as in-use
3. Sweep filesystem for unmarked blobs
4. Move orphaned blobs to trash
5. Permanently delete after retention period

**Reference Counting** (Alternative):
- Track reference count in blob metadata
- GC blobs with zero references
- Faster but requires consistent ref count maintenance

**Triggers**:
- Scheduled (cron expression)
- Storage pressure threshold
- Manual (`bib admin gc`)

### Tiering (Hybrid Mode)

**Hot Tier** (Local):
- Fast SSD/NVMe storage
- Recently accessed blobs
- Size-limited

**Cold Tier** (S3):
- Cost-effective object storage
- Rarely accessed blobs
- Unlimited capacity

**Strategies**:
- **LRU**: Evict least recently used when hot tier full
- **Age**: Move blobs older than X days to cold tier
- **Manual**: User tags blobs for tiering

### Audit Logging

- All Put/Delete operations logged (default)
- Optional Get operation logging
- Integration with audit trail system
- Operation metadata: hash, size, dataset/chunk references

## Configuration

```yaml
storage:
  blob:
    # Storage mode: local, s3, hybrid
    mode: local
    
    # Local filesystem storage
    local:
      enabled: true
      path: ""  # defaults to <data_dir>/blobs
      max_size_gb: 0  # 0 = unlimited
      encryption:
        enabled: false
        algorithm: aes256-gcm
        key_derivation: node-identity
      compression:
        enabled: true
        algorithm: zstd  # gzip, zstd
        level: 3
    
    # S3-compatible storage
    s3:
      enabled: false
      endpoint: ""
      region: us-east-1
      bucket: ""
      prefix: blobs/
      access_key_id: ""
      secret_access_key: ""
      use_iam: false
      server_side_encryption: AES256
      client_side_encryption:
        enabled: false
        algorithm: aes256-gcm
      compression:
        enabled: true
        algorithm: zstd
        level: 3
    
    # Tiering (hybrid mode)
    tiering:
      enabled: false
      strategy: lru  # lru, age, manual
      hot_max_size_gb: 100
      hot_max_age_days: 30
      cold_backend: s3
    
    # Garbage collection
    gc:
      enabled: true
      method: mark-and-sweep  # reference-counting
      schedule: "0 2 * * *"  # 2 AM daily
      storage_pressure_threshold: 90  # % disk usage
      min_age_days: 7  # don't GC newer blobs
      trash_retention_days: 30
      trash_path: ""  # defaults to <data_dir>/blobs/.trash
    
    # Audit logging
    audit:
      log_reads: false
      log_writes: true
      log_deletes: true
```

## Integration Points

### Storage Lifecycle

Blob storage integrates into the existing storage initialization:

```go
// In storage/open.go (planned integration)
func Open(cfg Config, dataDir string) (*Store, *blob.Manager, error) {
    // ... open database store ...
    
    // Initialize blob storage
    blobManager, err := blob.Open(
        cfg.Blob,
        dataDir,
        encryptionKey,
        dbStore,
        s3Client,
        auditLogger,
        log,
    )
    if err != nil {
        return nil, nil, err
    }
    
    // Start background processes
    if err := blobManager.Start(ctx); err != nil {
        return nil, nil, err
    }
    
    return dbStore, blobManager, nil
}
```

### Dataset Ingestion

Data ingestion uses the `Ingestion` layer:

```go
ingestion := blobManager.Ingestion()

// Ingest a chunk
chunk := &domain.Chunk{
    DatasetID: datasetID,
    VersionID: versionID,
    Index:     0,
    Hash:      hashStr,
}

err := ingestion.IngestChunk(ctx, chunk, dataReader)
```

### P2P Transfer Integration

P2P transfer hooks into blob storage via callbacks:

```go
transferManager.SetChunkCallback(func(download *domain.Download, chunk *domain.Chunk) {
    // Store chunk via ingestion layer
    ingestion.IngestChunk(ctx, chunk, bytes.NewReader(chunk.Data))
})
```

## CLI Commands (Planned)

```bash
# Manually trigger garbage collection
bib admin gc

# GC with options
bib admin gc --permanent --force

# Empty trash
bib admin gc --empty-trash

# Blob statistics
bib admin blob stats

# Verify blob integrity
bib admin blob verify --dataset <id>

# Manual tiering
bib admin blob tier --cool <hash>
bib admin blob tier --warm <hash>

# Apply tiering policy
bib admin blob tier --apply
```

## Testing

Comprehensive test suite in `internal/storage/blob/local_test.go`:

- **TestLocalStore_PutGet**: Basic put/get operations
- **TestLocalStore_WithEncryption**: Encryption roundtrip
- **TestLocalStore_WithCompression**: Compression roundtrip
- **TestLocalStore_List**: Blob listing
- **TestLocalStore_Stats**: Statistics gathering

All tests passing ✅

## File Structure

```
internal/storage/blob/
├── types.go          # Core interfaces and types
├── config.go         # Configuration type aliases
├── local.go          # Local filesystem storage
├── s3.go             # S3-compatible storage
├── hybrid.go         # Hybrid tiered storage
├── gc.go             # Garbage collection
├── ingestion.go      # Data ingestion integration
├── manager.go        # Lifecycle management
├── compression.go    # Compression utilities
└── local_test.go     # Unit tests
```

## Dependencies

```go
// New dependencies added:
github.com/klauspost/compress/zstd  // High-performance compression
```

## Performance Characteristics

### Local Storage

- **Write**: O(1) - direct file write with optional compression/encryption
- **Read**: O(1) - direct file read with decompression/decryption
- **List**: O(n) - filesystem walk (can be optimized with caching)
- **GC Mark**: O(m) - database scan for m chunks
- **GC Sweep**: O(n) - filesystem scan for n blobs

### S3 Storage

- **Write**: O(1) - single PUT request
- **Read**: O(1) - single GET request
- **List**: O(n/1000) - paginated ListObjects calls

### Hybrid Storage

- **Read**: O(1) hot tier, O(1) + network latency cold tier
- **Tiering**: O(k) - moves k blobs based on policy

## Security Considerations

1. **Encryption Keys**: Derived from node identity or custom key file
2. **Access Control**: Only daemon has access to blob directory
3. **S3 Credentials**: Managed by daemon, same security model as audit logs
4. **Audit Trail**: All write/delete operations logged
5. **Trash Protection**: Soft deletes prevent accidental data loss

## Future Enhancements

1. **Streaming**: Large blob support with streaming encryption/compression
2. **Replication**: Cross-node blob replication for HA
3. **Caching**: In-memory LRU cache for hot blobs
4. **Deduplication Stats**: Track storage savings from dedup
5. **Blob Packing**: Pack small blobs into larger files
6. **Metadata Index**: Database index for faster blob lookups
7. **Cron Library**: Replace simplified cron parser with robfig/cron/v3

## Compliance

- ✅ **DB-020**: Local blob storage with CAS, integrity verification, GC, audit logging
- ✅ **DB-021**: S3-compatible storage with tiering and credential management

## Notes

- Encryption uses AES-256-GCM for authenticated encryption
- Blob metadata stored as JSON alongside blobs
- SHA-256 used for content addressing (64-character hex)
- Trash retention prevents accidental permanent deletion
- Mark-and-sweep GC is self-healing (no corrupt ref counts)
- Hybrid mode enables cost-effective cold storage

