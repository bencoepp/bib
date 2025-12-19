//go:build tools

// Package tools tracks tool dependencies for development.
// This file ensures tool versions are tracked in go.mod.
//
// To install all tools, run:
//
//	go generate -tags tools ./tools/...
//
// Or use the Makefile:
//
//	make tools-all
//
// See: https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go
//go:generate go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
//go:generate go install github.com/bufbuild/buf/cmd/buf
