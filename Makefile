SHELL := /usr/bin/env bash
MODULE := github.com/bencoepp/bib

.PHONY: all build build-all run-bib run-bibd gen tidy test lint

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

gen:
	buf generate

tidy:
	go mod tidy

test:
	go test ./...

lint:
	@echo "Add golangci-lint or staticcheck here"