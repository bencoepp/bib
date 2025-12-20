// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"errors"
	"fmt"

	"bib/internal/domain"
	"bib/internal/storage"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// domainErrorMapping maps domain errors to gRPC codes and descriptions.
var domainErrorMapping = map[error]struct {
	code codes.Code
	desc string
}{
	// Storage errors (generic)
	storage.ErrNotFound:      {codes.NotFound, "Resource not found"},
	storage.ErrAlreadyExists: {codes.AlreadyExists, "Resource already exists"},
	storage.ErrInvalidInput:  {codes.InvalidArgument, "Invalid input"},

	// Topic errors
	domain.ErrInvalidTopicID:        {codes.InvalidArgument, "Invalid topic ID"},
	domain.ErrInvalidTopicName:      {codes.InvalidArgument, "Invalid topic name"},
	domain.ErrInvalidTopicStatus:    {codes.InvalidArgument, "Invalid topic status"},
	domain.ErrTopicNotFound:         {codes.NotFound, "Topic not found"},
	domain.ErrTopicArchived:         {codes.FailedPrecondition, "Topic is archived"},
	domain.ErrCannotRemoveLastOwner: {codes.FailedPrecondition, "Cannot remove the last owner"},
	domain.ErrOwnerNotFound:         {codes.NotFound, "Owner not found"},

	// Dataset errors
	domain.ErrInvalidDatasetID:     {codes.InvalidArgument, "Invalid dataset ID"},
	domain.ErrInvalidDatasetName:   {codes.InvalidArgument, "Invalid dataset name"},
	domain.ErrInvalidDatasetStatus: {codes.InvalidArgument, "Invalid dataset status"},
	domain.ErrDatasetNotFound:      {codes.NotFound, "Dataset not found"},
	domain.ErrInvalidHash:          {codes.InvalidArgument, "Invalid hash"},
	domain.ErrHashMismatch:         {codes.DataLoss, "Hash mismatch - data may be corrupted"},
	domain.ErrInvalidSize:          {codes.InvalidArgument, "Invalid size"},
	domain.ErrInvalidChunkCount:    {codes.InvalidArgument, "Invalid chunk count"},
	domain.ErrNoOwners:             {codes.InvalidArgument, "No owners specified"},

	// Version errors
	domain.ErrInvalidVersionID:     {codes.InvalidArgument, "Invalid version ID"},
	domain.ErrInvalidVersionString: {codes.InvalidArgument, "Invalid version string"},
	domain.ErrVersionNotFound:      {codes.NotFound, "Version not found"},
	domain.ErrEmptyVersion:         {codes.InvalidArgument, "Version must have content or instructions"},

	// Chunk errors
	domain.ErrInvalidChunkIndex: {codes.InvalidArgument, "Invalid chunk index"},
	domain.ErrChunkNotFound:     {codes.NotFound, "Chunk not found"},

	// User errors
	domain.ErrInvalidUserID:     {codes.InvalidArgument, "Invalid user ID"},
	domain.ErrInvalidUserName:   {codes.InvalidArgument, "Invalid user name"},
	domain.ErrInvalidPublicKey:  {codes.InvalidArgument, "Invalid public key"},
	domain.ErrInvalidKeyType:    {codes.InvalidArgument, "Invalid key type"},
	domain.ErrInvalidUserStatus: {codes.InvalidArgument, "Invalid user status"},
	domain.ErrInvalidUserRole:   {codes.InvalidArgument, "Invalid user role"},
	domain.ErrUserNotFound:      {codes.NotFound, "User not found"},
	domain.ErrUserExists:        {codes.AlreadyExists, "User already exists"},
	domain.ErrUserSuspended:     {codes.PermissionDenied, "User account is suspended"},
	domain.ErrUserPending:       {codes.PermissionDenied, "User account is pending approval"},
	domain.ErrInvalidSignature:  {codes.Unauthenticated, "Invalid signature"},
	domain.ErrInvalidOperation:  {codes.InvalidArgument, "Invalid operation"},
	domain.ErrUnauthorized:      {codes.PermissionDenied, "Unauthorized"},
	domain.ErrAutoRegDisabled:   {codes.PermissionDenied, "Auto-registration is disabled"},

	// Session errors
	domain.ErrSessionNotFound: {codes.NotFound, "Session not found"},
	domain.ErrSessionExpired:  {codes.Unauthenticated, "Session has expired"},

	// Ownership errors
	domain.ErrInvalidResourceType:  {codes.InvalidArgument, "Invalid resource type"},
	domain.ErrInvalidResourceID:    {codes.InvalidArgument, "Invalid resource ID"},
	domain.ErrInvalidOwnershipRole: {codes.InvalidArgument, "Invalid ownership role"},
	domain.ErrOwnershipDenied:      {codes.PermissionDenied, "Ownership denied"},
	domain.ErrSelfTransfer:         {codes.InvalidArgument, "Cannot transfer to self"},
	domain.ErrNotOwner:             {codes.PermissionDenied, "Not an owner of this resource"},

	// Instruction errors
	domain.ErrInvalidInstruction: {codes.InvalidArgument, "Invalid instruction"},
	domain.ErrNoInstructions:     {codes.InvalidArgument, "No instructions provided"},

	// Task errors
	domain.ErrInvalidTaskID:   {codes.InvalidArgument, "Invalid task ID"},
	domain.ErrInvalidTaskName: {codes.InvalidArgument, "Invalid task name"},
	domain.ErrTaskNotFound:    {codes.NotFound, "Task not found"},
	domain.ErrEmptyTask:       {codes.InvalidArgument, "Task has no instructions"},

	// Schedule errors
	domain.ErrInvalidScheduleType: {codes.InvalidArgument, "Invalid schedule type"},
	domain.ErrInvalidCronExpr:     {codes.InvalidArgument, "Invalid cron expression"},
	domain.ErrInvalidRepeatCount:  {codes.InvalidArgument, "Invalid repeat count"},
	domain.ErrInvalidInterval:     {codes.InvalidArgument, "Invalid interval"},
	domain.ErrInvalidTimeRange:    {codes.InvalidArgument, "End time must be after start time"},
}

