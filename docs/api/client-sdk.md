# BIB Go Client SDK

The BIB Go client SDK provides a convenient way to interact with bibd from Go applications.

## Installation

```bash
go get bib/internal/grpc/client
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "bib/internal/grpc/client"
)

func main() {
    // Create client with default options (connects to local daemon)
    c, err := client.New(client.DefaultOptions())
    if err != nil {
        log.Fatal(err)
    }
    
    // Connect to the daemon
    ctx := context.Background()
    if err := c.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    // Authenticate with SSH key
    if err := c.Authenticate(ctx); err != nil {
        log.Fatal(err)
    }
    
    // Use the API
    topics, err := c.Topic().ListTopics(ctx, &services.ListTopicsRequest{})
    if err != nil {
        log.Fatal(err)
    }
    
    for _, topic := range topics.Topics {
        log.Printf("Topic: %s", topic.Name)
    }
}
```

## Connection Options

### Default Local Connection

```go
// Uses Unix socket (Linux/macOS) or named pipe (Windows)
opts := client.DefaultOptions()
```

### TCP Connection

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
}
```

### TLS Connection

```go
opts := client.Options{
    TCPAddress: "bibd.example.com:9090",
    TLS: &client.TLSOptions{
        Enabled:    true,
        CertFile:   "/path/to/client.crt",
        KeyFile:    "/path/to/client.key",
        CAFile:     "/path/to/ca.crt",
        ServerName: "bibd.example.com",
    },
}
```

### P2P Connection

```go
opts := client.Options{
    P2PPeerID: "12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN",
    P2PConfig: &config.P2PConfig{
        Enabled: true,
    },
}
```

### Connection Pool

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
    PoolSize:   4,  // Maintain 4 connections
}
```

### Parallel Connection Mode

Try multiple connection methods simultaneously:

```go
opts := client.Options{
    UnixSocket: "/var/run/bibd.sock",
    TCPAddress: "localhost:9090",
    Mode:       client.ConnectionModeParallel,  // Use first successful
}
```

## Authentication

### Using SSH Agent

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
    Auth: client.AuthOptions{
        UseAgent: true,  // Use SSH agent
    },
}

c, _ := client.New(opts)
c.Connect(ctx)
c.Authenticate(ctx)  // Will use SSH agent for signing
```

### Using Key File

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
    Auth: client.AuthOptions{
        KeyFile:    "~/.ssh/id_ed25519",
        Passphrase: "optional-passphrase",
    },
}
```

### Using Key Bytes

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
    Auth: client.AuthOptions{
        PrivateKey: privateKeyBytes,
    },
}
```

### Manual Token

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
    Auth: client.AuthOptions{
        Token: "existing-session-token",
    },
}
```

## Service Clients

Access service clients through the main client:

```go
c, _ := client.New(opts)
c.Connect(ctx)
c.Authenticate(ctx)

// Service clients
health := c.Health()     // HealthServiceClient
auth := c.Auth()         // AuthServiceClient
user := c.User()         // UserServiceClient
topic := c.Topic()       // TopicServiceClient
dataset := c.Dataset()   // DatasetServiceClient
job := c.Job()           // JobServiceClient
query := c.Query()       // QueryServiceClient
admin := c.Admin()       // AdminServiceClient
node := c.Node()         // NodeServiceClient
```

## Error Handling

### Checking Error Types

```go
import (
    "bib/internal/grpc/client"
    "google.golang.org/grpc/codes"
)

resp, err := c.Topic().GetTopic(ctx, req)
if err != nil {
    // Check for specific error types
    if client.IsNotFound(err) {
        log.Println("Topic not found")
        return
    }
    if client.IsPermissionDenied(err) {
        log.Println("Access denied")
        return
    }
    if client.IsUnauthenticated(err) {
        // Re-authenticate
        c.Authenticate(ctx)
        // Retry...
    }
    
    // Get gRPC code
    code := client.Code(err)
    log.Printf("Error code: %v", code)
    
    // Get error details
    details := client.ErrorDetails(err)
    for _, d := range details {
        log.Printf("Detail: %v", d)
    }
}
```

### Error Helper Functions

```go
// Check error types
client.IsNotFound(err) bool
client.IsPermissionDenied(err) bool
client.IsUnauthenticated(err) bool
client.IsInvalidArgument(err) bool
client.IsUnavailable(err) bool
client.IsDeadlineExceeded(err) bool

// Get error info
client.Code(err) codes.Code
client.Message(err) string
client.ErrorDetails(err) []interface{}
client.FieldViolations(err) []FieldViolation
```

## Retry and Timeout

### Request Timeouts

```go
// Per-request timeout
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

resp, err := c.Dataset().GetDataset(ctx, req)
```

### Global Timeouts

```go
opts := client.Options{
    TCPAddress:     "localhost:9090",
    ConnectTimeout: 10 * time.Second,
    RequestTimeout: 30 * time.Second,
}
```

