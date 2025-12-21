# Quick Start Guide

Get up and running with Bib in under 5 minutes. This guide walks you through installation, configuration, and your first interactions with the distributed data network.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [Starting the Daemon](#starting-the-daemon)
5. [Verifying Your Setup](#verifying-your-setup)
6. [Understanding Node Modes](#understanding-node-modes)
7. [Next Steps](#next-steps)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before you begin, ensure you have:

| Requirement | Description |
|-------------|-------------|
| **Go 1.25+** | Required for building from source |
| **Network Access** | Outbound connectivity for P2P networking |
| **Terminal** | Command-line access (bash, zsh, or similar) |

### Optional (for Production)

| Requirement | Description |
|-------------|-------------|
| **Docker/Podman** | For managed PostgreSQL storage |
| **PostgreSQL 16+** | For full replication mode |

---

## Installation

### Option 1: Homebrew (macOS/Linux)

```bash
# Add the tap
brew tap bencoepp/bib

# Install CLI
brew install bib

# Install daemon
brew install bibd

# Verify installation
bib version
bibd -version
```

### Option 2: Windows (winget)

```powershell
# Install CLI
winget install bencoepp.bib

# Install daemon
winget install bencoepp.bibd

# Verify installation
bib version
bibd -version
```

### Option 3: Linux Packages

**Debian/Ubuntu (.deb):**
```bash
# Download from GitHub releases
curl -LO https://github.com/bencoepp/bib/releases/latest/download/bibd_VERSION_linux_amd64.deb

# Install
sudo dpkg -i bibd_VERSION_linux_amd64.deb
```

**RHEL/Fedora/CentOS (.rpm):**
```bash
# Download from GitHub releases
curl -LO https://github.com/bencoepp/bib/releases/latest/download/bibd-VERSION.x86_64.rpm

# Install
sudo rpm -i bibd-VERSION.x86_64.rpm
```

### Option 4: Docker

```bash
# Pull the daemon image
docker pull ghcr.io/bencoepp/bibd:latest

# Run the daemon
docker run -d --name bibd \
  -v bibd-data:/data \
  -p 8080:8080 \
  -p 4001:4001 \
  ghcr.io/bencoepp/bibd:latest
```

### Option 5: Build from Source

```bash
# Clone the repository
git clone https://github.com/bencoepp/bib.git
cd bib

# Build using Make
make build

# Or build directly with Go
go build -o bib ./cmd/bib
go build -o bibd ./cmd/bibd

# Move to your PATH
sudo mv bin/bib bin/bibd /usr/local/bin/

# Verify installation
bib version
bibd -version
```

**Expected output:**
```
bib version 1.0.0
  commit:  abc1234
  built:   2024-01-15T10:30:00Z
```


---

## Configuration

Bib provides two setup modes: **Quick Start** for getting running fast, and **Guided Setup** for full configuration.

> 📘 For comprehensive setup documentation, see [Setup Flow](setup-flow.md).

### Important Notes

- **bib CLI does NOT require a local bibd instance**. You can connect to remote nodes or the public `bib.dev` network.
- **Local bibd is encouraged** for best performance and offline capability, but not required.
- **bibd can be deployed** locally, in Docker/Podman containers, or on Kubernetes.

### First Run Behavior

When you run `bib` for the first time without configuration:

1. **Auto-detection**: bib discovers local and nearby bibd instances (mDNS, localhost scan)
2. **Node selection**: Choose which nodes to connect to (can select multiple)
3. **bib.dev option**: Connect to public network (requires explicit confirmation)
4. **Setup wizard**: Configure identity, output preferences, and test connections

### Option 1: Quick Start (Recommended for New Users)

Get running in seconds with minimal prompts:

```bash
# Quick CLI setup
bib setup --quick

# Quick daemon setup (local, Proxy mode)
bib setup --daemon --quick

# Quick daemon setup with Docker
bib setup --daemon --quick --target docker

# Quick daemon setup with Podman
bib setup --daemon --quick --target podman
```

Quick Start:
- Prompts only for name and email
- Generates Ed25519 identity key automatically
- Discovers and connects to local/nearby nodes
- Requires confirmation for bib.dev connection
- Uses sensible defaults (Proxy mode, SQLite, public bootstrap)
- Starts bibd/containers immediately after daemon setup

### Option 2: Guided Setup

Full interactive wizard with all configuration options:

```bash
# CLI setup
bib setup

# Daemon setup (local)
bib setup --daemon

# Daemon setup (Docker)
bib setup --daemon --target docker

# Daemon setup (Kubernetes)
bib setup --daemon --target kubernetes
```

**CLI Setup Steps:**

| Step | Description |
|------|-------------|
| **Identity** | Name, email → generates `~/.config/bib/identity.pem` |
| **Output** | Default format (table/json/yaml), colors |
| **Node Discovery** | Scan for local/nearby bibd instances |
| **Node Selection** | Choose nodes to connect to (multi-select) |
| **bib.dev Confirmation** | Explicit confirmation for public network |
| **Logging** | Log verbosity level |
| **Connection Test** | Verify connectivity to all selected nodes |
| **Auth Test** | Authenticate with generated identity |
| **Network Health** | Check peer connections |

**Daemon Setup Steps:**

| Step | Description |
|------|-------------|
| **Deployment Target** | Local, Docker, Podman, or Kubernetes |
| **Identity** | Node name, admin email |
| **Server** | Host, port, data directory/volumes |
| **TLS/Security** | TLS setup, certificate pinning, hardening options |
| **Storage** | SQLite or PostgreSQL (options vary by deployment target) |
| **P2P Mode** | Proxy, Selective, or Full (with mode-specific config) |
| **Bootstrap** | Public (bib.dev, requires confirmation) + custom peers |
| **Clustering** | Optional HA cluster configuration |
| **Break Glass** | Optional emergency access |
| **Deployment** | Generate files, start containers/service, verify |

**Navigation:**
- `Tab` / `↓` — Next field
- `Shift+Tab` / `↑` — Previous field
- `Enter` — Proceed to next step
- `Esc` — Go back to previous step
- `Ctrl+C` — Save progress and exit (can resume later)

### Deployment Targets

| Target | Command | Description |
|--------|---------|-------------|
| Local | `bib setup --daemon` | Run bibd directly on host |
| Docker | `bib setup --daemon --target docker` | Run in Docker containers |
| Podman | `bib setup --daemon --target podman` | Run in Podman (rootful/rootless) |
| Kubernetes | `bib setup --daemon --target kubernetes` | Deploy to K8s cluster |

### Configuration File Locations

| Platform | CLI Config | Daemon Config |
|----------|------------|---------------|
| **macOS** | `~/.config/bib/config.yaml` | `~/.config/bibd/config.yaml` |
| **Linux** | `~/.config/bib/config.yaml` | `~/.config/bibd/config.yaml` |
| **Windows** | `%APPDATA%\bib\config.yaml` | `%APPDATA%\bibd\config.yaml` |

### Identity Keys

Setup generates an Ed25519 keypair for authentication, stored separately from SSH keys:

| Purpose | Location |
|---------|----------|
| **bib CLI identity** | `~/.config/bib/identity.pem` |
| **bibd P2P identity** | `~/.config/bibd/identity.pem` |

---

## Starting the Daemon

After `bib setup --daemon`, bibd starts automatically. For manual control:

### Basic Start

```bash
bibd
```

**Expected output:**
```
INFO starting bibd                           host=0.0.0.0 port=8080 log_level=info
INFO P2P networking enabled                  mode=proxy listen=/ip4/0.0.0.0/tcp/4001
INFO connecting to bootstrap peers           count=1
INFO daemon ready                            peer_id=QmXyz...
```

### Run as a Background Service

#### Using systemd (Linux)

Create `/etc/systemd/system/bibd.service`:

```ini
[Unit]
Description=Bib Daemon
After=network.target

[Service]
Type=simple
User=bibd
ExecStart=/usr/local/bin/bibd
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable bibd
sudo systemctl start bibd
sudo systemctl status bibd
```

#### Using launchd (macOS)

Create `~/Library/LaunchAgents/dev.bib.bibd.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>dev.bib.bibd</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/bibd</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

Load and start:

```bash
launchctl load ~/Library/LaunchAgents/dev.bib.bibd.plist
```

---

## Verifying Your Setup

### Test CLI-to-Daemon Connection

In a separate terminal:

```bash
bib version
```

If the daemon is running and accessible, this should complete without errors.

### View Configuration

```bash
# Show current CLI configuration
bib config show

# Show configuration file path
bib config path
```

### Check Daemon Status (Future Command)

```bash
# View daemon status and connected peers
bib status
```

---

## Understanding Node Modes

Bib supports three operational modes that determine how your node participates in the network:

| Mode | Local Storage | Sync Behavior | Best For |
|------|--------------|---------------|----------|
| **Proxy** (default) | Cache only | Pass-through | Development, edge gateways |
| **Selective** | Subscribed topics | On-demand | Team nodes, domain-specific access |
| **Full** | Everything | Continuous | Archive nodes, data providers |

### Changing Modes

Edit `~/.config/bibd/config.yaml`:

```yaml
p2p:
  mode: selective  # Options: proxy, selective, full
```

Then restart bibd:

```bash
# If running manually
Ctrl+C
bibd

# If running as a service
sudo systemctl restart bibd
```

See [Node Modes](../concepts/node-modes.md) for detailed information about each mode.

---

## Next Steps

Now that you have Bib running, here's what to explore next:

### Explore the Network

```bash
# View available topics (future command)
bib topic list

# Search for datasets
bib catalog query --name "weather*"
```

### Create Your First Topic

```bash
# Create a topic for organizing related datasets
bib topic create weather --description "Weather data collection"

# Create a dataset within the topic
bib dataset create daily-temps --topic weather --file ./temps.csv
```

### Run a Data Processing Job

```bash
# View available task templates
bib task list

# Submit a job using a task
bib job submit --task ingest-csv --input file=./data.csv
```

### Set Up High Availability

For production deployments requiring fault tolerance:

```bash
# On the first node (bootstrap)
bib setup --daemon --cluster

# On additional nodes (join existing cluster)
bib setup --daemon --cluster-join <join-token>
```

See [Clustering Guide](../guides/clustering.md) for detailed instructions.

---

## Troubleshooting

### Connection Refused

**Symptom:** `bib version` fails with "connection refused"

**Solution:**
1. Verify bibd is running: `ps aux | grep bibd`
2. Check the server address in your config matches where bibd is listening
3. Ensure no firewall is blocking port 8080 (or your configured port)

### Daemon Won't Start

**Symptom:** bibd exits immediately after starting

**Solutions:**
1. Check for port conflicts: `lsof -i :8080`
2. Review logs: `bibd 2>&1 | head -50`
3. Verify configuration: Check `~/.config/bibd/config.yaml` for syntax errors

### P2P Connection Issues

**Symptom:** No peers connecting, "0 peers" in status

**Solutions:**
1. Check network connectivity to bootstrap nodes
2. Verify firewall allows outbound connections on ports 4001 (TCP/UDP)
3. If behind NAT, consider enabling UPnP in your router

### Configuration Errors

**Symptom:** "invalid configuration" errors

**Solution:**
1. Run `bib setup` or `bib setup --daemon` to regenerate configuration
2. Check YAML syntax (proper indentation, no tabs)
3. Review [Configuration Guide](configuration.md) for valid options

---

## What's Next?

| Guide | When to Read |
|-------|--------------|
| [Configuration Guide](configuration.md) | Deep dive into all configuration options |
| [Node Modes](../concepts/node-modes.md) | Understand proxy, selective, and full modes |
| [Architecture Overview](../concepts/architecture.md) | Learn how the system works internally |
| [CLI Reference](../guides/cli-reference.md) | Complete command documentation |


