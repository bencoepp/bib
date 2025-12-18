# Kubernetes Deployment Guide

This document describes how bibd manages PostgreSQL deployments in Kubernetes environments.

---

## Overview

bibd supports automatic PostgreSQL deployment and management in Kubernetes clusters. This implementation provides:

- **Automatic StatefulSet creation** with proper volume management
- **Service discovery** via Kubernetes Services (ClusterIP or NodePort)
- **Network isolation** via NetworkPolicies
- **Security** via RBAC, ServiceAccounts, and pod security contexts
- **Backup automation** via CronJobs
- **CloudNativePG (CNPG) operator support** (optional)
- **Automatic fallback** to Docker/Podman if Kubernetes fails

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐         ┌────────────────────────────────┐   │
│  │  bibd Pod    │◄───────►│  PostgreSQL StatefulSet        │   │
│  │              │         │  ┌──────────────────────────┐  │   │
│  │  - P2P Layer │         │  │  postgres:16-alpine      │  │   │
│  │  - Scheduler │         │  │  - PVC mounted           │  │   │
│  │  - Storage   │         │  │  - Secrets for creds     │  │   │
│  └──────────────┘         │  │  - Probes configured     │  │   │
│         │                 │  └──────────────────────────┘  │   │
│         │                 └────────────────────────────────┘   │
│         │                              │                       │
│         │                 ┌────────────┴──────────┐           │
│         └────────────────►│  Service (ClusterIP)  │           │
│                           │  - Port 5432          │           │
│                           └───────────────────────┘           │
│                                                                 │
│  ┌──────────────────┐    ┌──────────────────────────────┐    │
│  │  NetworkPolicy   │    │  Backup CronJob              │    │
│  │  - Allow bibd→DB │    │  - Daily pg_dump             │    │
│  └──────────────────┘    │  - Retention: 7 days         │    │
│                           └──────────────────────────────┘    │
│                                                                 │
│  ┌──────────────────┐    ┌──────────────────────────────┐    │
│  │  PVC (Data)      │    │  PVC (Backup)                │    │
│  │  - 10Gi default  │    │  - 20Gi default              │    │
│  └──────────────────┘    └──────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

## Configuration

### Basic Configuration

```yaml
database:
  backend: postgres
  postgres:
    managed: true
    container_runtime: kubernetes  # or "" for auto-detect
    
    # Kubernetes-specific configuration
    kubernetes:
      namespace: ""                    # Auto-detect (defaults to bibd pod namespace)
      storage_class_name: ""           # Use cluster default
      storage_size: "10Gi"
      
      # Service configuration
      service_type: ""                 # Auto-detect: ClusterIP (in-cluster) or NodePort (out-of-cluster)
      node_port: 0                     # Auto-assign if NodePort
      
      # Security
      network_policy_enabled: true
      network_policy_allowed_labels:
        app: bibd
      
      security_context:
        run_as_non_root: true
        run_as_user: 999              # postgres user
        run_as_group: 999
        fs_group: 999
        seccomp_profile: "runtime/default"
      
      # Resources
      resources:
        requests:
          cpu: "500m"
          memory: "512Mi"
        limits:
          cpu: "2"
          memory: "2Gi"
      
      # Backup configuration
      backup_enabled: true
      backup_schedule: "0 2 * * *"    # 2 AM daily
      backup_retention: 7              # Keep 7 backups
      backup_storage_size: "20Gi"
```

### Advanced Configuration

```yaml
database:
  postgres:
    kubernetes:
      # Use CloudNativePG operator (requires CNPG installed)
      use_cnpg: false
      cnpg_cluster_version: "16"
      
      # Pod scheduling
      pod_anti_affinity: true
      pod_anti_affinity_labels:
        app: bibd
      
      node_selector:
        disktype: ssd
      
      tolerations:
        - key: "dedicated"
          operator: "Equal"
          value: "database"
          effect: "NoSchedule"
      
      priority_class_name: "high-priority"
      
      # RBAC
      create_rbac: true
      service_account_name: ""        # Auto-generated
      
      # Probes
      liveness_probe:
        enabled: true
        initial_delay_seconds: 30
        period_seconds: 10
        timeout_seconds: 5
        failure_threshold: 3
      
      readiness_probe:
        enabled: true
        initial_delay_seconds: 5
        period_seconds: 10
        timeout_seconds: 5
        failure_threshold: 3
      
      startup_probe:
        enabled: true
        initial_delay_seconds: 0
        period_seconds: 10
        timeout_seconds: 5
        failure_threshold: 30          # 5 minutes for startup
      
      # Update strategy
      update_strategy: "RollingUpdate"
      delete_on_cleanup: true          # Delete resources on `bibd cleanup`
      
      # Custom labels and annotations
      labels:
        environment: production
        team: platform
      
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9187"
```

