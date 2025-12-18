# Break Glass Emergency Access

This document describes the break glass emergency access feature for bibd's PostgreSQL storage layer. Break glass provides controlled emergency access to the database for disaster recovery and debugging scenarios.

---

## Overview

Break glass is a security mechanism that allows authorized administrators to temporarily bypass normal access controls in emergency situations. The feature is:

- **Disabled by default** - Must be explicitly enabled in configuration
- **Time-limited** - Sessions automatically expire after a configured duration
- **Fully audited** - All operations are logged without redaction
- **Requires acknowledgment** - Completed sessions must be acknowledged by an administrator

```
┌─────────────────────────────────────────────────────────────┐
│                    Break Glass Flow                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Admin requests break glass                               │
│     └─> bib admin break-glass enable --reason "..."          │
│                                                              │
│  2. Interactive authentication challenge                     │
│     └─> Sign nonce with Ed25519 private key                  │
│                                                              │
│  3. Session created                                          │
│     └─> Time-limited PostgreSQL user created                 │
│     └─> Connection string provided                           │
│     └─> Notifications sent                                   │
│                                                              │
│  4. Emergency access                                         │
│     └─> All queries logged (paranoid mode)                   │
│     └─> Session recorded                                     │
│                                                              │
│  5. Session ends (expiry or manual disable)                  │
│     └─> Database user dropped                                │
│     └─> Report generated                                     │
│                                                              │
│  6. Acknowledgment required                                  │
│     └─> Admin reviews report                                 │
│     └─> bib admin break-glass acknowledge                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

Break glass is configured in the `database.break_glass` section of the bibd configuration:

```yaml
database:
  break_glass:
    # Enable break glass functionality (requires restart to change)
    enabled: false
    
    # Require bibd restart to enable/disable (security feature)
    require_restart: true
    
    # Maximum session duration
    max_duration: 1h
    
    # Default access level: "readonly" or "readwrite"
    default_access_level: readonly
    
    # Pre-configured emergency access users
    allowed_users:
      - name: "emergency_admin"
        public_key: "ssh-ed25519 AAAA..."
        access_level: readonly  # Optional, overrides default
    
    # Audit verbosity: "normal" or "paranoid"
    # Paranoid mode logs full query parameters without redaction
    audit_level: paranoid
    
    # Require admin acknowledgment after session ends
    require_acknowledgment: true
    
    # Enable terminal session recording
    session_recording: true
    
    # Path for session recordings (defaults to audit log path)
    recording_path: ""
    
    # Notification endpoints
    notification:
      webhook: "https://alerts.example.com/break-glass"
      email: "security@example.com"
```

## Setup

### 1. Generate an Emergency Access Key

Create a dedicated Ed25519 SSH key for break glass access:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_breakglass -C "emergency_admin@bib"
```

### 2. Configure Break Glass Users

Add the public key to your bibd configuration:

```yaml
database:
  break_glass:
    enabled: true
    allowed_users:
      - name: "emergency_admin"
        public_key: "ssh-ed25519 AAAA... emergency_admin@bib"
```

### 3. Restart bibd

Break glass configuration changes require a daemon restart:

```bash
systemctl restart bibd
```

## Usage

### Enabling a Break Glass Session

```bash
# Enable with default settings (max duration, read-only)
bib admin break-glass enable --reason "investigating data corruption"

# Enable with custom duration
bib admin break-glass enable --reason "emergency fix" --duration 30m

# Enable with write access
bib admin break-glass enable --reason "data recovery" --access-level readwrite

# Specify a different key file
bib admin break-glass enable --reason "DR drill" --key ~/.ssh/emergency_key
```

The command will:
1. Request an authentication challenge from the daemon
2. Prompt you to sign the challenge with your private key
3. Display a PostgreSQL connection string

### Using the Connection

The connection string can be used directly with `psql` or any PostgreSQL client:

```bash
psql "postgresql://breakglass_abc12345:PASSWORD@localhost:5432/bib?sslmode=require"
```

**Important Notes:**
- The `audit_log` table is **always** off-limits, regardless of access level
- All queries are logged without redaction (paranoid mode)
- The session will automatically expire at the configured time

### Checking Status

```bash
bib admin break-glass status
```

Shows:
- Whether break glass is enabled in configuration
- Any active sessions
- Sessions pending acknowledgment

### Disabling a Session Early

```bash
bib admin break-glass disable
```

This immediately:
- Terminates any active database connections
- Drops the temporary database user
- Generates a session report

### Acknowledging Sessions

After a session ends, it must be acknowledged:

```bash
# View pending acknowledgments
bib admin break-glass status

# Acknowledge a specific session
bib admin break-glass acknowledge --session <session-id>

# View session report
bib admin break-glass report <session-id>
```

## Access Levels

### Read-Only (default)

- `SELECT` on all tables except `audit_log`
- Cannot modify any data

### Read-Write

- `SELECT`, `INSERT`, `UPDATE`, `DELETE` on all tables except `audit_log`
- Full data modification capability

**Note:** The `audit_log` table is protected regardless of access level to prevent tampering with audit records.

## Audit Trail

All break glass sessions are comprehensively audited:

### Session Events

- `session_started` - When a session is enabled
- `session_expired` - When a session times out
- `session_disabled` - When a session is manually disabled
- `session_acknowledged` - When a session report is acknowledged

### Query Logging

During break glass sessions, all queries are logged with:
- Full SQL query text
- **Unredacted parameter values** (paranoid mode)
- Execution duration
- Rows affected
- Table name

Audit entries are marked with `flag_break_glass = true` for easy filtering.

### Session Recording

If `session_recording` is enabled, all session activity is recorded to a compressed file:

```
<recording_path>/breakglass_<session-id>.rec.gz
```

The recording includes:
- All queries executed
- Query results (summary)
- Timing information

## Notifications

When configured, notifications are sent for:

- Session started
- Session expired
- Session manually disabled
- Session acknowledged

Notification content includes:
- Session ID
- Username
- Reason
- Duration
- Access level
- Node ID
- Timestamp

## Security Considerations

1. **Key Security**: Protect your break glass private key. Consider storing it on a hardware token or in a secure vault.

2. **Limit Users**: Only configure users who genuinely need emergency access capability.

3. **Review Sessions**: Always review session reports promptly. The acknowledgment requirement ensures sessions don't go unnoticed.

4. **Monitor Notifications**: Configure webhook/email notifications and monitor them for unauthorized access attempts.

5. **Audit Retention**: Ensure break glass audit logs are retained for compliance requirements.

## Troubleshooting

### "Break glass is not enabled"

Break glass must be enabled in the configuration and bibd must be restarted.

### "User not found"

The username must match one of the pre-configured `allowed_users` in the configuration.

### "Invalid signature"

Ensure you're using the correct private key that corresponds to the configured public key.

### "A break glass session is already active"

Only one break glass session can be active at a time. Disable the current session first.

---

## Related Documentation

| Document | Topic |
|----------|-------|
| [Database Security](database-security.md) | Security architecture overview |
| [Configuration](../getting-started/configuration.md) | Break glass configuration options |
| [CLI Reference](../guides/cli-reference.md) | Admin commands reference |
| [Storage Lifecycle](storage-lifecycle.md) | Database backend management |


