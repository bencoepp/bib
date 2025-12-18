# Core Concepts

This section explains the fundamental concepts and architecture of Bib.

## In This Section

| Document | Description |
|----------|-------------|
| [Architecture Overview](architecture.md) | System design, components, and data flow |
| [Domain Entities](domain-entities.md) | Core data model (Topics, Datasets, Jobs, Users) |
| [Node Modes](node-modes.md) | Understanding Proxy, Selective, and Full replication modes |

## Key Concepts

### System Components

Bib consists of two main components:

| Component | Description |
|-----------|-------------|
| **`bib`** | Command-line interface for user interaction |
| **`bibd`** | Background daemon handling P2P, storage, and job execution |

### Node Modes

| Mode | Storage | Use Case |
|------|---------|----------|
| **Proxy** | Cache only | Development, lightweight gateways |
| **Selective** | Subscribed topics | Team nodes, domain-specific access |
| **Full** | Everything | Archive nodes, HA clusters |

### Core Entities

| Entity | Purpose |
|--------|---------|
| **Topic** | Categories for organizing datasets |
| **Dataset** | Versioned data containers |
| **Job** | Data processing execution units |
| **Task** | Reusable job templates |

---

[← Back to Documentation](../README.md)