### S3 Backup Configuration

```yaml
database:
  postgres:
    kubernetes:
      backup_enabled: true
      backup_to_s3: true
      backup_s3:
        endpoint: "s3.amazonaws.com"
        region: "us-east-1"
        bucket: "bibd-backups"
        prefix: "postgres/"
        
        # Option 1: Use credentials (stored in Secret)
        access_key_id: "${AWS_ACCESS_KEY_ID}"
        secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
        
        # Option 2: Use IAM Roles for Service Accounts (EKS)
        use_irsa: true
        iam_role: "arn:aws:iam::123456789012:role/bibd-backup-role"
```

## In-Cluster vs Out-of-Cluster

bibd automatically detects whether it's running inside or outside a Kubernetes cluster:

### In-Cluster Deployment

When bibd runs as a pod in Kubernetes:

- Uses in-cluster config from ServiceAccount
- Creates ClusterIP Service (default)
- Connects to PostgreSQL via internal DNS: `bibd-postgres-<node-id>.<namespace>.svc.cluster.local`
- NetworkPolicy restricts access to bibd pods only

### Out-of-Cluster Deployment

When bibd runs outside the cluster (e.g., developer workstation):

- Uses kubeconfig from `~/.kube/config` or `--kubeconfig-path`
- Creates NodePort Service (default)
- Connects to PostgreSQL via Node IP and NodePort
- NetworkPolicy still restricts access (configure allowed IPs if needed)

## RBAC Permissions

