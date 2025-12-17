# Quick Start Guide

Get up and running with bib in 5 minutes.

## Prerequisites

- Go 1.25 or later
- Network access (for P2P connectivity)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourorg/bib.git
cd bib

# Build both binaries
go build -o bib ./cmd/bib
go build -o bibd ./cmd/bibd

# Move to PATH
sudo mv bib bibd /usr/local/bin/
```

### Verify Installation

```bash
bib version
# bib version 0.1.0

bibd -version
# bibd version 0.1.0
```

## Quick Setup

### 1. Configure the CLI

```bash
bib setup
```

This launches an **interactive setup wizard** that guides you through configuration:
- **Identity** - Enter your name and email (for attribution)
- **Output** - Choose default format (table/json/yaml) and color preferences
- **Connection** - Specify bibd server address
- **Logging** - Select log verbosity level

Use `Tab` to move between fields, `Enter` to proceed, and `Esc` to go back. Configuration is saved to `~/.config/bib/config.yaml`.

### 2. Configure the Daemon

```bash
bib setup --daemon
```

The daemon wizard includes additional configuration:
- **Server** - Host, port, and data directory
- **TLS** - Optional TLS encryption
- **Storage** - SQLite (lightweight) or PostgreSQL (full replication)
- **P2P** - Enable networking and select mode (proxy/selective/full)
- **Cluster** - Optional high-availability clustering

This creates `~/.config/bibd/config.yaml`.

### 3. Start the Daemon

```bash
bibd
```

You should see:
```
INFO starting bibd host=0.0.0.0 port=8080 log_level=info
```

The daemon is now running and connecting to the P2P network.

### 4. Test the Connection

In another terminal:

```bash
bib version
# Should work without errors
```

## Understanding Node Modes

By default, bibd runs in **proxy mode** with minimal resource usage:

| Mode | Description |
|------|-------------|
| `proxy` (default) | No local storage, forwards requests to peers |
| `selective` | Sync only subscribed topics |
| `full` | Sync all data from network |

To change modes, edit `~/.config/bibd/config.yaml`:

```yaml
p2p:
  mode: selective  # or "full"
```

Then restart bibd.

## Next Steps

### Explore the Network

```bash
# List connected peers (future command)
bib peer list

# View available topics
bib topic list

# Search for datasets
bib catalog query --name "weather*"
```

### Create Your First Topic

```bash
# Create a topic
bib topic create weather --description "Weather data collection"

# Create a dataset
bib dataset create daily-temps --topic weather --file ./temps.csv
```

### Run a Job

```bash
# View available tasks
bib task list

# Run a data ingestion task
bib job submit --task ingest-csv --input file=./data.csv
```

### Set Up High Availability

For production deployments with multiple nodes:

```bash
# On first node
bib setup --daemon --cluster

# On additional nodes
bib setup --daemon --cluster-join <token>
```

## Common Commands

| Command | Description |
|---------|-------------|
| `bib setup` | Configure bib CLI |
| `bib setup --daemon` | Configure bibd |
| `bib config show` | Show current config |
| `bib version` | Show version info |
| `bibd` | Start the daemon |

## Troubleshooting

### Daemon Won't Start

```bash
# Check if port is in use
lsof -i :8080

# Try different port
# Edit ~/.config/bibd/config.yaml
server:
  port: 9090
```

### Can't Connect to Peers

```bash
# Check network connectivity
nc -zv bib.dev 4001

# Enable debug logging
# Edit ~/.config/bibd/config.yaml
log:
  level: debug
```

### Configuration Issues

```bash
# Show config file location
bib config path

# Reset to defaults
rm ~/.config/bib/config.yaml
bib setup
```

## Further Reading

- [Architecture Overview](architecture.md)
- [Configuration Guide](configuration.md)
- [Node Modes](node-modes.md)
- [CLI Reference](cli-reference.md)
- [P2P Networking](p2p-networking.md)
- [Clustering](clustering.md)

### For Developers

- [Developer Guide](developer-guide.md) - Complete codebase overview
- [TUI Component System](tui-components.md) - Terminal UI components and themes

