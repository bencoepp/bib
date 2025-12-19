# User Identity System

This document describes the user identity and authentication system for bibd.

## Overview

The user identity system provides:
- Unified user identity across SSH (Wish) and gRPC/API connections
- Support for Ed25519 and RSA public keys
- Configurable auto-registration or admin-controlled user creation
- Session tracking for security auditing
- Role-based access control (admin, user, readonly)

## Architecture

### User Entity

Users are identified by their public key. The `User` entity includes:

| Field | Description |
|-------|-------------|
| `ID` | Unique identifier derived from SHA256 hash of public key |
| `PublicKey` | Ed25519 or RSA public key bytes |
| `KeyType` | Key type: `ed25519` or `rsa` |
| `PublicKeyFingerprint` | SHA256 fingerprint of the public key |
| `Name` | Human-readable display name |
| `Email` | Optional contact email |
| `Status` | Account status: `active`, `pending`, `suspended`, `deleted` |
| `Role` | Permission level: `admin`, `user`, `readonly` |
| `Locale` | Preferred locale for i18n |

### Session Entity

Sessions track authenticated connections:

| Field | Description |
|-------|-------------|
| `ID` | Random 32-byte session identifier |
| `UserID` | User who owns this session |
| `Type` | Session type: `ssh`, `grpc`, `api` |
| `ClientIP` | Client's IP address |
| `NodeID` | Node the session is connected to |
| `StartedAt` | When session started |
| `EndedAt` | When session ended (null if active) |
| `LastActivityAt` | Last activity time |

## gRPC API

The user identity system exposes two gRPC services:

### AuthService (`auth.proto`)

Handles authentication operations:

| RPC | Description |
|-----|-------------|
| `Authenticate` | Authenticate with public key, create session |
| `Register` | Register new user (admin only) |
| `Logout` | End current session |
| `RefreshSession` | Extend session expiry |
| `ValidateSession` | Check if session is valid |
| `GetAuthConfig` | Get authentication configuration |
| `Challenge` | Request challenge for signature auth |
| `VerifyChallenge` | Verify signed challenge |

### UserService (`user.proto`)

Handles user management:

| RPC | Description |
|-----|-------------|
| `GetUser` | Get user by ID |
| `GetUserByPublicKey` | Get user by public key |
| `ListUsers` | List users with filtering |
| `CreateUser` | Create user (admin only) |
| `UpdateUser` | Update user profile |
| `DeleteUser` | Soft-delete user (admin only) |
| `SuspendUser` | Suspend user (admin only) |
| `ActivateUser` | Activate user (admin only) |
| `SetUserRole` | Change user role (admin only) |
| `GetCurrentUser` | Get authenticated user |
| `UpdateCurrentUser` | Update own profile |
| `ListSessions` | List user sessions |
| `EndSession` | End specific session |
| `EndAllSessions` | End all sessions |

### Generating Go Code

```bash
cd api/proto
buf generate
```

This generates Go code in `api/gen/go/`.

## Configuration

### bibd Configuration

Add to `bibd.yaml`:

```yaml
auth:
  # Allow users to auto-register on first connection
  # If false, admin must pre-create users with their public keys
  allow_auto_registration: true
  
  # Require email during registration
  require_email: false
  
  # Default role for new users (first user is always admin)
  default_role: "user"
  
  # Session timeout (0 = no timeout)
  session_timeout: 24h
  
  # Maximum concurrent sessions per user (0 = unlimited)
  max_sessions_per_user: 10
```

## Authentication Flow

### First User Bootstrap

1. First user to connect is automatically granted admin role
2. Subsequent users get the `default_role` from config
3. Admin can then manage other users' roles

### Auto-Registration (Enabled)

```
Client                          bibd
  │                               │
  │── SSH/gRPC connect ──────────►│
  │   (with public key)           │
  │                               │
  │                      ┌────────┴────────┐
  │                      │ User not found  │
  │                      │ Create new user │
  │                      │ Create session  │
  │                      └────────┬────────┘
  │                               │
  │◄─── Authenticated ────────────│
```

### Admin-Controlled Registration (Auto-reg Disabled)

