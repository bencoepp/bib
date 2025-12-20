# Error Codes Reference

This document provides a comprehensive reference for all error codes returned by the BIB gRPC API.

## gRPC Status Codes

BIB uses standard gRPC status codes with additional error details.

### Standard Codes Used

| Code | Name | HTTP Equivalent | Description |
|------|------|-----------------|-------------|
| 0 | `OK` | 200 | Request succeeded |
| 1 | `CANCELLED` | 499 | Request cancelled by client |
| 2 | `UNKNOWN` | 500 | Unknown error |
| 3 | `INVALID_ARGUMENT` | 400 | Invalid request parameters |
| 4 | `DEADLINE_EXCEEDED` | 504 | Request timeout |
| 5 | `NOT_FOUND` | 404 | Resource not found |
| 6 | `ALREADY_EXISTS` | 409 | Resource already exists |
| 7 | `PERMISSION_DENIED` | 403 | Insufficient permissions |
| 8 | `RESOURCE_EXHAUSTED` | 429 | Rate limit exceeded |
| 9 | `FAILED_PRECONDITION` | 400 | Operation precondition failed |
| 10 | `ABORTED` | 409 | Operation aborted |
| 11 | `OUT_OF_RANGE` | 400 | Value out of valid range |
| 12 | `UNIMPLEMENTED` | 501 | Operation not implemented |
| 13 | `INTERNAL` | 500 | Internal server error |
| 14 | `UNAVAILABLE` | 503 | Service unavailable |
| 15 | `DATA_LOSS` | 500 | Data corruption detected |
| 16 | `UNAUTHENTICATED` | 401 | Authentication required |

## Domain-Specific Errors

### Topic Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrInvalidTopicID` | `INVALID_ARGUMENT` | Topic ID format invalid |
| `ErrInvalidTopicName` | `INVALID_ARGUMENT` | Topic name format invalid |
| `ErrInvalidTopicStatus` | `INVALID_ARGUMENT` | Topic status value invalid |
| `ErrTopicNotFound` | `NOT_FOUND` | Topic does not exist |
| `ErrTopicArchived` | `FAILED_PRECONDITION` | Topic is archived |
| `ErrCannotRemoveLastOwner` | `FAILED_PRECONDITION` | Topic must have at least one owner |

### Dataset Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrInvalidDatasetID` | `INVALID_ARGUMENT` | Dataset ID format invalid |
| `ErrInvalidDatasetName` | `INVALID_ARGUMENT` | Dataset name format invalid |
| `ErrInvalidDatasetStatus` | `INVALID_ARGUMENT` | Dataset status value invalid |
| `ErrDatasetNotFound` | `NOT_FOUND` | Dataset does not exist |
| `ErrInvalidHash` | `INVALID_ARGUMENT` | Hash format invalid |
| `ErrHashMismatch` | `DATA_LOSS` | Content hash doesn't match |
| `ErrInvalidSize` | `INVALID_ARGUMENT` | Size value invalid |
| `ErrNoOwners` | `INVALID_ARGUMENT` | No owners specified |

### User Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrInvalidUserID` | `INVALID_ARGUMENT` | User ID format invalid |
| `ErrInvalidUserName` | `INVALID_ARGUMENT` | User name format invalid |
| `ErrInvalidPublicKey` | `INVALID_ARGUMENT` | Public key format invalid |
| `ErrInvalidKeyType` | `INVALID_ARGUMENT` | Key type not supported |
| `ErrUserNotFound` | `NOT_FOUND` | User does not exist |
| `ErrUserExists` | `ALREADY_EXISTS` | User already exists |
| `ErrUserSuspended` | `PERMISSION_DENIED` | User account suspended |
| `ErrUserPending` | `PERMISSION_DENIED` | User account pending approval |
| `ErrAutoRegDisabled` | `PERMISSION_DENIED` | Auto-registration disabled |

### Session Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrSessionNotFound` | `NOT_FOUND` | Session does not exist |
| `ErrSessionExpired` | `UNAUTHENTICATED` | Session has expired |
| `ErrInvalidSignature` | `UNAUTHENTICATED` | Signature verification failed |

### Version Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrInvalidVersionID` | `INVALID_ARGUMENT` | Version ID format invalid |
| `ErrInvalidVersionString` | `INVALID_ARGUMENT` | Version string format invalid |
| `ErrVersionNotFound` | `NOT_FOUND` | Version does not exist |
| `ErrEmptyVersion` | `INVALID_ARGUMENT` | Version has no content |

