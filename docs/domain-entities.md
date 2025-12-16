# Bib Domain Entities Documentation

This document provides a comprehensive overview of the domain entities in the `bib` distributed data management system. These entities form the core data model used across all layers of the system including P2P networking, storage, and gRPC APIs.

## Table of Contents

1. [Overview](#overview)
2. [Core Entities](#core-entities)
   - [User](#user)
   - [Topic](#topic)
   - [Dataset](#dataset)
   - [DatasetVersion](#datasetversion)
3. [Content & Instructions](#content--instructions)
   - [DatasetContent](#datasetcontent)
   - [DatasetInstructions](#datasetinstructions)
   - [Instruction](#instruction)
   - [Task](#task)
4. [Job Execution](#job-execution)
   - [Job](#job)
   - [Pipeline](#pipeline)
   - [Schedule](#schedule)
5. [Discovery & Sync](#discovery--sync)
   - [Catalog](#catalog)
   - [CatalogEntry](#catalogentry)
   - [Download](#download)
   - [Subscription](#subscription)
6. [Access Control](#access-control)
   - [Ownership](#ownership)
   - [OwnershipTransfer](#ownershiptransfer)
7. [Querying](#querying)
   - [QueryRequest](#queryrequest)
   - [QueryResult](#queryresult)
8. [Entity Relationships](#entity-relationships)
9. [Class Diagram](#class-diagram)

---

## Overview

The `bib` system is a decentralized data management platform that enables peers to:

- **Organize data** in hierarchical topics and datasets
- **Version control** data with immutable versions
- **Share data** across a P2P network
- **Execute jobs** to acquire, transform, and process data
- **Query data** using SQL or metadata filters
- **Control access** through cryptographic user identities

All entities support JSON serialization and include validation methods for data integrity.

---

## Core Entities

### User

**Purpose**: Represents a user with a cryptographic identity in the bib network.

**Key Characteristics**:
- Identity derived from Ed25519 public key
- UserID is the hex-encoded first 20 bytes of the public key
- Can sign operations for authentication
- Independent from node identity

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | UserID | Unique identifier derived from public key |
| PublicKey | []byte | Ed25519 public key (32 bytes) |
| Name | string | Human-readable display name |
| Email | string | Optional contact email |
| CreatedAt | time.Time | Creation timestamp |
| UpdatedAt | time.Time | Last modification timestamp |
| Metadata | map[string]string | Additional profile data |

**Related Types**:
- `SignedOperation` - Represents an operation signed by a user for authentication

---

### Topic

**Purpose**: Categories for organizing datasets hierarchically (e.g., "weather", "finance/stocks").

**Key Characteristics**:
- Supports hierarchical structure via ParentID
- Defines optional SQL table schema for datasets
- Tracks multiple owners
- Has lifecycle status (active, archived, deleted)

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | TopicID | Unique identifier |
| ParentID | TopicID | Parent topic for hierarchy (optional) |
| Name | string | Human-readable name |
| Description | string | Topic details |
| TableSchema | string | SQL DDL schema for datasets |
| Status | TopicStatus | Lifecycle status |
| Owners | []UserID | Users who own this topic |
| CreatedBy | UserID | Creator user |
| DatasetCount | int | Number of datasets |
| Tags | []string | Labels for categorization |
| Metadata | map[string]string | Additional key-value pairs |

**Status Values**: `active`, `archived`, `deleted`

---

### Dataset

**Purpose**: A unit of data within a topic. Can contain actual data content and/or instructions for acquiring data.

**Key Characteristics**:
- Belongs to exactly one Topic
- Supports multiple versions (immutable)
- Can have content (actual data) and/or instructions
- Tracks ownership separately from topic

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | DatasetID | Unique identifier |
| TopicID | TopicID | Parent topic |
| Name | string | Human-readable name |
| Description | string | Dataset details |
| Status | DatasetStatus | Current status |
| LatestVersionID | DatasetVersionID | Most recent version |
| VersionCount | int | Total versions |
| HasContent | bool | Has actual data |
| HasInstructions | bool | Has acquisition instructions |
| Owners | []UserID | Users who own this dataset |
| CreatedBy | UserID | Creator user |
| Tags | []string | Labels for categorization |
| Metadata | map[string]string | Additional data |

**Status Values**: `draft`, `active`, `archived`, `deleted`, `ingesting`, `failed`

---

### DatasetVersion

**Purpose**: An immutable snapshot of a dataset at a point in time.

**Key Characteristics**:
- Immutable once created
- Links to previous version for history chain
- Contains either Content, Instructions, or both
- Uses semantic versioning (e.g., "1.0.0")

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | DatasetVersionID | Unique version identifier |
| DatasetID | DatasetID | Parent dataset |
| Version | string | Semantic version string |
| PreviousVersionID | DatasetVersionID | Link to previous version |
| Content | *DatasetContent | Actual data (optional) |
| Instructions | *DatasetInstructions | Acquisition instructions (optional) |
| TableSchema | string | SQL DDL for this version |
| CreatedBy | UserID | Creator user |
| Message | string | Version message (like commit message) |
| Metadata | map[string]string | Version-specific metadata |

---

## Content & Instructions

### DatasetContent

**Purpose**: Represents the actual data content of a dataset version.

**Key Characteristics**:
- Content is chunked for efficient P2P transfer
- Uses SHA-256 hash for integrity verification
- Supports various storage formats

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| Hash | string | SHA-256 content hash |
| Size | int64 | Total size in bytes |
| RowCount | int64 | Number of data rows |
| ChunkCount | int | Number of chunks |
| ChunkSize | int64 | Size per chunk |
| Format | string | Storage format |
| Checksum | string | Additional checksum |

**Related**: `Chunk` - Represents a piece of dataset for chunked transfer

---

### DatasetInstructions

**Purpose**: Contains CEL-based instructions for data acquisition.

**Key Characteristics**:
- Can reference a reusable Task
- Or contain inline Instructions
- Supports scheduled re-execution
- Tracks execution status

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| TaskID | TaskID | Reference to Task (optional) |
| Instructions | []Instruction | Inline instructions |
| InputVariables | map[string]string | Required inputs |
| SourceMetadata | *SourceMetadata | Data source info |
| Schedule | *Schedule | Auto re-execution schedule |
| LastExecutedAt | *time.Time | Last execution time |
| LastExecutionStatus | string | Last execution result |

---

### Instruction

**Purpose**: A single CEL-based operation in a task or dataset.

**Key Characteristics**:
- Uses predefined safe operations (no shell exec)
- Supports error handling and retries
- Can have conditional execution
- Stores output in variables

**Operations Categories**:
- **HTTP**: `http.get`, `http.post`
- **File**: `file.unzip`, `file.read`, `file.write`, `file.hash`, etc.
- **Parse**: `csv.parse`, `json.parse`, `xml.parse`
- **Transform**: `transform.map`, `transform.filter`, `transform.reduce`, etc.
- **Validate**: `validate.schema`, `validate.checksum`, etc.
- **String**: `string.split`, `string.replace`, etc.
- **Control**: `control.if`, `control.foreach`, `control.retry`
- **Variable**: `var.set`, `var.get`, `var.delete`
- **Output**: `output.store`, `output.append`

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | string | Optional identifier |
| Operation | InstructionOp | CEL operation type |
| Params | map[string]any | Operation parameters |
| OutputVar | string | Variable for result |
| Condition | string | CEL condition expression |
| OnError | string | Error handling: fail/skip/retry |
| RetryCount | int | Retries if OnError="retry" |
| Description | string | Human-readable description |

---

### Task

**Purpose**: A reusable sequence of instructions that Jobs can execute.

**Key Characteristics**:
- Template for Job execution
- Versioned with semver
- Defines input/output schemas
- Can be shared and reused

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | TaskID | Unique identifier |
| Name | string | Human-readable name |
| Description | string | What the task does |
| Version | string | Semantic version |
| Instructions | []Instruction | Ordered instruction sequence |
| InputSchema | string | Expected inputs (JSON Schema) |
| OutputSchema | string | Expected outputs (JSON Schema) |
| CreatedBy | UserID | Creator user |
| Tags | []string | Labels for categorization |

---

## Job Execution

### Job

**Purpose**: A scheduled execution instance of a Task or inline instructions.

**Key Characteristics**:
- Runtime instance that executes Task templates
- Supports multiple execution modes (goroutine, container, pod)
- Can have dependencies for pipeline execution
- Tracks progress and status

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | JobID | Unique identifier |
| Type | JobType | Job category |
| Status | JobStatus | Current status |
| TaskID | TaskID | Task to execute |
| InlineInstructions | []Instruction | Alternative to TaskID |
| ExecutionMode | ExecutionMode | How job runs |
| Schedule | *Schedule | When/how often to run |
| Inputs | []JobInput | Job inputs |
| Outputs | []JobOutput | Job outputs |
| Dependencies | []JobID | Jobs that must complete first |
| ResourceLimits | *ResourceLimits | Execution constraints |
| Progress | int | Execution progress (0-100) |
| NodeID | string | Executing node |

**Job Types**: `scrape`, `transform`, `clean`, `analyze`, `ml`, `etl`, `ingest`, `export`, `custom`

**Status Values**: `pending`, `queued`, `running`, `completed`, `failed`, `cancelled`, `waiting`, `retrying`

**Execution Modes**: `goroutine`, `container`, `pod`

---

### Pipeline

**Purpose**: A collection of Jobs with dependencies forming a DAG workflow.

**Key Characteristics**:
- Organizes multiple jobs into a workflow
- Validates DAG (no cyclic dependencies)
- Tracks overall pipeline status

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | string | Unique identifier |
| Name | string | Pipeline name |
| Description | string | Pipeline details |
| Jobs | []*Job | Jobs in pipeline |
| Status | JobStatus | Overall status |
| CreatedBy | UserID | Creator user |

---

### Schedule

**Purpose**: Defines when and how often a job runs.

**Types**:
- `once` - Runs exactly once
- `cron` - Runs on cron schedule
- `repeat` - Runs a specific number of times
- `interval` - Runs at fixed intervals

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| Type | ScheduleType | Schedule type |
| CronExpr | string | Cron expression |
| RepeatCount | int | Number of repeats |
| Interval | time.Duration | Duration between runs |
| StartAt | *time.Time | When schedule activates |
| EndAt | *time.Time | When schedule expires |
| Timezone | string | Timezone for cron |
| RunCount | int | Completed runs |
| NextRunAt | *time.Time | Next run time |
| Enabled | bool | Is schedule active |

---

## Discovery & Sync

### Catalog

**Purpose**: Represents a peer's available data for discovery.

**Key Characteristics**:
- Each peer maintains a catalog
- Contains lightweight entries for discovery
- Versioned for change detection

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| PeerID | string | Owning peer |
| Entries | []CatalogEntry | Available data |
| LastUpdated | time.Time | Last refresh time |
| Version | uint64 | Catalog version |

---

### CatalogEntry

**Purpose**: A lightweight entry in a peer's catalog for discovery without full content transfer.

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| TopicID | TopicID | Topic identifier |
| TopicName | string | Topic name |
| DatasetID | DatasetID | Dataset identifier |
| DatasetName | string | Dataset name |
| VersionID | DatasetVersionID | Version identifier |
| Version | string | Semantic version |
| Hash | string | Content hash |
| Size | int64 | Size in bytes |
| ChunkCount | int | Number of chunks |
| HasContent | bool | Has data content |
| HasInstructions | bool | Has instructions |
| Owners | []UserID | Dataset owners |
| UpdatedAt | time.Time | Last update time |

---

### Download

**Purpose**: Tracks a dataset download in progress.

**Key Characteristics**:
- Uses bitmap for chunk tracking
- Supports resumable downloads
- Can switch peers for multi-peer download

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | string | Download identifier |
| DatasetID | DatasetID | Dataset being downloaded |
| DatasetHash | string | Expected content hash |
| PeerID | string | Source peer |
| TotalChunks | int | Total chunks |
| CompletedChunks | int | Downloaded chunks |
| ChunkBitmap | []byte | Completion bitmap |
| Status | DownloadStatus | Download status |

**Status Values**: `active`, `paused`, `completed`, `failed`

---

### Subscription

**Purpose**: A topic subscription for selective sync mode.

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| TopicPattern | string | Pattern with wildcards |
| CreatedAt | time.Time | Creation time |
| LastSync | time.Time | Last sync time |

---

## Access Control

### Ownership

**Purpose**: Represents ownership or access grant for a resource.

**Roles**:
- `owner` - Full control including deletion and transfer
- `admin` - Can modify but not delete or transfer
- `contributor` - Can add data but not modify structure
- `reader` - Read-only access

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ResourceType | ResourceType | Type: topic/dataset/task |
| ResourceID | string | Resource identifier |
| UserID | UserID | User with access |
| Role | OwnershipRole | Permission level |
| GrantedAt | time.Time | Grant time |
| GrantedBy | UserID | Granting user |
| ExpiresAt | *time.Time | Optional expiration |

---

### OwnershipTransfer

**Purpose**: Request to transfer ownership between users.

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ResourceType | ResourceType | Resource type |
| ResourceID | string | Resource identifier |
| FromUserID | UserID | Current owner |
| ToUserID | UserID | New owner |
| RequestedAt | time.Time | Request time |
| Signature | []byte | Owner's signature |

---

## Querying

### QueryRequest

**Purpose**: A request to query data or metadata.

**Query Types**:
- `metadata` - Query topic/dataset metadata
- `sql` - Query data via SQL SELECT

**Key Characteristics**:
- SQL queries validate for SELECT-only
- Supports pagination and streaming
- Can target multiple datasets

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| ID | string | Query identifier |
| Type | QueryType | metadata or sql |
| TopicID | TopicID | Filter by topic |
| DatasetID | DatasetID | Filter by dataset |
| TargetDatasets | []DatasetTarget | SQL query targets |
| SQL | string | SELECT query |
| NamePattern | string | Wildcard filter |
| Expression | string | CEL filter expression |
| Limit | int | Max results |
| Offset | int | Pagination offset |
| StreamResults | bool | Enable streaming |
| Timeout | time.Duration | Query timeout |

---

### QueryResult

**Purpose**: Result of a query operation.

**Fields**:
| Field | Type | Description |
|-------|------|-------------|
| QueryID | string | Original query ID |
| Type | QueryType | Result type |
| Entries | []CatalogEntry | Metadata results |
| Columns | []QueryColumn | SQL column definitions |
| Rows | [][]any | SQL result rows |
| TotalCount | int | Total matches |
| FromCache | bool | Served from cache |
| ExecutionTimeMs | int64 | Execution duration |
| Truncated | bool | Results truncated |

---

## Entity Relationships

### Hierarchical Relationships

1. **User → Topic/Dataset/Task**: Users own and create resources
2. **Topic → Dataset**: Topics contain multiple datasets
3. **Topic → Topic**: Topics can have parent topics (hierarchy)
4. **Dataset → DatasetVersion**: Datasets have multiple versions
5. **DatasetVersion → DatasetContent/DatasetInstructions**: Versions contain content or instructions

### Execution Relationships

1. **Task → Instruction**: Tasks contain ordered instructions
2. **Job → Task**: Jobs execute tasks
3. **Job → Job**: Jobs can depend on other jobs
4. **Pipeline → Job**: Pipelines organize multiple jobs
5. **Job/DatasetInstructions → Schedule**: Scheduled execution

### Discovery Relationships

1. **Catalog → CatalogEntry**: Catalogs contain entries
2. **CatalogEntry → Topic/Dataset/Version**: Entries reference data
3. **Download → Dataset**: Downloads track dataset transfers

### Access Control Relationships

1. **Ownership → User**: Grants user access to resources
2. **Ownership → Topic/Dataset/Task**: Applies to resources
3. **OwnershipTransfer → User**: Transfers between users

---

## Class Diagram

```
+------------------+          +-------------------+
|      User        |          |   SignedOperation |
+------------------+          +-------------------+
| - ID: UserID     |          | - UserID          |
| - PublicKey      |<-------->| - Operation       |
| - Name           |  signs   | - Payload         |
| - Email          |          | - Timestamp       |
| - Metadata       |          | - Signature       |
+------------------+          +-------------------+
        |
        | owns
        v
+------------------+       +-------------------+       +--------------------+
|     Topic        |       |     Dataset       |       |  DatasetVersion    |
+------------------+       +-------------------+       +--------------------+
| - ID: TopicID    |1    * | - ID: DatasetID   |1    * | - ID: VersionID    |
| - ParentID       |<----->| - TopicID         |<----->| - DatasetID        |
| - Name           |       | - Name            |       | - Version          |
| - Description    |       | - Description     |       | - PreviousVersionID|
| - TableSchema    |       | - Status          |       | - Content          |
| - Status         |       | - LatestVersionID |       | - Instructions     |
| - Owners[]       |       | - HasContent      |       | - TableSchema      |
| - DatasetCount   |       | - HasInstructions |       | - CreatedBy        |
| - Tags[]         |       | - Owners[]        |       | - Message          |
+------------------+       +-------------------+       +--------------------+
        ^                                                    |      |
        |                                                    v      v
        | parent                              +---------------+    +---------------------+
        |                                     | DatasetContent|    | DatasetInstructions |
+------------------+                          +---------------+    +---------------------+
|   TopicTree      |                          | - Hash        |    | - TaskID            |
+------------------+                          | - Size        |    | - Instructions[]    |
| - Topic          |                          | - RowCount    |    | - InputVariables    |
| - Children[]     |                          | - ChunkCount  |    | - SourceMetadata    |
+------------------+                          | - ChunkSize   |    | - Schedule          |
                                              +---------------+    +---------------------+
                                                     |                      |
                                                     v                      v
                                              +---------------+    +-------------------+
                                              |    Chunk      |    |    Instruction    |
                                              +---------------+    +-------------------+
                                              | - DatasetID   |    | - ID              |
                                              | - VersionID   |    | - Operation       |
                                              | - Index       |    | - Params          |
                                              | - Hash        |    | - OutputVar       |
                                              | - Size        |    | - Condition       |
                                              | - Data        |    | - OnError         |
                                              +---------------+    +-------------------+
                                                                          ^
                                                                          |
+------------------+       +-------------------+       +-------------------+
|    Pipeline      |       |       Job         |       |       Task        |
+------------------+       +-------------------+       +-------------------+
| - ID             |1    * | - ID: JobID       |       | - ID: TaskID      |
| - Name           |<----->| - Type            |       | - Name            |
| - Description    |       | - Status          |       | - Description     |
| - Jobs[]         |       | - TaskID          |------>| - Version         |
| - Status         |       | - InlineInstr[]   |       | - Instructions[]  |
| - CreatedBy      |       | - ExecutionMode   |       | - InputSchema     |
+------------------+       | - Schedule        |       | - OutputSchema    |
                           | - Dependencies[]  |       | - CreatedBy       |
                           | - ResourceLimits  |       +-------------------+
                           | - Progress        |
                           +-------------------+
                                   |
                                   v
+------------------+       +-------------------+
|    Schedule      |       |  ResourceLimits   |
+------------------+       +-------------------+
| - Type           |       | - MaxMemoryMB     |
| - CronExpr       |       | - MaxCPUCores     |
| - RepeatCount    |       | - TimeoutSeconds  |
| - Interval       |       | - MaxRetries      |
| - StartAt        |       | - MaxOutputSizeMB |
| - EndAt          |       +-------------------+
| - RunCount       |
| - NextRunAt      |
| - Enabled        |
+------------------+

+------------------+       +-------------------+       +-------------------+
|    Catalog       |       |   CatalogEntry    |       |     Download      |
+------------------+       +-------------------+       +-------------------+
| - PeerID         |1    * | - TopicID         |       | - ID              |
| - Entries[]      |<----->| - DatasetID       |       | - DatasetID       |
| - LastUpdated    |       | - VersionID       |       | - DatasetHash     |
| - Version        |       | - Hash            |       | - PeerID          |
+------------------+       | - Size            |       | - TotalChunks     |
                           | - ChunkCount      |       | - CompletedChunks |
                           | - HasContent      |       | - ChunkBitmap     |
                           | - HasInstructions |       | - Status          |
                           | - Owners[]        |       +-------------------+
                           +-------------------+

+------------------+       +-------------------+       +-------------------+
|   Ownership      |       | OwnershipTransfer |       |   Subscription    |
+------------------+       +-------------------+       +-------------------+
| - ResourceType   |       | - ResourceType    |       | - TopicPattern    |
| - ResourceID     |       | - ResourceID      |       | - CreatedAt       |
| - UserID         |       | - FromUserID      |       | - LastSync        |
| - Role           |       | - ToUserID        |       +-------------------+
| - GrantedAt      |       | - RequestedAt     |
| - GrantedBy      |       | - Signature       |
| - ExpiresAt      |       +-------------------+
+------------------+

+------------------+       +-------------------+
|  QueryRequest    |       |   QueryResult     |
+------------------+       +-------------------+
| - ID             |       | - QueryID         |
| - Type           |------>| - Type            |
| - TopicID        |       | - Entries[]       |
| - DatasetID      |       | - Columns[]       |
| - TargetDatasets |       | - Rows[]          |
| - SQL            |       | - TotalCount      |
| - NamePattern    |       | - FromCache       |
| - Limit/Offset   |       | - ExecutionTimeMs |
| - StreamResults  |       | - Truncated       |
+------------------+       +-------------------+
```

---

## Summary

The bib domain model is designed around these key concepts:

1. **Cryptographic Identity**: Users are identified by Ed25519 public keys, enabling secure, decentralized authentication.

2. **Hierarchical Organization**: Topics provide a tree structure for organizing datasets, similar to filesystem directories.

3. **Immutable Versioning**: Dataset versions are immutable, providing a complete history chain with content hashes for integrity.

4. **Dual Content Model**: Datasets can contain actual data AND/OR instructions for acquiring data, supporting both static and dynamic data sources.

5. **CEL-Based Execution**: Instructions use a safe, sandboxed CEL operation set for data acquisition and transformation.

6. **DAG Pipelines**: Jobs can form complex workflows with dependencies, validated to prevent cycles.

7. **P2P Discovery**: Catalogs enable peers to discover and sync data without transferring full content.

8. **Role-Based Access**: Fine-grained ownership and access control with expiring grants.

9. **Flexible Scheduling**: Jobs and instructions support one-time, cron, interval, and repeat scheduling.

10. **SQL Querying**: Read-only SQL queries can span multiple datasets with streaming support.