```
Admin                           bibd                          Client
  │                               │                               │
  │── Create user (public key) ──►│                               │
  │                               │                               │
  │                               │◄── SSH/gRPC connect ──────────│
  │                               │    (with public key)          │
  │                               │                               │
  │                      ┌────────┴────────┐                      │
  │                      │ User found      │                      │
  │                      │ Create session  │                      │
  │                      └────────┬────────┘                      │
  │                               │                               │
  │                               │──── Authenticated ───────────►│
```

## Database Schema

### Users Table

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    public_key BLOB NOT NULL UNIQUE,
    key_type TEXT NOT NULL CHECK (key_type IN ('ed25519', 'rsa')),
    public_key_fingerprint TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    email TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'active',
    role TEXT NOT NULL DEFAULT 'user',
    locale TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    last_login_at TIMESTAMP,
    metadata JSONB
);
```

### Sessions Table

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    type TEXT NOT NULL CHECK (type IN ('ssh', 'grpc', 'api')),
    client_ip TEXT NOT NULL,
    client_agent TEXT,
    public_key_fingerprint TEXT NOT NULL,
    node_id TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    last_activity_at TIMESTAMP NOT NULL,
    metadata JSONB
);
```

## Auth Service API

### Authentication

```go
// Authenticate a user by public key
result, err := authService.Authenticate(ctx, auth.AuthenticateRequest{
    PublicKey:   pubKeyBytes,
    KeyType:     domain.KeyTypeEd25519,
    Name:        "User Name",     // For auto-registration
    Email:       "user@example.com",
    SessionType: storage.SessionTypeSSH,
    ClientIP:    "192.168.1.1",
})

if err != nil {
    if errors.Is(err, domain.ErrAutoRegDisabled) {
        // User not found and auto-registration is disabled
    }
}

user := result.User
session := result.Session
isNewUser := result.IsNew
```

### Session Management

```go
// Get session
session, err := authService.GetSession(ctx, sessionID)

// Update activity
err := authService.UpdateSessionActivity(ctx, sessionID)

// End session
err := authService.EndSession(ctx, sessionID)

// End all sessions for a user
err := authService.EndAllSessions(ctx, userID)
```

### User Management

```go
// Create user (admin function)
user := domain.NewUser(pubKey, domain.KeyTypeEd25519, "Name", "email@example.com", false)
err := authService.CreateUser(ctx, user)

// Suspend user
err := authService.SuspendUser(ctx, userID)

// Activate user
err := authService.ActivateUser(ctx, userID)

// Change role
err := authService.SetUserRole(ctx, userID, domain.UserRoleAdmin)

// Delete user
err := authService.DeleteUser(ctx, userID)
```

## SSH Key Support

The system supports both Ed25519 and RSA SSH keys:

```go
import "bib/internal/auth"

// Parse SSH public key from ssh.PublicKey
keyBytes, keyType, err := auth.ParseSSHPublicKey(sshPubKey)

// Parse from authorized_keys format
keyBytes, keyType, err := auth.ParseAuthorizedKey([]byte("ssh-ed25519 AAAA..."))
```

## Security Considerations

1. **First User Admin**: The first user is automatically admin. Ensure the first connection is from a trusted source.

2. **Session Limits**: Configure `max_sessions_per_user` to prevent session exhaustion attacks.

3. **Session Timeout**: Set `session_timeout` to automatically expire inactive sessions.

4. **Key Rotation**: Users can have their public key updated by an admin if needed (not yet implemented - future feature).

5. **Audit Logging**: All authentication events and session changes are logged to the audit system.

## File Locations

| File | Description |
|------|-------------|
| `api/proto/bib/v1/user.proto` | User and Session protobuf definitions |
| `api/proto/bib/v1/auth.proto` | Authentication service protobuf definitions |
| `api/proto/buf.yaml` | Buf configuration |
| `api/proto/buf.gen.yaml` | Buf code generation configuration |
| `internal/domain/user.go` | User domain entity |
| `internal/storage/repository.go` | UserRepository and SessionRepository interfaces |
| `internal/storage/sqlite/users.go` | SQLite UserRepository implementation |
| `internal/storage/sqlite/sessions.go` | SQLite SessionRepository implementation |
| `internal/storage/postgres/users.go` | PostgreSQL UserRepository implementation |
| `internal/storage/postgres/sessions.go` | PostgreSQL SessionRepository implementation |
| `internal/auth/service.go` | Authentication service |
| `internal/auth/keys.go` | SSH key parsing utilities |
| `internal/config/types.go` | AuthConfig definition |

