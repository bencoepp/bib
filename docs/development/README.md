# Development

This section provides documentation for developers contributing to Bib.

## In This Section

| Document | Description |
|----------|-------------|
| [Developer Guide](developer-guide.md) | Codebase overview for contributors |
| [TUI Components](tui-components.md) | Terminal UI component system documentation |
| [CLI i18n](cli-i18n.md) | Internationalization guide for CLI commands |
| [Release Setup](release-setup.md) | How to set up GPG keys and release pipeline |

## Project Structure

```
bib/
├── api/proto/bib/v1/        # Protobuf definitions
├── cmd/
│   ├── bib/                  # CLI entry point
│   │   ├── main.go
│   │   └── cmd/              # Cobra commands
│   └── bibd/                 # Daemon entry point
├── docs/                     # Documentation (you are here)
├── internal/
│   ├── cluster/              # Raft clustering
│   ├── config/               # Configuration management
│   ├── domain/               # Domain entities
│   ├── logger/               # Structured logging
│   ├── p2p/                  # libp2p networking
│   ├── storage/              # Database layer
│   │   ├── postgres/         # PostgreSQL implementation
│   │   └── sqlite/           # SQLite implementation
│   └── tui/                  # Terminal UI components
└── test/                     # Integration tests
```

## Key Technologies

| Technology | Purpose |
|------------|---------|
| [Go](https://go.dev/) | Primary language |
| [Cobra](https://github.com/spf13/cobra) | CLI framework |
| [Viper](https://github.com/spf13/viper) | Configuration |
| [Bubble Tea](https://github.com/charmbracelet/bubbletea) | Terminal UI |
| [libp2p](https://libp2p.io/) | P2P networking |
| [Hashicorp Raft](https://github.com/hashicorp/raft) | Consensus |
| [GoReleaser](https://goreleaser.com/) | Release automation |

## Getting Started

```bash
# Clone repository
git clone https://github.com/bencoepp/bib.git
cd bib

# Build using Make
make build

# Or build directly with Go
go build -o bib ./cmd/bib
go build -o bibd ./cmd/bibd

# Run tests
go test ./...

# Run with debug logging
BIBD_LOG_LEVEL=debug ./bibd
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` and `goimports`
- Document public APIs with godoc comments
- Write tests for new functionality

---

[← Back to Documentation](../README.md)

