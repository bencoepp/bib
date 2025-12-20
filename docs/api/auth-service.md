# AuthService API

The AuthService handles authentication and session management using SSH key-based challenge-response authentication.

## Service Definition

```protobuf
service AuthService {
  // Authentication
  rpc Challenge(ChallengeRequest) returns (ChallengeResponse);
  rpc VerifyChallenge(VerifyChallengeRequest) returns (VerifyChallengeResponse);
  
  // Session Management
  rpc Logout(LogoutRequest) returns (LogoutResponse);
  rpc RefreshSession(RefreshSessionRequest) returns (RefreshSessionResponse);
  rpc ValidateSession(ValidateSessionRequest) returns (ValidateSessionResponse);
  
  // Configuration
  rpc GetAuthConfig(GetAuthConfigRequest) returns (GetAuthConfigResponse);
  rpc GetPublicKeyInfo(GetPublicKeyInfoRequest) returns (GetPublicKeyInfoResponse);
  
  // Session Operations
  rpc ListMySessions(ListMySessionsRequest) returns (ListMySessionsResponse);
  rpc RevokeSession(RevokeSessionRequest) returns (RevokeSessionResponse);
  rpc RevokeAllSessions(RevokeAllSessionsRequest) returns (RevokeAllSessionsResponse);
}
```

## Authentication Methods

### Challenge

Request an authentication challenge. This is the first step in the authentication flow.

**Authentication:** Not required

**Request:**
```protobuf
message ChallengeRequest {
  bytes public_key = 1;  // SSH public key (authorized_keys format)
  string key_type = 2;   // Optional: "ed25519" or "rsa"
}
```

**Response:**
```protobuf
message ChallengeResponse {
  string challenge_id = 1;        // Challenge identifier
  bytes challenge = 2;            // Bytes to sign
  Timestamp expires_at = 3;       // Expiration (30 seconds)
  string signature_algorithm = 4; // Algorithm to use
}
```

**Errors:**
- `INVALID_ARGUMENT`: Invalid or unsupported public key format

**Example:**
```go
resp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
    PublicKey: pubKeyBytes,
})
// resp.Challenge contains bytes to sign
```

### VerifyChallenge

Verify a signed challenge and obtain a session token.

**Authentication:** Not required

**Request:**
```protobuf
message VerifyChallengeRequest {
  string challenge_id = 1;
  bytes signature = 2;
  string name = 3;           // For auto-registration
  string email = 4;          // May be required
  ClientInfo client_info = 5;
}
```

**Response:**
```protobuf
message VerifyChallengeResponse {
  string session_token = 1;
  Timestamp expires_at = 2;
  UserInfo user = 3;
  bool is_new_user = 4;
  SessionInfo session = 5;
}
```

**Errors:**
- `INVALID_ARGUMENT`: Missing required fields
- `NOT_FOUND`: Challenge expired or not found
- `UNAUTHENTICATED`: Signature verification failed
- `PERMISSION_DENIED`: Auto-registration disabled, user suspended/pending

**Example:**
```go
verifyResp, err := authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
    ChallengeId: challengeResp.ChallengeId,
    Signature:   signature,
    Name:        "John Doe",
    Email:       "john@example.com",
})
// verifyResp.SessionToken for subsequent requests
```

## Session Methods

### Logout

End the current session.

**Authentication:** Required

**Request:**
```protobuf
message LogoutRequest {
  string session_id = 1;  // Optional: specific session, defaults to current
}
```

**Response:**
```protobuf
message LogoutResponse {
  bool success = 1;
}
```

### RefreshSession

Extend the current session's expiration.

**Authentication:** Required

**Request:**
```protobuf
message RefreshSessionRequest {}
```

**Response:**
```protobuf
message RefreshSessionResponse {
  string session_token = 1;  // Same token
  Timestamp expires_at = 2;  // New expiration
}
```

### ValidateSession

Check if a session token is valid.

**Authentication:** Not required (token passed in request)

**Request:**
```protobuf
message ValidateSessionRequest {
  string session_token = 1;
}
```

**Response:**
```protobuf
message ValidateSessionResponse {
  bool valid = 1;
  string invalid_reason = 2;  // If not valid
  UserInfo user = 3;          // If valid
  SessionInfo session = 4;    // If valid
  Timestamp expires_at = 5;   // If valid
}
```

