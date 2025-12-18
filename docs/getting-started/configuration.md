# Configuration Guide

This comprehensive guide covers all configuration options for both the `bib` CLI and `bibd` daemon. Configuration can be set via YAML files, environment variables, or command-line flags.

---

## Table of Contents

1. [Configuration Files](#configuration-files)
2. [bib CLI Configuration](#bib-cli-configuration)
3. [bibd Daemon Configuration](#bibd-daemon-configuration)
4. [Environment Variables](#environment-variables)
5. [Command-Line Flags](#command-line-flags)
6. [Configuration Precedence](#configuration-precedence)
7. [Configuration Examples](#configuration-examples)

---

## Configuration Files

### File Locations

Configuration files are stored in platform-specific locations:

| Platform | bib CLI | bibd Daemon |
|----------|---------|-------------|
| **macOS** | `~/.config/bib/config.yaml` | `~/.config/bibd/config.yaml` |
| **Linux** | `~/.config/bib/config.yaml` | `~/.config/bibd/config.yaml` |
| **Windows** | `%APPDATA%\bib\config.yaml` | `%APPDATA%\bibd\config.yaml` |

### Supported Formats

| Format | Extensions | Recommended |
|--------|------------|-------------|
| YAML | `.yaml`, `.yml` | ✅ Yes |
| TOML | `.toml` | No |
| JSON | `.json` | No |

### Auto-Generation

Both `bib` and `bibd` automatically generate default configuration files on first run. For interactive configuration, use the setup wizard:

```bash
# Configure bib CLI interactively
bib setup

# Configure bibd daemon interactively
bib setup --daemon
```

### Viewing Configuration

```bash
# Show current CLI configuration
bib config show

# Show configuration file path
bib config path
```

---

## bib CLI Configuration

The `bib` CLI configuration controls client behavior when interacting with the bibd daemon.

### Complete Example

```yaml
# ~/.config/bib/config.yaml

# Logging configuration
log:
  level: info                    # debug, info, warn, error
  format: text                   # text, json, pretty
  output: stderr                 # stdout, stderr, or file path
  file_path: ""                  # Additional log file (optional)
  enable_caller: false           # Include source file:line in logs
  audit_path: ""                 # Audit log file path
  audit_max_age_days: 365        # Audit log retention period
  redact_fields:                 # Fields to redact from logs
    - password
    - token
    - key
    - secret

# User identity (for attribution)
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
| `level` | string | `info` | Minimum log level: `debug`, `info`, `warn`, `error` |
| `format` | string | `text` | Output format: `text`, `json`, `pretty` |
| `output` | string | `stderr` | Output destination: `stdout`, `stderr`, or file path |
| `file_path` | string | `""` | Additional log file (in addition to output) |
| `max_size_mb` | int | `100` | Max log file size before rotation |
| `max_backups` | int | `3` | Number of old log files to keep |
| `max_age_days` | int | `28` | Days to retain old log files |
| `enable_caller` | bool | `false` | Include source file/line in logs |
| `no_color` | bool | `false` | Disable colored output (for pretty format) |
| `audit_path` | string | `""` | Path to audit log file |
| `audit_max_age_days` | int | `365` | Days to retain audit logs |
| `redact_fields` | []string | See above | Field names to automatically redact |

#### Identity Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `""` | Your display name (for dataset attribution) |
| `email` | string | `""` | Your email address |
| `key` | string | `""` | Path to Ed25519 private key file |

#### Output Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `format` | string | `text` | Default output format: `text`, `json`, `yaml`, `table` |
| `color` | bool | `true` | Enable colored output |

#### Server

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `server` | string | `localhost:8080` | bibd server address (`host:port`) |

---

## bibd Daemon Configuration

The `bibd` daemon configuration controls all aspects of the server's behavior.

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
  
  # Mode-specific settings
  full_replica:
    sync_interval: 5m
  
  selective:
    subscriptions: []
    subscription_store_path: ""
  
  proxy:
    cache_ttl: 2m
    max_cache_size: 1000
    favorite_peers: []

# Database configuration
database:
  backend: postgres              # sqlite or postgres
  
  sqlite:
    path: ""                     # Defaults to <data_dir>/cache.db
    max_open_conns: 10
  
  postgres:
    managed: true                # bibd manages PostgreSQL container
    container_runtime: ""        # Auto-detect: docker, podman
    image: "postgres:16-alpine"
    data_dir: ""                 # Defaults to <data_dir>/postgres
    port: 5432
    max_connections: 100
    
    network:
      use_bridge_network: true
      bridge_network_name: "bibd-network"
      use_unix_socket: true      # Linux only
      bind_address: "127.0.0.1"
    
    health:
      interval: 5s
      timeout: 5s
      startup_timeout: 60s
      action: "retry_limit"      # shutdown, retry_always, retry_limit
      max_retries: 5
    
    tls:
      enabled: true
      auto_generate: true

# Credential management
credentials:
  encryption_method: "hybrid"    # x25519, hkdf, hybrid
  rotation_interval: 168h        # 7 days
  rotation_grace_period: 5m
  password_length: 64

# Encryption at rest
encryption_at_rest:
  enabled: false
  method: "application"          # none, luks, tde, application, hybrid
  
  luks:
    volume_size: "50GB"
    cipher: "aes-xts-plain64"
    key_size: 512
  
  application:
    algorithm: "aes-256-gcm"
    encrypted_fields:
      - table: datasets
        columns: [content, metadata]
      - table: jobs
        columns: [parameters, result]
  
  recovery:
    method: "shamir"
    shamir:
      total_shares: 5
      threshold: 3

# Security configuration
security:
  fallback_mode: "warn"          # strict, warn, permissive
  minimum_level: "moderate"      # maximum, high, moderate, reduced
  log_security_report: true
  require_client_cert: true
  allow_client_cert_fallback: false

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
| `host` | string | `0.0.0.0` | Listen address for gRPC server |
| `port` | int | `8080` | Listen port for gRPC server |
| `data_dir` | string | `~/.local/share/bibd` | Data storage directory |
| `pid_file` | string | `/var/run/bibd.pid` | PID file location |

##### TLS Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `tls.enabled` | bool | `false` | Enable TLS for gRPC connections |
| `tls.cert_file` | string | `""` | Path to TLS certificate file |
| `tls.key_file` | string | `""` | Path to TLS private key file |

#### P2P Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable P2P networking |
| `mode` | string | `proxy` | Node mode: `proxy`, `selective`, `full` |
| `identity.key_path` | string | `""` | Path to Ed25519 identity key file |
| `listen_addresses` | []string | TCP+QUIC on 4001 | libp2p multiaddr listen addresses |

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
| `min_peers` | int | `1` | Minimum bootstrap peers required |
| `retry_interval` | duration | `5s` | Initial retry interval |
| `max_retry_interval` | duration | `1h` | Maximum retry interval (exponential backoff cap) |

##### mDNS (Local Discovery)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable mDNS discovery for local network |
| `service_name` | string | `bib.local` | mDNS service name |

##### DHT (Global Discovery)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable Kademlia DHT |
| `mode` | string | `auto` | DHT mode: `auto`, `server`, `client` |

**DHT Modes:**
- `auto` — libp2p decides based on network reachability
- `server` — Full DHT participant (requires public IP)
- `client` — Query-only, doesn't store records (works behind NAT)

##### Mode-Specific Settings

**Full Replica Mode (`p2p.full_replica`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sync_interval` | duration | `5m` | Poll interval for new data |

**Selective Mode (`p2p.selective`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `subscriptions` | []string | `[]` | Topic patterns to subscribe to |
| `subscription_store_path` | string | `""` | Subscription persistence file |

**Proxy Mode (`p2p.proxy`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cache_ttl` | duration | `2m` | Cache entry time-to-live |
| `max_cache_size` | int | `1000` | Maximum cache entries |
| `favorite_peers` | []string | `[]` | Preferred peers for forwarding |

#### Database Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `backend` | string | `sqlite` | Storage backend: `sqlite` or `postgres` |

##### SQLite Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | `""` | Database path (defaults to `<data_dir>/cache.db`) |
| `max_open_conns` | int | `10` | Maximum open connections |

##### PostgreSQL Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `managed` | bool | `true` | bibd manages PostgreSQL container lifecycle |
| `container_runtime` | string | `""` | Container runtime: `docker`, `podman` (auto-detect if empty) |
| `image` | string | `postgres:16-alpine` | PostgreSQL container image |
| `data_dir` | string | `""` | PostgreSQL data directory |
| `port` | int | `5432` | PostgreSQL port |
| `max_connections` | int | `100` | Maximum database connections |

**Network Settings (`postgres.network`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `use_bridge_network` | bool | `true` | Create isolated Docker network |
| `bridge_network_name` | string | `bibd-network` | Docker network name |
| `use_unix_socket` | bool | `true` | Use Unix socket (Linux only) |
| `bind_address` | string | `127.0.0.1` | TCP bind address |

**Health Check Settings (`postgres.health`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `interval` | duration | `5s` | Health check interval |
| `timeout` | duration | `5s` | Health check timeout |
| `startup_timeout` | duration | `60s` | Maximum startup wait time |
| `action` | string | `retry_limit` | Action on failure: `shutdown`, `retry_always`, `retry_limit` |
| `max_retries` | int | `5` | Maximum retries (for `retry_limit` action) |

**TLS Settings (`postgres.tls`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable TLS for database connections |
| `auto_generate` | bool | `true` | Auto-generate TLS certificates |

> 📖 For detailed database security documentation, see [Database Security](database-security.md).

#### Credentials Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `encryption_method` | string | `hybrid` | Encryption method: `x25519`, `hkdf`, `hybrid` |
| `rotation_interval` | duration | `168h` | Credential rotation interval (default: 7 days) |
| `rotation_grace_period` | duration | `5m` | Grace period during rotation |
| `password_length` | int | `64` | Generated password length (minimum 32) |
| `encrypted_path` | string | `""` | Credential storage path |

**Encryption Methods:**
- `x25519` — XSalsa20-Poly1305 (NaCl), well-audited
- `hkdf` — AES-256-GCM with HKDF-SHA256, FIPS-compatible
- `hybrid` — Both methods for maximum compatibility (default)

#### Encryption at Rest Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable encryption at rest |
| `method` | string | `application` | Method: `none`, `luks`, `tde`, `application`, `hybrid` |

**LUKS Settings (`encryption_at_rest.luks`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `volume_size` | string | `50GB` | Encrypted volume size |
| `cipher` | string | `aes-xts-plain64` | Encryption cipher |
| `key_size` | int | `512` | Key size in bits |

**Application-Level Settings (`encryption_at_rest.application`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `algorithm` | string | `aes-256-gcm` | Encryption algorithm |
| `encrypted_fields` | []object | See config | Fields to encrypt |

**Recovery Settings (`encryption_at_rest.recovery`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `method` | string | `shamir` | Recovery method (Shamir's Secret Sharing) |
| `total_shares` | int | `5` | Total shares to generate |
| `threshold` | int | `3` | Minimum shares for recovery |

#### Security Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fallback_mode` | string | `warn` | Behavior when requirements unmet: `strict`, `warn`, `permissive` |
| `minimum_level` | string | `moderate` | Minimum security level: `maximum`, `high`, `moderate`, `reduced` |
| `log_security_report` | bool | `true` | Log security report on startup |
| `require_client_cert` | bool | `true` | Require mTLS client certificates |
| `allow_client_cert_fallback` | bool | `false` | Allow password auth if cert fails |

#### Cluster Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable high-availability clustering |
| `node_id` | string | `""` | Unique node identifier (auto-generated) |
| `cluster_name` | string | `bib-cluster` | Cluster name for discovery |
| `data_dir` | string | `""` | Raft data directory |
| `listen_addr` | string | `0.0.0.0:4002` | Raft listen address |
| `advertise_addr` | string | `""` | Address advertised to other nodes |
| `is_voter` | bool | `true` | Can participate in leader election |
| `bootstrap` | bool | `false` | Is initial cluster node |
| `join_token` | string | `""` | Token for joining existing cluster |
| `join_addrs` | []string | `[]` | Addresses of existing cluster members |
| `enable_dht_discovery` | bool | `false` | Discover cluster via DHT (experimental) |

**Raft Settings (`cluster.raft`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `heartbeat_timeout` | duration | `1s` | Leader heartbeat interval |
| `election_timeout` | duration | `5s` | Leader election timeout |
| `commit_timeout` | duration | `50ms` | Log commit timeout |
| `max_append_entries` | int | `64` | Max entries per append RPC |
| `trailing_logs` | uint64 | `10000` | Logs to keep after snapshot |
| `max_inflight` | int | `256` | Maximum in-flight append entries |

**Snapshot Settings (`cluster.snapshot`):**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `interval` | duration | `30m` | Automatic snapshot interval |
| `threshold` | uint64 | `8192` | Log entries before triggering snapshot |
| `retain_count` | int | `3` | Number of snapshots to retain |

> 📖 For detailed clustering documentation, see [Clustering Guide](clustering.md).

---

## Environment Variables

Configuration can be set via environment variables using the pattern:
- CLI: `BIB_<SECTION>_<FIELD>`
- Daemon: `BIBD_<SECTION>_<FIELD>`

### Examples

```bash
# bib CLI environment variables
export BIB_LOG_LEVEL=debug
export BIB_SERVER=localhost:8080
export BIB_OUTPUT_FORMAT=json
export BIB_IDENTITY_NAME="Your Name"

# bibd environment variables
export BIBD_LOG_LEVEL=debug
export BIBD_SERVER_PORT=9090
export BIBD_P2P_MODE=full
export BIBD_P2P_ENABLED=true
export BIBD_DATABASE_BACKEND=postgres
```

### Nested Fields

For nested configuration, use underscores:

```bash
# p2p.bootstrap.min_peers = 3
export BIBD_P2P_BOOTSTRAP_MIN_PEERS=3

# database.postgres.managed = true
export BIBD_DATABASE_POSTGRES_MANAGED=true
```

---

## Command-Line Flags

Command-line flags override both configuration files and environment variables.

### bib CLI Flags

```bash
# Specify config file
bib --config /path/to/config.yaml command

# Override output format
bib --output json query ...

# Enable verbose logging
bib --verbose command
```

### bibd Flags

```bash
# Specify config file
bibd -config /path/to/config.yaml

# Print version and exit
bibd -version
```

---

## Configuration Precedence

Configuration is loaded in the following order (later sources override earlier ones):

```
1. Default values (built-in)
        ↓
2. Configuration file (~/.config/bib/config.yaml)
        ↓
3. Environment variables (BIB_*, BIBD_*)
        ↓
4. Command-line flags (--config, --output, etc.)
```

---

## Configuration Examples

### Minimal Development Setup

```yaml
# ~/.config/bibd/config.yaml
log:
  level: debug

p2p:
  enabled: true
  mode: proxy

database:
  backend: sqlite
```

### Production Full Node

```yaml
# ~/.config/bibd/config.yaml
log:
  level: info
  format: json
  audit_path: "/var/log/bibd/audit.log"

server:
  host: "0.0.0.0"
  port: 8080
  tls:
    enabled: true
    cert_file: "/etc/bibd/server.crt"
    key_file: "/etc/bibd/server.key"

p2p:
  enabled: true
  mode: full
  full_replica:
    sync_interval: 5m

database:
  backend: postgres
  postgres:
    managed: true
    max_connections: 200

credentials:
  rotation_interval: 72h  # 3 days
```

### High-Availability Cluster Node

```yaml
# ~/.config/bibd/config.yaml
log:
  level: info
  format: json

server:
  host: "0.0.0.0"
  port: 8080

p2p:
  enabled: true
  mode: full

database:
  backend: postgres
  postgres:
    managed: true

cluster:
  enabled: true
  cluster_name: "prod-cluster"
  listen_addr: "0.0.0.0:4002"
  advertise_addr: "10.0.1.10:4002"
  is_voter: true
```

### Selective Sync Node

```yaml
# ~/.config/bibd/config.yaml
p2p:
  enabled: true
  mode: selective
  selective:
    subscriptions:
      - "weather/*"
      - "finance/stocks"
      - "research/papers/2024"

database:
  backend: postgres
  postgres:
    managed: true
```

---

## Related Documentation

| Document | Topic |
|----------|-------|
| [Quick Start](quickstart.md) | Getting started with configuration |
| [Node Modes](../concepts/node-modes.md) | Detailed mode configuration |
| [Database Security](../storage/database-security.md) | Database security options |
| [Clustering Guide](../guides/clustering.md) | HA cluster configuration |
