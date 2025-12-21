# Bib Documentation

<p align="center">
  <strong>A Distributed Data Management Platform for Research and Analysis</strong>
</p>

<p align="center">
  <a href="getting-started/quickstart.md">Quick Start</a> •
  <a href="concepts/architecture.md">Architecture</a> •
  <a href="guides/cli-reference.md">CLI Reference</a> •
  <a href="getting-started/configuration.md">Configuration</a>
</p>

---

## What is Bib?

**Bib** is a decentralized data management platform designed for researchers, data engineers, and teams who need to organize, share, version, and process datasets across distributed networks. Built on peer-to-peer technology, Bib eliminates the need for central servers while providing enterprise-grade features like high availability clustering, cryptographic identity, and comprehensive audit logging.

### Core Components

| Component | Description |
|-----------|-------------|
| **`bib`** | Command-line interface for interacting with the system |
| **`bibd`** | Background daemon that handles P2P networking, storage, job execution, and cluster coordination |
| **`bib.dev`** | Public bootstrap node for initial peer discovery |

### Key Features

| Feature | Description |
|---------|-------------|
| **Decentralized Architecture** | No central server required—data is shared directly between peers using libp2p |
| **Flexible Node Modes** | Choose between full replication, selective sync, or lightweight proxy mode |
| **Version-Controlled Data** | Immutable dataset versions with complete history tracking |
| **P2P Data Transfer** | Efficient chunked transfers with resumable downloads and content-addressed storage |
| **CEL-Based Jobs** | Safe, sandboxed data processing using Common Expression Language instructions |
| **High Availability** | Optional Raft-based clustering for fault-tolerant deployments |
| **Cryptographic Identity** | Ed25519-based user authentication and data signing |
| **Zero Trust Security** | Encrypted credentials, role-based database access, and comprehensive audit logging |

---

## Documentation Structure

### 🚀 [Getting Started](getting-started/README.md)

Essential guides for new users.

| Document | Description |
|----------|-------------|
| [Quick Start Guide](getting-started/quickstart.md) | Get up and running in 5 minutes |
| [Setup Flow](getting-started/setup-flow.md) | Complete setup and initialization guide |
| [Configuration Guide](getting-started/configuration.md) | Complete configuration reference |

### 🏗️ [Core Concepts](concepts/README.md)

Understanding Bib's architecture and design.

| Document | Description |
|----------|-------------|
| [Architecture Overview](concepts/architecture.md) | System design, components, and data flow |
| [Domain Entities](concepts/domain-entities.md) | Core data model (Topics, Datasets, Jobs, Users) |
| [Node Modes](concepts/node-modes.md) | Proxy, Selective, and Full replication modes |

### 📖 [User Guides](guides/README.md)

Comprehensive guides for using Bib.

| Document | Description |
|----------|-------------|
| [CLI Reference](guides/cli-reference.md) | Complete command reference for `bib` |
| [Jobs & Tasks](guides/jobs-tasks.md) | Creating and running data processing jobs |
| [Clustering Guide](guides/clustering.md) | Setting up high-availability Raft clusters |

### 🌐 [Networking](networking/README.md)

P2P networking and protocols.

| Document | Description |
|----------|-------------|
| [P2P Networking](networking/p2p-networking.md) | Peer discovery, data transfer, and pub/sub |
| [Protocols Reference](networking/protocols.md) | Wire protocols and message formats |

### 💾 [Storage & Security](storage/README.md)

Database and security architecture.

| Document | Description |
|----------|-------------|
| [Storage Lifecycle](storage/storage-lifecycle.md) | Database backend management |
| [Database Security](storage/database-security.md) | Credentials, roles, and encryption |
| [Break Glass Access](storage/break-glass.md) | Emergency database access procedures |

### ☸️ [Deployment](deployment/README.md)

Deployment guides for various environments.

| Document | Description |
|----------|-------------|
| [Kubernetes Deployment](deployment/kubernetes.md) | Deploying bibd on Kubernetes |

### 🛠️ [Development](development/README.md)

Documentation for contributors.

| Document | Description |
|----------|-------------|
| [Developer Guide](development/developer-guide.md) | Codebase overview for contributors |
| [TUI Components](development/tui-components.md) | Terminal UI component system |

### 📁 [Examples](examples/)

Sample configuration files.

| File | Description |
|------|-------------|
| [kubernetes-config.yaml](examples/kubernetes-config.yaml) | Example Kubernetes configuration |

---

## Quick Example

```bash
# Install bib (from source)
git clone https://github.com/yourorg/bib.git
cd bib && go build -o bib ./cmd/bib && go build -o bibd ./cmd/bibd
sudo mv bib bibd /usr/local/bin/

# Configure the CLI interactively
bib setup

# Configure and start the daemon
bib setup --daemon
bibd

# (In another terminal) Verify everything works
bib version
```

For detailed instructions, see the [Quick Start Guide](getting-started/quickstart.md).

---

## System Requirements

### Minimum Requirements

| Requirement | Specification |
|-------------|---------------|
| **Operating System** | Linux, macOS, or Windows |
| **Go Version** | 1.25 or later (for building from source) |
| **Network** | Outbound connectivity for P2P networking |

### Recommended for Production

| Requirement | Specification |
|-------------|---------------|
| **Storage Backend** | PostgreSQL 16+ (for full replication mode) |
| **Container Runtime** | Docker or Podman (for managed PostgreSQL) |
| **Cluster Size** | Minimum 3 nodes for HA clustering |

---

## Version Information

| Property | Value |
|----------|-------|
| **Current Version** | 0.1.0 (Development) |
| **API Version** | v1 |
| **Protocol Version** | 1.0.0 |

---

## Getting Help

- **Documentation Issues**: If you find errors or gaps in the documentation, please open an issue
- **Bug Reports**: Include version information (`bib version`) and reproduction steps
- **Feature Requests**: Describe your use case and the problem you're trying to solve

---

## License

[License information to be added]

