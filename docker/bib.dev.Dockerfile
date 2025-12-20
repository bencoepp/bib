# Development Dockerfile for bib CLI
# This builds from source for development use

FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X 'bib/internal/version.Version=dev' -X 'bib/internal/version.DevMode=true' -X 'bib/cmd/bib/cmd/version.Version=dev'" \
    -o /bib ./cmd/bib

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

# Copy binary from builder
COPY --from=builder /bib /usr/local/bin/bib

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Run as non-root
USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/bib"]