### ListMySessions

List all active sessions for the current user.

**Authentication:** Required

**Request:**
```protobuf
message ListMySessionsRequest {
  int32 limit = 1;
  bool include_expired = 2;
}
```

**Response:**
```protobuf
message ListMySessionsResponse {
  repeated SessionInfo sessions = 1;
  int32 total_count = 2;
}

message SessionInfo {
  string id = 1;
  string type = 2;           // "grpc", "ssh", "web"
  string client_ip = 3;
  string client_agent = 4;
  string node_id = 5;
  Timestamp started_at = 6;
  Timestamp last_activity_at = 7;
  Timestamp expires_at = 8;
  bool is_current = 9;
}
```

### RevokeSession

Revoke a specific session.

**Authentication:** Required

**Request:**
```protobuf
message RevokeSessionRequest {
  string session_id = 1;
}
```

**Response:**
```protobuf
message RevokeSessionResponse {
  bool success = 1;
}
```

**Errors:**
- `NOT_FOUND`: Session not found
- `PERMISSION_DENIED`: Cannot revoke another user's session

### RevokeAllSessions

Revoke all sessions except the current one (optionally including current).

**Authentication:** Required

**Request:**
```protobuf
message RevokeAllSessionsRequest {
  bool include_current = 1;  // Also revoke current session
}
```

**Response:**
```protobuf
message RevokeAllSessionsResponse {
  int32 revoked_count = 1;
}
```

## Configuration Methods

### GetAuthConfig

Get the server's authentication configuration.

**Authentication:** Not required

**Request:**
```protobuf
message GetAuthConfigRequest {}
```

**Response:**
```protobuf
message GetAuthConfigResponse {
  bool allow_auto_registration = 1;
  bool require_email = 2;
  string default_role = 3;
  int64 session_timeout_seconds = 4;
  int64 max_session_lifetime_seconds = 5;
  repeated string supported_key_types = 6;
  string server_version = 7;
  string node_id = 8;
  string node_mode = 9;
}
```

**Example:**
```go
config, err := authClient.GetAuthConfig(ctx, &services.GetAuthConfigRequest{})
if !config.AllowAutoRegistration {
    log.Println("Auto-registration disabled, user must be pre-created")
}
```

### GetPublicKeyInfo

Get information about a public key without authenticating.

**Authentication:** Not required

**Request:**
```protobuf
message GetPublicKeyInfoRequest {
  bytes public_key = 1;
}
```

**Response:**
```protobuf
message GetPublicKeyInfoResponse {
  string key_type = 1;            // "ed25519", "rsa"
  string fingerprint_sha256 = 2;  // SHA256 fingerprint
  string fingerprint_md5 = 3;     // MD5 fingerprint
  int32 key_size = 4;             // Key size in bits
  string openssh_format = 5;      // Formatted public key
  bool has_user = 6;              // User exists with this key
  string user_id = 7;             // User ID if exists
}
```

## Using Session Tokens

Include the session token in gRPC metadata for authenticated requests:

```go
// Option 1: x-session-token header
ctx = metadata.AppendToOutgoingContext(ctx, "x-session-token", token)

// Option 2: Authorization header
ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
```

## Security Best Practices

1. **Store tokens securely**: Use OS keychain or encrypted storage
2. **Refresh proactively**: Refresh sessions before expiration
3. **Revoke on logout**: Always call Logout when done
4. **Monitor sessions**: Use ListMySessions to detect unauthorized access
5. **Use short-lived tokens**: Configure appropriate session timeouts

## Example: Complete Session Lifecycle

```go
// 1. Authenticate
token, err := authenticate(conn, privateKey)
if err != nil {
    log.Fatal(err)
}

// 2. Create authenticated context
ctx := metadata.AppendToOutgoingContext(context.Background(), 
    "x-session-token", token)

// 3. Use authenticated APIs
topicResp, err := topicClient.ListTopics(ctx, &services.ListTopicsRequest{})

// 4. Periodically refresh
go func() {
    ticker := time.NewTicker(1 * time.Hour)
    for range ticker.C {
        authClient.RefreshSession(ctx, &services.RefreshSessionRequest{})
    }
}()

// 5. Logout when done
authClient.Logout(ctx, &services.LogoutRequest{})
```

