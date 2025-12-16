# CLI Reference

This document provides a complete reference for the `bib` command-line interface.

## Overview

The `bib` CLI is the primary user interface for interacting with the bib ecosystem. It communicates with the `bibd` daemon via gRPC.

## Global Flags

These flags are available for all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | | Path to config file (default: `~/.config/bib/config.yaml`) |
| `--output` | `-o` | Output format: text, json, yaml, table |
| `--help` | `-h` | Show help for command |

## Commands

### Root Command

```bash
bib [flags] [command]
```

Running `bib` without arguments displays help.

---

### version

Print version information.

```bash
bib version
```

**Output:**
```
bib version 0.1.0
  commit:  abc1234
  built:   2024-01-15T10:30:00Z
```

---

### setup

Interactive configuration setup.

```bash
bib setup [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--daemon` | `-d` | false | Configure bibd daemon instead of bib CLI |
| `--format` | `-f` | yaml | Config file format: yaml, toml, json |
| `--cluster` | | false | Initialize new HA cluster (with --daemon) |
| `--cluster-join` | | | Join existing cluster with token (with --daemon) |

**Examples:**

```bash
# Configure bib CLI interactively
bib setup

# Configure bibd daemon
bib setup --daemon

# Initialize HA cluster
bib setup --daemon --cluster

# Join existing cluster
bib setup --daemon --cluster-join <token>
```

**Interactive Prompts (CLI):**
- Your name
- Your email
- Default output format
- Enable colored output
- bibd server address
- Log level

**Interactive Prompts (Daemon):**
- Daemon identity name
- Daemon identity email
- Listen host/port
- TLS settings
- Data directory
- Log level/format

---

### config

Manage configuration.

```bash
bib config [command]
```

#### config show

Display current configuration.

```bash
bib config show
```

**Output:**
```json
{
  "log": {
    "level": "info",
    "format": "text",
    ...
  },
  "identity": {
    "name": "Your Name",
    ...
  },
  ...
}
```

#### config path

Show configuration file path.

```bash
bib config path
```

**Output:**
```
/Users/you/.config/bib/config.yaml
```

---

## Planned Commands

The following commands are planned for future releases:

### topic

Manage topics.

```bash
# List topics
bib topic list

# Create topic
bib topic create <name> [--description <desc>] [--schema <file>]

# Show topic details
bib topic show <topic-id>

# Delete topic
bib topic delete <topic-id>
```

### dataset

Manage datasets.

```bash
# List datasets
bib dataset list [--topic <topic-id>]

# Create dataset
bib dataset create <name> --topic <topic-id> [--file <path>]

# Show dataset details
bib dataset show <dataset-id>

# Download dataset
bib dataset download <dataset-id> [--output <path>]

# Show versions
bib dataset versions <dataset-id>
```

### query

Query data.

```bash
# Metadata query
bib query metadata --topic <pattern> [--name <pattern>]

# SQL query
bib query sql "SELECT * FROM dataset_alias LIMIT 10" \
  --dataset <id>=<alias>
```

### job

Manage jobs.

```bash
# List jobs
bib job list [--status <status>]

# Submit job
bib job submit --task <task-id> [--input <name>=<value>]

# Show job status
bib job status <job-id>

# Cancel job
bib job cancel <job-id>

# View job logs
bib job logs <job-id>
```

### task

Manage reusable tasks.

```bash
# List tasks
bib task list

# Create task from file
bib task create <name> --file <instructions.yaml>

# Show task details
bib task show <task-id>

# Run task
bib task run <task-id> [--input <name>=<value>]
```

### peer

Manage P2P peers.

```bash
# List connected peers
bib peer list

# Show peer info
bib peer show <peer-id>

# Connect to peer
bib peer connect <multiaddr>

# Disconnect from peer
bib peer disconnect <peer-id>
```

### catalog

View catalog.

```bash
# List local catalog
bib catalog list

# Query catalog
bib catalog query --topic <pattern> --name <pattern>

# Sync catalog from peer
bib catalog sync <peer-id>
```

### sync

Manage synchronization.

```bash
# Show sync status
bib sync status

# Start sync
bib sync start

# Stop sync
bib sync stop
```

### subscribe

Manage subscriptions (selective mode).

```bash
# List subscriptions
bib subscribe list

# Add subscription
bib subscribe add <topic-pattern>

# Remove subscription
bib subscribe remove <topic-pattern>
```

### cluster

Manage HA cluster.

```bash
# Show cluster status
bib cluster status

# Generate join token
bib cluster token generate

# List members
bib cluster members

# Remove member
bib cluster remove <node-id>

# Promote non-voter
bib cluster promote <node-id>

# Demote voter
bib cluster demote <node-id>
```

### user

Manage user identity.

```bash
# Show current identity
bib user whoami

# Generate new identity
bib user keygen [--output <path>]

# Export public key
bib user export-key [--output <path>]
```

---

## Output Formats

Use `--output` or `-o` to specify output format:

### text (default)

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

JSON output for scripting.

```bash
bib topic list -o json
```
```json
[
  {"id": "weather", "name": "Weather", "dataset_count": 15},
  {"id": "finance", "name": "Finance", "dataset_count": 42}
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
- id: finance
  name: Finance
  dataset_count: 42
```

### table

Formatted table output.

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
| 0 | Success |
| 1 | General error |
| 2 | Invalid usage/arguments |
| 3 | Configuration error |
| 4 | Connection error |
| 5 | Authentication error |
| 6 | Permission denied |

---

## Environment Variables

Override configuration with environment variables:

| Variable | Description |
|----------|-------------|
| `BIB_CONFIG` | Path to config file |
| `BIB_LOG_LEVEL` | Log level |
| `BIB_SERVER` | bibd server address |
| `BIB_OUTPUT_FORMAT` | Default output format |

---

## Shell Completion

Generate shell completion scripts:

```bash
# Bash
bib completion bash > /etc/bash_completion.d/bib

# Zsh
bib completion zsh > "${fpath[1]}/_bib"

# Fish
bib completion fish > ~/.config/fish/completions/bib.fish

# PowerShell
bib completion powershell > bib.ps1
```

---

## Examples

### First-Time Setup

```bash
# Configure CLI
bib setup

# Check configuration
bib config show

# Verify version
bib version
```

### Working with Data

```bash
# List available topics
bib topic list

# Show topic details
bib topic show weather

# List datasets in topic
bib dataset list --topic weather

# Download a dataset
bib dataset download weather-2024 --output ./data/
```

### Running Jobs

```bash
# List available tasks
bib task list

# Run a data processing task
bib task run etl-daily --input date=2024-01-15

# Check job status
bib job status job-12345

# View job logs
bib job logs job-12345
```

### Cluster Management

```bash
# Check cluster status
bib cluster status

# Add new node
bib cluster token generate
# (use token on new node)

# Remove failed node
bib cluster remove node-xyz
```

