# Storage & Security

This section covers database storage, security architecture, and emergency access procedures.

## In This Section

| Document | Description |
|----------|-------------|
| [Storage Lifecycle](storage-lifecycle.md) | Database backend management (SQLite/PostgreSQL) |
| [Database Security](database-security.md) | Credentials, roles, encryption, and hardening |
| [Break Glass Access](break-glass.md) | Emergency database access procedures |

## Overview

Bib supports two storage backends:

| Backend | Type | Use Case |
|---------|------|----------|
| **SQLite** | Embedded file-based | Development, proxy mode |
| **PostgreSQL** | Managed container | Production, full mode |

## Security Features

```
┌─────────────────────────────────────────────────────────────┐
│                    Security Architecture                     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐         ┌────────────────────────────┐   │
│  │    bibd      │◄──mTLS──│   Managed PostgreSQL       │   │
│  │              │         │                            │   │
│  │ • Credential │         │ • Unix socket (Linux)      │   │
│  │   Manager    │         │ • 127.0.0.1 only (TCP)     │   │
│  │ • Role Pool  │         │ • Role-based access        │   │
│  │ • Audit Log  │         │ • Full audit logging       │   │
│  └──────────────┘         └────────────────────────────┘   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Key Security Features

| Feature | Description |
|---------|-------------|
| **Auto-generated Credentials** | 256-bit cryptographic passwords |
| **Role-based Access** | 6 PostgreSQL roles with minimal permissions |
| **Credential Rotation** | Zero-downtime automatic rotation |
| **Encryption at Rest** | X25519/HKDF/AES-256-GCM options |
| **Full Audit Logging** | Complete operation audit trail |
| **Break Glass** | Controlled emergency access |

---

[← Back to Documentation](../README.md)

