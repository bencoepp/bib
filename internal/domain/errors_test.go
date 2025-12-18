package domain

import (
	"errors"
	"testing"
)

func TestErrors_NotNil(t *testing.T) {
	allErrors := []struct {
		name string
		err  error
	}{
		// Topic errors
		{"ErrInvalidTopicID", ErrInvalidTopicID},
		{"ErrInvalidTopicName", ErrInvalidTopicName},
		{"ErrInvalidTopicStatus", ErrInvalidTopicStatus},
		{"ErrTopicNotFound", ErrTopicNotFound},
		{"ErrTopicArchived", ErrTopicArchived},
		{"ErrCannotRemoveLastOwner", ErrCannotRemoveLastOwner},
		{"ErrOwnerNotFound", ErrOwnerNotFound},

		// Dataset errors
		{"ErrInvalidDatasetID", ErrInvalidDatasetID},
		{"ErrInvalidDatasetName", ErrInvalidDatasetName},
		{"ErrInvalidDatasetStatus", ErrInvalidDatasetStatus},
		{"ErrDatasetNotFound", ErrDatasetNotFound},
		{"ErrInvalidHash", ErrInvalidHash},
		{"ErrHashMismatch", ErrHashMismatch},
		{"ErrInvalidSize", ErrInvalidSize},
		{"ErrInvalidChunkCount", ErrInvalidChunkCount},
		{"ErrNoOwners", ErrNoOwners},

		// Version errors
		{"ErrInvalidVersionID", ErrInvalidVersionID},
		{"ErrInvalidVersionString", ErrInvalidVersionString},
		{"ErrVersionNotFound", ErrVersionNotFound},
		{"ErrEmptyVersion", ErrEmptyVersion},

		// Chunk errors
		{"ErrInvalidChunkIndex", ErrInvalidChunkIndex},
		{"ErrChunkNotFound", ErrChunkNotFound},

		// User errors
		{"ErrInvalidUserID", ErrInvalidUserID},
		{"ErrInvalidUserName", ErrInvalidUserName},
		{"ErrInvalidPublicKey", ErrInvalidPublicKey},
		{"ErrUserNotFound", ErrUserNotFound},
		{"ErrInvalidSignature", ErrInvalidSignature},
		{"ErrInvalidOperation", ErrInvalidOperation},
		{"ErrUnauthorized", ErrUnauthorized},

		// Ownership errors
		{"ErrInvalidResourceType", ErrInvalidResourceType},
		{"ErrInvalidResourceID", ErrInvalidResourceID},
		{"ErrInvalidOwnershipRole", ErrInvalidOwnershipRole},
		{"ErrOwnershipDenied", ErrOwnershipDenied},
		{"ErrSelfTransfer", ErrSelfTransfer},
		{"ErrNotOwner", ErrNotOwner},

		// Instruction errors
		{"ErrInvalidInstruction", ErrInvalidInstruction},
		{"ErrNoInstructions", ErrNoInstructions},

		// Task errors
		{"ErrInvalidTaskID", ErrInvalidTaskID},
		{"ErrInvalidTaskName", ErrInvalidTaskName},
		{"ErrTaskNotFound", ErrTaskNotFound},
		{"ErrEmptyTask", ErrEmptyTask},

		// Schedule errors
		{"ErrInvalidScheduleType", ErrInvalidScheduleType},
		{"ErrInvalidCronExpr", ErrInvalidCronExpr},
		{"ErrInvalidRepeatCount", ErrInvalidRepeatCount},
		{"ErrInvalidInterval", ErrInvalidInterval},
		{"ErrInvalidTimeRange", ErrInvalidTimeRange},

		// Job errors
		{"ErrInvalidJobID", ErrInvalidJobID},
		{"ErrInvalidJobType", ErrInvalidJobType},
		{"ErrJobNotFound", ErrJobNotFound},
		{"ErrNoTaskOrInstructions", ErrNoTaskOrInstructions},
		{"ErrInvalidExecutionMode", ErrInvalidExecutionMode},
		{"ErrCyclicDependency", ErrCyclicDependency},

		// Pipeline errors
		{"ErrInvalidPipelineID", ErrInvalidPipelineID},
		{"ErrInvalidPipelineName", ErrInvalidPipelineName},
		{"ErrEmptyPipeline", ErrEmptyPipeline},

		// Query errors
		{"ErrInvalidQueryID", ErrInvalidQueryID},
		{"ErrInvalidQueryType", ErrInvalidQueryType},
		{"ErrEmptySQLQuery", ErrEmptySQLQuery},
		{"ErrNonSelectQuery", ErrNonSelectQuery},
		{"ErrNoTargetDatasets", ErrNoTargetDatasets},
		{"ErrInvalidLimit", ErrInvalidLimit},
		{"ErrInvalidOffset", ErrInvalidOffset},
		{"ErrQueryTimeout", ErrQueryTimeout},

		// Download errors
		{"ErrDownloadNotFound", ErrDownloadNotFound},
		{"ErrDownloadFailed", ErrDownloadFailed},

		// Catalog errors
		{"ErrCatalogNotFound", ErrCatalogNotFound},
		{"ErrEntryNotFound", ErrEntryNotFound},

		// Protocol errors
		{"ErrUnsupportedProtocol", ErrUnsupportedProtocol},
		{"ErrInvalidMessage", ErrInvalidMessage},
		{"ErrTimeout", ErrTimeout},
	}

	for _, tt := range allErrors {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s is nil", tt.name)
			}
		})
	}
}