// MapDomainError converts a domain error to a gRPC status error with rich details.
func MapDomainError(err error) error {
	if err == nil {
		return nil
	}

	// Check if it's already a gRPC status error
	if _, ok := status.FromError(err); ok {
		return err
	}

	// Look up the error in our mapping
	for domainErr, mapping := range domainErrorMapping {
		if errors.Is(err, domainErr) {
			return NewDetailedError(mapping.code, mapping.desc, err)
		}
	}

	// Default to internal error for unknown errors
	return status.Errorf(codes.Internal, "internal error: %v", err)
}

// NewDetailedError creates a gRPC error with rich error details.
func NewDetailedError(code codes.Code, message string, cause error) error {
	st := status.New(code, message)

	// Add error info details
	details := &errdetails.ErrorInfo{
		Reason: codeToReason(code),
		Domain: "bib.dev",
		Metadata: map[string]string{
			"error_type": fmt.Sprintf("%T", cause),
		},
	}

	// Add the original error message if different
	if cause != nil && cause.Error() != message {
		details.Metadata["original_error"] = cause.Error()
	}

	st, err := st.WithDetails(details)
	if err != nil {
		// Fall back to simple error if details can't be added
		return status.Error(code, message)
	}

	return st.Err()
}

// NewValidationError creates a gRPC error for validation failures with field-level details.
func NewValidationError(message string, fieldViolations map[string]string) error {
	st := status.New(codes.InvalidArgument, message)

	br := &errdetails.BadRequest{}
	for field, desc := range fieldViolations {
		br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
			Field:       field,
			Description: desc,
		})
	}

	st, err := st.WithDetails(br)
	if err != nil {
		return status.Error(codes.InvalidArgument, message)
	}

	return st.Err()
}

