# HealthService API

The HealthService provides health check and node information operations. This service is unauthenticated and can be used for load balancer health checks and monitoring.

## Service Definition

```protobuf
service HealthService {
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse);
  rpc GetNodeInfo(GetNodeInfoRequest) returns (GetNodeInfoResponse);
  rpc Ping(PingRequest) returns (PingResponse);
}
```

## Methods

### Check

Performs a health check of the server and its components.

**Authentication:** Not required

**Request:**
```protobuf
message HealthCheckRequest {
  string service = 1;  // Optional: specific service to check
}
```

**Response:**
```protobuf
message HealthCheckResponse {
  ServingStatus status = 1;
  map<string, ComponentHealth> components = 2;
  Timestamp timestamp = 3;
}
```

**Status Values:**
- `SERVING_STATUS_SERVING` - All components healthy
- `SERVING_STATUS_NOT_SERVING` - One or more components unhealthy
- `SERVING_STATUS_UNKNOWN` - Health status cannot be determined

**Example (Go):**
```go
resp, err := healthClient.Check(ctx, &services.HealthCheckRequest{})
if err != nil {
    log.Fatal(err)
}

if resp.Status == services.ServingStatus_SERVING_STATUS_SERVING {
    log.Println("Server is healthy")
} else {
    log.Printf("Server unhealthy: %v", resp.Components)
}
```

**Example (grpcurl):**
```bash
grpcurl -plaintext localhost:9090 bib.v1.services.HealthService/Check
```

### Watch

Streams health status changes in real-time.

**Authentication:** Not required

**Request:** Same as Check

**Response:** Stream of `HealthCheckResponse`

**Example (Go):**
```go
stream, err := healthClient.Watch(ctx, &services.HealthCheckRequest{})
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
    log.Printf("Health status: %v", resp.Status)
}
```

### GetNodeInfo

Returns detailed information about the node.

**Authentication:** Not required

**Request:**
```protobuf
message GetNodeInfoRequest {
  bool include_components = 1;  // Include component details
  bool include_network = 2;     // Include network statistics
  bool include_storage = 3;     // Include storage statistics
}
```

**Response:**
```protobuf
message GetNodeInfoResponse {
  string node_id = 1;        // P2P peer ID
  string mode = 2;           // "full", "selective", "proxy"
  string version = 3;        // Software version
  string commit = 4;         // Git commit hash
  Timestamp build_time = 5;
  Timestamp started_at = 6;
  Duration uptime = 7;
  // ... additional fields based on request options
}
```

**Example (Go):**
```go
resp, err := healthClient.GetNodeInfo(ctx, &services.GetNodeInfoRequest{
    IncludeComponents: true,
    IncludeNetwork:    true,
    IncludeStorage:    true,
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Node: %s (mode: %s, version: %s)", resp.NodeId, resp.Mode, resp.Version)
log.Printf("Uptime: %s", resp.Uptime.AsDuration())
```

### Ping

Simple connectivity check that echoes back the payload.

**Authentication:** Not required

**Request:**
```protobuf
message PingRequest {
  bytes payload = 1;  // Optional payload to echo
}
```

**Response:**
```protobuf
message PingResponse {
  bytes payload = 1;         // Echoed payload
  Timestamp timestamp = 2;   // Server timestamp
}
```

**Example (Go):**
```go
resp, err := healthClient.Ping(ctx, &services.PingRequest{
    Payload: []byte("hello"),
})
if err != nil {
    log.Fatal(err)
}

log.Printf("Pong: %s (server time: %v)", resp.Payload, resp.Timestamp.AsTime())
```

## Component Health

The health check includes status of individual components:

| Component | Description |
|-----------|-------------|
| `storage` | Database connectivity and health |
| `storage.postgres` | PostgreSQL connection pool |
| `storage.sqlite` | SQLite database status |
| `p2p` | P2P networking layer |
| `p2p.host` | libp2p host status |
| `p2p.dht` | DHT routing table |
| `p2p.pubsub` | GossipSub mesh |
| `cluster` | Raft cluster (if enabled) |
| `cluster.raft` | Raft consensus state |
| `cluster.peers` | Cluster peer connectivity |

## Use Cases

### Kubernetes Liveness Probe

```yaml
livenessProbe:
  grpc:
    port: 9090
    service: "bib.v1.services.HealthService"
  initialDelaySeconds: 10
  periodSeconds: 10
```

### Kubernetes Readiness Probe

```yaml
readinessProbe:
  grpc:
    port: 9090
    service: "bib.v1.services.HealthService"
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Load Balancer Health Check

```bash
# Using grpc-health-probe
grpc-health-probe -addr=localhost:9090

# Using grpcurl
grpcurl -plaintext localhost:9090 bib.v1.services.HealthService/Check
```

### Monitoring Dashboard

Use the Watch endpoint to stream health updates:

```go
func monitorHealth(client services.HealthServiceClient) {
    for {
        stream, err := client.Watch(context.Background(), &services.HealthCheckRequest{})
        if err != nil {
            log.Printf("Watch error: %v, retrying...", err)
            time.Sleep(5 * time.Second)
            continue
        }
        
        for {
            resp, err := stream.Recv()
            if err != nil {
                break
            }
            updateDashboard(resp)
        }
    }
}
```

