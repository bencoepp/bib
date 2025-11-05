SHELL := /usr/bin/env bash
MODULE := github.com/bencoepp/bib

# proto layout
PROTO_DIR := api
OUT_DIR := internal/pb

.PHONY: all build build-all run-bib run-bibd gen buf-gen proto-gen proto-clean proto-check tidy test lint

all: build

build:
	go build ./...

build-all:
	go build -o bin/bib ./cmd/bib
	go build -o bin/bibd ./cmd/bibd

run-bib:
	go run ./cmd/bib

run-bibd:
	go run ./cmd/bibd

# Cross-platform shell handling:
# On Windows, Make uses OS=Windows_NT. We prefer to run the Makefile in an environment
# with bash available (Git Bash, MSYS2, or WSL). If you run make from cmd/powershell
# without bash in PATH, proto-gen will fail with a helpful message.
ifeq ($(OS),Windows_NT)
  # Use "bash" on Windows — ensure Git Bash or MSYS2 is in PATH, or run in WSL.
  SHELL := bash
endif

# generate using buf if configured, otherwise use protoc + plugins
gen: proto-gen
	@if [ -f buf.yaml ] || [ -f buf.gen.yaml ]; then \
	  echo "Running buf generate..."; \
	  command -v buf >/dev/null 2>&1 || (echo "buf not found; install buf or remove buf.yaml/buf.gen.yaml"; exit 1); \
	  buf generate; \
	else \
	  echo "No buf config found, proto-gen already ran."; \
	fi

# explicit buf target
buf-gen:
	@command -v buf >/dev/null 2>&1 || (echo "Install buf: https://docs.buf.build/installation"; exit 1)
	buf generate

# Find proto files
PROTO_FILES := $(shell find $(PROTO_DIR) -name '*.proto' 2>/dev/null || true)

# Generate Go protobuf + gRPC code into $(OUT_DIR)
proto-gen:
	@echo "Generating protobuf Go code to $(OUT_DIR)..."
	@command -v protoc >/dev/null 2>&1 || (echo "protoc not found; install protoc (see README or docs)"; exit 1)
	@command -v protoc-gen-go >/dev/null 2>&1 || (echo "protoc-gen-go not found; install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1)
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || (echo "protoc-gen-go-grpc not found; install with: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; exit 1)
	@mkdir -p $(OUT_DIR)
	protoc \
	  -I $(PROTO_DIR) \
	  --go_out=paths=source_relative:$(OUT_DIR) \
	  --go-grpc_out=paths=source_relative:$(OUT_DIR) \
	  $(PROTO_FILES)

proto-clean:
	@echo "Cleaning generated protos in $(OUT_DIR)"
	-rm -rf $(OUT_DIR)/*

# CI helper: fail if generated files are out of date
proto-check:
	@$(MAKE) proto-gen
	@if git status --porcelain | grep -E '\.pb\.go$$' >/dev/null 2>&1; then \
	  echo "Generated protobuf files are out of date. Commit the changes."; \
	  git status --porcelain | grep -E '\.pb\.go$$'; \
	  exit 1; \
	else \
	  echo "Protobuf generated files up-to-date."; \
	fi

tidy:
	go mod tidy

test:
	go test ./...

lint:
	@echo "Add golangci-lint or staticcheck here"