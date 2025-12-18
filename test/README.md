# Testing Infrastructure

This directory contains integration and end-to-end tests for the bib project.

## Directory Structure

```
test/
├── README.md              # This file
├── integration/           # Integration tests (component-level)
│   ├── storage/          # Storage layer integration tests
│   ├── p2p/              # P2P networking integration tests
│   ├── cluster/          # Raft cluster integration tests
│   └── api/              # gRPC API integration tests
├── e2e/                   # End-to-end tests (system-level)
│   ├── scenarios/        # Test scenarios
│   └── cli/              # CLI end-to-end tests
├── testutil/              # Shared test utilities
│   ├── containers/       # Docker container helpers
│   ├── fixtures/         # Test data fixtures
│   └── helpers/          # Common test helpers
└── docker/                # Docker resources for testing
    ├── docker-compose.test.yaml
    └── Dockerfile.test
```

## Running Tests

### Unit Tests
```bash
go test ./...
```

### Integration Tests
Integration tests require Docker to be running.

```bash
# Run all integration tests
go test -tags=integration ./test/integration/...

# Run specific integration tests
go test -tags=integration ./test/integration/storage/...
go test -tags=integration ./test/integration/p2p/...
```

### End-to-End Tests
E2E tests require Docker and test the complete system.

```bash
# Run all e2e tests
go test -tags=e2e ./test/e2e/...

# Run with verbose output
go test -tags=e2e -v ./test/e2e/...
```

### Using Docker Compose
```bash
# Start test infrastructure
docker-compose -f test/docker/docker-compose.test.yaml up -d

# Run tests
go test -tags=integration,e2e ./test/...

# Cleanup
docker-compose -f test/docker/docker-compose.test.yaml down -v
```

## Test Tags

- `integration` - Integration tests that test component interactions
- `e2e` - End-to-end tests that test the complete system
- `slow` - Tests that take longer than 30 seconds

## Writing Tests

### Integration Tests
Integration tests should:
- Test interactions between components
- Use real dependencies (PostgreSQL, etc.) via containers
- Be isolated and not depend on external state
- Clean up after themselves

### E2E Tests
E2E tests should:
- Test complete user workflows
- Use the actual CLI and daemon binaries
- Simulate real deployment scenarios
- Test multi-node scenarios

## Test Utilities

### Container Helpers
Use `testutil/containers` to manage Docker containers:
- PostgreSQL containers
- Multi-node cluster setups
- Network simulation

### Fixtures
Use `testutil/fixtures` for:
- Sample configuration files
- Test data
- Mock responses

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TEST_POSTGRES_IMAGE` | PostgreSQL Docker image | `postgres:16-alpine` |
| `TEST_TIMEOUT` | Test timeout | `5m` |
| `TEST_VERBOSE` | Enable verbose logging | `false` |
| `TEST_KEEP_CONTAINERS` | Don't cleanup containers on failure | `false` |

