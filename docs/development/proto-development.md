# Protocol Buffer Development Guide

This document describes how to work with Protocol Buffer definitions in the bib project.

## Prerequisites

Before working with proto files, you need to install the required tools:

```bash
# Install all proto tools at once
make tools-all

# Or install individually:
make tools-buf     # Installs buf CLI
make tools-proto   # Installs protoc-gen-go and protoc-gen-go-grpc
```

### Verify Installation

```bash
buf --version
# Should output: 1.x.x

which protoc-gen-go
which protoc-gen-go-grpc
# Both should point to files in your GOPATH/bin
```

## Directory Structure

```
api/
├── proto/                    # Proto source files
│   ├── buf.yaml              # Buf module configuration
│   ├── buf.gen.yaml          # Code generation configuration
│   └── bib/v1/               # Proto definitions
│       ├── common.proto      # Shared types
│       ├── auth.proto        # AuthService
│       ├── user.proto        # UserService
│       ├── breakglass.proto  # BreakGlassService
│       ├── discovery.proto   # P2P discovery protocol
│       ├── data.proto        # P2P data transfer protocol
│       ├── sync.proto        # P2P sync protocol
│       ├── pubsub.proto      # GossipSub messages
│       └── jobs.proto        # Job protocol (placeholder)
│
└── gen/go/                   # Generated Go code (committed)
    └── bib/v1/
        ├── *.pb.go           # Protobuf types
        └── *_grpc.pb.go      # gRPC service stubs
```

## Common Tasks

### Generate Code

After modifying any `.proto` file:

```bash
make proto
```

This will:
1. Update buf dependencies (if any)
2. Lint proto files
3. Generate Go code to `api/gen/go/`

### Lint Proto Files

To check proto files for style issues:

```bash
make proto-lint
```

### Format Proto Files

To auto-format proto files:

```bash
make proto-fmt
```

### Check for Breaking Changes

Before merging changes, check for breaking API changes:

```bash
make proto-breaking
```

### Clean Generated Code

To remove all generated code:

```bash
make proto-clean
```

## Writing Proto Files

### Package Naming

All proto files use the `bib.v1` package:

```protobuf
syntax = "proto3";
package bib.v1;
option go_package = "bib/api/gen/go/bib/v1;bibv1";
```

### Service Definitions

gRPC services should be defined with clear RPC methods:

```protobuf
service ExampleService {
  // GetExample retrieves an example by ID.
  rpc GetExample(GetExampleRequest) returns (GetExampleResponse);
  
  // ListExamples lists examples with filtering.
  rpc ListExamples(ListExamplesRequest) returns (ListExamplesResponse);
  
  // StreamExamples streams example updates.
  rpc StreamExamples(StreamExamplesRequest) returns (stream ExampleEvent);
}
```

### Message Naming Conventions

- Request messages: `<RpcName>Request`
- Response messages: `<RpcName>Response`
- Use singular nouns for entity messages
- Use plural for collections

### Common Types

Import common types from `common.proto`:

```protobuf
import "bib/v1/common.proto";
```

Available common types:
- `PeerInfo` - Node/peer information
- `TopicInfo` - Topic metadata
- `DatasetInfo` - Dataset metadata
- `CatalogEntry` - Lightweight catalog entry
- `Error` - Error response

### Timestamps

Use `google.protobuf.Timestamp` for time fields:

```protobuf
import "google/protobuf/timestamp.proto";

message Example {
  google.protobuf.Timestamp created_at = 1;
}
```

## Using Generated Code

### Importing

```go
import (
    bibv1 "bib/api/gen/go/bib/v1"
)
```

### Creating Messages

```go
user := &bibv1.User{
    Id:   "user123",
    Name: "John Doe",
}
```

### Implementing Services

```go
type authServer struct {
    bibv1.UnimplementedAuthServiceServer
}

func (s *authServer) Challenge(ctx context.Context, req *bibv1.ChallengeRequest) (*bibv1.ChallengeResponse, error) {
    // Implementation
}
```

## Troubleshooting

### "buf: command not found"

Make sure your `GOPATH/bin` is in your PATH:

```bash
# Add to your shell profile:
export PATH="$PATH:$(go env GOPATH)/bin"
```

### "protoc-gen-go: program not found"

Install the proto plugins:

```bash
make tools-proto
```

### Import errors in generated code

Run `go mod tidy` after generating:

```bash
make proto
go mod tidy
```

## Version Control

Generated code is committed to the repository to:
- Allow building without proto tools installed
- Ensure consistent generated code across environments
- Make code review easier for API changes

When modifying proto files:
1. Update the `.proto` file
2. Run `make proto`
3. Commit both the `.proto` and generated `.pb.go` files together

