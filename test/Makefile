# Test Makefile
# Include this in your main Makefile or use directly

.PHONY: test test-unit test-integration test-e2e test-all
.PHONY: test-docker-up test-docker-down test-coverage
.PHONY: build-test-binaries

# Go test flags
TEST_FLAGS ?= -v
TEST_TIMEOUT ?= 10m
TEST_PARALLEL ?= 2
COVERAGE_FILE ?= coverage.out

# Docker settings
DOCKER_COMPOSE := docker-compose -f test/docker/docker-compose.test.yaml
TEST_POSTGRES_PORT ?= 5433

# Binary output directory
BIN_DIR := bin

#------------------------------------------------------------------------------
# Build
#------------------------------------------------------------------------------

build-test-binaries:
	@echo "Building test binaries..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/bib ./cmd/bib
	go build -o $(BIN_DIR)/bibd ./cmd/bibd

#------------------------------------------------------------------------------
# Unit Tests
#------------------------------------------------------------------------------

test-unit:
	@echo "Running unit tests..."
	go test $(TEST_FLAGS) -short -timeout $(TEST_TIMEOUT) ./internal/... ./cmd/...

#------------------------------------------------------------------------------
# Integration Tests
#------------------------------------------------------------------------------

test-integration: test-docker-up
	@echo "Running integration tests..."
	TEST_POSTGRES_PORT=$(TEST_POSTGRES_PORT) \
	go test $(TEST_FLAGS) -tags=integration -timeout $(TEST_TIMEOUT) -parallel $(TEST_PARALLEL) ./test/integration/...

test-integration-storage:
	@echo "Running storage integration tests..."
	go test $(TEST_FLAGS) -tags=integration -timeout $(TEST_TIMEOUT) ./test/integration/storage/...

test-integration-p2p:
	@echo "Running P2P integration tests..."
	go test $(TEST_FLAGS) -tags=integration -timeout $(TEST_TIMEOUT) ./test/integration/p2p/...

test-integration-grpc:
	@echo "Running gRPC integration tests..."
	go test $(TEST_FLAGS) -tags=integration -timeout $(TEST_TIMEOUT) ./test/integration/grpc/...

test-integration-cluster:
	@echo "Running cluster integration tests..."
	go test $(TEST_FLAGS) -tags=integration -timeout $(TEST_TIMEOUT) ./test/integration/cluster/...

#------------------------------------------------------------------------------
# End-to-End Tests
#------------------------------------------------------------------------------

test-e2e: build-test-binaries test-docker-up
	@echo "Running e2e tests..."
	BIB_BINARY=$(BIN_DIR)/bib \
	BIBD_BINARY=$(BIN_DIR)/bibd \
	TEST_POSTGRES_PORT=5434 \
	go test $(TEST_FLAGS) -tags=e2e -timeout $(TEST_TIMEOUT) ./test/e2e/...

test-e2e-scenarios: build-test-binaries test-docker-up
	@echo "Running e2e scenario tests..."
	BIB_BINARY=$(BIN_DIR)/bib \
	BIBD_BINARY=$(BIN_DIR)/bibd \
	go test $(TEST_FLAGS) -tags=e2e -timeout $(TEST_TIMEOUT) ./test/e2e/scenarios/...

#------------------------------------------------------------------------------
# All Tests
#------------------------------------------------------------------------------

test: test-unit

test-all: test-unit test-integration test-e2e
	@echo "All tests completed."

#------------------------------------------------------------------------------
# Docker Infrastructure
#------------------------------------------------------------------------------

test-docker-up:
	@echo "Starting test infrastructure..."
	$(DOCKER_COMPOSE) up -d postgres postgres-e2e
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5
	$(DOCKER_COMPOSE) exec -T postgres pg_isready -U bib_test -d bib_test || \
		(echo "Waiting longer for PostgreSQL..." && sleep 10)

test-docker-down:
	@echo "Stopping test infrastructure..."
	$(DOCKER_COMPOSE) down -v

test-docker-logs:
	$(DOCKER_COMPOSE) logs -f

test-docker-cluster-up:
	@echo "Starting cluster for testing..."
	$(DOCKER_COMPOSE) --profile cluster up -d

test-docker-cluster-down:
	@echo "Stopping cluster..."
	$(DOCKER_COMPOSE) --profile cluster down -v

#------------------------------------------------------------------------------
# Coverage
#------------------------------------------------------------------------------

test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./internal/... ./cmd/...
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report: coverage.html"

test-coverage-integration: test-docker-up
	go test -tags=integration -coverprofile=$(COVERAGE_FILE) -covermode=atomic \
		./internal/... ./test/integration/...
	go tool cover -html=$(COVERAGE_FILE) -o coverage-integration.html

#------------------------------------------------------------------------------
# CI Helpers
#------------------------------------------------------------------------------

ci-test:
	@echo "Running CI tests..."
	go test -v -race -short -timeout 5m ./...

ci-integration: test-docker-up
	@echo "Running CI integration tests..."
	go test -v -tags=integration -timeout 15m ./test/integration/...

ci-e2e: build-test-binaries test-docker-up
	@echo "Running CI e2e tests..."
	BIB_BINARY=$(BIN_DIR)/bib \
	BIBD_BINARY=$(BIN_DIR)/bibd \
	go test -v -tags=e2e -timeout 30m ./test/e2e/...

#------------------------------------------------------------------------------
# Cleanup
#------------------------------------------------------------------------------

test-clean:
	@echo "Cleaning test artifacts..."
	rm -f $(COVERAGE_FILE) coverage.html coverage-integration.html
	rm -rf $(BIN_DIR)
	$(DOCKER_COMPOSE) down -v --remove-orphans

#------------------------------------------------------------------------------
# Help
#------------------------------------------------------------------------------

test-help:
	@echo "Test Targets:"
	@echo "  test-unit              Run unit tests"
	@echo "  test-integration       Run integration tests (requires Docker)"
	@echo "  test-e2e               Run end-to-end tests (requires Docker)"
	@echo "  test-all               Run all tests"
	@echo ""
	@echo "  test-integration-storage   Run storage integration tests"
	@echo "  test-integration-p2p       Run P2P integration tests"
	@echo "  test-integration-grpc      Run gRPC integration tests"
	@echo "  test-integration-cluster   Run cluster integration tests"
	@echo ""
	@echo "  test-docker-up         Start test infrastructure"
	@echo "  test-docker-down       Stop test infrastructure"
	@echo "  test-docker-cluster-up Start cluster for testing"
	@echo ""
	@echo "  test-coverage          Generate coverage report"
	@echo "  test-clean             Clean test artifacts"

