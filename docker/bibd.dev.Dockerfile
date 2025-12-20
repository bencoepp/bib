# Development Dockerfile for bibd
# This builds from source for development use
# For production, use bibd.Dockerfile with pre-built binary

FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X 'bib/internal/version.Version=dev' -X 'bib/internal/version.DevMode=true'" \
    -o /bibd ./cmd/bibd

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary from builder
COPY --from=builder /bibd /usr/local/bin/bibd

# Copy timezone data and CA certificates
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Default data directory
VOLUME ["/data"]

# Expose ports
EXPOSE 8080 9090 4001

# Run as non-root
USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/bibd"]
CMD ["--config", "/etc/bibd/config.yaml"]

