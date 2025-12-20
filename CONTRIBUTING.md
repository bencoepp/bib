# Contributing to Bib

Thank you for your interest in contributing to Bib! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Documentation](#documentation)
- [Release Process](#release-process)

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment. Please be kind and constructive in all interactions.

## Getting Started

### Prerequisites

- **Go 1.25+** - [Download](https://go.dev/dl/)
- **Git** - For version control
- **Make** - For build automation
- **Docker** (optional) - For containerized development

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/bib.git
   cd bib
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/bencoepp/bib.git
   ```

## Development Setup

### Quick Start

```bash
# Install dependencies
go mod download

# Build both binaries
make build

# Run tests
make test

# Run linter
make lint
```

### Using Docker Compose

For a complete development environment with PostgreSQL:

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f bibd

# Run CLI commands
docker-compose exec bib bib version

# Stop services
docker-compose down
```

### Development Tools

Install recommended tools:

```bash
# Install all development tools
make tools-all

# Individual tools
make tools-buf        # Protocol buffer compiler
make tools-proto      # Protobuf Go plugins
make tools-lint       # golangci-lint
make tools-goreleaser # Release automation
```

## Making Changes

### Branch Naming

Use descriptive branch names:

- `feature/add-topic-search` - New features
- `fix/connection-timeout` - Bug fixes
- `docs/update-quickstart` - Documentation
- `refactor/storage-layer` - Code refactoring
- `test/add-p2p-tests` - Test additions

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation only
- `style` - Formatting, no code change
- `refactor` - Code change that neither fixes a bug nor adds a feature
- `perf` - Performance improvement
- `test` - Adding or updating tests
- `chore` - Build process or auxiliary tool changes

**Examples:**
```
feat(p2p): add peer reputation scoring

fix(storage): handle connection timeout gracefully

docs(api): update authentication flow diagram

test(cluster): add Raft consensus integration tests
```

### Keep Changes Focused

- One logical change per commit
- One feature/fix per pull request
- Break large changes into smaller PRs when possible

## Pull Request Process

### Before Submitting

1. **Sync with upstream:**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run all checks:**
   ```bash
   make fmt      # Format code
   make lint     # Run linter
   make test     # Run tests
   make build    # Verify build
   ```

3. **Update documentation** if needed

### Submitting a PR

1. Push your branch to your fork
2. Create a Pull Request against `bencoepp/bib:main`
3. Fill out the PR template completely
4. Link any related issues

### PR Review

- All PRs require at least one approval
- CI checks must pass
- Address review feedback promptly
- Keep the PR updated with main branch

## Coding Standards

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` and `goimports` for formatting
- Run `golangci-lint` before committing

### Project Conventions

- **Package names**: Short, lowercase, no underscores
- **File names**: Lowercase with underscores (`user_service.go`)
- **Test files**: Same name with `_test` suffix (`user_service_test.go`)
- **Interface names**: Noun or noun phrase, no `I` prefix
- **Error handling**: Wrap errors with context using `fmt.Errorf("...: %w", err)`

### Code Organization

```
cmd/           # Application entry points
├── bib/       # CLI application
└── bibd/      # Daemon application

internal/      # Private packages
├── config/    # Configuration
├── domain/    # Domain entities
├── storage/   # Database layer
├── p2p/       # Networking
└── ...

api/           # API definitions
├── proto/     # Protobuf files
└── gen/       # Generated code

docs/          # Documentation
test/          # Integration tests
```

### Error Handling

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to connect to peer %s: %w", peerID, err)
}

// Good: Use sentinel errors for expected conditions
var ErrNotFound = errors.New("not found")

// Good: Custom error types for complex cases
type ValidationError struct {
    Field   string
    Message string
}
```

## Testing

### Running Tests

```bash
# Unit tests
make test

# Integration tests
make test-integration

# All tests
make test-all

# With coverage
make test-coverage
```

### Writing Tests

- Use table-driven tests when appropriate
- Use `testify` assertions sparingly
- Mock external dependencies
- Test error cases, not just happy paths

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "hello", "HELLO", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Something(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Something() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Something() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Test Coverage

- Aim for >70% coverage on critical packages
- Focus on testing behavior, not implementation
- Don't sacrifice test quality for coverage numbers

## Documentation

### Code Documentation

- All exported functions, types, and packages need doc comments
- Start doc comments with the name being documented
- Include examples for complex APIs

```go
// UserService manages user operations including creation,
// authentication, and profile management.
type UserService struct {
    // ...
}

// CreateUser creates a new user with the given email and name.
// It returns the created user or an error if the email already exists.
func (s *UserService) CreateUser(email, name string) (*User, error) {
    // ...
}
```

### Updating Docs

- Update `docs/` when changing user-facing behavior
- Update `README.md` for major features
- Keep examples up to date

## Release Process

Releases are automated via GitHub Actions when a tag is pushed:

```bash
# Create a release tag
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

The release workflow will:
1. Build binaries for all platforms
2. Create Linux packages (.deb, .rpm)
3. Build and push Docker images
4. Update Homebrew tap
5. Submit to Winget
6. Create GitHub release with changelog

See [Release Setup](docs/development/release-setup.md) for maintainer documentation.

## Questions?

- **Bug reports**: [GitHub Issues](https://github.com/bencoepp/bib/issues)
- **Feature requests**: [GitHub Issues](https://github.com/bencoepp/bib/issues)
- **Security issues**: See [SECURITY.md](SECURITY.md)

Thank you for contributing! 🎉

