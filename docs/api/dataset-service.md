# DatasetService API

The DatasetService manages datasets - collections of data within topics.

## Service Definition

```protobuf
service DatasetService {
  // CRUD Operations
  rpc CreateDataset(CreateDatasetRequest) returns (CreateDatasetResponse);
  rpc GetDataset(GetDatasetRequest) returns (GetDatasetResponse);
  rpc ListDatasets(ListDatasetsRequest) returns (ListDatasetsResponse);
  rpc UpdateDataset(UpdateDatasetRequest) returns (UpdateDatasetResponse);
  rpc DeleteDataset(DeleteDatasetRequest) returns (DeleteDatasetResponse);
  
  // Versioning
  rpc CreateVersion(CreateVersionRequest) returns (CreateVersionResponse);
  rpc GetVersion(GetVersionRequest) returns (GetVersionResponse);
  rpc ListVersions(ListVersionsRequest) returns (ListVersionsResponse);
  
  // Content Transfer
  rpc UploadContent(stream UploadContentRequest) returns (UploadContentResponse);
  rpc DownloadContent(DownloadContentRequest) returns (stream DownloadContentResponse);
  
  // Chunked Transfer
  rpc GetChunk(GetChunkRequest) returns (GetChunkResponse);
  rpc ListChunks(ListChunksRequest) returns (ListChunksResponse);
}
```

## Methods

### CreateDataset

Create a new dataset within a topic.

**Authentication:** Required (Topic Editor)

**Request:**
```protobuf
message CreateDatasetRequest {
  string topic_id = 1;
  string name = 2;
  string description = 3;
  repeated string tags = 4;
  map<string, string> metadata = 5;
}
```

**Response:**
```protobuf
message CreateDatasetResponse {
  Dataset dataset = 1;
}
```

**Example:**
```go
resp, err := datasetClient.CreateDataset(ctx, &services.CreateDatasetRequest{
    TopicId:     "topic-123",
    Name:        "genome-sequences-2025",
    Description: "Human genome sequences from 2025 study",
    Tags:        []string{"genome", "sequences", "2025"},
})
```

### GetDataset

Retrieve dataset metadata.

**Authentication:** Required (Topic Member)

**Request:**
```protobuf
message GetDatasetRequest {
  string id = 1;
  bool include_versions = 2;  // Include version history
}
```

**Response:**
```protobuf
message GetDatasetResponse {
  Dataset dataset = 1;
  repeated DatasetVersion versions = 2;  // If requested
}
```

### ListDatasets

List datasets with filtering.

**Authentication:** Required

**Request:**
```protobuf
message ListDatasetsRequest {
  string topic_id = 1;        // Filter by topic
  string status = 2;          // Filter by status
  repeated string tags = 3;   // Filter by tags
  PageRequest page = 4;
  SortRequest sort = 5;
}
```

**Response:**
```protobuf
message ListDatasetsResponse {
  repeated Dataset datasets = 1;
  PageInfo page_info = 2;
}
```

### UpdateDataset

Update dataset metadata.

**Authentication:** Required (Dataset Owner)

**Request:**
```protobuf
message UpdateDatasetRequest {
  string id = 1;
  string name = 2;
  string description = 3;
  repeated string tags = 4;
  map<string, string> metadata = 5;
}
```

### DeleteDataset

Delete a dataset.

**Authentication:** Required (Dataset Owner)

**Request:**
```protobuf
message DeleteDatasetRequest {
  string id = 1;
  bool force = 2;  // Skip confirmation
}
```

## Version Management

### CreateVersion

Create a new version of a dataset.

**Authentication:** Required (Dataset Owner/Editor)

**Request:**
```protobuf
message CreateVersionRequest {
  string dataset_id = 1;
  string version = 2;           // Semantic version (e.g., "1.2.0")
  string message = 3;           // Version message/changelog
  repeated Instruction instructions = 4;  // Data transformation instructions
  bytes content_hash = 5;       // SHA-256 of complete content
  int64 size_bytes = 6;
}
```

**Response:**
```protobuf
message CreateVersionResponse {
  DatasetVersion version = 1;
  string upload_url = 2;        // If content needs uploading
}
```

### GetVersion

Get a specific version.

**Authentication:** Required (Topic Member)

**Request:**
```protobuf
message GetVersionRequest {
  string dataset_id = 1;
  string version_id = 2;  // Or "latest"
}
```

### ListVersions

List all versions of a dataset.

**Authentication:** Required (Topic Member)

**Request:**
```protobuf
message ListVersionsRequest {
  string dataset_id = 1;
  PageRequest page = 2;
}
```

## Content Transfer

### UploadContent

Stream upload dataset content.

**Authentication:** Required (Dataset Owner/Editor)

**Request Stream:**
```protobuf
message UploadContentRequest {
  oneof data {
    UploadMetadata metadata = 1;  // First message
    bytes chunk = 2;               // Subsequent messages
  }
}

message UploadMetadata {
  string dataset_id = 1;
  string version = 2;
  string content_type = 3;
  int64 total_size = 4;
  bytes expected_hash = 5;
}
```

