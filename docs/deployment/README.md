# Deployment

This section covers deploying Bib in various environments.

## In This Section

| Document | Description |
|----------|-------------|
| [Kubernetes Deployment](kubernetes.md) | Deploying bibd with managed PostgreSQL on Kubernetes |

## Deployment Options

### Local Development

For local development, use the default configuration:

```bash
bib setup --daemon
bibd
```

This runs with:
- SQLite storage (lightweight)
- Proxy mode (no local data)
- No clustering

### Production (Single Node)

For production single-node deployments:

```yaml
# ~/.config/bibd/config.yaml
p2p:
  mode: full

database:
  backend: postgres
  postgres:
    managed: true
```

### Production (High Availability)

For HA deployments with clustering:

```yaml
# ~/.config/bibd/config.yaml
p2p:
  mode: full

database:
  backend: postgres
  postgres:
    managed: true

cluster:
  enabled: true
  cluster_name: "prod-cluster"
```

See [Clustering Guide](../guides/clustering.md) for setup instructions.

### Kubernetes

For Kubernetes deployments:

```yaml
database:
  postgres:
    managed: true
    container_runtime: kubernetes
```

See [Kubernetes Deployment](kubernetes.md) for detailed instructions.

## Deployment Checklist

- [ ] Configure TLS for gRPC connections
- [ ] Set appropriate node mode (proxy/selective/full)
- [ ] Configure storage backend (SQLite/PostgreSQL)
- [ ] Enable audit logging
- [ ] Set up credential rotation
- [ ] Configure backups
- [ ] Set up monitoring and alerting
- [ ] Document break glass procedures

---

[← Back to Documentation](../README.md)

