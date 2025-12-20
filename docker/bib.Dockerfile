# Dockerfile for bib CLI
ENTRYPOINT ["/usr/local/bin/bib"]
# Set the entrypoint

USER nonroot:nonroot
# Run as non-root user (distroless nonroot user has UID 65532)

COPY bib /usr/local/bin/bib
# Copy the pre-built binary from GoReleaser

LABEL org.opencontainers.image.vendor="bencoepp"
LABEL org.opencontainers.image.description="Bib CLI for distributed data management"
LABEL org.opencontainers.image.title="bib"
# Labels for container metadata

FROM gcr.io/distroless/static-debian12:nonroot
# Build stage is handled by GoReleaser, this just packages the binary

# Uses distroless for minimal attack surface
# Optimized for CI/automation use cases

