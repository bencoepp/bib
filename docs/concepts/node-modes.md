# Node Modes

This document explains the three operational modes for bibd nodes: **Proxy**, **Selective**, and **Full**. Each mode offers different trade-offs between resource usage, data availability, and network participation.

> 📘 **Setup Wizard:** Mode selection is part of the `bib setup --daemon` wizard. See [Setup Flow](../getting-started/setup-flow.md) for the complete setup process, including mode-specific configuration steps.

---

## Table of Contents

1. [Overview](#overview)
2. [Proxy Mode](#proxy-mode)
3. [Selective Mode](#selective-mode)
4. [Full Mode](#full-mode)
5. [Mode Comparison](#mode-comparison)
6. [Switching Modes](#switching-modes)
7. [Deployment Recommendations](#deployment-recommendations)

---

## Overview

Node modes determine how a bibd daemon participates in the network, particularly regarding data storage and synchronization.

| Mode | Local Storage | Sync Behavior | Resource Usage | Best For |
|------|--------------|---------------|----------------|----------|
| **Proxy** | None (cache only) | Pass-through | Low | Development, gateways |
| **Selective** | Partial | On-demand | Medium | Team nodes, domains |
| **Full** | Complete | Continuous | High | Archive, HA clusters |

### Mode Selection Criteria

Choose your mode based on:

- **Available disk space** — Full mode requires storage for all network data
- **Network bandwidth** — Full and selective modes use more bandwidth
- **Use case requirements** — Do you need offline access? Serve data to others?
- **Desired data availability** — How important is local data access?

---

## Proxy Mode

**Proxy mode** is the default. The node acts as a lightweight gateway with no persistent storage.

### Configuration

```yaml
p2p:
  mode: proxy
  proxy:
    cache_ttl: 2m           # How long to cache results
    max_cache_size: 1000    # Maximum cache entries
    favorite_peers: []      # Preferred peers for forwarding
```

### How It Works

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Client    │────────►│  Proxy Node │────────►│  Full Node  │
│             │◄────────│   (bibd)    │◄────────│   (bibd)    │
└─────────────┘         └─────────────┘         └─────────────┘
     request               forward              has data
     response              cache + return
```

**Request Flow:**

1. Client sends query to proxy node
2. Proxy checks in-memory cache
3. If cache miss, forwards request to peers (preferring `favorite_peers`)
4. Peer responds with data
5. Proxy caches result (up to `cache_ttl`)
6. Subsequent requests served from cache until TTL expires

### Use Cases

| Use Case | Why Proxy Mode |
|----------|---------------|
| **Development/testing** | Quick setup, no storage needed |
| **Edge gateways** | Lightweight entry points to the network |
| **Resource-constrained** | Limited disk space or memory |
| **Temporary access** | No data persistence needed |
| **API endpoints** | Stateless request handling |

### Advantages

| Advantage | Description |
|-----------|-------------|
| ✅ Minimal resources | Very low disk, memory, and CPU usage |
| ✅ No storage management | No database maintenance required |
| ✅ Fast startup | No data sync delay on startup |
| ✅ Easy deployment | Simplest configuration |
| ✅ Stateless | Easy to scale horizontally |

### Limitations

| Limitation | Description |
|------------|-------------|
| ❌ Cannot serve data | Other peers can't fetch data from proxy nodes |
| ❌ Network dependent | Requires connected peers for all data access |
| ❌ Cache-only | No persistence across restarts |
| ❌ Higher latency | Network round-trip for cache misses |
| ❌ No offline access | Completely non-functional without network |

### Configuration Details

#### cache_ttl

Duration to keep cached entries. Shorter TTL = more network requests but fresher data.

```yaml
proxy:
  cache_ttl: 2m    # Default: 2 minutes
  # cache_ttl: 30s  # For frequently changing data
  # cache_ttl: 1h   # For stable data
```

#### max_cache_size

Maximum number of cache entries. When full, oldest entries are evicted (LRU).

```yaml
proxy:
  max_cache_size: 1000    # Default: 1000 entries
  # max_cache_size: 5000  # For high-traffic nodes
```

#### favorite_peers

Preferred peers for forwarding requests. Useful when you know which peers have the data.

```yaml
proxy:
  favorite_peers:
    - "QmXyz123..."    # Peer IDs
    - "QmAbc456..."
```

---

## Selective Mode

**Selective mode** subscribes to specific topics and syncs only that data locally.

### Configuration

```yaml
p2p:
  mode: selective
  selective:
    subscriptions:
      - "weather/*"              # All weather sub-topics
      - "finance/stocks"         # Specific topic
      - "research/papers/2024"   # Specific sub-path
    subscription_store_path: ""  # Persisted to config dir
```

### How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                     Selective Node                           │
├─────────────────────────────────────────────────────────────┤
│  Subscribed Topics (stored locally):                         │
│  ┌──────────┐ ┌─────────────────┐ ┌────────────────────┐    │
│  │ weather  │ │ finance/stocks  │ │ research/papers/*  │    │
│  │  (synced)│ │     (synced)    │ │      (synced)      │    │
│  └──────────┘ └─────────────────┘ └────────────────────┘    │
│                                                              │
│  Non-subscribed Topics:                                      │
│  → Forwarded to peers (like proxy mode)                     │
└─────────────────────────────────────────────────────────────┘
```

**Sync Behavior:**

1. On startup, load subscriptions from persistent store
2. Connect to peers and fetch subscribed topic catalogs
3. Sync all datasets for subscribed topics
4. Listen for real-time updates via PubSub
5. Auto-download new datasets in subscribed topics
6. Non-subscribed queries forwarded to peers (proxy behavior)

### Subscription Patterns

| Pattern | Matches | Example |
|---------|---------|---------|
| `weather` | Only the exact "weather" topic | ✓ weather, ✗ weather/daily |
| `weather/*` | weather and all sub-topics | ✓ weather, ✓ weather/daily, ✓ weather/hourly |
| `finance/stocks` | Only finance/stocks | ✓ finance/stocks, ✗ finance/bonds |
| `*/papers` | Any topic ending in /papers | ✓ research/papers, ✓ science/papers |
| `*` | All top-level topics | ✓ weather, ✓ finance, ✗ weather/daily |

### Managing Subscriptions

```bash
# Add subscription (CLI)
bib subscribe add "weather/*"

# Remove subscription
bib subscribe remove "weather/*"

# List subscriptions
bib subscribe list
```

Currently, subscriptions can also be managed via config file.

### Use Cases

| Use Case | Why Selective Mode |
|----------|-------------------|
| **Domain-specific nodes** | Only need certain data categories |
| **Team/project nodes** | Sync data relevant to your team |
| **Resource-balanced** | Some local storage without everything |
| **Persistent cache** | Keep frequently accessed data local |
| **Regional nodes** | Sync region-specific data |

### Advantages

| Advantage | Description |
|-----------|-------------|
| ✅ Local access | Fast queries for subscribed data |
| ✅ Can serve data | Subscribed data available to other peers |
| ✅ Controlled resources | Predictable storage requirements |
| ✅ Offline access | Subscribed data available without network |
| ✅ Real-time updates | Auto-sync new data in subscribed topics |

### Limitations

| Limitation | Description |
|------------|-------------|
| ❌ Subscription management | Need to maintain subscription list |
| ❌ Network for non-subscribed | Still need peers for other data |
| ❌ Partial availability | Can't serve data you haven't subscribed to |
| ❌ Sync delay | Initial sync takes time for large topics |

### Configuration Details

#### subscriptions

List of topic patterns to subscribe to.

```yaml
selective:
  subscriptions:
    - "weather/*"
    - "finance/stocks"
    - "research/papers/2024"
```

#### subscription_store_path

Path to persist subscriptions. Defaults to config directory.

```yaml
selective:
  subscription_store_path: "/var/lib/bibd/subscriptions.json"
```

---

## Full Mode

**Full mode** replicates all available data from connected peers.

### Configuration

```yaml
p2p:
  mode: full
  full_replica:
    sync_interval: 5m    # How often to poll for new data
```

### How It Works

```
┌───────────────────────────────────────────────────────────────┐
│                        Full Node                               │
├───────────────────────────────────────────────────────────────┤
│  Complete replica of all topics and datasets                  │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                    All Topics                            │  │
│  │  weather  finance  research  sports  news  ...          │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                               │
│  • Continuous sync from all connected peers                   │
│  • Serves as data provider for proxy/selective nodes          │
│  • Maintains complete catalog of all network data             │
└───────────────────────────────────────────────────────────────┘
```

**Sync Behavior:**

1. On startup, fetch catalogs from all connected peers
2. Begin syncing all datasets not already stored locally
3. Poll peers at `sync_interval` for new data
4. Listen for PubSub announcements for immediate updates
5. Immediately fetch newly announced datasets
6. Serve data to any requesting peer

### Use Cases

| Use Case | Why Full Mode |
|----------|--------------|
| **Bootstrap nodes** | Provide data to network newcomers |
| **Archive nodes** | Complete data preservation |
| **HA clusters** | Full redundancy across nodes |
| **Analysis platforms** | Need access to all data |
| **Compliance** | Complete audit trail requirements |

### Advantages

| Advantage | Description |
|-----------|-------------|
| ✅ Complete data | All network data available locally |
| ✅ Zero network latency | All queries served from local storage |
| ✅ Full participation | Serve any data to any peer |
| ✅ Offline capable | Works completely offline after sync |
| ✅ Ideal for HA | Perfect for high-availability clusters |

### Limitations

| Limitation | Description |
|------------|-------------|
| ❌ High disk usage | Storage grows with network size |
| ❌ High initial bandwidth | Complete sync can be very large |
| ❌ Longer startup | Initial sync delays availability |
| ❌ Not for constrained environments | Needs substantial resources |
| ❌ PostgreSQL required | SQLite not supported for full mode |

### Configuration Details

#### sync_interval

How often to poll peers for new data (in addition to real-time PubSub).

```yaml
full_replica:
  sync_interval: 5m    # Default: 5 minutes
  # sync_interval: 1m  # For more aggressive sync
  # sync_interval: 15m # For reduced bandwidth
```

### Storage Requirements

Full mode requires PostgreSQL backend:

```yaml
database:
  backend: postgres    # Required for full mode
  postgres:
    managed: true
```

SQLite is not supported for full mode and will cause a startup error.

---

## Mode Comparison

### Resource Usage

| Resource | Proxy | Selective | Full |
|----------|-------|-----------|------|
| **Disk** | ~0 | Variable (subscriptions) | Network size |
| **Memory** | ~50 MB | ~100-500 MB | ~500 MB - 2 GB |
| **Bandwidth** | On-demand | Moderate (subscribed) | High (continuous) |
| **CPU** | Low | Low-Medium | Medium |

**Visual Comparison:**

```
              Disk     Memory    Bandwidth    CPU
Proxy:        [  ]     [=]       [=]          [=]
Selective:    [==]     [==]      [==]         [==]
Full:         [=====]  [===]     [====]       [===]
```

### Query Latency

| Query Type | Proxy | Selective | Full |
|------------|-------|-----------|------|
| **Local data** | N/A | ~1ms | ~1ms |
| **Subscribed data** | N/A | ~1ms | ~1ms |
| **Non-subscribed data** | 100-500ms | 100-500ms | ~1ms |
| **Cache hit** | ~1ms | N/A | N/A |

### Data Availability

| Scenario | Proxy | Selective | Full |
|----------|-------|-----------|------|
| Network available | ✅ All data | ✅ All data | ✅ All data |
| Network down | ❌ None | ⚠️ Subscribed only | ✅ All data |
| Can serve to peers | ❌ No | ⚠️ Subscribed only | ✅ All data |
| New peer joins | ❌ Cannot help | ⚠️ Partial | ✅ Full provider |

### Backend Support

| Backend | Proxy | Selective | Full |
|---------|-------|-----------|------|
| **SQLite** | ✅ Supported | ⚠️ Cache only | ❌ Not supported |
| **PostgreSQL** | ✅ Supported | ✅ Full support | ✅ Required |

---

## Switching Modes

Modes can be changed by updating configuration and restarting bibd.

### Configuration Change

```yaml
# Before (proxy mode)
p2p:
  mode: proxy

# After (selective mode)
p2p:
  mode: selective
  selective:
    subscriptions:
      - "weather/*"
```

### Mode Transition Behavior

| From | To | What Happens |
|------|-----|--------------|
| Proxy → Selective | Begin syncing subscribed topics |
| Proxy → Full | Begin full sync (can be slow) |
| Selective → Proxy | Stop sync; local data remains |
| Selective → Full | Begin syncing all remaining topics |
| Full → Selective | Stop syncing non-subscribed; data remains |
| Full → Proxy | Stop sync; local data remains |

> **Note:** Switching modes does not automatically delete local data. Use explicit cleanup commands if you want to free disk space.

### Cleaning Up After Mode Change

```bash
# Future command to clean non-subscribed data
bib storage cleanup --keep-subscribed

# Future command to clean all local data
bib storage cleanup --all
```

---

## Deployment Recommendations

### Single User / Development

```yaml
p2p:
  mode: proxy
```

Start with proxy mode for minimal setup. Switch to selective as you identify useful topics.

### Team / Project

```yaml
p2p:
  mode: selective
  selective:
    subscriptions:
      - "myproject/*"
      - "shared-data/*"
```

Subscribe to project-relevant topics for local access and reduced latency.

### Organization / Production

```yaml
p2p:
  mode: full
  full_replica:
    sync_interval: 1m

cluster:
  enabled: true
```

Full replication with HA clustering for maximum reliability.

### Mixed Deployment Architecture

A typical production deployment combines all three modes:

```
┌─────────────────────────────────────────────────────────────┐
│  Recommended Architecture                                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │  Full    │  │  Full    │  │  Full    │  ◄── HA Cluster  │
│  │  Node    │  │  Node    │  │  Node    │      (3+ nodes)  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                  │
│       │             │             │                          │
│       └─────────────┼─────────────┘                          │
│                     │                                        │
│       ┌─────────────┼─────────────┐                          │
│       │             │             │                          │
│  ┌────┴────┐  ┌────┴────┐  ┌────┴────┐                      │
│  │Selective│  │Selective│  │  Proxy  │  ◄── Edge nodes     │
│  │ (Team A)│  │ (Team B)│  │(Gateway)│                      │
│  └─────────┘  └─────────┘  └─────────┘                      │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

| Tier | Mode | Purpose |
|------|------|---------|
| **Core** | Full (3+ nodes) | HA cluster for reliability and complete data |
| **Team** | Selective | Domain-specific access with local storage |
| **Edge** | Proxy | Lightweight access points and gateways |

---

## Related Documentation

| Document | Topic |
|----------|-------|
| [Configuration](../getting-started/configuration.md) | Mode configuration options |
| [Clustering](../guides/clustering.md) | HA cluster with full nodes |
| [P2P Networking](../networking/p2p-networking.md) | Network layer details |
| [Storage Lifecycle](../storage/storage-lifecycle.md) | Backend management |
