# Architecture Overview

This document describes the high-level architecture of the bib distributed data management system.

## System Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              BIB ECOSYSTEM                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌──────────────┐         ┌──────────────────────────────────────────┐     │
│   │   bib CLI    │◄───────►│              bibd daemon                 │     │
│   │   (Cobra)    │  gRPC   │                                          │     │
│   └──────────────┘  mTLS   │  ┌────────────┐  ┌───────────────────┐   │     │
│                            │  │  P2P Layer │  │    Scheduler      │   │     │
│                            │  │  (libp2p)  │  │  (CEL + Workers)  │   │     │
│                            │  └────────────┘  └───────────────────┘   │     │
│                            │                                          │     │
│                            │  ┌────────────┐  ┌───────────────────┐   │     │
│                            │  │  Storage   │  │   Cluster (Raft)  │   │     │
│                            │  │ (SQL/Blob) │  │   (HA Mode)       │   │     │
│                            │  └────────────┘  └───────────────────┘   │     │
│   ┌──────────────┐         │                                          │     │
│   │  Bootstrap   │         └──────────────────────────────────────────┘     │
│   │   bib.dev    │                        │                                  │
│   └──────────────┘                        ▼                                  │
│                    ┌──────────────────────────────────────┐                 │
│                    │     SQLite / PostgreSQL Storage       │                 │
│                    └──────────────────────────────────────┘                 │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Components

### bib CLI

The `bib` command-line interface is the primary user-facing component. It provides:

- **Configuration Management** - Setup and manage bib/bibd configuration
- **Data Operations** - Query, download, and manage datasets
- **Job Management** - Submit and monitor data processing jobs
- **Cluster Operations** - Initialize and join HA clusters

**Technology**: Go with Cobra for command parsing, Viper for configuration.

### bibd Daemon

The `bibd` daemon is the core of the system, providing:

- **P2P Networking** - Peer discovery, data transfer, and pub/sub messaging
- **Storage Backend** - Local data storage with SQLite or PostgreSQL
- **Job Execution** - CEL-based task execution with resource limits
- **Cluster Coordination** - Raft consensus for HA deployments
- **gRPC Server** - API for CLI and programmatic access

**Technology**: Go with libp2p for P2P networking.

### Bootstrap Node (bib.dev)

The public bootstrap node at `bib.dev` provides:

- Initial peer discovery for new nodes
- DHT bootstrap for the Kademlia network
- Always-available entry point to the network

## Layered Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      API Layer                               │
│  (gRPC Server, CLI Commands)                                │
├─────────────────────────────────────────────────────────────┤
│                    Service Layer                             │
│  (Job Scheduler, Query Engine, Sync Manager)                │
├─────────────────────────────────────────────────────────────┤
│                    Domain Layer                              │
│  (Entities: Topic, Dataset, Job, User, etc.)                │
├─────────────────────────────────────────────────────────────┤
│                  Infrastructure Layer                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │    P2P      │  │   Storage   │  │      Cluster        │  │
│  │  (libp2p)   │  │ (SQL/Blob)  │  │      (Raft)         │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### API Layer

Exposes functionality via:
- **gRPC** - For `bib` CLI and programmatic access
- **P2P Protocols** - For inter-node communication

### Service Layer

Business logic for:
- **Job Scheduling** - Queue, execute, and monitor jobs
- **Query Engine** - SQL queries across datasets
- **Sync Manager** - Data synchronization based on node mode

### Domain Layer

Core entities that model the problem domain:
- Topics, Datasets, Versions
- Jobs, Tasks, Instructions
- Users, Ownership
- See [Domain Entities](domain-entities.md) for details

### Infrastructure Layer

External concerns:
- **P2P** - Networking with libp2p
- **Storage** - SQLite/PostgreSQL + blob storage
- **Cluster** - Raft consensus for HA

## Data Flow

### Dataset Publishing Flow

```
User creates dataset
        │
        ▼
┌───────────────┐
│   bib CLI     │
└───────┬───────┘
        │ gRPC
        ▼
┌───────────────┐
│    bibd       │
│  ┌─────────┐  │
│  │ Storage │  │─────► Local SQLite/Postgres
│  └─────────┘  │
│  ┌─────────┐  │
│  │  P2P    │  │─────► Announce to peers via PubSub
│  └─────────┘  │
└───────────────┘
```

### Dataset Discovery Flow

```
┌───────────────┐         ┌───────────────┐
│   Peer A      │         │   Peer B      │
│  ┌─────────┐  │ PubSub  │  ┌─────────┐  │
│  │ PubSub  │◄─┼─────────┼──┤ PubSub  │  │
│  └─────────┘  │         │  └─────────┘  │
│       │       │         │               │
│       ▼       │         │               │
│  ┌─────────┐  │         │               │
│  │ Catalog │  │         │               │
│  └─────────┘  │         │               │
└───────────────┘         └───────────────┘

Peer B announces new dataset via /bib/topics/<topic-id>
Peer A receives announcement and updates local catalog
```

### Dataset Download Flow

```
┌───────────────┐         ┌───────────────┐
│   Requester   │         │   Provider    │
│  ┌─────────┐  │ Request │  ┌─────────┐  │
│  │Transfer │──┼────────►│  │Protocol │  │
│  │ Manager │  │         │  │ Handler │  │
│  └─────────┘  │         │  └─────────┘  │
│       │       │◄────────┤       │       │
│       │       │ Chunks  │       │       │
│       ▼       │         │       ▼       │
│  ┌─────────┐  │         │  ┌─────────┐  │
│  │ Storage │  │         │  │ Storage │  │
│  └─────────┘  │         │  └─────────┘  │
└───────────────┘         └───────────────┘

1. Requester identifies peers with dataset via catalog
2. Opens /bib/data/1.0.0 stream to provider
3. Requests chunks (can be parallel from multiple peers)
4. Verifies each chunk hash
5. Assembles and verifies complete dataset hash
```