### Automatic Retry

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
    Retry: &client.RetryOptions{
        MaxRetries:     3,
        InitialBackoff: 100 * time.Millisecond,
        MaxBackoff:     5 * time.Second,
        BackoffMultiplier: 2.0,
        RetryableCodes: []codes.Code{
            codes.Unavailable,
            codes.ResourceExhausted,
        },
    },
}
```

## Streaming

### Receiving Streams

```go
// Watch health changes
stream, err := c.Health().Watch(ctx, &services.HealthCheckRequest{})
if err != nil {
    log.Fatal(err)
}

for {
    resp, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Health: %v", resp.Status)
}
```

### Upload Streams

```go
// Upload dataset content
stream, err := c.Dataset().UploadContent(ctx)
if err != nil {
    log.Fatal(err)
}

// Send chunks
for _, chunk := range chunks {
    if err := stream.Send(&services.UploadContentRequest{
        Chunk: chunk,
    }); err != nil {
        log.Fatal(err)
    }
}

// Close and get response
resp, err := stream.CloseAndRecv()
if err != nil {
    log.Fatal(err)
}
log.Printf("Upload complete: %s", resp.DatasetId)
```

## Interceptors

### Logging

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
    Interceptors: []grpc.UnaryClientInterceptor{
        client.LoggingInterceptor(logger),
    },
}
```

### Metrics

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
    Interceptors: []grpc.UnaryClientInterceptor{
        client.MetricsInterceptor(registry),
    },
}
```

### Tracing

```go
opts := client.Options{
    TCPAddress: "localhost:9090",
    Interceptors: []grpc.UnaryClientInterceptor{
        client.TracingInterceptor(tracer),
    },
}
```

## Connection State

### Checking Connection

```go
if c.IsConnected() {
    log.Println("Connected")
}

state := c.State()  // connectivity.State
log.Printf("Connection state: %v", state)
```

### Reconnection

```go
// Manual reconnect
if err := c.Reconnect(ctx); err != nil {
    log.Fatal(err)
}

// Auto-reconnect is enabled by default
opts := client.Options{
    TCPAddress:   "localhost:9090",
    AutoReconnect: true,  // default
}
```

### Connection Events

```go
c.OnStateChange(func(old, new connectivity.State) {
    log.Printf("Connection state: %v -> %v", old, new)
})
```

## Complete Example

```go
package main

import (
    "context"
    "log"
    "time"
    
    "bib/internal/grpc/client"
    services "bib/api/gen/go/bib/v1/services"
)

func main() {
    ctx := context.Background()
    
    // Create client with full configuration
    c, err := client.New(client.Options{
        TCPAddress:     "localhost:9090",
        ConnectTimeout: 10 * time.Second,
        RequestTimeout: 30 * time.Second,
        PoolSize:       2,
        Auth: client.AuthOptions{
            UseAgent: true,
        },
        Retry: &client.RetryOptions{
            MaxRetries:     3,
            InitialBackoff: 100 * time.Millisecond,
        },
    })
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer c.Close()
    
    // Connect
    if err := c.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    
    // Authenticate
    if err := c.Authenticate(ctx); err != nil {
        log.Fatalf("Failed to authenticate: %v", err)
    }
    log.Println("Authenticated successfully")
    
    // List topics
    topicsResp, err := c.Topic().ListTopics(ctx, &services.ListTopicsRequest{
        Page: &services.PageRequest{Limit: 10},
    })
    if err != nil {
        if client.IsPermissionDenied(err) {
            log.Println("No permission to list topics")
            return
        }
        log.Fatalf("Failed to list topics: %v", err)
    }
    
    log.Printf("Found %d topics:", len(topicsResp.Topics))
    for _, topic := range topicsResp.Topics {
        log.Printf("  - %s: %s", topic.Id, topic.Name)
    }
    
    // Create a topic (admin only)
    createResp, err := c.Topic().CreateTopic(ctx, &services.CreateTopicRequest{
        Name:        "my-research-topic",
        Description: "A topic for research data",
        Tags:        []string{"research", "data"},
    })
    if err != nil {
        if client.IsPermissionDenied(err) {
            log.Println("Admin role required to create topics")
            return
        }
        log.Fatalf("Failed to create topic: %v", err)
    }
    
    log.Printf("Created topic: %s", createResp.Topic.Id)
    
    // Logout
    if _, err := c.Auth().Logout(ctx, &services.LogoutRequest{}); err != nil {
        log.Printf("Logout warning: %v", err)
    }
    
    log.Println("Done")
}
```

## Best Practices

1. **Reuse clients**: Create one client and reuse it
2. **Use connection pooling**: Set `PoolSize` for concurrent requests
3. **Handle errors properly**: Check for specific error types
4. **Set timeouts**: Always use context timeouts
5. **Cleanup resources**: Call `Close()` when done
6. **Monitor connection state**: Handle disconnections gracefully
7. **Refresh sessions**: Refresh before expiration

