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
7. [Deployment Targets](#deployment-targets)
8. [Mode-Specific Configuration](#mode-specific-configuration)
9. [Peer Connection & Bootstrap](#peer-connection--bootstrap)
10. [Security & Trust](#security--trust)
11. [Post-Setup Actions](#post-setup-actions)
12. [Error Recovery](#error-recovery)
13. [Reconfiguration](#reconfiguration)

---

## Overview

The bib setup process is designed to get users operational quickly while supporting advanced configurations for production deployments.

### Setup Philosophy

| Principle | Description |
|-----------|-------------|
| **Auto-detect** | Detect existing configurations, running daemons, and nearby peers |
| **Progressive disclosure** | Simple defaults with optional deep customization |
| **Fail gracefully** | Save progress on failure, allow resume |
| **Verify everything** | Test connections, authentication, and network health |

### Components

| Component | Purpose | Setup Command |
|-----------|---------|---------------|
| **bib** | CLI client for interacting with bibd nodes | `bib setup` |
| **bibd** | Background daemon for P2P, storage, jobs | `bib setup --daemon` |

### Important Notes

- **bib CLI does NOT require a local bibd instance**. Users can connect to remote bibd nodes, including the public `bib.dev` network.
- **Local bibd is encouraged** for best performance and offline capability, but not required.
- **bibd can be deployed** locally, in Docker/Podman containers, or on Kubernetes.

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

### Detection & Discovery

During CLI setup, bib discovers available bibd instances using multiple methods:

| Method | Scope | Description |
|--------|-------|-------------|
| **Localhost scan** | Local machine | Check ports 4000, 8080 on localhost |
| **Unix socket** | Local machine | Check `/var/run/bibd.sock`, `~/.config/bibd/bibd.sock` |
| **mDNS** | Local network | Discover `_bib._tcp.local` services |
| **P2P Discovery** | Nearby peers | DHT-based peer discovery |

### Node Selection

After discovery, the wizard presents all found nodes plus the public bib.dev network:

```
┌─────────────────────────────────────────────────────────────┐
│                   Connect to bibd Nodes                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Select bibd nodes to connect to.                            │
│  You can select multiple nodes for redundancy.               │
│                                                              │
│  Discovered Nodes:                                           │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ [✓] localhost:4000          (local, 2ms)            │    │
│  │ [ ] 192.168.1.50:4000       (LAN, mDNS, 5ms)        │    │
│  │ [ ] workstation.local:4000  (LAN, mDNS, 8ms)        │    │
│  │ [ ] 10.0.0.25:4000          (nearby peer, 15ms)     │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  Public Network:                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ [ ] bib.dev                 (public bootstrap)       │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  [Select All Local] [Add Custom...] [Continue]               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### bib.dev Connection Confirmation

If the user selects `bib.dev`, explicit confirmation is required:

```
┌─────────────────────────────────────────────────────────────┐
│              Connect to Public Network (bib.dev)             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  You've selected to connect to the public bib.dev network.   │
│                                                              │
│  This will:                                                  │
│  • Connect you to the global bib peer-to-peer network       │
│  • Allow access to public datasets and topics               │
│  • Enable discovery of other public nodes                   │
│                                                              │
│  ⚠️  Data you publish will be visible to other network       │
│     participants unless you run your own private bibd.       │
│                                                              │
│  Confirm connection to bib.dev?                              │
│                                                              │
│  [Yes, Connect] [No, Skip] [Learn More]                      │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### No Nodes Found

If no local nodes are discovered:

```
┌─────────────────────────────────────────────────────────────┐
│                   No Local Nodes Found                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  No bibd instances were detected on your local machine       │
│  or network.                                                 │
│                                                              │
│  Options:                                                    │
│                                                              │
│  ● Connect to bib.dev (public network)                       │
│    Access the global bib network without running bibd        │
│                                                              │
│  ○ Set up local bibd                                         │
│    Run your own bibd instance for best performance           │
│                                                              │
│  ○ Enter custom address                                      │
│    Connect to a specific bibd node                           │
│                                                              │
│  [Continue]                                                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Quick Start Mode

Quick Start creates a minimal working configuration in seconds.

### CLI Quick Start

```bash
bib setup --quick
```

**Prompts:**
1. **Name**: Your display name
2. **Email**: Your email address
3. **bib.dev confirmation**: Confirm connection to public network (if no local nodes found)

**Discovery:**
- Scans for local/nearby bibd instances
- If found: automatically connects to local nodes
- If not found: prompts to connect to bib.dev

**Actions:**
- Generates Ed25519 identity key at `~/.config/bib/identity.pem`
- Discovers and connects to available nodes
- If connecting to bib.dev: requires explicit confirmation
- Tests connection to selected nodes

**Resulting Config:**

```yaml
# ~/.config/bib/config.yaml (quick start)
identity:
  name: "John Doe"
  email: "john@example.com"
  key: "~/.config/bib/identity.pem"

# Multiple nodes can be configured
nodes:
  - address: "localhost:4000"
    alias: "local"
    default: true
  - address: "bib.dev:4000"
    alias: "public"

# Legacy single-server fallback
server: "localhost:4000"

output:
  format: table
  color: true

log:
  level: info
```

### Daemon Quick Start

```bash
# Local deployment (default)
bib setup --daemon --quick

# Docker/Podman deployment
bib setup --daemon --quick --target docker
bib setup --daemon --quick --target podman

# Kubernetes deployment
bib setup --daemon --quick --target kubernetes
```

**Prompts:**
1. **Name**: Node display name
2. **Email**: Admin contact email
3. **Deployment target** (if not specified via flag): Local / Docker / Podman / Kubernetes

**Actions (varies by target):**

| Target | Actions |
|--------|---------|
| **Local** | Generate config, create systemd/launchd service, start bibd |
| **Docker** | Generate docker-compose.yaml, run `docker compose up -d` |
| **Podman** | Generate podman-compose.yaml or pod, run containers |
| **Kubernetes** | Generate manifests, optionally apply with kubectl |

**Quick Start Defaults:**
- Proxy mode (no PostgreSQL required)
- SQLite backend
- Public bootstrap (bib.dev)
- Minimal resource usage

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

The CLI setup configures the `bib` command-line tool for interacting with bibd nodes.

> **Note:** Running a local bibd instance is encouraged for best performance and offline capability, but is **not required**. You can connect to remote bibd nodes or the public bib.dev network.

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
Step 4: Node Discovery ───────────────────────────────────────┤
    │   • Scan localhost, mDNS, nearby peers                   │
    │   • Display discovered nodes with latency                │
    │   • Show bib.dev public network option                   │
    │                                                          │
    ▼                                                          │
Step 5: Node Selection                                         │
    │   • Multi-select from discovered nodes                   │
    │   • Add custom node addresses                            │
    │   • Confirm bib.dev connection (if selected)             │
    │   • Set default node                                     │
    │                                                          │
    ▼                                                          │
Step 6: Logging                                                │
    │   • Log level (debug/info/warn/error)                    │
    │                                                          │
    ▼                                                          │
Step 7: Connection Test ──────────────────────────────────────┤
    │   • Test connectivity to all selected nodes              │
    │   • Show node info, version, peer count                  │
    │   • If failed: offer to retry or remove node             │
    │                                                          │
    ▼                                                          │
Step 8: Authentication Test                                    │
    │   • Authenticate with generated identity key             │
    │   • Register on each node (if auto-registration enabled) │
    │   • Show session info on success                         │
    │                                                          │
    ▼                                                          │
Step 9: Network Health Check                                   │
    │   • Query connected peers on each node                   │
    │   • Show bootstrap connection status                     │
    │   • Display network summary                              │
    │                                                          │
    ▼                                                          │
Step 10: Confirmation & Save                                   │
        • Review all settings and connected nodes              │
        • Save configuration                                   │
        • Show next steps                                      │
────────────────────────────────────────────────────────────────
```

### Node Discovery Details

The wizard discovers bibd instances using multiple methods:

```
┌─────────────────────────────────────────────────────────────┐
│                   🔍 Discovering Nodes...                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Scanning for bibd instances...                              │
│                                                              │
│  ✓ Localhost scan       Found 1 instance                     │
│  ✓ mDNS discovery       Found 2 instances                    │
│  ✓ Peer discovery       Found 1 nearby peer                  │
│                                                              │
│  4 nodes discovered in 2.3 seconds                           │
│                                                              │
└─────────────────────────────────────────────────────────────┘
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

The daemon setup configures the bibd background service. bibd can be deployed in multiple ways depending on your environment.

### Deployment Targets

| Target | Description | PostgreSQL Options |
|--------|-------------|-------------------|
| **Local** | Run bibd directly on host | Any (local, remote, container-managed) |
| **Docker** | Run bibd in Docker container | Separate container in same compose |
| **Podman** | Run bibd in Podman container (rootful or rootless) | Separate container in same pod/compose |
| **Kubernetes** | Deploy bibd to K8s cluster | StatefulSet, CloudNativePG, or external |

### Setup Steps Overview

```
Step 1: Welcome
    │
    ▼
Step 2: Deployment Target ────────────────────────────────────┐
    │   • Local / Docker / Podman / Kubernetes                 │
    │   → Determines subsequent configuration options          │
    │                                                          │
    ▼                                                          │
Step 3: Identity                                               │
    │   • Node name                                            │
    │   • Admin email                                          │
    │   → Generates P2P identity                               │
    │                                                          │
    ▼                                                          │
Step 4: Server Configuration                                   │
    │   • Listen host/port (varies by target)                  │
    │   • Data directory/volume configuration                  │
    │                                                          │
    ▼                                                          │
Step 5: TLS / Security Hardening                               │
    │   • Enable TLS (yes/no)                                  │
    │   • Certificate source (generate/provide)                │
    │   • Client certificate requirements                      │
    │   • Certificate pinning options                          │
    │                                                          │
    ▼                                                          │
Step 6: Storage Backend                                        │
    │   • SQLite (lightweight) or PostgreSQL (production)      │
    │   • PostgreSQL setup (varies by deployment target)       │
    │                                                          │
    ▼                                                          │
Step 7: P2P Networking                                         │
    │   • Enable P2P (yes/no)                                  │
    │   • Listen addresses / port mappings                     │
    │                                                          │
    ▼                                                          │
Step 8: P2P Mode Selection                                     │
    │   • Proxy / Selective / Full                             │
    │   → Mode-specific configuration (see below)              │
    │                                                          │
    ▼                                                          │
Step 9: Bootstrap Peers                                        │
    │   • Use public bootstrap (bib.dev) - requires confirm    │
    │   • Add custom bootstrap peers                           │
    │                                                          │
    ▼                                                          │
Step 10: Logging                                               │
    │   • Log level and format                                 │
    │   • Audit logging                                        │
    │                                                          │
    ▼                                                          │
Step 11: Clustering (Optional)                                 │
    │   • Enable HA clustering                                 │
    │   • Cluster configuration                                │
    │                                                          │
    ▼                                                          │
Step 12: Break Glass (Optional)                                │
    │   • Emergency access configuration                       │
    │                                                          │
    ▼                                                          │
Step 13: Confirmation                                          │
    │   • Review all settings                                  │
    │   • Confirm configuration                                │
    │                                                          │
    ▼                                                          │
Step 14: Connectivity Test                                     │
    │   • Test bootstrap peer connectivity                     │
    │   • Verify P2P identity                                  │
    │                                                          │
    ▼                                                          │
Step 15: Deployment ──────────────────────────────────────────┤
        • Generate configuration files                         │
        • Create manifests/compose files (if applicable)       │
        • Deploy and start bibd                                │
        • Verify startup                                       │
────────────────────────────────────────────────────────────────
```

---

## Deployment Targets

### Deployment Target Selection

The first major choice in daemon setup is the deployment target:

```
┌─────────────────────────────────────────────────────────────┐
│                   Deployment Target                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Where will bibd run?                                        │
│                                                              │
│  ● Local                                                     │
│    Run bibd directly on this machine                         │
│    Best for: development, single-user, dedicated servers     │
│                                                              │
│  ○ Docker                                                    │
│    Run bibd in a Docker container                            │
│    Best for: isolated deployments, easy updates              │
│                                                              │
│  ○ Podman                                                    │
│    Run bibd in a Podman container (rootful or rootless)      │
│    Best for: rootless containers, RHEL/Fedora environments   │
│                                                              │
│  ○ Kubernetes                                                │
│    Deploy bibd to a Kubernetes cluster                       │
│    Best for: production, high availability, scaling          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Local Deployment

bibd runs directly on the host machine as a system service.

**PostgreSQL Options for Local:**
- **SQLite**: Embedded, no setup required (Proxy/Selective modes only)
- **Managed Container**: bibd manages a Docker/Podman PostgreSQL container
- **Local PostgreSQL**: Connect to PostgreSQL installed on host
- **Remote PostgreSQL**: Connect to external PostgreSQL server

**Generated Files:**
- `~/.config/bibd/config.yaml`
- `~/.config/bibd/identity.pem`
- `/etc/systemd/system/bibd.service` (Linux) or `~/Library/LaunchAgents/dev.bib.bibd.plist` (macOS)

### Docker Deployment

bibd and PostgreSQL run in separate Docker containers managed by Docker Compose.

```
┌─────────────────────────────────────────────────────────────┐
│                   Docker Deployment                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ✓ Docker detected and running                               │
│                                                              │
│  Configuration:                                              │
│  ├── Compose file:  ./bibd/docker-compose.yaml               │
│  ├── Config dir:    ./bibd/config/                           │
│  ├── Data volume:   bibd-data                                │
│  └── Network:       bibd-network                             │
│                                                              │
│  Services:                                                   │
│  ├── bibd:     ghcr.io/bencoepp/bibd:latest                  │
│  └── postgres: postgres:16-alpine (if Full mode)             │
│                                                              │
│  Ports:                                                      │
│  ├── 4000:4000  (gRPC API)                                   │
│  └── 4001:4001  (P2P)                                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Generated Files:**
```
./bibd/
├── docker-compose.yaml
├── config/
│   ├── config.yaml
│   └── identity.pem
└── .env
```

**Auto-Start:**
After generation, the wizard runs:
```bash
cd ./bibd && docker compose up -d
```

### Podman Deployment

bibd and PostgreSQL run in Podman containers, supporting both rootful and rootless modes.

```
┌─────────────────────────────────────────────────────────────┐
│                   Podman Deployment                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ✓ Podman detected                                           │
│                                                              │
│  Container Mode:                                             │
│  ● Rootless (recommended, running as user)                   │
│  ○ Rootful (running as root)                                 │
│                                                              │
│  Deployment Style:                                           │
│  ● Pod (containers share network namespace)                  │
│  ○ Compose (podman-compose, separate networks)               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Rootless Podman:**
- Containers run without root privileges
- Data stored in `~/.local/share/containers/`
- Ports > 1024 unless configured with `net.ipv4.ip_unprivileged_port_start`

**Rootful Podman:**
- Containers run with root privileges
- Data stored in `/var/lib/containers/`
- Can bind to privileged ports

**Generated Files (Pod mode):**
```
./bibd/
├── bibd-pod.yaml           # Kubernetes-style pod definition
├── config/
│   ├── config.yaml
│   └── identity.pem
└── start.sh                # Convenience script
```

**Generated Files (Compose mode):**
```
./bibd/
├── podman-compose.yaml
├── config/
│   ├── config.yaml
│   └── identity.pem
└── .env
```

**Auto-Start:**
```bash
# Pod mode
podman play kube ./bibd/bibd-pod.yaml

# Compose mode  
cd ./bibd && podman-compose up -d
```

### Kubernetes Deployment

bibd is deployed to a Kubernetes cluster with full production configuration.

```
┌─────────────────────────────────────────────────────────────┐
│                 Kubernetes Deployment                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ✓ kubectl configured                                        │
│  ✓ Current context: my-cluster                               │
│                                                              │
│  Namespace: bibd (will be created)                           │
│                                                              │
│  Output Options:                                             │
│  ● Generate manifests and apply                              │
│  ○ Generate manifests only (manual apply)                    │
│  ○ Generate Helm values only                                 │
│                                                              │
│  Output Directory: ./bibd-k8s/                               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**PostgreSQL Options for Kubernetes:**

```
┌─────────────────────────────────────────────────────────────┐
│              Kubernetes PostgreSQL Setup                     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  PostgreSQL deployment strategy:                             │
│                                                              │
│  ● StatefulSet                                               │
│    Deploy PostgreSQL as a StatefulSet in the cluster         │
│    Creates: StatefulSet, Service, PVC, Secret                │
│                                                              │
│  ○ CloudNativePG Operator                                    │
│    Use CloudNativePG for production PostgreSQL               │
│    Requires: CloudNativePG operator installed                │
│    Creates: Cluster CR, Secrets                              │
│                                                              │
│  ○ External                                                  │
│    Connect to external PostgreSQL (RDS, Cloud SQL, etc.)     │
│    Creates: Secret with connection string                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Generated Kubernetes Resources:**

| Resource | Purpose |
|----------|---------|
| `Namespace` | Isolated namespace for bibd resources |
| `Deployment` or `StatefulSet` | bibd workload |
| `Service` | Internal ClusterIP service |
| `Service` (LoadBalancer/Ingress) | External access |
| `ConfigMap` | bibd configuration |
| `Secret` | Identity keys, database credentials |
| `PersistentVolumeClaim` | Data storage |
| `ServiceAccount` | RBAC identity |
| `NetworkPolicy` | Network security (optional) |

**PostgreSQL Resources (if StatefulSet):**

| Resource | Purpose |
|----------|---------|
| `StatefulSet` | PostgreSQL workload |
| `Service` | PostgreSQL internal service |
| `PersistentVolumeClaim` | Database storage |
| `Secret` | Database credentials |

**Generated Files:**
```
./bibd-k8s/
├── namespace.yaml
├── configmap.yaml
├── secret.yaml
├── bibd-deployment.yaml    # or statefulset.yaml
├── bibd-service.yaml
├── bibd-ingress.yaml       # if external access configured
├── postgres-statefulset.yaml  # if StatefulSet selected
├── postgres-service.yaml
├── postgres-pvc.yaml
├── postgres-secret.yaml
├── kustomization.yaml      # for kustomize users
└── values.yaml             # Helm values (for future Helm chart)
```

**Apply Options:**

```
┌─────────────────────────────────────────────────────────────┐
│                   Apply Manifests                            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Manifests generated in ./bibd-k8s/                          │
│                                                              │
│  Apply to cluster now?                                       │
│                                                              │
│  [Yes, Apply Now] [No, Manual Apply Later]                   │
│                                                              │
│  To apply manually:                                          │
│  kubectl apply -k ./bibd-k8s/                                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**External Access Configuration:**

```
┌─────────────────────────────────────────────────────────────┐
│                   External Access                            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  How should bibd be accessible from outside the cluster?     │
│                                                              │
│  ○ None (internal only)                                      │
│                                                              │
│  ● LoadBalancer                                              │
│    Cloud provider provisions external IP                     │
│                                                              │
│  ○ NodePort                                                  │
│    Expose on node ports (30000-32767)                        │
│                                                              │
│  ○ Ingress                                                   │
│    Use Ingress controller with hostname                      │
│    Hostname: bibd.example.com                                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

> **Note:** Helm chart for bibd is planned but not yet available. The wizard generates Helm-compatible `values.yaml` for future use.

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

Requires PostgreSQL and extensive configuration confirmation. PostgreSQL setup options depend on the deployment target selected earlier.

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
└─────────────────────────────────────────────────────────────┘
```

#### PostgreSQL Options by Deployment Target

| Deployment Target | PostgreSQL Options |
|-------------------|-------------------|
| **Local** | Managed container (Docker/Podman), local install, remote server |
| **Docker** | Separate container in same docker-compose (required) |
| **Podman** | Separate container in same pod/compose (required) |
| **Kubernetes** | StatefulSet, CloudNativePG, or external (RDS, Cloud SQL) |

#### Local Deployment - PostgreSQL Setup

```
┌─────────────────────────────────────────────────────────────┐
│           PostgreSQL Setup (Local Deployment)                │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  How should PostgreSQL be provided?                          │
│                                                              │
│  ● Managed Container                                         │
│    bibd manages a Docker/Podman PostgreSQL container         │
│                                                              │
│  ○ Local Installation                                        │
│    Connect to PostgreSQL installed on this machine           │
│                                                              │
│  ○ Remote Server                                             │
│    Connect to an external PostgreSQL server                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

If "Managed Container" is selected:

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

#### Docker Deployment - PostgreSQL Setup

When deploying bibd in Docker, PostgreSQL runs as a separate container in the same compose file:

```
┌─────────────────────────────────────────────────────────────┐
│           PostgreSQL Setup (Docker Deployment)               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  PostgreSQL will run as a separate container in the same    │
│  docker-compose configuration.                               │
│                                                              │
│  PostgreSQL Image: postgres:16-alpine                        │
│  Container Name:   bibd-postgres                             │
│  Volume:          bibd-postgres-data                         │
│  Network:         bibd-network (internal)                    │
│                                                              │
│  Generated docker-compose.yaml will include:                 │
│  • bibd service                                              │
│  • postgres service                                          │
│  • Shared network                                            │
│  • Persistent volumes                                        │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### Podman Deployment - PostgreSQL Setup

When deploying bibd in Podman, PostgreSQL runs as a separate container:

```
┌─────────────────────────────────────────────────────────────┐
│           PostgreSQL Setup (Podman Deployment)               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  PostgreSQL will run as a separate container.                │
│                                                              │
│  Container Mode: Rootless                                    │
│                                                              │
│  Deployment Style:                                           │
│  ● Pod (bibd and postgres in same pod)                       │
│    Containers share localhost, simpler networking            │
│                                                              │
│  ○ Compose (separate containers with podman-compose)         │
│    More flexible, similar to Docker Compose                  │
│                                                              │
│  PostgreSQL Image: postgres:16-alpine                        │
│  Volume:          bibd-postgres-data                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### Kubernetes Deployment - PostgreSQL Setup

```
┌─────────────────────────────────────────────────────────────┐
│          PostgreSQL Setup (Kubernetes Deployment)            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  How should PostgreSQL be deployed?                          │
│                                                              │
│  ● StatefulSet                                               │
│    Deploy PostgreSQL as a StatefulSet in the cluster         │
│    Simple, suitable for dev/test and small production        │
│                                                              │
│  ○ CloudNativePG                                             │
│    Use CloudNativePG operator for production PostgreSQL      │
│    Requires: CloudNativePG operator pre-installed            │
│    Features: HA, backups, monitoring                         │
│                                                              │
│  ○ External                                                  │
│    Connect to external managed PostgreSQL                    │
│    Examples: AWS RDS, Google Cloud SQL, Azure Database       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

If "StatefulSet" is selected:

```
┌─────────────────────────────────────────────────────────────┐
│            PostgreSQL StatefulSet Configuration              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Namespace:     bibd                                         │
│  Image:         postgres:16-alpine                           │
│  Replicas:      1 (single instance)                          │
│                                                              │
│  Storage:                                                    │
│  ├── Storage Class: standard (cluster default)               │
│  └── PVC Size:      50Gi                                     │
│                                                              │
│  Resources:                                                  │
│  ├── CPU:     500m request, 2000m limit                      │
│  └── Memory:  512Mi request, 2Gi limit                       │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

If "External" is selected:

```
┌─────────────────────────────────────────────────────────────┐
│               External PostgreSQL Setup                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Connection String:                                          │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ postgres://user:pass@rds.example.com:5432/bibd      │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                              │
│  Or configure individually:                                  │
│                                                              │
│  Host:     rds.example.com                                   │
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
│  Public Bootstrap:                                           │
│  ☐ Use public bootstrap nodes (bib.dev)                      │
│    ⚠️  Requires confirmation in next step                    │
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

### bib.dev Confirmation

If the public bootstrap is selected, explicit confirmation is required:

```
┌─────────────────────────────────────────────────────────────┐
│           Connect to Public Bootstrap (bib.dev)              │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  You've selected to use the public bib.dev bootstrap.        │
│                                                              │
│  This will:                                                  │
│  • Connect your node to the global bib P2P network          │
│  • Enable discovery of public peers                         │
│  • Allow your node to be discovered by others               │
│                                                              │
│  ⚠️  Your node will be visible to other network participants │
│     and may serve data to them (depending on mode).          │
│                                                              │
│  For private networks, use only custom bootstrap peers.      │
│                                                              │
│  Confirm connection to bib.dev?                              │
│                                                              │
│  [Yes, Connect to Public Network] [No, Private Only]         │
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

After configuration is complete, the wizard performs deployment actions based on the selected target.

### Deployment Actions by Target

| Target | Actions |
|--------|---------|
| **Local** | Generate config, create system service, start bibd |
| **Docker** | Generate docker-compose.yaml, run `docker compose up -d` |
| **Podman** | Generate pod/compose files, run containers |
| **Kubernetes** | Generate manifests, optionally apply with kubectl |

### Local Deployment

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

**Service Installation (Linux):**

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

### Docker Deployment

**Deployment Steps:**

1. **Generate files** in output directory:
   - `docker-compose.yaml`
   - `config/config.yaml`
   - `config/identity.pem`
   - `.env` (environment variables)
2. **Run `docker compose up -d`** to start containers
3. **Wait for containers** to be healthy
4. **Run health check** against bibd container

```
┌─────────────────────────────────────────────────────────────┐
│               Docker Deployment Complete! ✓                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Files generated in ./bibd/                                  │
│                                                              │
│  Starting containers...                                      │
│  ✓ Network bibd-network created                              │
│  ✓ Volume bibd-data created                                  │
│  ✓ Container bibd-postgres started (healthy)                 │
│  ✓ Container bibd started (healthy)                          │
│                                                              │
│  Services:                                                   │
│  ├── bibd:     Running (ghcr.io/bencoepp/bibd:latest)        │
│  └── postgres: Running (postgres:16-alpine)                  │
│                                                              │
│  Ports:                                                      │
│  ├── 4000 → bibd gRPC API                                    │
│  └── 4001 → bibd P2P                                         │
│                                                              │
│  Management:                                                 │
│  • cd ./bibd && docker compose logs -f                       │
│  • cd ./bibd && docker compose down                          │
│  • cd ./bibd && docker compose up -d                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Podman Deployment

**Deployment Steps:**

1. **Generate files** in output directory:
   - `bibd-pod.yaml` (pod mode) or `podman-compose.yaml` (compose mode)
   - `config/config.yaml`
   - `config/identity.pem`
   - `start.sh` (convenience script)
2. **Run containers**:
   - Pod mode: `podman play kube bibd-pod.yaml`
   - Compose mode: `podman-compose up -d`
3. **Wait for containers** to be healthy
4. **Run health check** against bibd container

```
┌─────────────────────────────────────────────────────────────┐
│               Podman Deployment Complete! ✓                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Mode: Rootless, Pod                                         │
│  Files generated in ./bibd/                                  │
│                                                              │
│  Starting pod...                                             │
│  ✓ Pod bibd-pod created                                      │
│  ✓ Container bibd-pod-postgres started                       │
│  ✓ Container bibd-pod-bibd started                           │
│                                                              │
│  Pod Status: Running                                         │
│                                                              │
│  Management:                                                 │
│  • podman pod logs -f bibd-pod                               │
│  • podman pod stop bibd-pod                                  │
│  • podman pod start bibd-pod                                 │
│  • podman play kube --down bibd-pod.yaml                     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Kubernetes Deployment

**Deployment Steps:**

1. **Generate manifests** in output directory
2. **Optionally apply** with `kubectl apply -k`
3. **Wait for pods** to be ready (if applied)
4. **Show connection instructions**

```
┌─────────────────────────────────────────────────────────────┐
│             Kubernetes Deployment Complete! ✓                │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Manifests generated in ./bibd-k8s/                          │
│                                                              │
│  Applied to cluster: my-cluster                              │
│  Namespace: bibd                                             │
│                                                              │
│  ✓ Namespace created                                         │
│  ✓ ConfigMap created                                         │
│  ✓ Secret created                                            │
│  ✓ PostgreSQL StatefulSet created                            │
│  ✓ PostgreSQL Service created                                │
│  ✓ bibd Deployment created                                   │
│  ✓ bibd Service created                                      │
│  ✓ bibd LoadBalancer created                                 │
│                                                              │
│  Waiting for pods...                                         │
│  ✓ postgres-0: Running                                       │
│  ✓ bibd-xxxxx: Running                                       │
│                                                              │
│  External Access:                                            │
│  └── LoadBalancer: 203.0.113.50:4000 (pending...)            │
│                                                              │
│  Management:                                                 │
│  • kubectl -n bibd get pods                                  │
│  • kubectl -n bibd logs -f deployment/bibd                   │
│  • kubectl -n bibd port-forward svc/bibd 4000:4000           │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Verification (Local Deployment)

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
│  Connected Nodes:                                            │
│  ├── localhost:4000 ✓ (default)                              │
│  ├── 192.168.1.50:4000 ✓                                     │
│  └── bib.dev:4000 ✓ (public)                                 │
│                                                              │
│  Authentication:                                             │
│  ├── Identity:    ~/.config/bib/identity.pem ✓               │
│  ├── User:        john@example.com                           │
│  └── Sessions:    3 active                                   │
│                                                              │
│  Network Health:                                             │
│  ├── Connected Peers:  12                                    │
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
| `bib setup --daemon --quick` | Quick daemon setup (local, Proxy mode) |
| `bib setup --daemon --target <target>` | Specify deployment target |
| `bib setup --daemon --cluster` | Initialize new HA cluster |
| `bib setup --daemon --cluster-join <token>` | Join existing cluster |
| `bib setup --reconfigure [section]` | Reconfigure specific sections |
| `bib setup --fresh` | Reset and start fresh |

### Deployment Target Options

| Flag | Target | Description |
|------|--------|-------------|
| `--target local` | Local | Run bibd directly on host (default) |
| `--target docker` | Docker | Run in Docker containers |
| `--target podman` | Podman | Run in Podman containers (rootful/rootless) |
| `--target kubernetes` | Kubernetes | Deploy to Kubernetes cluster |

**Examples:**

```bash
# Quick start with Docker
bib setup --daemon --quick --target docker

# Full setup for Kubernetes
bib setup --daemon --target kubernetes

# Podman with rootless mode
bib setup --daemon --target podman
```

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

