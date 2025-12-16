# Protocols Reference

This document describes the wire protocols used for P2P communication in bib.

## Overview

Bib uses custom protocols built on libp2p for inter-node communication. Messages are currently encoded as JSON (migrating to Protocol Buffers for production).

## Protocol IDs

| Protocol | ID | Purpose |
|----------|-----|---------|
| Discovery | `/bib/discovery/1.0.0` | Peer and catalog discovery |
| Data | `/bib/data/1.0.0` | Dataset transfers |
| Jobs | `/bib/jobs/1.0.0` | Job distribution |
| Sync | `/bib/sync/1.0.0` | State synchronization |

## Message Format

All protocol messages follow this structure:

```json
{
  "type": "message_type",
  "request_id": "unique-uuid",
  "payload": { ... },
  "error": null
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Message type identifier |
| `request_id` | string | UUID for request/response matching |
| `payload` | object | Type-specific payload |
| `error` | object | Error details (if applicable) |

### Error Format

```json
{
  "error": {
    "code": 404,
    "message": "Dataset not found",
    "details": {
      "dataset_id": "weather-2024"
    }
  }
}
```

---

## Discovery Protocol

**Protocol ID:** `/bib/discovery/1.0.0`

Handles peer discovery, catalog exchange, and data announcements.

### Message Types

#### get_catalog / catalog

Request and receive a peer's catalog.

**Request:**
```json
{
  "type": "get_catalog",
  "request_id": "uuid",
  "payload": {
    "since_version": 0
  }
}
```

**Response:**
```json
{
  "type": "catalog",
  "request_id": "uuid",
  "payload": {
    "peer_id": "QmXyz...",
    "entries": [
      {
        "topic_id": "weather",
        "topic_name": "Weather",
        "dataset_id": "daily-temps",
        "dataset_name": "Daily Temperatures",
        "version_id": "v1.0.0",
        "version": "1.0.0",
        "hash": "sha256:abc123...",
        "size": 1048576,
        "chunk_count": 1,
        "has_content": true,
        "has_instructions": false,
        "owners": ["user-abc"],
        "updated_at": "2024-01-15T10:30:00Z"
      }
    ],
    "last_updated": "2024-01-15T10:30:00Z",
    "version": 42
  }
}
```

#### query_catalog / query_result

Query for specific catalog entries.

**Request:**
```json
{
  "type": "query_catalog",
  "request_id": "uuid",
  "payload": {
    "topic_id": "weather",
    "dataset_id": "",
    "name_pattern": "*temps*",
    "limit": 10,
    "offset": 0
  }
}
```

**Response:**
```json
{
  "type": "query_result",
  "request_id": "uuid",
  "payload": {
    "entries": [...],
    "total_count": 25
  }
}
```

#### get_peer_info / peer_info

Request peer information.

**Request:**
```json
{
  "type": "get_peer_info",
  "request_id": "uuid",
  "payload": {}
}
```

**Response:**
```json
{
  "type": "peer_info",
  "request_id": "uuid",
  "payload": {
    "peer_id": "QmXyz...",
    "addresses": [
      "/ip4/192.168.1.100/tcp/4001",
      "/ip4/192.168.1.100/udp/4001/quic-v1"
    ],
    "node_mode": "full",
    "version": "0.1.0",
    "supported_protocols": [
      "/bib/discovery/1.0.0",
      "/bib/data/1.0.0",
      "/bib/jobs/1.0.0",
      "/bib/sync/1.0.0"
    ]
  }
}
```

#### announce / announce_ack

Announce new or updated data.

**Request:**
```json
{
  "type": "announce",
  "request_id": "uuid",
  "payload": {
    "new_entries": [...],
    "removed_hashes": ["sha256:old123..."]
  }
}
```

**Response:**
```json
{
  "type": "announce_ack",
  "request_id": "uuid",
  "payload": {
    "entries_received": 5
  }
}
```

---

## Data Protocol

**Protocol ID:** `/bib/data/1.0.0`

Handles dataset metadata and chunk transfers.

### Message Types

#### get_dataset_info / dataset_info

Request dataset metadata.

**Request:**
```json
{
  "type": "get_dataset_info",
  "request_id": "uuid",
  "payload": {
    "dataset_id": "weather-daily",
    "version_id": "v1.0.0"
  }
}
```

**Response:**
```json
{
  "type": "dataset_info",
  "request_id": "uuid",
  "payload": {
    "dataset": {
      "id": "weather-daily",
      "topic_id": "weather",
      "name": "Daily Weather",
      "status": "active",
      "has_content": true,
      "has_instructions": false
    },
    "version": {
      "id": "v1.0.0",
      "version": "1.0.0",
      "content": {
        "hash": "sha256:abc123...",
        "size": 10485760,
        "chunk_count": 10,
        "chunk_size": 1048576
      }
    }
  }
}
```

#### get_chunk / chunk

Request a single chunk.

**Request:**
```json
{
  "type": "get_chunk",
  "request_id": "uuid",
  "payload": {
    "dataset_id": "weather-daily",
    "version_id": "v1.0.0",
    "chunk_index": 0
  }
}
```

**Response:**
```json
{
  "type": "chunk",
  "request_id": "uuid",
  "payload": {
    "dataset_id": "weather-daily",
    "version_id": "v1.0.0",
    "index": 0,
    "hash": "sha256:chunk0...",
    "size": 1048576,
    "data": "<base64-encoded-data>"
  }
}
```

#### get_chunks / chunks

Request multiple chunks.

**Request:**
```json
{
  "type": "get_chunks",
  "request_id": "uuid",
  "payload": {
    "dataset_id": "weather-daily",
    "version_id": "v1.0.0",
    "chunk_indices": [0, 1, 2, 3]
  }
}
```

**Response:**
```json
{
  "type": "chunks",
  "request_id": "uuid",
  "payload": {
    "chunks": [
      {
        "index": 0,
        "hash": "sha256:chunk0...",
        "size": 1048576,
        "data": "<base64>"
      },
      ...
    ]
  }
}
```

---

## Sync Protocol

**Protocol ID:** `/bib/sync/1.0.0`

Handles state synchronization between nodes.

### Message Types

#### get_sync_status / sync_status

Request sync status.

**Request:**
```json
{
  "type": "get_sync_status",
  "request_id": "uuid",
  "payload": {}
}
```

**Response:**
```json
{
  "type": "sync_status",
  "request_id": "uuid",
  "payload": {
    "in_progress": false,
    "last_sync_time": "2024-01-15T10:30:00Z",
    "last_sync_error": "",
    "pending_entries": 0,
    "synced_entries": 150
  }
}
```

#### sync_state / sync_state_response

Push state update.

**Request:**
```json
{
  "type": "sync_state",
  "request_id": "uuid",
  "payload": {
    "state_type": "catalog",
    "version": 42,
    "entries": [...],
    "deleted": ["id1", "id2"]
  }
}
```

**Response:**
```json
{
  "type": "sync_state_response",
  "request_id": "uuid",
  "payload": {
    "accepted": true,
    "new_version": 43
  }
}
```

---

## Jobs Protocol

**Protocol ID:** `/bib/jobs/1.0.0`

Handles distributed job submission and status.

### Message Types

#### submit_job / job_accepted

Submit a job for execution.

**Request:**
```json
{
  "type": "submit_job",
  "request_id": "uuid",
  "payload": {
    "job": {
      "id": "job-123",
      "type": "transform",
      "task_id": "etl-daily",
      "inputs": [
        {"name": "source", "dataset_id": "raw-data"}
      ],
      "outputs": [
        {"name": "result", "dataset_id": "processed-data"}
      ],
      "priority": 5
    }
  }
}
```

**Response:**
```json
{
  "type": "job_accepted",
  "request_id": "uuid",
  "payload": {
    "accepted": true,
    "job_id": "job-123"
  }
}
```

#### get_job_status / job_status

Query job status.

**Request:**
```json
{
  "type": "get_job_status",
  "request_id": "uuid",
  "payload": {
    "job_id": "job-123"
  }
}
```

**Response:**
```json
{
  "type": "job_status",
  "request_id": "uuid",
  "payload": {
    "job_id": "job-123",
    "status": "running",
    "progress": 65,
    "current_instruction": 5,
    "started_at": "2024-01-15T10:30:00Z",
    "node_id": "node-abc"
  }
}
```

---

## PubSub Topics

GossipSub is used for real-time event broadcasting.

### Topics

| Topic | Purpose |
|-------|---------|
| `/bib/global` | Network-wide announcements |
| `/bib/nodes` | Node status updates |
| `/bib/topics/<topic-id>` | Topic-specific updates |

### Message Format

```json
{
  "type": "new_dataset",
  "sender_peer_id": "QmXyz...",
  "timestamp": "2024-01-15T10:30:00Z",
  "signature": "<base64-signature>",
  "payload": { ... }
}
```

### Message Types

| Type | Topic | Description |
|------|-------|-------------|
| `node_join` | `/bib/nodes` | Node joined network |
| `node_leave` | `/bib/nodes` | Node leaving network |
| `node_status` | `/bib/nodes` | Periodic status update |
| `new_topic` | `/bib/global` | New topic created |
| `new_dataset` | `/bib/topics/<id>` | New dataset published |
| `topic_update` | `/bib/topics/<id>` | Topic metadata updated |
| `delete_dataset` | `/bib/topics/<id>` | Dataset deleted |

### Node Status Payload

```json
{
  "peer_id": "QmXyz...",
  "node_mode": "full",
  "connected_peers": 15,
  "dataset_count": 42,
  "storage_used_bytes": 1073741824,
  "storage_total_bytes": 10737418240,
  "cpu_usage_percent": 25.5,
  "memory_usage_percent": 45.2,
  "active_jobs": 3,
  "uptime_since": "2024-01-01T00:00:00Z"
}
```

---

## Protocol Buffers

For production, messages will be encoded using Protocol Buffers. Proto definitions are in `api/proto/bib/v1/`:

| File | Contents |
|------|----------|
| `common.proto` | Shared types (PeerInfo, CatalogEntry, etc.) |
| `discovery.proto` | Discovery protocol messages |
| `data.proto` | Data transfer messages |
| `sync.proto` | Sync protocol messages |
| `jobs.proto` | Jobs protocol messages |
| `pubsub.proto` | PubSub message types |

### Example Proto Definition

```protobuf
// common.proto
syntax = "proto3";
package bib.v1;

message CatalogEntry {
  string topic_id = 1;
  string topic_name = 2;
  string dataset_id = 3;
  string dataset_name = 4;
  string hash = 5;
  int64 size = 6;
  int32 chunk_count = 7;
  google.protobuf.Timestamp updated_at = 8;
}

message Catalog {
  string peer_id = 1;
  repeated CatalogEntry entries = 2;
  google.protobuf.Timestamp last_updated = 3;
  uint64 version = 4;
}
```

---

## Error Codes

| Code | Meaning |
|------|---------|
| 400 | Bad request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not found |
| 408 | Timeout |
| 409 | Conflict |
| 429 | Rate limited |
| 500 | Internal error |
| 503 | Service unavailable |

---

## Versioning

Protocol IDs include version numbers for compatibility:

- `/bib/discovery/1.0.0` - Version 1.0.0
- `/bib/discovery/1.1.0` - Version 1.1.0 (future)

Nodes advertise supported protocol versions. During connection, the highest mutually supported version is negotiated.

