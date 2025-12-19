package domain

import "errors"

// Domain errors
var (
	// Topic errors
	ErrInvalidTopicID        = errors.New("invalid topic ID")
	ErrInvalidTopicName      = errors.New("invalid topic name")
	ErrInvalidTopicStatus    = errors.New("invalid topic status")
	ErrTopicNotFound         = errors.New("topic not found")
	ErrTopicArchived         = errors.New("topic is archived")
	ErrCannotRemoveLastOwner = errors.New("cannot remove last owner")
	ErrOwnerNotFound         = errors.New("owner not found")

	// Dataset errors
	ErrInvalidDatasetID     = errors.New("invalid dataset ID")
	ErrInvalidDatasetName   = errors.New("invalid dataset name")
	ErrInvalidDatasetStatus = errors.New("invalid dataset status")
	ErrDatasetNotFound      = errors.New("dataset not found")
	ErrInvalidHash          = errors.New("invalid hash")
	ErrHashMismatch         = errors.New("hash mismatch")
	ErrInvalidSize          = errors.New("invalid size")
	ErrInvalidChunkCount    = errors.New("invalid chunk count")
	ErrNoOwners             = errors.New("no owners specified")

	// Version errors
	ErrInvalidVersionID     = errors.New("invalid version ID")
	ErrInvalidVersionString = errors.New("invalid version string")
	ErrVersionNotFound      = errors.New("version not found")
	ErrEmptyVersion         = errors.New("version must have content or instructions")

	// Chunk errors
	ErrInvalidChunkIndex = errors.New("invalid chunk index")
	ErrChunkNotFound     = errors.New("chunk not found")

	// User errors
	ErrInvalidUserID     = errors.New("invalid user ID")
	ErrInvalidUserName   = errors.New("invalid user name")
	ErrInvalidPublicKey  = errors.New("invalid public key")
	ErrInvalidKeyType    = errors.New("invalid key type")
	ErrInvalidUserStatus = errors.New("invalid user status")
	ErrInvalidUserRole   = errors.New("invalid user role")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("user already exists")
	ErrUserSuspended     = errors.New("user account is suspended")
	ErrUserPending       = errors.New("user account is pending approval")
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrInvalidOperation  = errors.New("invalid operation")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrAutoRegDisabled   = errors.New("auto-registration is disabled")

	// Session errors
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session has expired")

	// Ownership errors
	ErrInvalidResourceType  = errors.New("invalid resource type")
	ErrInvalidResourceID    = errors.New("invalid resource ID")
	ErrInvalidOwnershipRole = errors.New("invalid ownership role")
	ErrOwnershipDenied      = errors.New("ownership denied")
	ErrSelfTransfer         = errors.New("cannot transfer to self")
	ErrNotOwner             = errors.New("not an owner")

	// Instruction errors
	ErrInvalidInstruction = errors.New("invalid instruction")
	ErrNoInstructions     = errors.New("no instructions provided")

	// Task errors
	ErrInvalidTaskID   = errors.New("invalid task ID")
	ErrInvalidTaskName = errors.New("invalid task name")
	ErrTaskNotFound    = errors.New("task not found")
	ErrEmptyTask       = errors.New("task has no instructions")

	// Schedule errors
	ErrInvalidScheduleType = errors.New("invalid schedule type")
	ErrInvalidCronExpr     = errors.New("invalid cron expression")
	ErrInvalidRepeatCount  = errors.New("invalid repeat count")
	ErrInvalidInterval     = errors.New("invalid interval")
	ErrInvalidTimeRange    = errors.New("end time before start time")

	// Job errors
	ErrInvalidJobID         = errors.New("invalid job ID")
	ErrInvalidJobType       = errors.New("invalid job type")
	ErrJobNotFound          = errors.New("job not found")
	ErrNoTaskOrInstructions = errors.New("job must have task ID or inline instructions")
	ErrInvalidExecutionMode = errors.New("invalid execution mode")
	ErrCyclicDependency     = errors.New("cyclic dependency in job pipeline")

	// Pipeline errors
	ErrInvalidPipelineID   = errors.New("invalid pipeline ID")
	ErrInvalidPipelineName = errors.New("invalid pipeline name")
	ErrEmptyPipeline       = errors.New("pipeline has no jobs")

	// Query errors
	ErrInvalidQueryID   = errors.New("invalid query ID")
	ErrInvalidQueryType = errors.New("invalid query type")
	ErrEmptySQLQuery    = errors.New("empty SQL query")
	ErrNonSelectQuery   = errors.New("only SELECT queries are allowed")
	ErrNoTargetDatasets = errors.New("no target datasets specified")
	ErrInvalidLimit     = errors.New("invalid limit")
	ErrInvalidOffset    = errors.New("invalid offset")
	ErrQueryTimeout     = errors.New("query timed out")

	// Download errors
	ErrDownloadNotFound = errors.New("download not found")
	ErrDownloadFailed   = errors.New("download failed")

	// Catalog errors
	ErrCatalogNotFound = errors.New("catalog not found")
	ErrEntryNotFound   = errors.New("catalog entry not found")

	// Protocol errors
	ErrUnsupportedProtocol = errors.New("unsupported protocol version")
	ErrInvalidMessage      = errors.New("invalid protocol message")
	ErrTimeout             = errors.New("operation timed out")
)
