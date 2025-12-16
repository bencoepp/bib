# Node Modes

This document explains the three operational modes for bibd nodes: **Proxy**, **Selective**, and **Full**.

## Overview

Node modes determine how a bibd daemon participates in the network, particularly regarding data storage and synchronization.

| Mode | Local Storage | Sync Behavior | Resource Usage |
|------|--------------|---------------|----------------|
| **Proxy** | None (cache only) | Pass-through | Low |
| **Selective** | Partial | On-demand | Medium |
| **Full** | Complete | Continuous | High |

## Mode Selection

Choose your mode based on:

- **Available disk space**
- **Network bandwidth**
- **Use case requirements**
- **Desired data availability**

---

## Proxy Mode

**Default mode.** The node acts as a lightweight gateway with no persistent storage.

### Configuration

```yaml
p2p:
  mode: proxy
  proxy:
    cache_ttl: 2m           # How long to cache results
    max_cache_size: 1000    # Maximum cache entries
    favorite_peers: []      # Preferred peers for forwarding
```

### Behavior

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Client    │────────►│  Proxy Node │────────►│  Full Node  │
│             │◄────────│   (bibd)    │◄────────│   (bibd)    │
└─────────────┘         └─────────────┘         └─────────────┘
     request               forward              has data
     response              cache + return
```

1. Client sends query to proxy node
2. Proxy forwards request to peers (preferring favorite_peers)
3. Peer responds with data
4. Proxy caches result (up to cache_ttl)
5. Subsequent requests served from cache

### Use Cases

- **Development/testing** - Quick setup, no storage needed
- **Edge gateways** - Lightweight entry points to the network
- **Resource-constrained environments** - Limited disk/memory
- **Temporary access** - No data persistence needed

### Advantages

- ✅ Minimal resource usage
- ✅ No storage management
- ✅ Fast startup
- ✅ Easy to deploy

### Limitations

- ❌ Cannot serve data to other peers
- ❌ Depends on connected peers for all data
- ❌ Cache-only, no persistence
- ❌ Higher latency (network round-trip)

---

## Selective Mode

Subscribe to specific topics and sync only that data locally.

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

### Behavior

```
┌─────────────────────────────────────────────────────────────┐
│                     Selective Node                           │
├─────────────────────────────────────────────────────────────┤
│  Subscribed Topics:                                          │
│  ┌──────────┐ ┌─────────────────┐ ┌────────────────────┐    │
│  │ weather  │ │ finance/stocks  │ │ research/papers/*  │    │
│  │  (local) │ │     (local)     │ │      (local)       │    │
│  └──────────┘ └─────────────────┘ └────────────────────┘    │
│                                                              │
│  Non-subscribed: Forwarded to peers (like proxy mode)       │
└─────────────────────────────────────────────────────────────┘
```

1. On startup, load subscriptions from store
2. Connect to peers and fetch subscribed topic catalogs
3. Sync datasets for subscribed topics
4. Listen for updates via PubSub
5. Auto-download new datasets in subscribed topics
6. Non-subscribed queries forwarded to peers

### Subscription Patterns

| Pattern | Matches |
|---------|---------|
| `weather` | Only the "weather" topic |
| `weather/*` | weather and all sub-topics |
| `finance/stocks` | Only finance/stocks |
| `*/papers` | Any topic ending in /papers |

### Managing Subscriptions

```bash
# Add subscription (future CLI)
bib subscribe weather/*

# Remove subscription
bib unsubscribe weather/*

# List subscriptions
bib subscriptions list
```

Currently, subscriptions are managed via config file.

### Use Cases

- **Domain-specific nodes** - Only interested in certain data
- **Resource-balanced** - Some local storage, not everything
- **Team/project nodes** - Sync data relevant to team
- **Caching layers** - Persistent cache for specific topics

### Advantages

- ✅ Local access to subscribed data
- ✅ Can serve subscribed data to peers
- ✅ Controlled resource usage
- ✅ Fast queries for subscribed topics

### Limitations

- ❌ Need to manage subscriptions
- ❌ Non-subscribed data requires network
- ❌ Partial data availability

---

## Full Mode

Replicate all available data from connected peers.

### Configuration

```yaml
p2p:
  mode: full
  full_replica:
    sync_interval: 5m    # How often to poll for new data
```

### Behavior

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
│  Continuous sync from all connected peers                     │
│  Serves as data provider for proxy/selective nodes            │
└───────────────────────────────────────────────────────────────┘
```

1. On startup, fetch catalogs from all peers
2. Begin syncing all datasets not already local
3. Poll peers at sync_interval for new data
4. Listen for PubSub announcements
5. Immediately fetch newly announced datasets
6. Serve data to any requesting peer

### Use Cases

- **Bootstrap nodes** - Provide data to network newcomers
- **Archive nodes** - Complete data preservation
- **High-availability setups** - Full redundancy
- **Analysis nodes** - Need access to all data

### Advantages

- ✅ Complete local data
- ✅ No network latency for queries
- ✅ Serves all data to peers
- ✅ Works offline after sync
- ✅ Ideal for HA clusters

### Limitations

- ❌ High disk usage
- ❌ High bandwidth (initial sync)
- ❌ Longer startup time
- ❌ Not suitable for constrained environments

---

## Mode Comparison

### Resource Usage

```
              Disk     Memory    Bandwidth    CPU
Proxy:        [  ]     [=]       [=]          [=]
Selective:    [==]     [==]      [==]         [==]
Full:         [=====]  [===]     [====]       [===]
```

### Query Latency

```
Query Type        Proxy       Selective     Full
─────────────────────────────────────────────────
Local data:       N/A         ~1ms          ~1ms
Subscribed:       N/A         ~1ms          ~1ms
Non-subscribed:   100-500ms   100-500ms     ~1ms
Cache hit:        ~1ms        N/A           N/A
```

### Data Availability

| Scenario | Proxy | Selective | Full |
|----------|-------|-----------|------|
| Network available | ✅ | ✅ | ✅ |
| Network down | ❌ | Subscribed only | ✅ |
| New peer joins | Cannot serve | Partial | Full |

---

## Switching Modes

Modes can be changed by updating configuration:

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
| Proxy | Selective | Begin syncing subscribed topics |
| Proxy | Full | Begin full sync |
| Selective | Proxy | Stop sync, clear local data (optional) |
| Selective | Full | Begin syncing all remaining topics |
| Full | Selective | Stop syncing non-subscribed, keep existing |
| Full | Proxy | Stop sync, clear local data (optional) |

**Note:** Switching modes doesn't automatically delete local data. Use explicit cleanup commands if needed.

---

## Recommendations

### Single User / Development

```yaml
p2p:
  mode: proxy
```

Start with proxy mode, switch to selective as you identify useful topics.

### Team / Project

```yaml
p2p:
  mode: selective
  selective:
    subscriptions:
      - "myproject/*"
      - "shared-data/*"
```

Subscribe to project-relevant topics.

### Organization / Production

```yaml
p2p:
  mode: full
  full_replica:
    sync_interval: 1m
```

Full replication with HA clustering for reliability.

### Mixed Deployment

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

- **Core:** Full nodes in HA cluster for reliability
- **Teams:** Selective nodes for domain-specific access
- **Edge:** Proxy nodes for lightweight access points

