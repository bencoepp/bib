# Authentication

This document describes the authentication flow used by bib for secure, passwordless authentication using SSH keys.

## Overview

bib uses a challenge-response authentication mechanism based on SSH keys (Ed25519 or RSA). This provides:

- **Passwordless authentication**: No passwords to remember or leak
- **Cryptographic security**: Based on proven SSH key cryptography
- **Key reuse**: Use your existing SSH keys (e.g., from GitHub, GitLab)
- **Auto-registration**: New users can self-register (if enabled)

## Supported Key Types

| Key Type | Algorithm | Recommended |
|----------|-----------|-------------|
| Ed25519 | ssh-ed25519 | ✅ Yes |
| RSA | rsa-sha2-256 | ⚠️ Legacy support |

**Recommendation**: Use Ed25519 keys for better security and performance.

## Authentication Flow

The authentication process follows a secure challenge-response protocol:

```
┌─────────┐                                    ┌─────────┐
│  Client │                                    │  Server │
└────┬────┘                                    └────┬────┘
     │                                              │
     │  1. Challenge(public_key, key_type)          │
     │─────────────────────────────────────────────►│
     │                                              │
     │                                              │ Generate random nonce
     │                                              │ Store challenge with TTL
     │                                              │
     │  2. ChallengeResponse(challenge_id,          │
     │     nonce, expires_at, signature_algorithm)  │
     │◄─────────────────────────────────────────────│
     │                                              │
     │ Sign nonce with                              │
     │ private key                                  │
     │                                              │
     │  3. VerifyChallenge(challenge_id,            │
     │     signature, name, email, client_info)     │
     │─────────────────────────────────────────────►│
     │                                              │
     │                                              │ Verify signature
     │                                              │ Look up or create user
     │                                              │ Create session
     │                                              │
     │  4. VerifyChallengeResponse(session_token,   │
     │     expires_at, user, is_new_user, session)  │
     │◄─────────────────────────────────────────────│
     │                                              │
     │  5. Subsequent requests include              │
     │     session_token in metadata                │
     │─────────────────────────────────────────────►│
     │                                              │
```

### Step-by-Step

1. **Challenge Request**: Client sends their public key to request an authentication challenge
2. **Challenge Response**: Server generates a random 32-byte nonce, stores it with a 30-second TTL, and returns it with a challenge ID
3. **Verify Challenge**: Client signs the nonce with their private key and sends the signature along with optional user info
4. **Verification**: Server verifies the signature, looks up or creates the user (if auto-registration is enabled), and creates a session
5. **Session Token**: Client receives an opaque session token to use for subsequent requests

## Session Management

### Session Token

Session tokens are opaque identifiers (64-character hex strings). They are:

- Stored server-side with associated user/session metadata
- Passed in gRPC metadata as `x-session-token` or `Authorization: Bearer <token>`
- Valid until the session timeout or explicit logout

### Session Timeout

Sessions have a configurable timeout (default: 24 hours). The timeout is reset on each activity. Configure via:

```yaml
auth:
  session_timeout: 24h
  max_sessions_per_user: 5
```

### Session Operations

| Operation | Description |
|-----------|-------------|
| `ListMySessions` | List all active sessions for current user |
| `RefreshSession` | Extend session timeout |
| `RevokeSession` | End a specific session |
| `RevokeAllSessions` | End all sessions (optional: include current) |
| `Logout` | End current session |

## Configuration

### Server Configuration

```yaml
auth:
  # Allow new users to self-register on first authentication
  allow_auto_registration: true
  
  # Require email during registration
  require_email: false
  
  # Default role for new users (user, readonly)
  # First user is always admin regardless
  default_role: user
  
  # Session inactivity timeout
  session_timeout: 24h
  
  # Maximum concurrent sessions per user (0 = unlimited)
  max_sessions_per_user: 5
```

### Auto-Registration

When `allow_auto_registration` is enabled:

1. Unknown public keys automatically create a new user account
2. The first user registered becomes an admin
3. Subsequent users get the `default_role`
4. Optional name/email can be provided during authentication

When disabled, an admin must pre-register users by adding their public keys.

## gRPC API Reference

### AuthService

