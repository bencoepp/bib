# Bib

[![CI](https://github.com/bencoepp/bib/actions/workflows/ci.yaml/badge.svg)](https://github.com/bencoepp/bib/actions/workflows/ci.yaml)
[![Release](https://github.com/bencoepp/bib/actions/workflows/release.yaml/badge.svg)](https://github.com/bencoepp/bib/actions/workflows/release.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/bencoepp/bib)](https://goreportcard.com/report/github.com/bencoepp/bib)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**A distributed data management platform for research and analysis.**

Bib is a decentralized data management tool for researchers, data engineers, and teams who need to organize, share, version, and process datasets across distributed networks. Built on peer-to-peer technology, Bib eliminates the need for central servers while providing enterprise-grade features.

## Features

- 🌐 **Decentralized Architecture** - No central server required; data is shared directly between peers
- 🔄 **Flexible Node Modes** - Choose between full replication, selective sync, or lightweight proxy mode
- 📦 **Version-Controlled Data** - Immutable dataset versions with complete history tracking
- 🚀 **Efficient P2P Transfer** - Chunked transfers with resumable downloads and parallel fetching
- ⚡ **CEL-Based Jobs** - Safe, sandboxed data processing using Common Expression Language
- 🔒 **Zero Trust Security** - Ed25519 authentication, encrypted storage, and comprehensive audit logging
- 🏢 **High Availability** - Optional Raft-based clustering for fault-tolerant deployments

## Quick Start

### Installation

**Homebrew (macOS/Linux):**
```bash
brew tap bencoepp/bib
brew install bib bibd
```

**Windows (winget):**
```powershell
winget install bencoepp.bib
winget install bencoepp.bibd
```

**Docker:**
```bash
docker pull ghcr.io/bencoepp/bibd:latest
docker run -d -p 8080:8080 -p 4001:4001 ghcr.io/bencoepp/bibd:latest
```

**From Source:**
```bash
git clone https://github.com/bencoepp/bib.git
cd bib
make build
```

### Configuration

```bash
# Configure the CLI
bib setup

# Configure the daemon
bib setup --daemon

# Start the daemon
bibd
```

### Verify Installation

```bash
bib version
bibd -version
```

## Components

| Component | Description |
|-----------|-------------|
| **`bib`** | Command-line interface for interacting with the system |
| **`bibd`** | Background daemon handling P2P networking, storage, and job execution |

## Documentation

- 📚 [Full Documentation](docs/README.md)
- 🚀 [Quick Start Guide](docs/getting-started/quickstart.md)
- ⚙️ [Configuration Guide](docs/getting-started/configuration.md)
- 🏗️ [Architecture Overview](docs/concepts/architecture.md)
- 🔧 [CLI Reference](docs/guides/cli-reference.md)

## Development

### Prerequisites

- Go 1.25+
- Make
- Docker (optional, for containerized development)

### Build

```bash
# Build both binaries
make build

# Run tests
make test

# Run linter
make lint
```

### Docker Compose (Development)

```bash
# Start development environment with PostgreSQL
docker-compose up -d

# View logs
docker-compose logs -f bibd

# Stop services
docker-compose down
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed development guidelines.

## Node Modes

| Mode | Description | Storage | Best For |
|------|-------------|---------|----------|
| **Proxy** | Pass-through requests, cache only | None/Cache | Development, edge gateways |
| **Selective** | Sync subscribed topics on-demand | Partial | Team nodes, domain-specific |
| **Full** | Replicate all topics continuously | Full | Archive nodes, data providers |

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please read our [Contributing Guidelines](CONTRIBUTING.md) before submitting a PR.

## Security

For security concerns, please see our [Security Policy](SECURITY.md).

