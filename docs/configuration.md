# Configuration Guide

This document describes all configuration options for both `bib` (CLI) and `bibd` (daemon).

## Configuration Files

### Locations

Configuration files are stored in platform-specific locations:

| Platform | bib CLI | bibd Daemon |
|----------|---------|-------------|
| macOS | `~/.config/bib/config.yaml` | `~/.config/bibd/config.yaml` |
| Linux | `~/.config/bib/config.yaml` | `~/.config/bibd/config.yaml` |
| Windows | `%APPDATA%\bib\config.yaml` | `%APPDATA%\bibd\config.yaml` |

### Supported Formats

- YAML (`.yaml`, `.yml`) - Recommended
- TOML (`.toml`)
- JSON (`.json`)

### Auto-Generation

Both `bib` and `bibd` automatically generate default configuration files on first run. Use the `setup` command for interactive configuration:

```bash
# Configure bib CLI
bib setup

# Configure bibd daemon
bib setup --daemon
```

---

## bib CLI Configuration

The `bib` CLI configuration (`BibConfig`) controls client behavior.

### Complete Example

```yaml
# ~/.config/bib/config.yaml

# Logging configuration
log:
  level: info                    # debug, info, warn, error
  format: text                   # text, json, pretty
  output: stderr                 # stdout, stderr, or file path
  file_path: ""                  # Additional log file (optional)
  enable_caller: false           # Include source file:line
  audit_path: ""                 # Audit log file path
  audit_max_age_days: 365        # Audit log retention
  redact_fields:                 # Fields to redact from logs
    - password
    - token
    - key
    - secret

# User identity
identity:
  name: "Your Name"
  email: "you@example.com"
  key: ""                        # Path to private key or secret reference

# Output formatting
output:
  format: text                   # text, json, yaml, table
  color: true                    # Enable colored output

# bibd server connection
server: "localhost:8080"
```

### Configuration Reference

#### Log Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `info` | Minimum log level: debug, info, warn, error |
| `format` | string | `text` | Output format: text, json, pretty |
| `output` | string | `stderr` | Output destination: stdout, stderr, or file path |
| `file_path` | string | `""` | Additional log file (in addition to output) |
| `max_size_mb` | int | `100` | Max log file size before rotation |
| `max_backups` | int | `3` | Number of old log files to keep |
| `max_age_days` | int | `28` | Days to retain old log files |
| `enable_caller` | bool | `false` | Include source file/line in logs |
| `no_color` | bool | `false` | Disable colored output (pretty format) |
| `audit_path` | string | `""` | Path to audit log file |
| `audit_max_age_days` | int | `365` | Days to retain audit logs |
| `redact_fields` | []string | `[password, token, ...]` | Field names to redact |

#### Identity Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `""` | Your display name |
| `email` | string | `""` | Your email address |
| `key` | string | `""` | Path to private key file or secret reference |

#### Output Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `format` | string | `text` | Default output format: text, json, yaml, table |
| `color` | bool | `true` | Enable colored output |

#### Server

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `server` | string | `localhost:8080` | bibd server address to connect to |

---

## bibd Daemon Configuration

The `bibd` daemon configuration (`BibdConfig`) controls all daemon behavior.

### Complete Example

```yaml
# ~/.config/bibd/config.yaml

# Logging configuration
log:
  level: info
  format: pretty
  output: stdout
  enable_caller: true
  audit_path: "/var/log/bibd/audit.log"
  redact_fields:
    - password
    - token
    - key
    - secret

# Daemon identity
identity:
  name: "My Node"
  email: "admin@example.com"

# Server configuration
server:
  host: "0.0.0.0"
  port: 8080
  data_dir: "~/.local/share/bibd"
  pid_file: "/var/run/bibd.pid"
  tls:
    enabled: false
    cert_file: ""
    key_file: ""

# P2P networking
p2p:
  enabled: true
  mode: proxy                    # proxy, selective, full
  
  identity:
    key_path: ""                 # Defaults to config dir + /identity.pem
  
  listen_addresses:
    - "/ip4/0.0.0.0/tcp/4001"
    - "/ip4/0.0.0.0/udp/4001/quic-v1"
  
  connection_manager:
    low_watermark: 100
    high_watermark: 400
    grace_period: 30s
  
  bootstrap:
    peers:
      - "/dns4/bib.dev/tcp/4001"
      - "/dns4/bib.dev/udp/4001/quic-v1"
    min_peers: 1
    retry_interval: 5s
    max_retry_interval: 1h
  
  mdns:
    enabled: true
    service_name: "bib.local"
  
  dht:
    enabled: true
    mode: auto                   # auto, server, client
  
  peer_store:
    path: ""                     # Defaults to config dir + /peers.db
  
  full_replica:
    sync_interval: 5m
  
  selective:
    subscriptions: []
    subscription_store_path: ""
  
  proxy:
    cache_ttl: 2m
    max_cache_size: 1000
    favorite_peers: []

# Cluster configuration (HA mode)
cluster:
  enabled: false
  node_id: ""                    # Auto-generated if empty
  cluster_name: "bib-cluster"
  data_dir: ""                   # Defaults to config dir + /raft
  listen_addr: "0.0.0.0:4002"
  advertise_addr: ""             # Defaults to listen_addr
  is_voter: true
  bootstrap: false
  join_token: ""
  join_addrs: []
  enable_dht_discovery: false
  
  raft:
    heartbeat_timeout: 1s
    election_timeout: 5s
    commit_timeout: 50ms
    max_append_entries: 64
    trailing_logs: 10000
    max_inflight: 256
  
  snapshot:
    interval: 30m
    threshold: 8192
    retain_count: 3
```

