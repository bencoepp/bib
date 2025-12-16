# P2P Networking

This document describes the peer-to-peer networking layer of bib, built on libp2p.

## Overview

The bib P2P layer enables decentralized communication between nodes for:

- **Peer Discovery** - Finding other nodes in the network
- **Data Transfer** - Sharing datasets between peers
- **Real-time Updates** - Broadcasting changes via pub/sub
- **Job Distribution** - Coordinating distributed processing

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     P2P Manager                              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │    Host     │  │  Discovery  │  │   Mode Handler      │  │
│  │  (libp2p)   │  │   Manager   │  │ (Proxy/Sel/Full)    │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  Protocol   │  │   PubSub    │  │     Transfer        │  │
│  │  Handler    │  │ (GossipSub) │  │     Manager         │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Host

The `Host` wraps a libp2p host with bib-specific functionality.

### Features

- **Identity Management** - Ed25519 keypair for node identity
- **Transport** - TCP and QUIC transports
- **Security** - Noise protocol encryption
- **Multiplexing** - Yamux for stream multiplexing
- **NAT Traversal** - UPnP/NAT-PMP port mapping and hole punching

### Default Listen Addresses

```
/ip4/0.0.0.0/tcp/4001
/ip4/0.0.0.0/udp/4001/quic-v1
```

### Identity

Node identity is stored as a PEM-encoded Ed25519 private key:

```
~/.config/bibd/identity.pem
```

The identity is auto-generated on first run if not present.

---

## Discovery

The Discovery system combines multiple mechanisms for finding peers.

### Bootstrap

Bootstrap nodes provide initial entry points to the network:

```yaml
bootstrap:
  peers:
    - "/dns4/bib.dev/tcp/4001"
    - "/dns4/bib.dev/udp/4001/quic-v1"
  min_peers: 1
  retry_interval: 5s
  max_retry_interval: 1h
```

**Features:**
- Exponential backoff on connection failures
- Minimum peer threshold before continuing
- Configurable bootstrap list

### mDNS (Local Discovery)

Multicast DNS discovers peers on the local network:

```yaml
mdns:
  enabled: true
  service_name: "bib.local"
```

**Use Cases:**
- Development environments
- Air-gapped networks
- Low-latency local transfers

### DHT (Global Discovery)

Kademlia DHT for global peer and content discovery:

```yaml
dht:
  enabled: true
  mode: auto    # auto, server, client
```

**Modes:**
- `auto` - libp2p decides based on reachability
- `server` - Full DHT participant (requires public IP)
- `client` - Query-only, doesn't store records (works behind NAT)

**DHT Records:**
- Peer routing records
- Provider records for dataset availability
- Content routing for data location

### Peer Store

Persistent peer storage using SQLite:

```yaml
peer_store:
  path: ""   # defaults to ~/.config/bibd/peers.db
```

**Stored Information:**
- Peer IDs and multiaddrs
- Last seen timestamps
- Connection success/failure history
- Peer scoring/reputation

---

## Protocols

bib defines custom protocols for inter-node communication.

### Protocol IDs

| Protocol | ID | Purpose |
|----------|-----|---------|
| Discovery | `/bib/discovery/1.0.0` | Peer and catalog discovery |
| Data | `/bib/data/1.0.0` | Dataset transfers |
| Jobs | `/bib/jobs/1.0.0` | Job distribution |
| Sync | `/bib/sync/1.0.0` | State synchronization |

### Message Format

Messages use JSON encoding (migrating to Protobuf):

```json
{
  "type": "get_catalog",
  "request_id": "uuid",
  "payload": { ... },
  "error": null
}
```

### Discovery Protocol

Handles catalog and peer information exchange:

| Message | Description |
|---------|-------------|
| `get_catalog` | Request peer's full catalog |
| `catalog` | Return catalog entries |
| `query_catalog` | Query for specific entries |
| `query_result` | Return matching entries |
| `get_peer_info` | Request peer information |
| `peer_info` | Return peer details |
| `announce` | Announce new data availability |
| `announce_ack` | Acknowledge announcement |

### Data Protocol

Handles dataset transfers:

| Message | Description |
|---------|-------------|
| `get_dataset_info` | Request dataset metadata |
| `dataset_info` | Return dataset metadata |
| `get_chunk` | Request a single chunk |
| `chunk` | Return chunk data |
| `get_chunks` | Request multiple chunks |
| `chunks` | Return multiple chunks |

### Sync Protocol

Handles state synchronization:

| Message | Description |
|---------|-------------|
| `get_sync_status` | Request sync status |
| `sync_status` | Return sync status |
| `sync_state` | Push state update |
| `sync_state_response` | Acknowledge state update |

### Jobs Protocol

Handles distributed job execution:

| Message | Description |
|---------|-------------|
| `submit_job` | Submit job for execution |
| `job_accepted` | Job acceptance confirmation |
| `get_job_status` | Query job status |
| `job_status` | Return job status |

