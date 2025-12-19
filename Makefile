# Bib Makefile
# Main build and development targets

.PHONY: all build install clean
.PHONY: proto proto-lint proto-breaking proto-clean
.PHONY: test test-unit test-integration test-e2e
.PHONY: lint fmt vet
.PHONY: tools deps

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

# Build flags
LDFLAGS := -ldflags "-s -w"
BUILD_FLAGS := -trimpath $(LDFLAGS)

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
	@echo "Building bib CLI..."
	@$(call MKDIR,$(BIN_DIR))
	$(GOBUILD) $(BUILD_FLAGS) -o $(BIN_DIR)/$(BIB_BINARY) ./$(CMD_DIR)/bib

build-bibd:
	@echo "Building bibd daemon..."
	@$(call MKDIR,$(BIN_DIR))
	$(GOBUILD) $(BUILD_FLAGS) -o $(BIN_DIR)/$(BIBD_BINARY) ./$(CMD_DIR)/bibd

install:
	@echo "Installing bib and bibd..."
	$(GOCMD) install ./$(CMD_DIR)/bib
	$(GOCMD) install ./$(CMD_DIR)/bibd

clean:
	@echo "Cleaning build artifacts..."
ifeq ($(OS),Windows_NT)
	@if exist bin $(RMDIR) bin
	@if exist dist $(RMDIR) dist
else
	$(RMDIR) $(BIN_DIR)
	$(RMDIR) dist/
endif

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

tools: tools-buf tools-proto tools-lint

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

# Install all tools at once
tools-all: tools-buf tools-proto
	@echo "All proto tools installed!"

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
	@echo "  make build        - Build bib and bibd"
	@echo "  make build-bib    - Build bib CLI only"
	@echo "  make build-bibd   - Build bibd daemon only"
	@echo "  make install      - Install to GOPATH/bin"
	@echo "  make clean        - Remove build artifacts"
	@echo ""
	@echo "Proto:"
	@echo "  make proto        - Generate Go code from proto files"
	@echo "  make proto-lint   - Lint proto files"
	@echo "  make proto-fmt    - Format proto files"
	@echo "  make proto-clean  - Remove generated proto code"
	@echo "  make proto-breaking - Check for breaking proto changes"
	@echo ""
	@echo "Test:"
	@echo "  make test         - Run unit tests"
	@echo "  make test-unit    - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-e2e     - Run end-to-end tests"
	@echo "  make test-all     - Run all tests"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint         - Run linters"
	@echo "  make fmt          - Format Go code"
	@echo "  make vet          - Run go vet"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps         - Download dependencies"
	@echo "  make tidy         - Tidy go.mod"
	@echo "  make tools        - Show tool installation instructions"
	@echo ""
	@echo "Development:"
	@echo "  make run-daemon   - Run bibd in development mode"
	@echo "  make run-cli ARGS='...' - Run bib CLI with arguments"
	@echo "  make gen          - Generate proto and build"

