# Clustering Guide

This document explains how to set up a high-availability (HA) cluster of bibd nodes using Raft consensus.

## Overview

Bib supports optional HA clustering using the Raft consensus algorithm. A cluster provides:

- **Leader Election** - Automatic failover if the leader fails
- **State Replication** - Consistent catalog, job state, and configuration across nodes
- **Fault Tolerance** - Continued operation with minority node failures

## Requirements

### Minimum Nodes

A Raft cluster requires a **minimum of 3 voting nodes** for proper quorum:

| Cluster Size | Tolerated Failures | Quorum |
|--------------|-------------------|--------|
| 3 nodes | 1 | 2 |
| 5 nodes | 2 | 3 |
| 7 nodes | 3 | 4 |

### Network

- Nodes must be able to reach each other on the Raft port (default: 4002)
- Low-latency network recommended (< 50ms RTT)
- Stable network preferred for consensus

---

## Quick Start

### Initialize First Node (Bootstrap)

```bash
bib setup --daemon --cluster
```

This will:
1. Configure the bibd daemon
2. Enable clustering
3. Generate a join token for other nodes

**Output:**
```
✓ Configuration saved to: ~/.config/bibd/config.yaml

=== Cluster Initialized ===
Node ID: a1b2c3d4e5f6...
Cluster Name: bib-cluster

Share this join token with other nodes:

  eyJjbHVzdGVyX25hbWUiOiJiaWItY2x1c3RlciIsImxlYWRlcl9hZGRyIjoiMC4wLjAuMDo0MDAyIiwidG9rZW4iOiIuLi4iLCJleHBpcmVzX2F0IjoxNzA1MzI0MDAwfQ==

To join another node to this cluster, run:
  bib setup --daemon --cluster-join <token>

NOTE: A minimum of 3 voting nodes is required for HA quorum.
```

### Join Additional Nodes

On each additional node:

```bash
bib setup --daemon --cluster-join <join-token>
```

This will:
1. Configure the bibd daemon
2. Configure it to join the existing cluster
3. Set up Raft with the leader address

### Start the Cluster

Start bibd on each node:

```bash
# On each node
bibd
```

The first node (bootstrap) becomes the initial leader. Other nodes join as followers.

---

## Configuration

### Cluster Section

```yaml
cluster:
  # Enable HA clustering
  enabled: true
  
  # Unique node identifier (auto-generated if empty)
  node_id: "a1b2c3d4e5f6..."
  
  # Cluster name for discovery
  cluster_name: "bib-cluster"
  
  # Raft data directory
  data_dir: "~/.config/bibd/raft"
  
  # Raft communication address
  listen_addr: "0.0.0.0:4002"
  
  # Address advertised to other nodes
  advertise_addr: "192.168.1.100:4002"
  
  # Can this node vote in leader elections?
  is_voter: true
  
  # Is this the initial cluster node?
  bootstrap: false
  
  # Token for joining existing cluster
  join_token: ""
  
  # Alternative: direct addresses of existing members
  join_addrs:
    - "192.168.1.100:4002"
    - "192.168.1.101:4002"
  
  # Discover cluster via DHT (experimental)
  enable_dht_discovery: false
```

### Raft Tuning

```yaml
cluster:
  raft:
    # Leader heartbeat interval
    heartbeat_timeout: 1s
    
    # Election timeout (must be > heartbeat)
    election_timeout: 5s
    
    # Log commit timeout
    commit_timeout: 50ms
    
    # Maximum entries per append RPC
    max_append_entries: 64
    
    # Logs to keep after snapshot
    trailing_logs: 10000
    
    # Maximum in-flight append entries
    max_inflight: 256
```

### Snapshot Configuration

```yaml
cluster:
  snapshot:
    # Automatic snapshot interval
    interval: 30m
    
    # Take snapshot after this many log entries
    threshold: 8192
    
    # Number of snapshots to retain
    retain_count: 3
```

---

## Node Roles

### Voters

Voters participate in leader elections and log replication.

```yaml
cluster:
  is_voter: true
```

- Can become leader
- Contribute to quorum
- Receive all log entries
- Required for cluster health

### Non-Voters

Non-voters replicate data but don't participate in consensus.

```yaml
cluster:
  is_voter: false
```

- Cannot become leader
- Don't count toward quorum
- Receive all log entries
- Useful for read replicas

**Use Cases for Non-Voters:**
- Geographic distribution (read-only replicas)
- Learning nodes (warming up before becoming voter)
- Backup nodes (disaster recovery)

---

## Cluster Operations

### Check Cluster Status

```bash
# Future CLI command
bib cluster status
```

**Status Information:**
```
Cluster: bib-cluster
State: healthy
Leader: node-1 (192.168.1.100:4002)
Term: 42
Commit Index: 12345

Members:
  node-1  192.168.1.100:4002  voter    leader    healthy
  node-2  192.168.1.101:4002  voter    follower  healthy
  node-3  192.168.1.102:4002  voter    follower  healthy

Quorum: 2/3 (healthy)
```