func TestErrors_Unique(t *testing.T) {
	allErrors := []error{
		ErrInvalidTopicID,
		ErrInvalidTopicName,
		ErrInvalidTopicStatus,
		ErrTopicNotFound,
		ErrTopicArchived,
		ErrCannotRemoveLastOwner,
		ErrOwnerNotFound,
		ErrInvalidDatasetID,
		ErrInvalidDatasetName,
		ErrInvalidDatasetStatus,
		ErrDatasetNotFound,
		ErrInvalidHash,
		ErrHashMismatch,
		ErrInvalidSize,
		ErrInvalidChunkCount,
		ErrNoOwners,
		ErrInvalidVersionID,
		ErrInvalidVersionString,
		ErrVersionNotFound,
		ErrEmptyVersion,
		ErrInvalidChunkIndex,
		ErrChunkNotFound,
		ErrInvalidUserID,
		ErrInvalidUserName,
		ErrInvalidPublicKey,
		ErrUserNotFound,
		ErrInvalidSignature,
		ErrInvalidOperation,
		ErrUnauthorized,
		ErrInvalidResourceType,
		ErrInvalidResourceID,
		ErrInvalidOwnershipRole,
		ErrOwnershipDenied,
		ErrSelfTransfer,
		ErrNotOwner,
		ErrInvalidInstruction,
		ErrNoInstructions,
		ErrInvalidTaskID,
		ErrInvalidTaskName,
		ErrTaskNotFound,
		ErrEmptyTask,
		ErrInvalidScheduleType,
		ErrInvalidCronExpr,
		ErrInvalidRepeatCount,
		ErrInvalidInterval,
		ErrInvalidTimeRange,
		ErrInvalidJobID,
		ErrInvalidJobType,
		ErrJobNotFound,
		ErrNoTaskOrInstructions,
		ErrInvalidExecutionMode,
		ErrCyclicDependency,
		ErrInvalidPipelineID,
		ErrInvalidPipelineName,
		ErrEmptyPipeline,
		ErrInvalidQueryID,
		ErrInvalidQueryType,
		ErrEmptySQLQuery,
		ErrNonSelectQuery,
		ErrNoTargetDatasets,
		ErrInvalidLimit,
		ErrInvalidOffset,
		ErrQueryTimeout,
		ErrDownloadNotFound,
		ErrDownloadFailed,
		ErrCatalogNotFound,
		ErrEntryNotFound,
		ErrUnsupportedProtocol,
		ErrInvalidMessage,
		ErrTimeout,
	}

	seen := make(map[string]bool)
	for _, err := range allErrors {
		msg := err.Error()
		if seen[msg] {
			t.Errorf("duplicate error message: %s", msg)
		}
		seen[msg] = true
	}
}

func TestErrors_IsComparable(t *testing.T) {
	err := ErrInvalidTopicID
	if !errors.Is(err, ErrInvalidTopicID) {
		t.Error("errors.Is should match ErrInvalidTopicID")
	}

	wrappedErr := errors.Join(ErrInvalidTopicID, errors.New("additional context"))
	if !errors.Is(wrappedErr, ErrInvalidTopicID) {
		t.Error("errors.Is should find ErrInvalidTopicID in joined error")
	}
}

func TestErrors_Messages(t *testing.T) {
	tests := []struct {
		err     error
		message string
	}{
		{ErrInvalidTopicID, "invalid topic ID"},
		{ErrTopicNotFound, "topic not found"},
		{ErrDatasetNotFound, "dataset not found"},
		{ErrUserNotFound, "user not found"},
		{ErrUnauthorized, "unauthorized"},
		{ErrNonSelectQuery, "only SELECT queries are allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			if tt.err.Error() != tt.message {
				t.Errorf("expected %q, got %q", tt.message, tt.err.Error())
			}
		})
	}
}
