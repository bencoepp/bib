# Authentication Flow

This document describes the SSH key-based authentication flow used by BIB.

## Overview

BIB uses a challenge-response authentication mechanism based on SSH keys (Ed25519 or RSA). This provides:

- **Strong security**: Leverages battle-tested SSH cryptography
- **No passwords**: Authentication via cryptographic signatures
- **Key reuse**: Use existing SSH keys from `~/.ssh/`
- **Auto-registration**: Optional automatic user creation on first login

## Authentication Flow

```
┌──────────┐                                    ┌──────────┐
│  Client  │                                    │  Server  │
└────┬─────┘                                    └────┬─────┘
     │                                               │
     │  1. Challenge(public_key)                     │
     │──────────────────────────────────────────────>│
     │                                               │
     │  2. ChallengeResponse(challenge_id, nonce)    │
     │<──────────────────────────────────────────────│
     │                                               │
     │  3. Sign nonce with private key               │
     │  (client-side)                                │
     │                                               │
     │  4. VerifyChallenge(challenge_id, signature)  │
     │──────────────────────────────────────────────>│
     │                                               │
     │  5. VerifyChallengeResponse(session_token)    │
     │<──────────────────────────────────────────────│
     │                                               │
     │  6. Subsequent requests with session token    │
     │──────────────────────────────────────────────>│
     │                                               │
```

## Step-by-Step Guide

### Step 1: Request Challenge

Send your public key to the server to receive a challenge.

**Request:**
```protobuf
message ChallengeRequest {
  bytes public_key = 1;  // SSH public key in authorized_keys format
  string key_type = 2;   // Optional: "ed25519" or "rsa"
}
```

**Example (Go):**
```go
resp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
    PublicKey: pubKeyBytes,
})
```

**Response:**
```protobuf
message ChallengeResponse {
  string challenge_id = 1;       // Unique challenge identifier
  bytes challenge = 2;           // Random bytes to sign
  Timestamp expires_at = 3;      // Challenge expiration (30 seconds)
  string signature_algorithm = 4; // Algorithm to use for signing
}
```

### Step 2: Sign the Challenge

Sign the challenge bytes using your SSH private key.

**Example (Go):**
```go
signer, _ := ssh.NewSignerFromKey(privateKey)
sig, _ := signer.Sign(rand.Reader, resp.Challenge)
signature := sig.Blob
```

**Example (CLI with ssh-agent):**
```bash
# The bib CLI handles this automatically using ssh-agent
bib login
```

### Step 3: Verify Challenge

Submit the signature for verification.

**Request:**
```protobuf
message VerifyChallengeRequest {
  string challenge_id = 1;  // From ChallengeResponse
  bytes signature = 2;      // Signed challenge bytes
  string name = 3;          // User name (for auto-registration)
  string email = 4;         // User email (may be required)
  ClientInfo client_info = 5; // Optional client information
}
```

**Response:**
```protobuf
message VerifyChallengeResponse {
  string session_token = 1;  // Token for subsequent requests
  Timestamp expires_at = 2;  // Session expiration
  UserInfo user = 3;         // User information
  bool is_new_user = 4;      // True if auto-registered
  SessionInfo session = 5;   // Session details
}
```

### Step 4: Use Session Token

Include the session token in subsequent requests.

**gRPC Metadata:**
```go
ctx = metadata.AppendToOutgoingContext(ctx, "x-session-token", sessionToken)
```

**Or Authorization Header:**
```go
ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+sessionToken)
```

## Supported Key Types

| Key Type | Algorithm | Recommended |
|----------|-----------|-------------|
| Ed25519 | `ssh-ed25519` | ✅ Yes (fastest, most secure) |
| RSA | `rsa-sha2-256`, `rsa-sha2-512` | For compatibility |

## Session Management

### Session Lifetime

- Default session timeout: 24 hours
- Sessions can be refreshed using `RefreshSession`
- Idle sessions expire based on `last_activity_at`

### Session Operations

| Operation | Description |
|-----------|-------------|
| `RefreshSession` | Extend session expiration |
| `ValidateSession` | Check if session is valid |
| `ListMySessions` | List all active sessions |
| `RevokeSession` | End a specific session |
| `RevokeAllSessions` | End all sessions |
| `Logout` | End current session |

## Configuration Options

Server-side authentication configuration:

```yaml
auth:
  # Allow automatic user creation on first login
  allow_auto_registration: true
  
  # Require email for auto-registration
  require_email: false
  
  # Default role for new users
  default_role: "user"
  
  # Session timeout duration
  session_timeout: "24h"
  
  # Supported key types
  allowed_key_types:
    - ed25519
    - rsa
```

## Error Handling

| Error | Code | Description |
|-------|------|-------------|
| Invalid public key | `INVALID_ARGUMENT` | Key format not recognized |
| Challenge expired | `NOT_FOUND` | Challenge TTL exceeded (30s) |
| Invalid signature | `UNAUTHENTICATED` | Signature verification failed |
| User not found | `NOT_FOUND` | No user with this key (auto-reg disabled) |
| Auto-reg disabled | `PERMISSION_DENIED` | Server doesn't allow auto-registration |
| User suspended | `PERMISSION_DENIED` | Account is suspended |
| User pending | `PERMISSION_DENIED` | Account awaiting approval |

## Security Considerations

1. **Challenge TTL**: Challenges expire after 30 seconds to prevent replay attacks
2. **One-time use**: Each challenge can only be used once
3. **Secure transport**: Always use TLS/mTLS in production
4. **Key storage**: Never store private keys on the server
5. **Session tokens**: Treat session tokens as secrets
6. **Audit logging**: All authentication events are logged

## Example: Full Authentication Flow

```go
package main

import (
    "context"
    "crypto/rand"
    
    "golang.org/x/crypto/ssh"
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
    
    services "bib/api/gen/go/bib/v1/services"
)

func authenticate(conn *grpc.ClientConn, privateKey interface{}) (string, error) {
    ctx := context.Background()
    authClient := services.NewAuthServiceClient(conn)
    
    // Get public key bytes
    signer, err := ssh.NewSignerFromKey(privateKey)
    if err != nil {
        return "", err
    }
    pubKeyBytes := ssh.MarshalAuthorizedKey(signer.PublicKey())
    
    // Step 1: Request challenge
    challengeResp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
        PublicKey: pubKeyBytes,
    })
    if err != nil {
        return "", err
    }
    
    // Step 2: Sign challenge
    sig, err := signer.Sign(rand.Reader, challengeResp.Challenge)
    if err != nil {
        return "", err
    }
    
    // Step 3: Verify and get session
    verifyResp, err := authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
        ChallengeId: challengeResp.ChallengeId,
        Signature:   sig.Blob,
        Name:        "My Name",
        Email:       "me@example.com",
    })
    if err != nil {
        return "", err
    }
    
    return verifyResp.SessionToken, nil
}

func authenticatedContext(ctx context.Context, token string) context.Context {
    return metadata.AppendToOutgoingContext(ctx, "x-session-token", token)
}
```