**Response:**
```protobuf
message UploadContentResponse {
  string version_id = 1;
  bytes content_hash = 2;
  int64 bytes_received = 3;
  int32 chunks_stored = 4;
}
```

**Example:**
```go
stream, err := datasetClient.UploadContent(ctx)
if err != nil {
    log.Fatal(err)
}

// Send metadata first
err = stream.Send(&services.UploadContentRequest{
    Data: &services.UploadContentRequest_Metadata{
        Metadata: &services.UploadMetadata{
            DatasetId:   "dataset-123",
            Version:     "1.0.0",
            ContentType: "application/octet-stream",
            TotalSize:   fileSize,
        },
    },
})

// Send chunks
buf := make([]byte, 256*1024) // 256KB chunks
for {
    n, err := file.Read(buf)
    if err == io.EOF {
        break
    }
    
    err = stream.Send(&services.UploadContentRequest{
        Data: &services.UploadContentRequest_Chunk{
            Chunk: buf[:n],
        },
    })
    if err != nil {
        log.Fatal(err)
    }
}

// Get response
resp, err := stream.CloseAndRecv()
```

### DownloadContent

Stream download dataset content.

**Authentication:** Required (Topic Member)

**Request:**
```protobuf
message DownloadContentRequest {
  string dataset_id = 1;
  string version_id = 2;       // Or "latest"
  int64 offset = 3;            // Resume from offset
  int64 length = 4;            // Max bytes (0 = all)
}
```

**Response Stream:**
```protobuf
message DownloadContentResponse {
  oneof data {
    DownloadMetadata metadata = 1;  // First message
    bytes chunk = 2;                 // Subsequent messages
  }
}

message DownloadMetadata {
  string version_id = 1;
  string content_type = 2;
  int64 total_size = 3;
  bytes content_hash = 4;
  int32 total_chunks = 5;
}
```

**Example:**
```go
stream, err := datasetClient.DownloadContent(ctx, &services.DownloadContentRequest{
    DatasetId: "dataset-123",
    VersionId: "latest",
})

// First message is metadata
resp, err := stream.Recv()
metadata := resp.GetMetadata()
log.Printf("Downloading %d bytes", metadata.TotalSize)

// Write chunks to file
for {
    resp, err := stream.Recv()
    if err == io.EOF {
        break
    }
    file.Write(resp.GetChunk())
}
```

## Chunked Transfer

For P2P data distribution, content is split into chunks.

### GetChunk

Get a specific chunk by index or hash.

**Request:**
```protobuf
message GetChunkRequest {
  string version_id = 1;
  int32 chunk_index = 2;  // By index
  string chunk_hash = 3;  // Or by hash
}
```

**Response:**
```protobuf
message GetChunkResponse {
  Chunk chunk = 1;
}

message Chunk {
  int32 index = 1;
  bytes data = 2;
  bytes hash = 3;
  int32 size = 4;
}
```

### ListChunks

List all chunks for a version (for P2P sync).

**Request:**
```protobuf
message ListChunksRequest {
  string version_id = 1;
}
```

**Response:**
```protobuf
message ListChunksResponse {
  repeated ChunkInfo chunks = 1;
}

message ChunkInfo {
  int32 index = 1;
  bytes hash = 2;
  int32 size = 3;
  bool available = 4;  // Available locally
}
```

## Data Models

### Dataset

```protobuf
message Dataset {
  string id = 1;
  string topic_id = 2;
  string name = 3;
  string description = 4;
  string status = 5;           // "draft", "active", "archived"
  repeated string owners = 6;
  string created_by = 7;
  Timestamp created_at = 8;
  Timestamp updated_at = 9;
  repeated string tags = 10;
  map<string, string> metadata = 11;
  DatasetStats stats = 12;
  DatasetVersion latest_version = 13;
}
```

### DatasetVersion

```protobuf
message DatasetVersion {
  string id = 1;
  string dataset_id = 2;
  string version = 3;          // Semantic version
  string message = 4;          // Changelog
  bytes content_hash = 5;      // SHA-256
  int64 size_bytes = 6;
  int32 chunk_count = 7;
  string created_by = 8;
  Timestamp created_at = 9;
  repeated Instruction instructions = 10;
}
```

## Error Codes

| Error | Code | Description |
|-------|------|-------------|
| Dataset not found | `NOT_FOUND` | Dataset doesn't exist |
| Version not found | `NOT_FOUND` | Version doesn't exist |
| Hash mismatch | `DATA_LOSS` | Content hash verification failed |
| Topic not found | `NOT_FOUND` | Parent topic doesn't exist |
| Permission denied | `PERMISSION_DENIED` | Insufficient role |
| Invalid version | `INVALID_ARGUMENT` | Version format invalid |