// NewResourceNotFoundError creates a NotFound error with resource details.
func NewResourceNotFoundError(resourceType, resourceID string) error {
	st := status.New(codes.NotFound, fmt.Sprintf("%s not found: %s", resourceType, resourceID))

	ri := &errdetails.ResourceInfo{
		ResourceType: resourceType,
		ResourceName: resourceID,
		Description:  fmt.Sprintf("The requested %s could not be found", resourceType),
	}

	st, err := st.WithDetails(ri)
	if err != nil {
		return status.Errorf(codes.NotFound, "%s not found: %s", resourceType, resourceID)
	}

	return st.Err()
}

// NewPreconditionError creates a FailedPrecondition error with details.
func NewPreconditionError(message string, violations map[string]string) error {
	st := status.New(codes.FailedPrecondition, message)

	pf := &errdetails.PreconditionFailure{}
	for condType, desc := range violations {
		pf.Violations = append(pf.Violations, &errdetails.PreconditionFailure_Violation{
			Type:        condType,
			Description: desc,
		})
	}

	st, err := st.WithDetails(pf)
	if err != nil {
		return status.Error(codes.FailedPrecondition, message)
	}

	return st.Err()
}

// NewQuotaExceededError creates a ResourceExhausted error for quota violations.
func NewQuotaExceededError(resourceType, subject string, limit, current int64) error {
	st := status.New(codes.ResourceExhausted, fmt.Sprintf("%s quota exceeded", resourceType))

	qi := &errdetails.QuotaFailure{
		Violations: []*errdetails.QuotaFailure_Violation{
			{
				Subject:     subject,
				Description: fmt.Sprintf("Quota for %s exceeded: limit=%d, current=%d", resourceType, limit, current),
			},
		},
	}

	st, err := st.WithDetails(qi)
	if err != nil {
		return status.Errorf(codes.ResourceExhausted, "%s quota exceeded: limit=%d", resourceType, limit)
	}

	return st.Err()
}

// NewPermissionDeniedError creates a PermissionDenied error with details.
func NewPermissionDeniedError(action, resource string, requiredRole string) error {
	message := fmt.Sprintf("permission denied: cannot %s %s", action, resource)
	st := status.New(codes.PermissionDenied, message)

	ei := &errdetails.ErrorInfo{
		Reason: "PERMISSION_DENIED",
		Domain: "bib.dev",
		Metadata: map[string]string{
			"action":        action,
			"resource":      resource,
			"required_role": requiredRole,
		},
	}

	st, err := st.WithDetails(ei)
	if err != nil {
		return status.Error(codes.PermissionDenied, message)
	}

	return st.Err()
}

// codeToReason converts a gRPC code to a reason string.
func codeToReason(code codes.Code) string {
	switch code {
	case codes.InvalidArgument:
		return "INVALID_ARGUMENT"
	case codes.NotFound:
		return "NOT_FOUND"
	case codes.AlreadyExists:
		return "ALREADY_EXISTS"
	case codes.PermissionDenied:
		return "PERMISSION_DENIED"
	case codes.Unauthenticated:
		return "UNAUTHENTICATED"
	case codes.ResourceExhausted:
		return "RESOURCE_EXHAUSTED"
	case codes.FailedPrecondition:
		return "FAILED_PRECONDITION"
	case codes.Aborted:
		return "ABORTED"
	case codes.Internal:
		return "INTERNAL"
	case codes.Unavailable:
		return "UNAVAILABLE"
	case codes.DataLoss:
		return "DATA_LOSS"
	default:
		return "UNKNOWN"
	}
}

// MinQueryLengthError is a specific validation error for search queries.
const MinQueryLength = 3

// ErrQueryTooShort is returned when a search query is too short.
var ErrQueryTooShort = fmt.Errorf("query must be at least %d characters", MinQueryLength)

// ValidateSearchQuery validates a search query meets minimum length requirements.
func ValidateSearchQuery(query string) error {
	if len(query) < MinQueryLength {
		return NewValidationError(
			ErrQueryTooShort.Error(),
			map[string]string{
				"query": fmt.Sprintf("must be at least %d characters, got %d", MinQueryLength, len(query)),
			},
		)
	}
	return nil
}
