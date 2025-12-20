# Dockerfile for bibd daemon
# Production-ready, security-hardened container image
# Uses distroless for minimal attack surface

# Build stage is handled by GoReleaser, this just packages the binary
FROM gcr.io/distroless/static-debian12:nonroot

# Labels for container metadata
LABEL org.opencontainers.image.title="bibd"
LABEL org.opencontainers.image.description="Bib daemon for distributed data management"
LABEL org.opencontainers.image.vendor="bencoepp"

# Copy the pre-built binary from GoReleaser
COPY bibd /usr/local/bin/bibd

# Default data directory (can be overridden with volume mount)
# Note: distroless doesn't have mkdir, so this must be mounted at runtime
VOLUME ["/data"]

# Expose default ports
# 8080 - gRPC API
# 9090 - Metrics (Prometheus)
# 4001 - libp2p swarm
EXPOSE 8080 9090 4001

# Run as non-root user (distroless nonroot user has UID 65532)
USER nonroot:nonroot

# Health check is handled externally (K8s probes, Docker Compose, etc.)
# The binary includes a health endpoint on the gRPC port

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/bibd"]

# Default command (can be overridden)
CMD ["--config", "/data/config.yaml"]

