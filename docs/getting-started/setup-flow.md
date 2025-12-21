# Setup Flow

This document describes the complete setup and initialization flow for bib and bibd, covering first-time installation through to a fully operational node.

---

## Table of Contents

1. [Overview](#overview)
2. [First Run Behavior](#first-run-behavior)
3. [Quick Start Mode](#quick-start-mode)
4. [Guided Setup Mode](#guided-setup-mode)
5. [CLI Setup (bib)](#cli-setup-bib)
6. [Daemon Setup (bibd)](#daemon-setup-bibd)
7. [Mode-Specific Configuration](#mode-specific-configuration)
8. [Peer Connection & Bootstrap](#peer-connection--bootstrap)
9. [Security & Trust](#security--trust)
10. [Post-Setup Actions](#post-setup-actions)
11. [Error Recovery](#error-recovery)
12. [Reconfiguration](#reconfiguration)

---

## Overview

The bib setup process is designed to get users operational quickly while supporting advanced configurations for production deployments.

### Setup Philosophy

| Principle | Description |
|-----------|-------------|
| **Auto-detect** | Detect existing configurations and running daemons |
| **Progressive disclosure** | Simple defaults with optional deep customization |
| **Fail gracefully** | Save progress on failure, allow resume |
| **Verify everything** | Test connections, authentication, and network health |

### Components

| Component | Purpose | Setup Command |
|-----------|---------|---------------|
| **bib** | CLI client for interacting with bibd | `bib setup` |
| **bibd** | Background daemon for P2P, storage, jobs | `bib setup --daemon` |

---

## First Run Behavior

When a user runs `bib` for the first time (no configuration exists), the system follows this decision tree:

```
┌─────────────────────────────────────────────────────────────┐
│                     First Run Detection                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  bib <command>                                               │
│       │                                                      │
│       ▼                                                      │
│  ┌─────────────────┐                                        │
│  │ Config exists?  │                                        │
│  └────────┬────────┘                                        │
│           │                                                  │
│     No    │    Yes                                          │
│           │     └──────────► Execute command normally        │
│           ▼                                                  │
│  ┌─────────────────┐                                        │
│  │ Detect local    │                                        │
│  │ bibd running?   │                                        │
│  └────────┬────────┘                                        │
│           │                                                  │
│     No    │    Yes                                          │
│           │     │                                           │
│           │     ▼                                           │
│           │  ┌─────────────────────────────────────┐        │
│           │  │ "Local bibd detected at localhost:  │        │
│           │  │  4000. Would you like to connect?"  │        │
│           │  │                                     │        │
│           │  │  [Connect] [Setup New] [Cancel]     │        │
│           │  └─────────────────────────────────────┘        │
│           │                                                  │
│           ▼                                                  │
│  ┌─────────────────────────────────────────────────┐        │
│  │          Launch Setup Wizard                     │        │
│  │                                                  │        │
│  │  "No configuration found. Let's get started!"   │        │
│  │                                                  │        │
│  │  [Quick Start] [Guided Setup] [Cancel]          │        │
│  └─────────────────────────────────────────────────┘        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Detection Logic

1. **Check for config file**: Look for `~/.config/bib/config.yaml`
2. **Scan for local bibd**:
   - Check Unix socket: `/var/run/bibd.sock` or `~/.config/bibd/bibd.sock`
   - Check localhost ports: `4000`, `8080`
   - Query health endpoint if found
3. **Offer appropriate action** based on detection results

---

## Quick Start Mode

Quick Start creates a minimal working configuration in seconds, defaulting to **Proxy mode** for the daemon.

### CLI Quick Start

```bash
bib setup --quick
```

**Prompts:**
1. **Name**: Your display name
2. **Email**: Your email address

**Actions:**
- Generates Ed25519 identity key at `~/.config/bib/identity.pem`
- Creates minimal config pointing to `localhost:4000`
- Tests connection if bibd is detected

**Resulting Config:**

```yaml
# ~/.config/bib/config.yaml (quick start)
identity:
  name: "John Doe"
  email: "john@example.com"
  key: "~/.config/bib/identity.pem"

server: "localhost:4000"

output:
  format: table
  color: true

log:
  level: info
```

### Daemon Quick Start

```bash
bib setup --daemon --quick
```

**Prompts:**
1. **Name**: Node display name
2. **Email**: Admin contact email

**Actions:**
- Generates Ed25519 P2P identity at `~/.config/bibd/identity.pem`
- Creates Proxy mode configuration
- Connects to public bootstrap nodes (`bib.dev`)
- Starts bibd immediately
- Installs as system service (if permissions allow)

**Resulting Config:**

```yaml
# ~/.config/bibd/config.yaml (quick start)
identity:
  name: "My Node"
  email: "admin@example.com"

server:
  host: "0.0.0.0"
  port: 4000
  data_dir: "~/.local/share/bibd"

p2p:
  enabled: true
  mode: proxy
  identity:
    key_path: "~/.config/bibd/identity.pem"
  bootstrap:
    peers:
      - "/dns4/bib.dev/tcp/4001/p2p/QmBootstrap..."
      - "/dns4/bib.dev/udp/4001/quic-v1/p2p/QmBootstrap..."

database:
  backend: sqlite

log:
  level: info
  format: pretty
```

---

## Guided Setup Mode

Guided Setup walks through all configuration options with contextual help.

### Launching Guided Setup

```bash
# CLI setup
bib setup

# Daemon setup
bib setup --daemon
```

### Wizard Navigation

| Key | Action |
|-----|--------|
| `Tab` / `↓` | Next field |
| `Shift+Tab` / `↑` | Previous field |
| `Enter` | Proceed to next step |
| `Esc` | Go back to previous step |
| `Ctrl+C` | Cancel (prompts to save progress) |

### Progress Saving

If setup is interrupted (Ctrl+C, error, or system issue):

1. **Prompt to save**: "Save progress and exit? [Yes/No]"
2. **Save partial config**: Written to `~/.config/bib/config.yaml.partial`
3. **Resume later**: Next `bib setup` detects partial config and offers to resume

```
┌─────────────────────────────────────────────────────────────┐
│              Partial Configuration Detected                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  A previous setup was interrupted at step 5 of 12:          │
│  "P2P Mode Selection"                                        │
│                                                              │
│  [Resume] [Start Over] [Cancel]                              │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## CLI Setup (bib)

The CLI setup configures the `bib` command-line tool for interacting with a bibd daemon.

### Setup Steps

```
Step 1: Welcome
    │
    ▼
Step 2: Identity ─────────────────────────────────────────────┐
    │   • Name (required)                                      │
    │   • Email (required)                                     │
    │   → Generates ~/.config/bib/identity.pem                 │
    │                                                          │
    ▼                                                          │
Step 3: Output Preferences                                     │
    │   • Default format (table/json/yaml/text)               │
    │   • Color output (yes/no)                                │
    │                                                          │
    ▼                                                          │
Step 4: Connection                                             │
    │   • Server address (default: localhost:4000)             │
    │   • TLS enabled (auto-detect)                            │
    │                                                          │
    ▼                                                          │
Step 5: Logging                                                │
    │   • Log level (debug/info/warn/error)                    │
    │                                                          │
    ▼                                                          │
Step 6: Connection Test ──────────────────────────────────────┤
    │   • Attempt connection to configured daemon              │
    │   • If successful: show node info, peer count            │
    │   • If failed: offer to continue or reconfigure          │
    │                                                          │
    ▼                                                          │
Step 7: Authentication Test                                    │
    │   • Authenticate with generated identity key             │
    │   • If new user: auto-register (if enabled on server)    │
    │   • Show session info on success                         │
    │                                                          │
    ▼                                                          │
Step 8: Network Health Check                                   │
    │   • Query connected peers                                │
    │   • Show bootstrap connection status                     │
    │   • Display network summary                              │
    │                                                          │
    ▼                                                          │
Step 9: Confirmation & Save                                    │
        • Review all settings                                  │
        • Save configuration                                   │
        • Show next steps                                      │
────────────────────────────────────────────────────────────────
```

### Identity Key Generation

During setup, an Ed25519 keypair is generated for authentication:

```
┌─────────────────────────────────────────────────────────────┐
│                    🔑 Identity Generation                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Generating Ed25519 identity key...                          │
│                                                              │
│  ✓ Key generated successfully                                │
│                                                              │
│  Location:    ~/.config/bib/identity.pem                     │
│  Public Key:  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...       │
│  Fingerprint: SHA256:xYz123AbC456...                         │
│                                                              │
│  ⚠️  Keep your identity key secure! It authenticates you     │
│     to all bib nodes.                                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

The identity key is stored separately from SSH keys:

| Purpose | Location | Used For |
|---------|----------|----------|
| **bib identity** | `~/.config/bib/identity.pem` | bib ↔ bibd authentication |
| **SSH keys** | `~/.ssh/id_ed25519` | SSH access (unchanged) |

---

## Daemon Setup (bibd)

The daemon setup configures the bibd background service.

### Setup Steps Overview

```
Step 1: Welcome
    │
    ▼
Step 2: Identity ─────────────────────────────────────────────┐
    │   • Node name                                            │
    │   • Admin email                                          │
    │   → Generates P2P identity                               │
    │                                                          │
    ▼                                                          │
Step 3: Server Configuration                                   │
    │   • Listen host (default: 0.0.0.0)                       │
    │   • Listen port (default: 4000)                          │
    │   • Data directory                                       │
    │                                                          │
    ▼                                                          │
Step 4: TLS / Security Hardening                               │
    │   • Enable TLS (yes/no)                                  │
    │   • Certificate source (generate/provide)                │
    │   • Client certificate requirements                      │
    │   • Certificate pinning options                          │
    │                                                          │
    ▼                                                          │
Step 5: Storage Backend                                        │
    │   • SQLite (lightweight) or PostgreSQL (production)      │
    │   • If PostgreSQL: configuration wizard                  │
    │                                                          │
    ▼                                                          │
Step 6: P2P Networking                                         │
    │   • Enable P2P (yes/no)                                  │
    │   • Listen addresses                                     │
    │                                                          │
    ▼                                                          │
Step 7: P2P Mode Selection                                     │
    │   • Proxy / Selective / Full                             │
    │   → Mode-specific configuration (see below)              │
    │                                                          │
    ▼                                                          │
Step 8: Bootstrap Peers                                        │
    │   • Use public bootstrap (bib.dev)                       │
    │   • Add custom bootstrap peers                           │
    │                                                          │
    ▼                                                          │
Step 9: Logging                                                │
    │   • Log level and format                                 │
    │   • Audit logging                                        │
    │                                                          │
    ▼                                                          │
Step 10: Clustering (Optional)                                 │
    │   • Enable HA clustering                                 │
    │   • Cluster configuration                                │
    │                                                          │
    ▼                                                          │
Step 11: Break Glass (Optional)                                │
    │   • Emergency access configuration                       │
    │                                                          │
    ▼                                                          │
Step 12: Confirmation                                          │
    │   • Review all settings                                  │
    │   • Confirm configuration                                │
    │                                                          │
    ▼                                                          │
Step 13: Connectivity Test                                     │
    │   • Test bootstrap peer connectivity                     │
    │   • Verify P2P identity                                  │
    │                                                          │
    ▼                                                          │
Step 14: Deployment                                            │
        • Create system user (if needed)                       │
        • Install systemd/launchd service                      │
        • Start bibd                                           │
        • Verify startup                                       │
────────────────────────────────────────────────────────────────
```

---

## Mode-Specific Configuration

Each P2P mode has specific configuration requirements.

### Proxy Mode (Default)

Minimal configuration - no additional steps required.

```yaml
p2p:
  mode: proxy
  proxy:
    cache_ttl: 2m
    max_cache_size: 1000
```

### Selective Mode

Prompts for initial topic subscriptions:

```
┌─────────────────────────────────────────────────────────────┐
│                   Selective Mode Setup                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Subscribe to topics to sync their data locally.             │
│  You can add more subscriptions later with:                  │
│  bib subscribe add <topic>                                   │
│                                                              │
│  Initial Subscriptions (optional):                           │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ weather/*                                            │    │
│  │ myproject/data                                       │    │
│  │                                                      │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  [Add Topic] [Remove] [Continue without subscriptions]       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Full Mode

Requires PostgreSQL and extensive configuration confirmation.

```
┌─────────────────────────────────────────────────────────────┐
│                     Full Mode Setup                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ⚠️  Full mode requires PostgreSQL and significant storage.  │
│                                                              │
│  Requirements:                                               │
│  • PostgreSQL 16+ database                                   │
│  • Sufficient disk space for all network data               │
│  • Stable network connection                                 │
│                                                              │
│  PostgreSQL Configuration:                                   │
│                                                              │
│  ○ Managed (bibd runs PostgreSQL container)                 │
│  ○ External (connect to existing PostgreSQL)                │
│  ○ Kubernetes (deploy PostgreSQL to cluster)                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### Managed PostgreSQL Setup

```
┌─────────────────────────────────────────────────────────────┐
│                Managed PostgreSQL Setup                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  bibd will manage a PostgreSQL container for you.           │
│                                                              │
│  Container Runtime:                                          │
│  ● Docker (detected)                                         │
│  ○ Podman                                                    │
│                                                              │
│  PostgreSQL Image: postgres:16-alpine                        │
│  Data Directory:   ~/.local/share/bibd/postgres              │
│  Port:            5432 (internal only)                       │
│                                                              │
│  ✓ Docker is running and accessible                          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### External PostgreSQL Setup

```
┌─────────────────────────────────────────────────────────────┐
│               External PostgreSQL Setup                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Connection String:                                          │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ postgres://user:pass@localhost:5432/bibd            │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  Or configure individually:                                  │
│                                                              │
│  Host:     localhost                                         │
│  Port:     5432                                              │
│  Database: bibd                                              │
│  User:     bibd                                              │
│  Password: ********                                          │
│  SSL Mode: require                                           │
│                                                              │
│  [Test Connection]                                           │
│                                                              │
│  ✓ Connection successful                                     │
│    PostgreSQL 16.1, bibd database exists                     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### Kubernetes PostgreSQL Setup

```
┌─────────────────────────────────────────────────────────────┐
│              Kubernetes PostgreSQL Setup                     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Deploy PostgreSQL to your Kubernetes cluster.              │
│                                                              │
│  Kubernetes Context: my-cluster (detected)                   │
│  Namespace:          bibd                                    │
│                                                              │
│  Deployment Options:                                         │
│  ● Deploy new PostgreSQL StatefulSet                        │
│  ○ Use existing PostgreSQL service                          │
│  ○ Use CloudNativePG operator                               │
│                                                              │
│  Storage Class: standard (default)                           │
│  PVC Size:      50Gi                                         │
│                                                              │
│  [Deploy] [Show YAML] [Skip - Configure Later]               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### Full Mode Confirmation

Before proceeding, all settings must be confirmed:

```
┌─────────────────────────────────────────────────────────────┐
│              Full Mode Configuration Review                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Please review and confirm your Full mode settings:          │
│                                                              │
│  Database                                                    │
│  ├── Backend:     PostgreSQL (managed)                       │
│  ├── Runtime:     Docker                                     │
│  ├── Image:       postgres:16-alpine                         │
│  └── Data Dir:    ~/.local/share/bibd/postgres               │
│                                                              │
│  Replication                                                 │
│  ├── Mode:        Full                                       │
│  ├── Sync:        Continuous (5m interval)                   │
│  └── Storage:     ~50GB estimated (will grow)                │
│                                                              │
│  Network                                                     │
│  ├── Bootstrap:   bib.dev (public)                           │
│  └── Listen:      /ip4/0.0.0.0/tcp/4001                      │
│                                                              │
│  ⚠️  Full mode will sync ALL network data locally.           │
│     This requires significant disk space and bandwidth.      │
│                                                              │
│  [Confirm & Continue] [Modify Settings] [Cancel]             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Peer Connection & Bootstrap

### Bootstrap Peer Configuration

```
┌─────────────────────────────────────────────────────────────┐
│                   Bootstrap Peers                            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Bootstrap peers help your node discover the network.        │
│                                                              │
│  ☑ Use public bootstrap nodes (bib.dev)                      │
│                                                              │
│  Custom Bootstrap Peers (optional):                          │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ /dns4/node1.mycompany.com/tcp/4001/p2p/QmXyz...     │    │
│  │                                                      │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  [Add Peer] [Remove] [Test Connectivity]                     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Connectivity Test

The setup wizard tests connectivity to bootstrap peers:

```
┌─────────────────────────────────────────────────────────────┐
│                  Connectivity Test                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Testing connection to bootstrap peers...                    │
│                                                              │
│  ✓ bib.dev:4001 (TCP)     35ms                               │
│  ✓ bib.dev:4001 (QUIC)    28ms                               │
│  ✗ custom.peer:4001       Connection refused                 │
│                                                              │
│  2 of 3 bootstrap peers reachable                            │
│                                                              │
│  [Continue] [Retry Failed] [Edit Peers]                      │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Security & Trust

### TLS Configuration

```
┌─────────────────────────────────────────────────────────────┐
│                   TLS Configuration                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  TLS encrypts connections between clients and the daemon.    │
│                                                              │
│  Enable TLS?  ● Yes  ○ No                                    │
│                                                              │
│  Certificate Source:                                         │
│  ● Generate self-signed (easy, suitable for testing)        │
│  ○ Provide certificate files (production)                   │
│  ○ Use Let's Encrypt (requires public domain)               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Security Hardening (Production)

```
┌─────────────────────────────────────────────────────────────┐
│                  Security Hardening                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Additional security options for production deployments:     │
│                                                              │
│  ☐ Require client certificates                               │
│    Clients must present a valid certificate to connect       │
│                                                              │
│  ☐ Enable certificate pinning                                │
│    Pin specific certificates for enhanced security           │
│                                                              │
│  ☐ Strict TLS verification                                   │
│    Disable TLS fallback and require TLS 1.3                  │
│                                                              │
│  ☐ Enable audit logging                                      │
│    Log all authentication and data access events             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Trust-On-First-Use (TOFU)

When connecting to a new bibd for the first time:

```
┌─────────────────────────────────────────────────────────────┐
│                  New Node Detected                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ⚠️  First connection to this node                           │
│                                                              │
│  Node ID:      QmXyz123...                                   │
│  Address:      node1.example.com:4000                        │
│  Fingerprint:  SHA256:Ab12Cd34Ef56...                        │
│                                                              │
│  To verify this node, confirm the fingerprint matches        │
│  what the node administrator provided out-of-band.           │
│                                                              │
│  Trust this node?                                            │
│                                                              │
│  [Trust Once] [Trust & Save] [Cancel]                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Automatic Trust Flag:**

```bash
# Skip TOFU prompt and auto-trust on first connection
bib connect --trust-first-use node1.example.com:4000
```

---

## Post-Setup Actions

After configuration is complete, the wizard performs deployment actions.

### Daemon Deployment

```
┌─────────────────────────────────────────────────────────────┐
│                     Deployment                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Setting up bibd...                                          │
│                                                              │
│  ✓ Configuration saved to ~/.config/bibd/config.yaml        │
│  ✓ Data directory created: ~/.local/share/bibd              │
│  ✓ P2P identity generated                                    │
│  ⠋ Creating system service...                                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Deployment Steps:**

1. **Save configuration** to `~/.config/bibd/config.yaml`
2. **Create data directories** with proper permissions
3. **Generate identity keys** (P2P and authentication)
4. **Install system service**:
   - **Linux**: Create systemd service, enable and start
   - **macOS**: Create launchd plist, load and start
   - **Windows**: Create Windows Service, start
5. **Start bibd** and verify it's running
6. **Run health check** to confirm operational

### Service Installation (Linux)

```
┌─────────────────────────────────────────────────────────────┐
│                  Service Installation                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Install bibd as a system service?                           │
│                                                              │
│  This will:                                                  │
│  • Create systemd service file                               │
│  • Enable automatic startup on boot                          │
│  • Start bibd immediately                                    │
│                                                              │
│  ● Install as user service (~/.config/systemd/user/)        │
│  ○ Install as system service (requires sudo)                │
│  ○ Don't install service (manual start only)                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Verification

```
┌─────────────────────────────────────────────────────────────┐
│                  Setup Complete! ✓                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  bibd is now running and connected to the network.          │
│                                                              │
│  Status:                                                     │
│  ├── Service:     Active (running)                           │
│  ├── PID:         12345                                      │
│  ├── Uptime:      5 seconds                                  │
│  ├── Mode:        Selective                                  │
│  ├── Peers:       3 connected                                │
│  └── Health:      Healthy                                    │
│                                                              │
│  Endpoints:                                                  │
│  ├── gRPC:        localhost:4000                             │
│  └── P2P:         /ip4/0.0.0.0/tcp/4001                      │
│                                                              │
│  Your Node ID:    QmXyz123...                                │
│                                                              │
│  Next Steps:                                                 │
│  • Run `bib status` to check node status                     │
│  • Run `bib topic list` to see available topics              │
│  • Run `bib help` for all available commands                 │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### CLI Post-Setup Verification

For CLI setup, verification includes:

```
┌─────────────────────────────────────────────────────────────┐
│                  CLI Setup Complete! ✓                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  bib CLI is configured and connected.                        │
│                                                              │
│  Connection Test:                                            │
│  ├── Server:      localhost:4000 ✓                           │
│  ├── TLS:         Enabled ✓                                  │
│  └── Latency:     2ms                                        │
│                                                              │
│  Authentication Test:                                        │
│  ├── Identity:    ~/.config/bib/identity.pem ✓               │
│  ├── User:        john@example.com                           │
│  ├── Role:        user                                       │
│  └── Session:     Active ✓                                   │
│                                                              │
│  Network Health:                                             │
│  ├── Connected Peers:  5                                     │
│  ├── Bootstrap:        2/2 connected                         │
│  └── DHT Status:       Healthy                               │
│                                                              │
│  You're all set! Try these commands:                         │
│  • bib status        - Check daemon status                   │
│  • bib topic list    - Browse available topics               │
│  • bib catalog query - Search for datasets                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Error Recovery

### Partial Configuration Save

If setup fails or is interrupted, progress is saved:

```
┌─────────────────────────────────────────────────────────────┐
│                  Setup Interrupted                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Setup was interrupted at step 7 (P2P Mode Selection).       │
│                                                              │
│  Your progress has been saved. You can:                      │
│                                                              │
│  • Resume setup:     bib setup --daemon                      │
│  • Start over:       bib setup --daemon --fresh              │
│  • View saved:       cat ~/.config/bibd/config.yaml.partial  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Error Handling

When an error occurs during setup:

```
┌─────────────────────────────────────────────────────────────┐
│                  Configuration Error                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ✗ PostgreSQL connection failed                              │
│                                                              │
│  Error: connection refused (localhost:5432)                  │
│                                                              │
│  This step requires PostgreSQL for Full mode.                │
│                                                              │
│  Options:                                                    │
│  [Retry]                    Try connecting again             │
│  [Configure PostgreSQL]     Change connection settings       │
│  [Switch to Selective]      Use Selective mode instead       │
│  [Save & Exit]              Save progress and exit           │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Resume Points

Each step is a resume point. The partial config tracks:

```yaml
# ~/.config/bibd/config.yaml.partial
_setup_metadata:
  version: 1
  started_at: "2024-01-15T10:30:00Z"
  last_step: "p2p-mode"
  last_step_index: 7
  total_steps: 14

# Completed configuration so far...
identity:
  name: "My Node"
  email: "admin@example.com"

server:
  host: "0.0.0.0"
  port: 4000
  # ...
```

---

## Reconfiguration

### Modify Individual Settings

Use `bib setup --reconfigure` to change specific settings without running the full wizard:

```bash
# Reconfigure specific sections
bib setup --reconfigure identity
bib setup --reconfigure p2p
bib setup --reconfigure storage

# Daemon reconfiguration
bib setup --daemon --reconfigure p2p-mode
bib setup --daemon --reconfigure cluster
```

### Interactive Reconfiguration

```
┌─────────────────────────────────────────────────────────────┐
│                   Reconfigure bib                            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Select sections to reconfigure:                             │
│                                                              │
│  ☐ Identity (name, email, key)                               │
│  ☐ Output (format, colors)                                   │
│  ☐ Connection (server address, TLS)                          │
│  ☐ Logging (level, format)                                   │
│                                                              │
│  Current Configuration:                                      │
│  ├── Identity:   John Doe <john@example.com>                 │
│  ├── Server:     localhost:4000                              │
│  ├── Output:     table, colors enabled                       │
│  └── Log Level:  info                                        │
│                                                              │
│  [Select All] [Continue] [Cancel]                            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Configuration Reset

```bash
# Reset to defaults and run fresh setup
bib setup --fresh
bib setup --daemon --fresh

# Reset specific sections
bib config reset p2p
bib config reset --all
```

---

## Command Reference

### Setup Commands

| Command | Description |
|---------|-------------|
| `bib setup` | Interactive CLI setup wizard |
| `bib setup --quick` | Quick start with minimal prompts |
| `bib setup --daemon` | Interactive daemon setup wizard |
| `bib setup --daemon --quick` | Quick daemon setup (Proxy mode) |
| `bib setup --daemon --cluster` | Initialize new HA cluster |
| `bib setup --daemon --cluster-join <token>` | Join existing cluster |
| `bib setup --reconfigure [section]` | Reconfigure specific sections |
| `bib setup --fresh` | Reset and start fresh |

### Connection Commands

| Command | Description |
|---------|-------------|
| `bib connect <address>` | Connect to a bibd daemon |
| `bib connect --save` | Save as default connection |
| `bib connect --trust-first-use` | Auto-trust on first connection |
| `bib connect --test` | Test connection only |

### Trust Commands

| Command | Description |
|---------|-------------|
| `bib trust list` | List trusted nodes |
| `bib trust add <node-id>` | Manually trust a node |
| `bib trust remove <node-id>` | Remove trust for a node |
| `bib trust pin <node-id>` | Pin a node's certificate |

---

## Related Documentation

| Document | Topic |
|----------|-------|
| [Quick Start](quickstart.md) | Get started in 5 minutes |
| [Configuration](configuration.md) | All configuration options |
| [Node Modes](../concepts/node-modes.md) | Proxy, Selective, Full modes |
| [Authentication](../concepts/authentication.md) | Auth flow and keys |
| [Clustering](../guides/clustering.md) | HA cluster setup |