bibd requires the following Kubernetes permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: bibd-postgres-manager
rules:
  # StatefulSets
  - apiGroups: ["apps"]
    resources: ["statefulsets"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  
  # Services
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  
  # Pods (for health checks)
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
  
  # PVCs
  - apiGroups: [""]
    resources: ["persistentvolumeclaims"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  
  # Secrets
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  
  # NetworkPolicies
  - apiGroups: ["networking.k8s.io"]
    resources: ["networkpolicies"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  
  # CronJobs (for backups)
  - apiGroups: ["batch"]
    resources: ["cronjobs"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  
  # ServiceAccounts, Roles, RoleBindings
  - apiGroups: [""]
    resources: ["serviceaccounts"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["roles", "rolebindings"]
    verbs: ["get", "list", "create", "update", "patch", "delete"]
```

## CloudNativePG (CNPG) Support

For production deployments, bibd can leverage the CloudNativePG operator:

### Prerequisites

1. Install CNPG operator:
```bash
kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.22/releases/cnpg-1.22.0.yaml
```

2. Enable CNPG in bibd config:
```yaml
database:
  postgres:
    kubernetes:
      use_cnpg: true
      cnpg_cluster_version: "16"
```

### Benefits of CNPG

- **High Availability**: Automatic failover and replica management
- **Backup & Recovery**: Built-in continuous backup and PITR
- **Connection Pooling**: Integrated PgBouncer
- **Monitoring**: Native Prometheus metrics
- **Rolling Updates**: Zero-downtime PostgreSQL upgrades

**Note**: CNPG support is planned but not yet fully implemented in this release.

## Backup and Recovery

### Automatic Backups

bibd creates a CronJob that performs daily backups:

```bash
# View backup CronJob
kubectl get cronjob -l app=bibd-postgres

# View backup history
kubectl get jobs -l app=bibd-postgres-backup

# Manual backup trigger
kubectl create job --from=cronjob/bibd-postgres-<node-id>-backup manual-backup-$(date +%s)
```

### Backup Storage

Backups are stored in:
- **PVC**: Default, stores in Kubernetes PersistentVolume
- **S3**: Optional, stores in S3-compatible object storage

### Restore Procedure

To restore from a backup:

```bash
# 1. Stop bibd
bib admin stop

# 2. Scale down PostgreSQL StatefulSet
kubectl scale statefulset bibd-postgres-<node-id> --replicas=0

# 3. Delete the PVC (this will delete data!)
kubectl delete pvc postgres-data-<node-id>

# 4. Restore from backup
kubectl exec -it bibd-postgres-<node-id>-backup-pod -- \
  gunzip -c /backup/backup-YYYYMMDD-HHMMSS.sql.gz | \
  psql -U postgres -d bibd

# 5. Scale up PostgreSQL
kubectl scale statefulset bibd-postgres-<node-id> --replicas=1

# 6. Start bibd
bib admin start
```

## Troubleshooting

### PostgreSQL Pod Not Starting

```bash
# Check pod status
kubectl get pods -l app=bibd-postgres

# Check pod logs
kubectl logs bibd-postgres-<node-id>-0

# Check events
kubectl describe pod bibd-postgres-<node-id>-0
```

Common issues:
- **Insufficient permissions**: Check RBAC configuration
- **Storage not available**: Check StorageClass and PVC status
- **Image pull errors**: Check ImagePullSecrets
- **Resource limits**: Check node resources and resource requests/limits

### Connection Issues

```bash
# Test connection from bibd pod
kubectl exec -it bibd-<pod-id> -- \
  psql -h bibd-postgres-<node-id> -U postgres -d bibd

# Check Service
kubectl get svc bibd-postgres-<node-id>

# Check NetworkPolicy
kubectl get networkpolicy bibd-postgres-policy-<node-id>
```

### Backup Failures

```bash
# Check CronJob configuration
kubectl get cronjob bibd-postgres-<node-id>-backup -o yaml

# Check recent backup jobs
kubectl get jobs -l cronjob-name=bibd-postgres-<node-id>-backup

# Check backup pod logs
kubectl logs job/bibd-postgres-<node-id>-backup-<job-id>
```

## Migration from Docker/Podman

To migrate from Docker/Podman to Kubernetes:

1. **Backup existing data**:
```bash
bib admin backup --output /path/to/backup.sql.gz
```

2. **Update configuration**:
```yaml
database:
  postgres:
    container_runtime: kubernetes
```

3. **Restart bibd**:
```bash
bib admin stop
bib admin start
```

4. **Verify migration**:
```bash
bib admin status
kubectl get pods -l app=bibd-postgres
```

## Security Considerations

### Network Isolation

- NetworkPolicy restricts access to PostgreSQL pods
- Only pods with `app: bibd` label can connect by default
- Configure `network_policy_allowed_labels` for custom access control

### Credential Management

- PostgreSQL credentials are generated automatically
- Stored in Kubernetes Secrets
- Secrets are mounted as environment variables
- Credentials rotate automatically (configurable interval)

### Pod Security

- Runs as non-root user (UID 999)
- SeccompProfile: `runtime/default`
- ReadOnlyRootFilesystem where possible
- Drop all capabilities except required ones

### TLS/mTLS

- TLS enabled by default for PostgreSQL connections
- Certificates auto-generated from node identity
- mTLS between bibd and PostgreSQL (when configured)

## Performance Tuning

### Resource Allocation

Adjust based on workload:

```yaml
kubernetes:
  resources:
    requests:
      cpu: "1"
      memory: "2Gi"
    limits:
      cpu: "4"
      memory: "8Gi"
```

### Storage Performance

Use appropriate StorageClass for your workload:

```yaml
kubernetes:
  storage_class_name: "fast-ssd"  # High-performance SSD
  storage_size: "100Gi"
```

### PostgreSQL Configuration

Configure PostgreSQL parameters via ConfigMaps (advanced):

```yaml
# Note: Direct PostgreSQL configuration not yet exposed
# Use CNPG for advanced PostgreSQL tuning
```

## Monitoring

### Metrics

bibd exposes PostgreSQL metrics when monitoring is enabled:

- Connection pool stats
- Query performance
- Storage usage
- Backup status

### Logging

PostgreSQL logs are available via:

```bash
kubectl logs -f bibd-postgres-<node-id>-0
```

### Health Checks

Monitor health via:

```bash
# Liveness probe
kubectl exec bibd-postgres-<node-id>-0 -- pg_isready -U postgres

# Readiness probe
kubectl get pod bibd-postgres-<node-id>-0 -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

---

## Related Documentation

| Document | Topic |
|----------|-------|
| [Storage Lifecycle](../storage/storage-lifecycle.md) | Database backend management |
| [Configuration](../getting-started/configuration.md) | Kubernetes configuration options |
| [Architecture Overview](../concepts/architecture.md) | System design overview |
| [Database Security](../storage/database-security.md) | Security hardening |