## Security Model

### Transport Security

- **P2P**: Noise protocol encryption for all libp2p connections
- **gRPC**: Optional mTLS for CLI-daemon communication

### Identity

- **Node Identity**: Ed25519 keypair, persisted in `identity.pem`
- **User Identity**: Ed25519 keypair, UserID derived from public key

### Authentication

- All P2P messages are signed
- Users sign operations for authentication
- Ownership verified via signature validation

### Authorization

- Role-based access control (Owner, Admin, Contributor, Reader)
- Resource-level permissions for Topics, Datasets, Tasks

### Database Security

The storage layer implements a **Zero Trust Database Access** model:

```
┌─────────────────────────────────────────────────────────────┐
│                   Database Security Stack                    │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌────────────────┐    ┌────────────────────────────────┐   │
│  │  Credential    │    │     Role-Aware Pool            │   │
│  │  Manager       │    │     (SET LOCAL ROLE)           │   │
│  │                │    │                                │   │
│  │ • X25519/HKDF  │    │ • Per-transaction roles        │   │
│  │ • Auto-rotate  │    │ • Minimal privilege            │   │
│  │ • Zero-downtime│    │ • Audit logging               │   │
│  └────────────────┘    └────────────────────────────────┘   │
│                                                              │
│  ┌────────────────┐    ┌────────────────────────────────┐   │
│  │  Network       │    │     Encryption at Rest         │   │
│  │  Isolation     │    │                                │   │
│  │                │    │ • Application-level AES-GCM    │   │
│  │ • Unix sockets │    │ • LUKS volume (Linux)          │   │
│  │ • mTLS         │    │ • Shamir key recovery          │   │
│  │ • Internal net │    │                                │   │
│  └────────────────┘    └────────────────────────────────┘   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Key security features:**

| Feature | Description |
|---------|-------------|
| Credential Encryption | Passwords encrypted with node identity key |
| Credential Rotation | Zero-downtime rotation every 7 days (configurable) |
| Role-Based Access | 6 roles with minimal required permissions |
| Network Isolation | Unix sockets (Linux) or localhost-only TCP + mTLS |
| Encryption at Rest | AES-256-GCM field encryption, optional LUKS |
| Audit Logging | All queries logged with role, duration, row count |
| Key Recovery | Shamir's Secret Sharing (3-of-5 threshold) |

See [Database Security & Hardening](database-security.md) for complete documentation.

## Scalability

### Horizontal Scaling

- Add more nodes to the network
- Data automatically distributes via P2P
- Optional HA clustering for critical deployments

### Node Modes for Resource Management

| Mode | Memory | Storage | Network |
|------|--------|---------|---------|
| Proxy | Low | Minimal | Pass-through |
| Selective | Medium | Partial | On-demand |
| Full | High | Complete | Continuous |

## High Availability

Optional Raft-based clustering provides:

- **Leader Election** - Automatic failover
- **State Replication** - Consistent catalog and job state
- **Minimum 3 Voters** - For proper quorum

See [Clustering](clustering.md) for setup instructions.

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.25+ |
| P2P | libp2p (TCP, QUIC, Noise, Kademlia DHT) |
| CLI | Cobra + Viper |
| TUI | Bubble Tea + Lip Gloss + Huh |
| Logging | log/slog with structured output |
| Database | SQLite (embedded), PostgreSQL (external) |
| Consensus | etcd/raft |
| Serialization | JSON (dev), Protobuf (production) |

## Terminal UI Architecture

The TUI system (`internal/tui/`) provides a comprehensive component library for terminal interfaces:

```
┌───────────────────────────────────────────────────────────────┐
│                      TUI Package (tui.go)                      │
│           Main entry, type aliases, convenience funcs          │
├───────────────────────────────────────────────────────────────┤
│                                                                │
│  ┌─────────────┐  ┌─────────────┐  ┌───────────────────────┐  │
│  │   themes/   │  │   layout/   │  │      component/       │  │
│  ├─────────────┤  ├─────────────┤  ├───────────────────────┤  │
│  │ Registry    │  │ Flex        │  │ Stateless:            │  │
│  │ Palettes    │  │ Grid        │  │   Card, Box, Badge    │  │
│  │ Icons       │  │ Responsive  │  │   ProgressBar, etc.   │  │
│  │ Theme       │  │ Context     │  │                       │  │
│  │             │  │             │  │ Stateful (tea.Model): │  │
│  │             │  │             │  │   Table, List, Tree   │  │
│  │             │  │             │  │   Modal, Toast, Tabs  │  │
│  └─────────────┘  └─────────────┘  └───────────────────────┘  │
│                                                                │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │              Wizard, Setup, Tabs (tui/*.go)              │  │
│  │           High-level composed components                  │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                                │
└───────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────┐
│              External Dependencies                             │
│  - Bubble Tea (github.com/charmbracelet/bubbletea)            │
│  - Lip Gloss (github.com/charmbracelet/lipgloss)              │
│  - Huh (github.com/charmbracelet/huh) - Forms                 │
└───────────────────────────────────────────────────────────────┘
```

See [TUI Component System](tui-components.md) for detailed documentation.