### Job Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrInvalidTaskID` | `INVALID_ARGUMENT` | Task ID format invalid |
| `ErrInvalidTaskName` | `INVALID_ARGUMENT` | Task name format invalid |
| `ErrTaskNotFound` | `NOT_FOUND` | Task does not exist |
| `ErrEmptyTask` | `INVALID_ARGUMENT` | Task has no instructions |

### Ownership Errors

| Error | gRPC Code | Description |
|-------|-----------|-------------|
| `ErrOwnerNotFound` | `NOT_FOUND` | Owner not found for resource |
| `ErrOwnershipDenied` | `PERMISSION_DENIED` | Ownership action denied |
| `ErrSelfTransfer` | `INVALID_ARGUMENT` | Cannot transfer to self |
| `ErrNotOwner` | `PERMISSION_DENIED` | Caller is not an owner |

## Rich Error Details

BIB includes structured error details using the `google.rpc.Status` pattern.

### Validation Errors

For `INVALID_ARGUMENT` errors, field-level validation details are included:

```protobuf
// google.rpc.BadRequest
message BadRequest {
  repeated FieldViolation field_violations = 1;
}

message FieldViolation {
  string field = 1;        // Field path (e.g., "topic_id")
  string description = 2;  // Human-readable description
}
```

**Example Response:**
```json
{
  "code": 3,
  "message": "invalid create dataset request",
  "details": [
    {
      "@type": "type.googleapis.com/google.rpc.BadRequest",
      "fieldViolations": [
        {"field": "topic_id", "description": "must not be empty"},
        {"field": "name", "description": "must not be empty"}
      ]
    }
  ]
}
```

### Permission Errors

For `PERMISSION_DENIED` errors, resource and required permission are included:

```protobuf
// google.rpc.ResourceInfo
message ResourceInfo {
  string resource_type = 1;  // e.g., "topic", "dataset"
  string resource_name = 2;  // Resource identifier
  string description = 3;    // Additional info
}
```

**Example Response:**
```json
{
  "code": 7,
  "message": "permission denied: requires owner role to update dataset",
  "details": [
    {
      "@type": "type.googleapis.com/google.rpc.ResourceInfo",
      "resourceType": "dataset",
      "resourceName": "dataset-123",
      "description": "requires owner role"
    }
  ]
}
```

## Handling Errors in Go

```go
import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/genproto/googleapis/rpc/errdetails"
)

func handleError(err error) {
    st, ok := status.FromError(err)
    if !ok {
        // Not a gRPC error
        log.Printf("Non-gRPC error: %v", err)
        return
    }
    
    log.Printf("gRPC error: code=%s message=%s", st.Code(), st.Message())
    
    // Check specific codes
    switch st.Code() {
    case codes.NotFound:
        // Handle not found
    case codes.PermissionDenied:
        // Handle permission denied
    case codes.InvalidArgument:
        // Extract field violations
        for _, detail := range st.Details() {
            if br, ok := detail.(*errdetails.BadRequest); ok {
                for _, violation := range br.FieldViolations {
                    log.Printf("Field %s: %s", violation.Field, violation.Description)
                }
            }
        }
    case codes.Unauthenticated:
        // Re-authenticate
    case codes.Unavailable:
        // Retry with backoff
    }
}
```

## Retry Recommendations

| Code | Retry | Recommendation |
|------|-------|----------------|
| `UNAVAILABLE` | Yes | Exponential backoff |
| `DEADLINE_EXCEEDED` | Yes | Increase timeout |
| `RESOURCE_EXHAUSTED` | Yes | Wait and retry |
| `ABORTED` | Yes | Retry immediately |
| `INTERNAL` | Maybe | Retry with caution |
| `INVALID_ARGUMENT` | No | Fix request |
| `NOT_FOUND` | No | Resource doesn't exist |
| `PERMISSION_DENIED` | No | Insufficient permissions |
| `UNAUTHENTICATED` | No* | Re-authenticate first |

*Retry after re-authentication.

## Rate Limiting

When `RESOURCE_EXHAUSTED` is returned, check the `Retry-After` metadata:

```go
func handleRateLimit(err error) time.Duration {
    st, _ := status.FromError(err)
    if st.Code() != codes.ResourceExhausted {
        return 0
    }
    
    // Check for retry info in details
    for _, detail := range st.Details() {
        if ri, ok := detail.(*errdetails.RetryInfo); ok {
            return ri.RetryDelay.AsDuration()
        }
    }
    
    // Default backoff
    return 5 * time.Second
}
```

