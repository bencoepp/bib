# Bib Documentation

Welcome to the **bib** documentation. Bib is a distributed research, analysis, and management tool for handling all types of research data.

## What is Bib?

Bib is a decentralized data management platform consisting of two main components:

- **`bib`** - A command-line interface (CLI) client for interacting with the system
- **`bibd`** - A daemon that handles P2P networking, data storage, job execution, and cluster coordination

Together, they enable researchers and data engineers to organize, share, version, and process datasets across a distributed peer-to-peer network.

## Key Features

- **Decentralized Architecture** - No central server; data is shared directly between peers
- **Flexible Node Modes** - Choose between full replication, selective sync, or proxy mode
- **Version-Controlled Data** - Immutable dataset versions with complete history
- **P2P Data Transfer** - Efficient chunked transfers with resumable downloads
- **CEL-Based Jobs** - Safe, sandboxed data processing using CEL instructions
- **High Availability** - Optional Raft-based clustering for fault tolerance
- **Cryptographic Identity** - Ed25519-based user authentication
- **Zero Trust Security** - Encrypted credentials, role-based access, full audit logging

## Documentation Index

### Getting Started
- [Quick Start](quickstart.md) - Get up and running in 5 minutes
- [Configuration](configuration.md) - Detailed configuration options

### Architecture
- [Architecture Overview](architecture.md) - System design and components
- [Domain Entities](domain-entities.md) - Core data model documentation
- [P2P Networking](p2p-networking.md) - Peer-to-peer networking layer
- [Protocols](protocols.md) - Wire protocols and message formats

### Storage & Security
- [Storage Lifecycle](storage-lifecycle.md) - Database container management
- [Database Security](database-security.md) - Credentials, roles, encryption, and hardening

### User Guides
- [CLI Reference](cli-reference.md) - Complete bib command reference
- [Node Modes](node-modes.md) - Understanding proxy, selective, and full modes
- [Clustering](clustering.md) - Setting up high-availability clusters

### Developer Guides
- [Developer Guide](developer-guide.md) - Complete guide for new developers and coding agents
- [Jobs & Tasks](jobs-tasks.md) - Creating and running data processing jobs
- [TUI Component System](tui-components.md) - Terminal UI components, themes, and layouts

## Quick Links

| Component | Description                            |
|-----------|----------------------------------------|
| `bib`     | CLI client for user interaction        |
| `bibd`    | Daemon for P2P, storage, and execution |
| `bib.dev` | Bootstrap node for peer discovery      |

## Version

Current version: **0.1.0** (Development)

## License

[License information here]