### Configuration Reference

#### Server Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `host` | string | `0.0.0.0` | Listen address |
| `port` | int | `8080` | Listen port |
| `data_dir` | string | `~/.local/share/bibd` | Data storage directory |
| `pid_file` | string | `/var/run/bibd.pid` | PID file location |
| `tls.enabled` | bool | `false` | Enable TLS |
| `tls.cert_file` | string | `""` | TLS certificate file |
| `tls.key_file` | string | `""` | TLS private key file |

#### P2P Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable P2P networking |
| `mode` | string | `proxy` | Node mode: proxy, selective, full |
| `identity.key_path` | string | `""` | Path to identity key file |
| `listen_addresses` | []string | TCP+QUIC on 4001 | Multiaddr listen addresses |

##### Connection Manager

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `low_watermark` | int | `100` | Minimum connections to maintain |
| `high_watermark` | int | `400` | Maximum connections before pruning |
| `grace_period` | duration | `30s` | New connection protection period |

##### Bootstrap

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `peers` | []string | bib.dev | Bootstrap peer multiaddrs |
| `min_peers` | int | `1` | Minimum bootstrap peers before continuing |
| `retry_interval` | duration | `5s` | Initial retry interval |
| `max_retry_interval` | duration | `1h` | Maximum retry interval (backoff cap) |

##### mDNS

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable mDNS discovery |
| `service_name` | string | `bib.local` | mDNS service name |

##### DHT

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable Kademlia DHT |
| `mode` | string | `auto` | DHT mode: auto, server, client |

##### Mode-Specific Settings

**Full Replica Mode:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sync_interval` | duration | `5m` | Poll interval for new data |

**Selective Mode:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `subscriptions` | []string | `[]` | Topic patterns to subscribe to |
| `subscription_store_path` | string | `""` | Subscription persistence file |

**Proxy Mode:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cache_ttl` | duration | `2m` | Cache entry TTL |
| `max_cache_size` | int | `1000` | Maximum cache entries |
| `favorite_peers` | []string | `[]` | Preferred forwarding peers |

#### Cluster Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable HA clustering |
| `node_id` | string | `""` | Unique node identifier (auto-generated) |
| `cluster_name` | string | `bib-cluster` | Cluster name for discovery |
| `data_dir` | string | `""` | Raft data directory |
| `listen_addr` | string | `0.0.0.0:4002` | Raft listen address |
| `advertise_addr` | string | `""` | Address advertised to other nodes |
| `is_voter` | bool | `true` | Can participate in leader election |
| `bootstrap` | bool | `false` | Is initial cluster node |
| `join_token` | string | `""` | Token for joining existing cluster |
| `join_addrs` | []string | `[]` | Addresses of existing cluster members |

##### Raft Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `heartbeat_timeout` | duration | `1s` | Leader heartbeat interval |
| `election_timeout` | duration | `5s` | Leader election timeout |
| `commit_timeout` | duration | `50ms` | Log commit timeout |
| `max_append_entries` | int | `64` | Max entries per append RPC |
| `trailing_logs` | uint64 | `10000` | Logs to keep after snapshot |

##### Snapshot Settings

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `interval` | duration | `30m` | Automatic snapshot interval |
| `threshold` | uint64 | `8192` | Log entries before snapshot |
| `retain_count` | int | `3` | Snapshots to retain |

---

## Environment Variables

Configuration can also be set via environment variables:

```bash
# bib CLI
export BIB_LOG_LEVEL=debug
export BIB_SERVER=localhost:8080
export BIB_OUTPUT_FORMAT=json

# bibd
export BIBD_LOG_LEVEL=debug
export BIBD_SERVER_PORT=9090
export BIBD_P2P_MODE=full
```

Environment variables use the pattern `BIB_<SECTION>_<FIELD>` or `BIBD_<SECTION>_<FIELD>`.

---

## Command-Line Flags

Flags override configuration file and environment variables:

```bash
# Specify config file
bib --config /path/to/config.yaml command

# Override output format
bib --output json query ...

# bibd with custom config
bibd -config /path/to/config.yaml
```

---

## Configuration Precedence

Configuration is loaded in this order (later overrides earlier):

1. Default values
2. Configuration file
3. Environment variables
4. Command-line flags

---

## Viewing Current Configuration

```bash
# Show current configuration
bib config show

# Show configuration file path
bib config path
```