---

## PubSub

GossipSub provides real-time pub/sub messaging.

### Topics

| Topic | Purpose |
|-------|---------|
| `/bib/global` | Network-wide announcements |
| `/bib/nodes` | Node status updates |
| `/bib/topics/<topic-id>` | Topic-specific updates |

### Message Types

| Type | Description |
|------|-------------|
| `node_join` | Node joined the network |
| `node_leave` | Node left the network |
| `node_status` | Node status update (periodic) |
| `new_topic` | New topic created |
| `new_dataset` | New dataset published |
| `topic_update` | Topic metadata updated |
| `delete_dataset` | Dataset deleted |

### Message Format

```json
{
  "type": "new_dataset",
  "sender_peer_id": "QmXyz...",
  "timestamp": "2024-01-15T10:30:00Z",
  "signature": "...",
  "payload": {
    "topic_id": "weather",
    "entry": { ... }
  }
}
```

### Configuration

```yaml
# Implicit via P2P config
# Message signing is always enabled
# Strict signature validation required
# Max message size: 1MB
# Message TTL: 5 minutes
```

---

## Data Transfer

The Transfer Manager handles efficient dataset downloads.

### Features

- **Chunked Transfer** - Large files split into chunks
- **Resumable Downloads** - Bitmap tracks completed chunks
- **Parallel Downloads** - Fetch from multiple peers
- **Integrity Verification** - SHA-256 hash per chunk and dataset

### Configuration

```yaml
# TransferConfig (internal defaults)
chunk_size: 1048576         # 1MB
max_concurrent_chunks: 4
chunk_timeout: 30s
max_retries: 3
parallel_peers: true
```

### Download Flow

```
1. Identify peers with dataset (via catalog)
2. Get dataset metadata (chunk count, hashes)
3. Initialize download with bitmap
4. Request chunks (parallel from multiple peers)
5. Verify each chunk hash
6. Mark chunk complete in bitmap
7. Assemble chunks
8. Verify final hash
9. Store dataset
```

### Resumable Downloads

Downloads track progress with a bitmap:

```go
type Download struct {
    ID              string
    DatasetID       DatasetID
    DatasetHash     string
    PeerID          string
    TotalChunks     int
    CompletedChunks int
    ChunkBitmap     []byte   // Bit per chunk
    Status          DownloadStatus
}
```

**Status Values:** `active`, `paused`, `completed`, `failed`

---

## Node Modes

The Mode Manager handles mode-specific behavior.

### Proxy Mode (Default)

No local storage, forwards requests to peers.

```yaml
p2p:
  mode: proxy
  proxy:
    cache_ttl: 2m
    max_cache_size: 1000
    favorite_peers: []
```

**Behavior:**
- Queries forwarded to connected peers
- Results cached temporarily
- Minimal resource usage
- No persistent data

### Selective Mode

Subscribe to specific topics on-demand.

```yaml
p2p:
  mode: selective
  selective:
    subscriptions:
      - "weather/*"
      - "finance/stocks"
    subscription_store_path: ""
```

**Behavior:**
- Only sync subscribed topics
- Partial local storage
- Subscriptions persist across restarts
- On-demand data fetch

### Full Mode

Replicate all data from connected peers.

```yaml
p2p:
  mode: full
  full_replica:
    sync_interval: 5m
```

**Behavior:**
- Continuous sync of all topics
- Complete local storage
- Serve as data provider
- Higher resource usage

### Mode Switching

Modes can be switched at runtime via configuration reload:

```bash
# Edit config file
vim ~/.config/bibd/config.yaml

# bibd detects changes and reloads
# Or send SIGHUP
kill -HUP $(cat /var/run/bibd.pid)
```

---

## Connection Management

The connection manager maintains peer connections.

### Configuration

```yaml
connection_manager:
  low_watermark: 100      # Keep at least this many
  high_watermark: 400     # Start pruning above this
  grace_period: 30s       # Protect new connections
```

### Pruning Strategy

When connections exceed `high_watermark`:

1. Identify candidates (beyond grace period)
2. Score by usefulness (data availability, latency, uptime)
3. Prune lowest-scored connections
4. Stop at `low_watermark`

---

## NAT Traversal

bib uses multiple techniques for NAT traversal:

- **UPnP** - Automatic port mapping if router supports it
- **NAT-PMP** - Alternative to UPnP
- **Hole Punching** - Direct connections through NAT
- **QUIC** - Better NAT traversal than TCP
- **Relay** - Fallback via circuit relay (if enabled)

---

## Security

### Transport Encryption

All connections use Noise protocol encryption:

- Forward secrecy
- Identity binding
- No certificates required

### Message Signing

PubSub messages are signed with the sender's private key:

- Prevents impersonation
- Ensures message integrity
- Rejects unsigned messages (strict mode)

### Peer Authentication

Peers are identified by their libp2p Peer ID:

- Derived from public key
- Cryptographically verifiable
- Consistent across connections

