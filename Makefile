# Bib Makefile
# Main build and development targets

.PHONY: all build install clean
.PHONY: build-bib build-bibd build-all-platforms
.PHONY: proto proto-lint proto-breaking proto-clean
.PHONY: test test-unit test-integration test-e2e
.PHONY: lint fmt vet
.PHONY: tools deps
.PHONY: release release-snapshot release-dry-run

#------------------------------------------------------------------------------
# Variables
#------------------------------------------------------------------------------

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOFMT := gofmt
GOMOD := $(GOCMD) mod

# Binary names
BIB_BINARY := bib
BIBD_BINARY := bibd

# Directories
BIN_DIR := bin
CMD_DIR := cmd
API_DIR := api
PROTO_DIR := $(API_DIR)/proto
GEN_DIR := $(API_DIR)/gen/go
DIST_DIR := dist

# Version information (from git)
# Note: On Windows, use PowerShell or set these manually
ifeq ($(OS),Windows_NT)
    VERSION ?= $(shell git describe --tags --always --dirty 2>NUL || echo dev)
    COMMIT ?= $(shell git rev-parse --short HEAD 2>NUL || echo unknown)
    BUILD_TIME ?= $(shell powershell -Command "Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ'" 2>NUL || echo unknown)
else
    VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
    COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
    BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo unknown)
endif
# For release builds, set DEV_MODE=false
DEV_MODE ?= true

# Build-time variable injection
VERSION_PKG := bib/internal/version
BIB_VERSION_PKG := bib/cmd/bib/cmd/version

# Common ldflags for both binaries
COMMON_LDFLAGS := -s -w \
	-X '$(VERSION_PKG).Version=$(VERSION)' \
	-X '$(VERSION_PKG).Commit=$(COMMIT)' \
	-X '$(VERSION_PKG).BuildTime=$(BUILD_TIME)' \
	-X '$(VERSION_PKG).DevMode=$(DEV_MODE)'

# bib CLI specific ldflags (also injects into cmd/bib/cmd/version)
BIB_LDFLAGS := $(COMMON_LDFLAGS) \
	-X '$(BIB_VERSION_PKG).Version=$(VERSION)' \
	-X '$(BIB_VERSION_PKG).Commit=$(COMMIT)' \
	-X '$(BIB_VERSION_PKG).BuildDate=$(BUILD_TIME)'

# bibd daemon uses common ldflags
BIBD_LDFLAGS := $(COMMON_LDFLAGS)

# Build flags
BUILD_FLAGS := -trimpath

# Cross-compilation targets
# Format: GOOS/GOARCH
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	linux/arm \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64 \
	freebsd/amd64 \
	freebsd/arm64

# Proto tools
BUF := buf

# Test parameters
TEST_FLAGS ?= -v
TEST_TIMEOUT ?= 10m

# OS detection for commands
ifeq ($(OS),Windows_NT)
    MKDIR = if not exist $(subst /,\,$(1)) mkdir $(subst /,\,$(1))
    RM = del /Q
    RMDIR = rmdir /S /Q
    # Path separator for Windows
    PATHSEP = $(subst /,\,)
else
    MKDIR = mkdir -p $(1)
    RM = rm -f
    RMDIR = rm -rf
    PATHSEP =
endif

#------------------------------------------------------------------------------
# Default target
#------------------------------------------------------------------------------

all: build

#------------------------------------------------------------------------------
# Build targets
#------------------------------------------------------------------------------

build: build-bib build-bibd

build-bib:
	@echo "Building bib CLI $(VERSION)..."
	@$(call MKDIR,$(BIN_DIR))
ifeq ($(OS),Windows_NT)
	set CGO_ENABLED=0&& $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(BIB_LDFLAGS)" -o $(BIN_DIR)/$(BIB_BINARY).exe ./$(CMD_DIR)/bib
else
	CGO_ENABLED=0 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(BIB_LDFLAGS)" -o $(BIN_DIR)/$(BIB_BINARY) ./$(CMD_DIR)/bib
endif

build-bibd:
	@echo "Building bibd daemon $(VERSION)..."
	@$(call MKDIR,$(BIN_DIR))
ifeq ($(OS),Windows_NT)
	set CGO_ENABLED=0&& $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(BIBD_LDFLAGS)" -o $(BIN_DIR)/$(BIBD_BINARY).exe ./$(CMD_DIR)/bibd
else
	CGO_ENABLED=0 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(BIBD_LDFLAGS)" -o $(BIN_DIR)/$(BIBD_BINARY) ./$(CMD_DIR)/bibd
endif

# Build release binaries (with DEV_MODE=false)
build-release: DEV_MODE=false
build-release: build

install:
	@echo "Installing bib and bibd..."