### Add New Node

1. Generate join token on leader:
```bash
# Future CLI command
bib cluster token generate
```

2. Join on new node:
```bash
bib setup --daemon --cluster-join <token>
bibd
```

### Remove Node

```bash
# Future CLI command
bib cluster remove <node-id>
```

### Promote Non-Voter

```bash
# Future CLI command
bib cluster promote <node-id>
```

### Demote Voter

```bash
# Future CLI command
bib cluster demote <node-id>
```

---

## State Replication

The following state is replicated across the cluster via Raft:

### Replicated State

| State | Description |
|-------|-------------|
| Catalog | Dataset metadata and availability |
| Jobs | Job definitions and status |
| Tasks | Reusable task definitions |
| Topics | Topic metadata and schemas |
| Configuration | Cluster-wide settings |

### Non-Replicated State

| State | Description |
|-------|-------------|
| Dataset Content | Actual data (handled by P2P) |
| Node Configuration | Per-node settings |
| Metrics | Local performance data |

### FSM (Finite State Machine)

The Raft FSM applies committed log entries to local state:

```
Log Entry ──► FSM.Apply() ──► Update State
                 │
                 ├── Catalog changes
                 ├── Job state changes
                 └── Configuration changes
```

---

## Failure Handling

### Leader Failure

1. Followers detect missing heartbeats
2. After election_timeout, followers become candidates
3. Candidate requests votes from other nodes
4. First to get quorum becomes new leader
5. New leader begins accepting writes

**Typical failover time:** 5-15 seconds

### Follower Failure

- No immediate impact on cluster
- Leader continues with remaining quorum
- Failed node rejoins and catches up when recovered

### Network Partition

**With Quorum:**
- Majority partition continues operating
- Minority partition becomes read-only

**Without Quorum:**
- No writes accepted
- Reads may still work (depending on consistency mode)
- Cluster heals when partition resolves

### Split-Brain Protection

If leader loses quorum:
1. Leader steps down
2. Stops accepting writes
3. Waits for quorum restoration

---

## Best Practices

### Deployment

1. **Use odd numbers of voters** (3, 5, 7)
2. **Distribute across failure domains** (different racks, zones)
3. **Stable network** - Avoid high-latency links
4. **Dedicated disk** - Raft logs benefit from fast storage

### Configuration

1. **Set proper advertise_addr** - Must be reachable by other nodes
2. **Tune timeouts** - Increase for high-latency networks
3. **Regular snapshots** - Prevent log unbounded growth

### Monitoring

Monitor these metrics:

| Metric | Healthy Range |
|--------|---------------|
| Leader elections | Rare (< 1/hour) |
| Log entries behind | < 100 |
| Snapshot interval | Regular |
| Peer latency | < 50ms |

### Upgrades

1. Upgrade non-voters first
2. Upgrade followers one at a time
3. Upgrade leader last
4. Allow catch-up between upgrades

---

## Troubleshooting

### Node Won't Join

**Symptoms:** Node fails to join cluster

**Causes:**
- Join token expired (24-hour validity)
- Network connectivity issues
- Firewall blocking port 4002
- Incorrect advertise_addr

**Solutions:**
- Generate new join token
- Check network connectivity
- Open firewall port
- Verify advertise_addr is reachable

### Frequent Leader Elections

**Symptoms:** Leader changes frequently

**Causes:**
- Network instability
- Leader overloaded
- Timeouts too aggressive

**Solutions:**
- Improve network stability
- Add resources to leader
- Increase election_timeout

### Log Growing Unbounded

**Symptoms:** Raft data directory growing continuously

**Causes:**
- Snapshots not happening
- Snapshot threshold too high

**Solutions:**
- Check snapshot configuration
- Lower snapshot threshold
- Trigger manual snapshot

### Split-Brain

**Symptoms:** Multiple nodes claim leadership

**Causes:**
- Network partition
- Misconfigured timeouts

**Solutions:**
- Fix network partition
- Increase election timeout
- Verify node connectivity

---

## Join Token Format

Join tokens are base64-encoded JSON:

```json
{
  "cluster_name": "bib-cluster",
  "leader_addr": "192.168.1.100:4002",
  "token": "random-secret-token",
  "expires_at": 1705324000
}
```

- **Validity:** 24 hours from generation
- **One-time use:** Can be used by multiple nodes
- **Security:** Contains cluster secret, protect accordingly

---

## Example: 3-Node Cluster

### Node 1 (Bootstrap)

```bash
# Initialize cluster
bib setup --daemon --cluster

# Start daemon
bibd

# Save the join token for other nodes
```

### Node 2

```bash
# Join with token
bib setup --daemon --cluster-join <token>

# Start daemon
bibd
```

### Node 3

```bash
# Join with token
bib setup --daemon --cluster-join <token>

# Start daemon
bibd
```

### Verify

```bash
# Check cluster status
bib cluster status

# Should show:
# - 3 members
# - 1 leader, 2 followers
# - Quorum: 2/3
```

