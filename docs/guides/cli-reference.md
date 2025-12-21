# CLI Reference

Complete reference documentation for the `bib` command-line interface. The CLI is the primary user interface for interacting with the Bib ecosystem.

---

## Table of Contents

1. [Overview](#overview)
2. [Global Flags](#global-flags)
3. [Core Commands](#core-commands)
4. [Data Management Commands](#data-management-commands)
5. [Job Commands](#job-commands)
6. [Network Commands](#network-commands)
7. [Cluster Commands](#cluster-commands)
8. [Admin Commands](#admin-commands)
9. [Output Formats](#output-formats)
10. [Exit Codes](#exit-codes)
11. [Shell Completion](#shell-completion)
12. [Examples](#examples)

---

## Overview

The `bib` CLI communicates with the `bibd` daemon via gRPC to perform all operations. Most commands require a running daemon.

```bash
bib [global-flags] <command> [command-flags] [arguments]
```

### Getting Help

```bash
# General help
bib --help

# Command-specific help
bib <command> --help
bib <command> <subcommand> --help
```

---

## Global Flags

These flags are available for all commands:

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--config` | | string | `~/.config/bib/config.yaml` | Path to configuration file |
| `--output` | `-o` | string | `text` | Output format: `text`, `json`, `yaml`, `table` |
| `--verbose` | `-v` | bool | `false` | Enable verbose output |
| `--help` | `-h` | bool | | Show help for command |

---

## Core Commands

### version

Print version information for both CLI and connected daemon.

```bash
bib version
```

**Output:**
```
bib version 0.1.0
  commit:  abc1234
  built:   2024-01-15T10:30:00Z
  go:      go1.25
```

---

### setup

Launch the interactive configuration wizard.

```bash
bib setup [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--daemon` | `-d` | bool | `false` | Configure bibd daemon instead of CLI |
| `--quick` | `-q` | bool | `false` | Quick start with minimal prompts |
| `--format` | `-f` | string | `yaml` | Config file format: `yaml`, `toml`, `json` |
| `--cluster` | | bool | `false` | Initialize new HA cluster (requires `--daemon`) |
| `--cluster-join` | | string | `""` | Join existing cluster with token (requires `--daemon`) |
| `--reconfigure` | | string | `""` | Reconfigure specific section only |
| `--fresh` | | bool | `false` | Reset configuration and start fresh |

**Examples:**

```bash
# Quick start - minimal prompts, sensible defaults
bib setup --quick

# Configure bib CLI interactively
bib setup

# Quick daemon setup (Proxy mode)
bib setup --daemon --quick

# Full interactive daemon setup
bib setup --daemon

# Initialize HA cluster on first node
bib setup --daemon --cluster

# Join existing cluster
bib setup --daemon --cluster-join "eyJjbHVzdGVy..."

# Reconfigure specific section
bib setup --reconfigure identity
bib setup --daemon --reconfigure p2p-mode

# Reset and start fresh
bib setup --fresh
```

**Wizard Navigation:**
- `Tab` / `↓` — Move to next field
- `Shift+Tab` / `↑` — Move to previous field
- `Enter` — Proceed to next step
- `Esc` — Go back to previous step
- `Ctrl+C` — Save progress and exit (can resume later)

**Quick Start Mode:**

Quick start (`--quick`) creates a working configuration with minimal input:
- Prompts only for name and email
- Generates Ed25519 identity key automatically
- Uses sensible defaults (Proxy mode, SQLite, public bootstrap)
- Starts bibd immediately after daemon setup

**CLI Setup Steps:**
1. **Welcome** — Introduction and overview
2. **Identity** — Name and email, generates `~/.config/bib/identity.pem`
3. **Output** — Default format and color preferences
4. **Connection** — bibd server address
5. **Logging** — Log level selection
6. **Connection Test** — Verify connectivity to daemon
7. **Auth Test** — Authenticate with generated identity
8. **Network Health** — Check peer connections
9. **Confirm** — Review and save

**Daemon Setup Steps:**
1. **Welcome** — Introduction and overview
2. **Identity** — Daemon name and contact email
3. **Server** — Host, port, and data directory
4. **TLS / Security** — TLS, certificate pinning, hardening
5. **Storage** — Database backend (SQLite/PostgreSQL)
6. **P2P Mode** — Proxy, Selective, or Full (with mode-specific config)
7. **Bootstrap** — Public (bib.dev) + custom peers, connectivity test
8. **Logging** — Log level and format
9. **Cluster** — Optional HA cluster configuration
10. **Break Glass** — Optional emergency access
11. **Confirm** — Review and save
12. **Deployment** — Install service, start bibd, verify

> 📘 See [Setup Flow](../getting-started/setup-flow.md) for detailed documentation.

---

### config

Manage configuration settings.

```bash
bib config <subcommand>
```

#### config show

Display the current configuration.

```bash
bib config show
```

**Output:**
```yaml
log:
  level: info
  format: text
identity:
  name: "Your Name"
  email: "you@example.com"
output:
  format: text
  color: true
server: "localhost:8080"
```

#### config path

Show the configuration file path.

```bash
bib config path
```

**Output:**
```
/Users/you/.config/bib/config.yaml
```

#### config reset

Reset configuration to defaults.

```bash
bib config reset [section] [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--all` | bool | Reset entire configuration |

**Examples:**
```bash
# Reset specific section
bib config reset p2p

# Reset everything
bib config reset --all
```

---

### connect

Connect to a bibd daemon.

```bash
bib connect [address] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--save` | bool | `false` | Save as default node in config |
| `--alias` | string | `""` | Alias for the node (used with `--save`) |
| `--test` | bool | `false` | Test connection only, don't authenticate |
| `--timeout` | duration | `10s` | Connection timeout |
| `--trust-first-use` | bool | `false` | Auto-trust on first connection (skip TOFU prompt) |

**Examples:**

```bash
# Connect to local daemon
bib connect localhost:4000

# Connect and save as default
bib connect --save node1.example.com:4000

# Connect with alias
bib connect --save --alias mynode node1.example.com:4000

# Test connection only (no auth)
bib connect --test node1.example.com:4000

# Auto-trust on first connection (for scripting)
bib connect --trust-first-use --save remote.node.com:4000
```

**TOFU (Trust-On-First-Use):**

When connecting to a new node for the first time, you'll be prompted to verify the node's certificate fingerprint:

```
⚠️  First connection to this node

Node ID:      QmXyz123...
Address:      node1.example.com:4000
Fingerprint:  SHA256:Ab12Cd34Ef56...

Trust this node? [y/N]
```

Use `--trust-first-use` to skip this prompt (for scripting or when trust has been verified separately).

---

### trust

Manage trusted nodes (TOFU).

```bash
bib trust <subcommand>
```

#### trust list

List all trusted nodes.

```bash
bib trust list
```

**Output:**
```
NODE ID         ALIAS      ADDRESS                   TRUSTED     PINNED
QmXyz123...     mynode     node1.example.com:4000    2024-01-15  no
QmAbc456...     -          192.168.1.50:4000         2024-01-10  yes
```

#### trust add

Manually add a trusted node.

```bash
bib trust add <node-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--fingerprint` | string | Certificate fingerprint (required) |
| `--alias` | string | Friendly name for the node |
| `--address` | string | Node address |

**Example:**
```bash
bib trust add QmXyz123... --fingerprint SHA256:Ab12Cd34Ef56... --alias mynode
```

#### trust remove

Remove trust for a node.

```bash
bib trust remove <node-id>
```

#### trust pin

Pin a node's certificate (prevent automatic trust updates).

```bash
bib trust pin <node-id>
```

Pinned certificates provide additional security by preventing certificate changes from being automatically trusted.

---

## Data Management Commands

### topic

Manage topics (data categories).

```bash
bib topic <subcommand>
```

#### topic list

List available topics.

```bash
bib topic list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--status` | string | Filter by status: `active`, `archived`, `deleted` |
| `--owner` | string | Filter by owner user ID |

**Example:**
```bash
bib topic list
```
```
TOPIC ID    NAME        DATASETS   STATUS    UPDATED
weather     Weather     15         active    2024-01-15
finance     Finance     42         active    2024-01-14
research    Research    8          archived  2024-01-10
```

#### topic create

Create a new topic.

```bash
bib topic create <name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--description` | string | Topic description |
| `--schema` | string | Path to SQL DDL schema file |
| `--parent` | string | Parent topic ID for hierarchy |
| `--tags` | []string | Tags for categorization |

**Example:**
```bash
bib topic create weather \
  --description "Weather data collection" \
  --tags "meteorology,timeseries"
```

#### topic show

Show topic details.

```bash
bib topic show <topic-id>
```

**Example:**
```bash
bib topic show weather
```
```
Topic: weather
Name: Weather
Description: Weather data collection
Status: active
Datasets: 15
Created: 2024-01-01 10:00:00
Updated: 2024-01-15 14:30:00
Owners: user-abc123
Tags: meteorology, timeseries
```

#### topic delete

Delete a topic.

```bash
bib topic delete <topic-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--force` | bool | Skip confirmation prompt |

---

### dataset

Manage datasets within topics.

```bash
bib dataset <subcommand>
```

#### dataset list

List datasets.

```bash
bib dataset list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--topic` | string | Filter by topic ID |
| `--status` | string | Filter by status |
| `--owner` | string | Filter by owner |

**Example:**
```bash
bib dataset list --topic weather
```
```
DATASET ID      NAME              VERSION   SIZE      UPDATED
daily-temps     Daily Temps       v1.2.0    1.2 MB    2024-01-15
hourly-wind     Hourly Wind       v2.0.0    4.5 MB    2024-01-14
precipitation   Precipitation     v1.0.0    890 KB    2024-01-10
```

#### dataset create

Create a new dataset.

```bash
bib dataset create <name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--topic` | string | **Required.** Parent topic ID |
| `--description` | string | Dataset description |
| `--file` | string | Path to data file to upload |
| `--schema` | string | Path to SQL DDL schema file |
| `--tags` | []string | Tags for categorization |

**Example:**
```bash
bib dataset create daily-temps \
  --topic weather \
  --file ./temps.csv \
  --description "Daily temperature readings"
```

#### dataset show

Show dataset details.

```bash
bib dataset show <dataset-id>
```

#### dataset download

Download a dataset to local storage.

```bash
bib dataset download <dataset-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | string | Output directory or file path |
| `--version` | string | Specific version to download |

**Example:**
```bash
bib dataset download daily-temps --output ./data/
```

#### dataset versions

List all versions of a dataset.

```bash
bib dataset versions <dataset-id>
```

**Example:**
```bash
bib dataset versions daily-temps
```
```
VERSION   CREATED             SIZE      MESSAGE
v1.2.0    2024-01-15 10:00    1.2 MB    Added December data
v1.1.0    2024-01-01 09:00    1.1 MB    Added November data
v1.0.0    2023-12-01 08:00    1.0 MB    Initial release
```

---

### catalog

View and search the data catalog.

```bash
bib catalog <subcommand>
```

#### catalog list

List local catalog entries.

```bash
bib catalog list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--limit` | int | Maximum entries to return |
| `--offset` | int | Pagination offset |

#### catalog query

Search the catalog.

```bash
bib catalog query [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--topic` | string | Topic pattern (supports wildcards) |
| `--name` | string | Name pattern (supports wildcards) |
| `--tag` | string | Filter by tag |
| `--limit` | int | Maximum results |

**Example:**
```bash
bib catalog query --topic "weather/*" --name "*2024*"
```

#### catalog sync

Sync catalog from a specific peer.

```bash
bib catalog sync <peer-id>
```

---

### query

Query data using SQL or metadata filters.

```bash
bib query <subcommand>
```

#### query metadata

Query dataset metadata.

```bash
bib query metadata [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--topic` | string | Topic pattern |
| `--name` | string | Name pattern |
| `--tag` | string | Filter by tag |

#### query sql

Execute SQL queries against datasets.

```bash
bib query sql "<sql-query>" [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--dataset` | string | Dataset alias mapping: `<id>=<alias>` |
| `--limit` | int | Override LIMIT clause |

**Example:**
```bash
bib query sql "SELECT date, temp_max, temp_min FROM temps WHERE date > '2024-01-01' LIMIT 10" \
  --dataset daily-temps=temps
```

---

## Job Commands

### job

Manage data processing jobs.

```bash
bib job <subcommand>
```

#### job list

List jobs.

```bash
bib job list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--status` | string | Filter by status: `pending`, `running`, `completed`, `failed`, `cancelled` |
| `--limit` | int | Maximum jobs to return |

**Example:**
```bash
bib job list --status running
```
```
JOB ID          TASK            STATUS    STARTED             PROGRESS
job-abc123      fetch-weather   running   2024-01-15 10:00    45%
job-def456      etl-daily       running   2024-01-15 09:30    80%
```

#### job submit

Submit a new job.

```bash
bib job submit [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--task` | string | **Required.** Task ID to execute |
| `--input` | string | Input parameter: `<name>=<value>` (repeatable) |
| `--priority` | int | Job priority (higher = more urgent) |

**Example:**
```bash
bib job submit --task fetch-weather \
  --input api_key="${WEATHER_API_KEY}" \
  --input location="NYC"
```

#### job status

Show job status and details.

```bash
bib job status <job-id>
```

**Example:**
```bash
bib job status job-abc123
```
```
Job ID: job-abc123
Task: fetch-weather
Status: running
Progress: 45%
Started: 2024-01-15 10:00:00
Current Step: 2/5 (parse)
```

#### job cancel

Cancel a running or pending job.

```bash
bib job cancel <job-id>
```

#### job logs

View job execution logs.

```bash
bib job logs <job-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--follow` | bool | Stream logs in real-time |
| `--tail` | int | Number of lines from end |

---

### task

Manage reusable task templates.

```bash
bib task <subcommand>
```

#### task list

List available tasks.

```bash
bib task list [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--tag` | string | Filter by tag |

#### task create

Create a task from a YAML definition file.

```bash
bib task create <name> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--file` | string | **Required.** Path to task definition YAML |
| `--description` | string | Task description |

**Example:**
```bash
bib task create fetch-weather --file ./tasks/fetch-weather.yaml
```

#### task show

Show task details.

```bash
bib task show <task-id>
```

#### task run

Execute a task immediately.

```bash
bib task run <task-id> [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--input` | string | Input parameter: `<name>=<value>` (repeatable) |

---

## Network Commands

### peer

Manage P2P network peers.

```bash
bib peer <subcommand>
```

#### peer list

List connected peers.

```bash
bib peer list
```

**Example:**
```
PEER ID                     ADDRESS                       MODE    LATENCY
QmXyz...abc                 /ip4/192.168.1.100/tcp/4001   full    12ms
QmDef...123                 /ip4/10.0.0.50/tcp/4001       select  45ms
```

#### peer show

Show detailed peer information.

```bash
bib peer show <peer-id>
```

#### peer connect

Connect to a specific peer.

```bash
bib peer connect <multiaddr>
```

**Example:**
```bash
bib peer connect /ip4/192.168.1.100/tcp/4001/p2p/QmXyz...
```

#### peer disconnect

Disconnect from a peer.

```bash
bib peer disconnect <peer-id>
```

---

### subscribe

Manage topic subscriptions (selective mode only).

```bash
bib subscribe <subcommand>
```

#### subscribe list

List current subscriptions.

```bash
bib subscribe list
```

**Example:**
```
PATTERN              SINCE           DATASETS
weather/*            2024-01-01      15
finance/stocks       2024-01-10      8
```

#### subscribe add

Add a topic subscription.

```bash
bib subscribe add <topic-pattern>
```

**Pattern Examples:**
- `weather` — Exact match
- `weather/*` — Topic and all sub-topics
- `*/papers` — Any topic ending in `/papers`

#### subscribe remove

Remove a subscription.

```bash
bib subscribe remove <topic-pattern>
```

---

### sync

Manage data synchronization.

```bash
bib sync <subcommand>
```

#### sync status

Show synchronization status.

```bash
bib sync status
```

**Example:**
```
Sync Status: active
Mode: selective
Subscriptions: 3
Local Datasets: 23
Pending Downloads: 2
Last Sync: 2024-01-15 14:30:00
```

#### sync start

Start synchronization.

```bash
bib sync start
```

#### sync stop

Stop synchronization.

```bash
bib sync stop
```

---

## Cluster Commands

### cluster

Manage high-availability cluster.

```bash
bib cluster <subcommand>
```

#### cluster status

Show cluster status.

```bash
bib cluster status
```

**Example:**
```
Cluster: bib-cluster
State: healthy
Leader: node-1 (10.0.1.10:4002)
Term: 42
Nodes: 3/3 healthy

NODE ID     ADDRESS           ROLE       STATUS    LAST SEEN
node-1      10.0.1.10:4002    leader     healthy   now
node-2      10.0.1.11:4002    follower   healthy   2s ago
node-3      10.0.1.12:4002    follower   healthy   3s ago
```

#### cluster token generate

Generate a join token for new nodes.

```bash
bib cluster token generate [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--expires` | duration | Token expiration time (default: 24h) |

**Example:**
```bash
bib cluster token generate --expires 1h
```
```
Join token (expires in 1 hour):

  eyJjbHVzdGVyX25hbWUiOiJiaWItY2x1c3RlciIsImxlYWRlcl9hZGRyIjoi...

To join a node, run:
  bib setup --daemon --cluster-join <token>
```

#### cluster members

List cluster members.

```bash
bib cluster members
```

#### cluster remove

Remove a node from the cluster.

```bash
bib cluster remove <node-id>
```

#### cluster promote

Promote a non-voter to voter.

```bash
bib cluster promote <node-id>
```

#### cluster demote

Demote a voter to non-voter.

```bash
bib cluster demote <node-id>
```

---

## Admin Commands

### admin break-glass

Emergency database access (requires authorization).

```bash
bib admin break-glass <subcommand>
```

> ⚠️ **Security Notice:** Break glass access is fully audited and requires acknowledgment after use. See [Break Glass Access](break-glass.md) for details.

#### admin break-glass enable

Enable break glass session.

```bash
bib admin break-glass enable [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--reason` | string | **Required.** Reason for emergency access |
| `--duration` | duration | Session duration (default: configured max) |
| `--access-level` | string | Access level: `readonly`, `readwrite` |
| `--key` | string | Path to Ed25519 private key |

#### admin break-glass disable

End break glass session early.

```bash
bib admin break-glass disable
```

#### admin break-glass status

Show break glass status.

```bash
bib admin break-glass status
```

#### admin break-glass acknowledge

Acknowledge a completed session.

```bash
bib admin break-glass acknowledge --session <session-id>
```

---

### user

Manage user identity.

```bash
bib user <subcommand>
```

#### user whoami

Show current identity.

```bash
bib user whoami
```

**Example:**
```
User ID: abc123def456...
Name: Your Name
Email: you@example.com
Public Key: ssh-ed25519 AAAA...
```

#### user keygen

Generate a new Ed25519 identity key.

```bash
bib user keygen [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | string | Output file path |
| `--force` | bool | Overwrite existing key |

#### user export-key

Export your public key.

```bash
bib user export-key [flags]
```

**Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--output` | string | Output file path |

---

## Output Formats

Use `--output` or `-o` to specify output format.

### text (Default)

Human-readable text output.

```bash
bib topic list
```
```
TOPIC ID    NAME        DATASETS   UPDATED
weather     Weather     15         2024-01-15
finance     Finance     42         2024-01-14
```

### json

JSON output for scripting and automation.

```bash
bib topic list -o json
```
```json
[
  {"id": "weather", "name": "Weather", "dataset_count": 15, "updated_at": "2024-01-15T14:30:00Z"},
  {"id": "finance", "name": "Finance", "dataset_count": 42, "updated_at": "2024-01-14T10:00:00Z"}
]
```

### yaml

YAML output.

```bash
bib topic list -o yaml
```
```yaml
- id: weather
  name: Weather
  dataset_count: 15
  updated_at: "2024-01-15T14:30:00Z"
- id: finance
  name: Finance
  dataset_count: 42
  updated_at: "2024-01-14T10:00:00Z"
```

### table

Formatted table with borders.

```bash
bib topic list -o table
```
```
┌───────────┬─────────┬──────────┬─────────────┐
│ TOPIC ID  │ NAME    │ DATASETS │ UPDATED     │
├───────────┼─────────┼──────────┼─────────────┤
│ weather   │ Weather │ 15       │ 2024-01-15  │
│ finance   │ Finance │ 42       │ 2024-01-14  │
└───────────┴─────────┴──────────┴─────────────┘
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |
| `2` | Invalid usage or arguments |
| `3` | Configuration error |
| `4` | Connection error (daemon unreachable) |
| `5` | Authentication error |
| `6` | Permission denied |

**Example: Checking exit code in scripts**

```bash
if bib dataset download my-dataset --output ./data/; then
  echo "Download successful"
else
  echo "Download failed with code $?"
fi
```

---

## Shell Completion

Generate shell completion scripts for tab-completion support.

### Bash

```bash
# Add to ~/.bashrc or install system-wide
bib completion bash > /etc/bash_completion.d/bib

# Or add directly to bashrc
echo 'source <(bib completion bash)' >> ~/.bashrc
```

### Zsh

```bash
# Add to zsh completions
bib completion zsh > "${fpath[1]}/_bib"

# Or add directly to zshrc
echo 'source <(bib completion zsh)' >> ~/.zshrc
```

### Fish

```bash
bib completion fish > ~/.config/fish/completions/bib.fish
```

### PowerShell

```powershell
bib completion powershell > bib.ps1
. .\bib.ps1
```

---

## Examples

### First-Time Setup

```bash
# Configure CLI interactively
bib setup

# Verify configuration
bib config show

# Check version
bib version
```

### Working with Data

```bash
# List available topics
bib topic list

# Create a new topic
bib topic create weather --description "Weather data collection"

# Create a dataset
bib dataset create daily-temps \
  --topic weather \
  --file ./temps.csv

# Download a dataset
bib dataset download daily-temps --output ./data/

# Query data
bib query sql "SELECT * FROM temps WHERE temp > 90" \
  --dataset daily-temps=temps
```

### Running Jobs

```bash
# List available tasks
bib task list

# Submit a job
bib job submit --task fetch-weather \
  --input location="NYC" \
  --input date="2024-01-15"

# Monitor job status
bib job status job-abc123

# View logs
bib job logs job-abc123 --follow

# Cancel if needed
bib job cancel job-abc123
```

### Managing Subscriptions (Selective Mode)

```bash
# Add subscriptions
bib subscribe add "weather/*"
bib subscribe add "finance/stocks"

# List subscriptions
bib subscribe list

# Check sync status
bib sync status

# Remove subscription
bib subscribe remove "finance/stocks"
```

### Cluster Management

```bash
# Check cluster status
bib cluster status

# Generate join token for new node
bib cluster token generate --expires 1h

# List members
bib cluster members

# Remove failed node
bib cluster remove node-xyz
```

---

## Related Documentation

| Document | Topic |
|----------|-------|
| [Quick Start](../getting-started/quickstart.md) | Getting started with bib |
| [Configuration](../getting-started/configuration.md) | Configuration options |
| [Jobs & Tasks](jobs-tasks.md) | Job execution system |
| [Clustering](clustering.md) | HA cluster setup |