ifeq ($(OS),Windows_NT)
	set CGO_ENABLED=0&& $(GOCMD) install -ldflags "$(BIB_LDFLAGS)" ./$(CMD_DIR)/bib
	set CGO_ENABLED=0&& $(GOCMD) install -ldflags "$(BIBD_LDFLAGS)" ./$(CMD_DIR)/bibd
else
	CGO_ENABLED=0 $(GOCMD) install -ldflags "$(BIB_LDFLAGS)" ./$(CMD_DIR)/bib
	CGO_ENABLED=0 $(GOCMD) install -ldflags "$(BIBD_LDFLAGS)" ./$(CMD_DIR)/bibd
endif

#------------------------------------------------------------------------------
# Cross-compilation targets
#------------------------------------------------------------------------------

# Build for a specific platform: make build-platform GOOS=linux GOARCH=amd64
build-platform:
	@echo "Building for $(GOOS)/$(GOARCH)..."
	@$(call MKDIR,$(DIST_DIR)/$(GOOS)_$(GOARCH))
ifeq ($(OS),Windows_NT)
	set CGO_ENABLED=0&& set GOOS=$(GOOS)&& set GOARCH=$(GOARCH)&& $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(BIB_LDFLAGS)" -o $(DIST_DIR)/$(GOOS)_$(GOARCH)/$(BIB_BINARY)$(if $(findstring windows,$(GOOS)),.exe,) ./$(CMD_DIR)/bib
	set CGO_ENABLED=0&& set GOOS=$(GOOS)&& set GOARCH=$(GOARCH)&& $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(BIBD_LDFLAGS)" -o $(DIST_DIR)/$(GOOS)_$(GOARCH)/$(BIBD_BINARY)$(if $(findstring windows,$(GOOS)),.exe,) ./$(CMD_DIR)/bibd
else
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(BIB_LDFLAGS)" \
		-o $(DIST_DIR)/$(GOOS)_$(GOARCH)/$(BIB_BINARY)$(if $(findstring windows,$(GOOS)),.exe,) ./$(CMD_DIR)/bib
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(BIBD_LDFLAGS)" \
		-o $(DIST_DIR)/$(GOOS)_$(GOARCH)/$(BIBD_BINARY)$(if $(findstring windows,$(GOOS)),.exe,) ./$(CMD_DIR)/bibd
endif

# Build for all platforms (Unix-only, use GoReleaser on Windows)
build-all-platforms: DEV_MODE=false
build-all-platforms:
ifeq ($(OS),Windows_NT)
	@echo "Cross-compilation from Windows is best done with GoReleaser:"
	@echo "  goreleaser build --snapshot --clean"
else
	@echo "Building for all platforms..."
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d'/' -f1); \
		GOARCH=$$(echo $$platform | cut -d'/' -f2); \
		echo "Building $$GOOS/$$GOARCH..."; \
		$(MAKE) build-platform GOOS=$$GOOS GOARCH=$$GOARCH DEV_MODE=false; \
	done
	@echo "All platform builds complete in $(DIST_DIR)/"
endif

clean:
	@echo "Cleaning build artifacts..."
ifeq ($(OS),Windows_NT)
	@if exist bin $(RMDIR) bin
	@if exist dist $(RMDIR) dist
else
	$(RMDIR) $(BIN_DIR)
	$(RMDIR) $(DIST_DIR)
endif

#------------------------------------------------------------------------------
# Release targets (GoReleaser)
#------------------------------------------------------------------------------

# Create a full release (requires GITHUB_TOKEN)
release:
	@echo "Creating release..."
	goreleaser release --clean

# Create a snapshot release (no publishing)
release-snapshot:
	@echo "Creating snapshot release..."
	goreleaser release --snapshot --clean

# Dry run release (validates configuration)
release-dry-run:
	@echo "Dry run release..."
	goreleaser release --skip=publish --clean

# Check GoReleaser configuration
release-check:
	@echo "Checking GoReleaser configuration..."
	goreleaser check

#------------------------------------------------------------------------------
# Proto targets
#------------------------------------------------------------------------------

# Generate Go code from proto files
proto: proto-deps proto-lint
	@echo "Generating Go code from proto files..."
	cd $(PROTO_DIR) && $(BUF) generate
	@echo "Proto generation complete. Output: $(GEN_DIR)"

# Install proto dependencies (buf modules)
proto-deps:
	@echo "Updating buf dependencies..."
	cd $(PROTO_DIR) && $(BUF) dep update

# Lint proto files
proto-lint:
	@echo "Linting proto files..."
	cd $(PROTO_DIR) && $(BUF) lint

# Check for breaking changes
proto-breaking:
	@echo "Checking for breaking changes..."
	cd $(PROTO_DIR) && $(BUF) breaking --against '.git#subdir=$(PROTO_DIR)'

# Clean generated proto code
proto-clean:
	@echo "Cleaning generated proto code..."
ifeq ($(OS),Windows_NT)
	@if exist api\gen\go $(RMDIR) api\gen\go