```protobuf
service AuthService {
  // Authentication
  rpc Challenge(ChallengeRequest) returns (ChallengeResponse);
  rpc VerifyChallenge(VerifyChallengeRequest) returns (VerifyChallengeResponse);
  
  // Session management
  rpc Logout(LogoutRequest) returns (LogoutResponse);
  rpc RefreshSession(RefreshSessionRequest) returns (RefreshSessionResponse);
  rpc ValidateSession(ValidateSessionRequest) returns (ValidateSessionResponse);
  
  // Configuration
  rpc GetAuthConfig(GetAuthConfigRequest) returns (GetAuthConfigResponse);
  
  // Key info
  rpc GetPublicKeyInfo(GetPublicKeyInfoRequest) returns (GetPublicKeyInfoResponse);
  
  // Session listing
  rpc ListMySessions(ListMySessionsRequest) returns (ListMySessionsResponse);
  rpc RevokeSession(RevokeSessionRequest) returns (RevokeSessionResponse);
  rpc RevokeAllSessions(RevokeAllSessionsRequest) returns (RevokeAllSessionsResponse);
}
```

## Client Implementation Example

### Go Client

```go
package main

import (
    "context"
    "crypto/ed25519"
    "os"
    
    "bib/api/gen/go/bib/v1/services"
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
    "golang.org/x/crypto/ssh"
)

func authenticate(client services.AuthServiceClient, privateKeyPath string) (string, error) {
    // Load private key
    keyData, _ := os.ReadFile(privateKeyPath)
    signer, _ := ssh.ParsePrivateKey(keyData)
    
    // Get public key bytes
    pubKey := signer.PublicKey().Marshal()
    
    // 1. Request challenge
    challenge, err := client.Challenge(context.Background(), &services.ChallengeRequest{
        PublicKey: pubKey,
        KeyType:   "ed25519",
    })
    if err != nil {
        return "", err
    }
    
    // 2. Sign the challenge
    sig, _ := signer.Sign(nil, challenge.Challenge)
    
    // 3. Verify challenge and get session
    resp, err := client.VerifyChallenge(context.Background(), &services.VerifyChallengeRequest{
        ChallengeId: challenge.ChallengeId,
        Signature:   sig.Blob,
        Name:        "My Name",
        Email:       "me@example.com",
    })
    if err != nil {
        return "", err
    }
    
    return resp.SessionToken, nil
}

func makeAuthenticatedRequest(conn *grpc.ClientConn, token string) {
    // Add session token to context
    ctx := metadata.AppendToOutgoingContext(
        context.Background(),
        "x-session-token", token,
    )
    
    // Use ctx for all subsequent requests
    client := services.NewHealthServiceClient(conn)
    client.Check(ctx, &services.HealthCheckRequest{})
}
```

## Security Considerations

### Trust-On-First-Use (TOFU)

When connecting to a bibd node for the first time, the client must establish trust with the server's certificate. Bib uses a Trust-On-First-Use model similar to SSH.

**Default Behavior (Manual Confirmation):**

```
⚠️  First connection to this node

Node ID:      QmXyz123...
Address:      node1.example.com:4000
Fingerprint:  SHA256:Ab12Cd34Ef56...

Trust this node? [y/N]
```

The user must manually confirm the fingerprint matches what was provided out-of-band.

**Automatic Trust Flag:**

For scripting or when trust has been verified separately:

```bash
bib connect --trust-first-use node1.example.com:4000
```

This flag:
- Automatically trusts the server certificate on first connection
- Saves the certificate to the trust store
- Subsequent connections verify against the saved certificate

**Trust Management:**

```bash
# List trusted nodes
bib trust list

# Manually add a trusted node
bib trust add <node-id> --fingerprint SHA256:...

# Remove trust for a node
bib trust remove <node-id>

# Pin a certificate (prevents automatic trust updates)
bib trust pin <node-id>
```

**Trust Store Location:**

| Platform | Location |
|----------|----------|
| Linux/macOS | `~/.config/bib/trust/` |
| Windows | `%APPDATA%\bib\trust\` |

### Challenge Security

- Challenges expire after 30 seconds
- Challenges are single-use (consumed on verification attempt)
- Challenge nonces are 32 bytes of cryptographic randomness
- Failed verification does not reveal whether the key exists

### Session Security

- Sessions are tied to the authenticating public key fingerprint
- Session tokens are 64-character cryptographic random hex strings
- Sessions can be revoked individually or in bulk
- Inactive sessions automatically expire

### Key Storage

- Never transmit private keys
- Store private keys securely (encrypted, proper file permissions)
- Use SSH agent for key management when possible

## Troubleshooting

### "challenge not found or expired"

The challenge expired (30s timeout) or was already used. Request a new challenge.

### "signature verification failed"

- Ensure you're signing the exact challenge bytes returned by the server
- Check that the key type matches (Ed25519 vs RSA)
- Verify you're using the correct private key

### "auto-registration is disabled"

The server requires users to be pre-registered. Contact an administrator to add your public key.

### "user account is suspended"

Your account has been suspended by an administrator. Contact support.

