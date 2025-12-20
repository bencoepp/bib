# Docker Images

This directory contains all Dockerfiles for the Bib project.

## Files

| File | Description |
|------|-------------|
| `bib.Dockerfile` | Production image for the bib CLI (used by GoReleaser) |
| `bib.dev.Dockerfile` | Development image for the bib CLI (builds from source) |
| `bibd.Dockerfile` | Production image for the bibd daemon (used by GoReleaser) |
| `bibd.dev.Dockerfile` | Development image for the bibd daemon (builds from source) |

## Production Images

Production images use [distroless](https://github.com/GoogleContainerTools/distroless) base images for minimal attack surface. They expect pre-built binaries (typically from GoReleaser).

```bash
# Build production image manually (after building the binary)
docker build -f docker/bibd.Dockerfile -t bibd:latest .
docker build -f docker/bib.Dockerfile -t bib:latest .
```

## Development Images

Development images build from source and are used for local development and testing.

```bash
# Build development image
docker build -f docker/bibd.dev.Dockerfile -t bibd:dev .
docker build -f docker/bib.dev.Dockerfile -t bib:dev .
```

## Docker Compose

Use the root `docker-compose.yaml` for local development:

```bash
docker-compose up -d
```

This will build and run the development containers with all required services (PostgreSQL, etc.).

## Image Details

### bibd (Daemon)

- **Ports**:
  - `8080` - gRPC API
  - `9090` - Prometheus metrics
  - `4001` - libp2p swarm
- **Volumes**: `/data` for persistent storage
- **User**: Runs as non-root (UID 65532)

### bib (CLI)

- **User**: Runs as non-root (UID 65532)
- Designed for CI/automation use cases