else
	$(RMDIR) $(GEN_DIR)
endif

# Format proto files
proto-fmt:
	@echo "Formatting proto files..."
	cd $(PROTO_DIR) && $(BUF) format -w
#------------------------------------------------------------------------------
# Test targets (delegate to test/Makefile)
#------------------------------------------------------------------------------

test: test-unit

test-unit:
	@echo "Running unit tests..."
	$(GOTEST) $(TEST_FLAGS) -short -timeout $(TEST_TIMEOUT) ./internal/... ./cmd/...

test-integration:
	@$(MAKE) -C test test-integration

test-e2e:
	@$(MAKE) -C test test-e2e

test-all:
	@$(MAKE) -C test test-all

test-coverage:
	@$(MAKE) -C test test-coverage

#------------------------------------------------------------------------------
# Code quality targets
#------------------------------------------------------------------------------

lint:
	@echo "Running linters..."
	golangci-lint run ./...

fmt:
	@echo "Formatting Go code..."
	$(GOFMT) -s -w .

vet:
	@echo "Running go vet..."
	$(GOVET) ./...

#------------------------------------------------------------------------------
# Dependency management
#------------------------------------------------------------------------------

deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

#------------------------------------------------------------------------------
# Tool installation
#------------------------------------------------------------------------------

tools: tools-buf tools-proto tools-lint tools-goreleaser

tools-buf:
	@echo "Installing buf CLI..."
	$(GOCMD) install github.com/bufbuild/buf/cmd/buf@latest

tools-proto:
	@echo "Installing protoc plugins..."
	$(GOCMD) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOCMD) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

tools-lint:
	@echo "Installing golangci-lint..."
	@echo "Please install from: https://golangci-lint.run/usage/install/"

tools-goreleaser:
	@echo "Installing GoReleaser..."
	$(GOCMD) install github.com/goreleaser/goreleaser/v2@latest

tools-nfpm:
	@echo "Installing nFPM..."
	$(GOCMD) install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest

# Install all tools at once
tools-all: tools-buf tools-proto tools-goreleaser tools-nfpm
	@echo "All tools installed!"

#------------------------------------------------------------------------------
# Development helpers
#------------------------------------------------------------------------------

# Run bibd in development mode
run-daemon:
	$(GOCMD) run ./$(CMD_DIR)/bibd

# Run bib CLI
run-cli:
	$(GOCMD) run ./$(CMD_DIR)/bib $(ARGS)

# Generate and build
gen: proto build

#------------------------------------------------------------------------------
# Help
#------------------------------------------------------------------------------

help:
	@echo "Bib Makefile targets:"
	@echo ""
	@echo "Build:"
	@echo "  make build              - Build bib and bibd for current platform"
	@echo "  make build-bib          - Build bib CLI only"
	@echo "  make build-bibd         - Build bibd daemon only"
	@echo "  make build-release      - Build with DEV_MODE=false"
	@echo "  make build-platform     - Build for specific GOOS/GOARCH"
	@echo "  make build-all-platforms - Build for all supported platforms"
	@echo "  make install            - Install to GOPATH/bin"
	@echo "  make clean              - Remove build artifacts"
	@echo ""
	@echo "Release:"
	@echo "  make release            - Create release with GoReleaser"
	@echo "  make release-snapshot   - Create snapshot (no publish)"
	@echo "  make release-dry-run    - Dry run (validate config)"
	@echo "  make release-check      - Check GoReleaser configuration"
	@echo ""
	@echo "Proto:"
	@echo "  make proto              - Generate Go code from proto files"
	@echo "  make proto-lint         - Lint proto files"
	@echo "  make proto-fmt          - Format proto files"
	@echo "  make proto-clean        - Remove generated proto code"
	@echo "  make proto-breaking     - Check for breaking proto changes"
	@echo ""
	@echo "Test:"
	@echo "  make test               - Run unit tests"
	@echo "  make test-unit          - Run unit tests"
	@echo "  make test-integration   - Run integration tests"
	@echo "  make test-e2e           - Run end-to-end tests"
	@echo "  make test-all           - Run all tests"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint               - Run linters"
	@echo "  make fmt                - Format Go code"
	@echo "  make vet                - Run go vet"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps               - Download dependencies"
	@echo "  make tidy               - Tidy go.mod"
	@echo "  make tools              - Install development tools"
	@echo "  make tools-all          - Install all tools"
	@echo ""
	@echo "Development:"
	@echo "  make run-daemon         - Run bibd in development mode"
	@echo "  make run-cli ARGS='...' - Run bib CLI with arguments"
	@echo "  make gen                - Generate proto and build"
	@echo ""
	@echo "Version Info:"
	@echo "  VERSION=$(VERSION)"
	@echo "  COMMIT=$(COMMIT)"
	@echo "  BUILD_TIME=$(BUILD_TIME)"

