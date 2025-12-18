package domain

import (
	"testing"
	"time"
)

func TestDatasetID_String(t *testing.T) {
	id := DatasetID("dataset-123")
	if id.String() != "dataset-123" {
		t.Errorf("expected 'dataset-123', got %q", id.String())
	}
}

func TestDatasetVersionID_String(t *testing.T) {
	id := DatasetVersionID("v1.0.0")
	if id.String() != "v1.0.0" {
		t.Errorf("expected 'v1.0.0', got %q", id.String())
	}
}

func TestChunkID_String(t *testing.T) {
	id := ChunkID("chunk-abc")
	if id.String() != "chunk-abc" {
		t.Errorf("expected 'chunk-abc', got %q", id.String())
	}
}

func TestDatasetStatus_IsValid(t *testing.T) {
	tests := []struct {
		status DatasetStatus
		valid  bool
	}{
		{DatasetStatusDraft, true},
		{DatasetStatusActive, true},
		{DatasetStatusArchived, true},
		{DatasetStatusDeleted, true},
		{DatasetStatusIngesting, true},
		{DatasetStatusFailed, true},
		{DatasetStatus("unknown"), false},
		{DatasetStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("DatasetStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestChunkStatus_IsValid(t *testing.T) {
	tests := []struct {
		status ChunkStatus
		valid  bool
	}{
		{ChunkStatusPending, true},
		{ChunkStatusDownloading, true},
		{ChunkStatusDownloaded, true},
		{ChunkStatusVerified, true},
		{ChunkStatusFailed, true},
		{ChunkStatus("unknown"), false},
		{ChunkStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("ChunkStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestDataset_Validate(t *testing.T) {
	owner := UserID("owner-123")

	tests := []struct {
		name    string
		dataset *Dataset
		wantErr error
	}{
		{
			name: "valid dataset",
			dataset: &Dataset{
				ID:      "dataset-1",
				TopicID: "topic-1",
				Name:    "Test Dataset",
				Status:  DatasetStatusActive,
				Owners:  []UserID{owner},
			},
			wantErr: nil,
		},
		{
			name: "empty ID",
			dataset: &Dataset{
				ID:      "",
				TopicID: "topic-1",
				Name:    "Test Dataset",
				Owners:  []UserID{owner},
			},
			wantErr: ErrInvalidDatasetID,
		},
		{
			name: "empty topic ID",
			dataset: &Dataset{
				ID:     "dataset-1",
				Name:   "Test Dataset",
				Owners: []UserID{owner},
			},
			wantErr: ErrInvalidTopicID,
		},
		{
			name: "empty name",
			dataset: &Dataset{
				ID:      "dataset-1",
				TopicID: "topic-1",
				Name:    "",
				Owners:  []UserID{owner},
			},
			wantErr: ErrInvalidDatasetName,
		},
		{
			name: "invalid status",
			dataset: &Dataset{
				ID:      "dataset-1",
				TopicID: "topic-1",
				Name:    "Test Dataset",
				Status:  DatasetStatus("invalid"),
				Owners:  []UserID{owner},
			},
			wantErr: ErrInvalidDatasetStatus,
		},
		{
			name: "no owners",
			dataset: &Dataset{
				ID:      "dataset-1",
				TopicID: "topic-1",
				Name:    "Test Dataset",
				Owners:  []UserID{},
			},
			wantErr: ErrNoOwners,
		},
		{
			name: "empty status is valid",
			dataset: &Dataset{
				ID:      "dataset-1",
				TopicID: "topic-1",
				Name:    "Test Dataset",
				Status:  "",
				Owners:  []UserID{owner},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dataset.Validate()
			if err != tt.wantErr {
				t.Errorf("Dataset.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDataset_IsOwner(t *testing.T) {
	owner1 := UserID("owner-1")
	owner2 := UserID("owner-2")
	nonOwner := UserID("non-owner")

	dataset := &Dataset{
		Owners: []UserID{owner1, owner2},
	}

	if !dataset.IsOwner(owner1) {
		t.Error("owner1 should be an owner")
	}
	if !dataset.IsOwner(owner2) {
		t.Error("owner2 should be an owner")
	}
	if dataset.IsOwner(nonOwner) {
		t.Error("nonOwner should not be an owner")
	}
}

func TestDatasetVersion_Validate(t *testing.T) {
	tests := []struct {
		name    string
		version *DatasetVersion
		wantErr error
	}{
		{
			name: "valid version with content",
			version: &DatasetVersion{
				ID:        "v1",
				DatasetID: "dataset-1",
				Version:   "1.0.0",
				Content:   &DatasetContent{Hash: "abc123", Size: 1024, ChunkCount: 1},
			},
			wantErr: nil,
		},
		{
			name: "valid version with instructions",
			version: &DatasetVersion{
				ID:           "v1",
				DatasetID:    "dataset-1",
				Version:      "1.0.0",
				Instructions: &DatasetInstructions{TaskID: "task-1"},
			},
			wantErr: nil,
		},
		{
			name: "empty ID",
			version: &DatasetVersion{
				ID:        "",
				DatasetID: "dataset-1",
				Version:   "1.0.0",
				Content:   &DatasetContent{Hash: "abc123"},
			},
			wantErr: ErrInvalidVersionID,
		},
		{
			name: "empty dataset ID",
			version: &DatasetVersion{
				ID:      "v1",
				Version: "1.0.0",
				Content: &DatasetContent{Hash: "abc123"},
			},
			wantErr: ErrInvalidDatasetID,
		},
		{
			name: "empty version string",
			version: &DatasetVersion{
				ID:        "v1",
				DatasetID: "dataset-1",
				Version:   "",
				Content:   &DatasetContent{Hash: "abc123"},
			},
			wantErr: ErrInvalidVersionString,
		},
		{
			name: "no content or instructions",
			version: &DatasetVersion{
				ID:        "v1",
				DatasetID: "dataset-1",
				Version:   "1.0.0",
			},
			wantErr: ErrEmptyVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.version.Validate()
			if err != tt.wantErr {
				t.Errorf("DatasetVersion.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDatasetVersion_HasContent(t *testing.T) {
	versionWithContent := &DatasetVersion{Content: &DatasetContent{}}
	versionWithoutContent := &DatasetVersion{}

	if !versionWithContent.HasContent() {
		t.Error("expected HasContent() to return true")
	}
	if versionWithoutContent.HasContent() {
		t.Error("expected HasContent() to return false")
	}
}

func TestDatasetVersion_HasInstructions(t *testing.T) {
	versionWithInstructions := &DatasetVersion{Instructions: &DatasetInstructions{}}
	versionWithoutInstructions := &DatasetVersion{}

	if !versionWithInstructions.HasInstructions() {
		t.Error("expected HasInstructions() to return true")
	}
	if versionWithoutInstructions.HasInstructions() {
		t.Error("expected HasInstructions() to return false")
	}
}

func TestDatasetContent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		content *DatasetContent
		wantErr error
	}{
		{
			name: "valid content",
			content: &DatasetContent{
				Hash:       "abc123",
				Size:       1024,
				ChunkCount: 1,
				ChunkSize:  1024,
			},
			wantErr: nil,
		},
		{
			name: "empty hash",
			content: &DatasetContent{
				Hash:       "",
				Size:       1024,
				ChunkCount: 1,
			},
			wantErr: ErrInvalidHash,
		},
		{
			name: "negative size",
			content: &DatasetContent{
				Hash:       "abc123",
				Size:       -1,
				ChunkCount: 1,
			},
			wantErr: ErrInvalidSize,
		},
		{
			name: "negative chunk count",
			content: &DatasetContent{
				Hash:       "abc123",
				Size:       1024,
				ChunkCount: -1,
			},
			wantErr: ErrInvalidChunkCount,
		},
		{
			name: "zero size is valid",
			content: &DatasetContent{
				Hash:       "abc123",
				Size:       0,
				ChunkCount: 0,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.content.Validate()
			if err != tt.wantErr {
				t.Errorf("DatasetContent.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDatasetInstructions_Validate(t *testing.T) {
	tests := []struct {
		name         string
		instructions *DatasetInstructions
		wantErr      error
	}{
		{
			name: "valid with task ID",
			instructions: &DatasetInstructions{
				TaskID: "task-1",
			},
			wantErr: nil,
		},
		{
			name: "valid with instructions",
			instructions: &DatasetInstructions{
				Instructions: []Instruction{
					{Operation: OpHTTPGet},
				},
			},
			wantErr: nil,
		},
		{
			name: "no task ID or instructions",
			instructions: &DatasetInstructions{
				TaskID:       "",
				Instructions: nil,
			},
			wantErr: ErrNoInstructions,
		},
		{
			name: "invalid instruction",
			instructions: &DatasetInstructions{
				Instructions: []Instruction{
					{Operation: InstructionOp("invalid")},
				},
			},
			wantErr: ErrInvalidInstruction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.instructions.Validate()
			if err != tt.wantErr {
				t.Errorf("DatasetInstructions.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChunk_Validate(t *testing.T) {
	tests := []struct {
		name    string
		chunk   *Chunk
		wantErr error
	}{
		{
			name: "valid chunk",
			chunk: &Chunk{
				ID:        "chunk-1",
				DatasetID: "dataset-1",
				VersionID: "v1",
				Index:     0,
				Hash:      "abc123",
				Size:      1024,
			},
			wantErr: nil,
		},
		{
			name: "empty dataset ID",
			chunk: &Chunk{
				ID:        "chunk-1",
				DatasetID: "",
				Index:     0,
				Hash:      "abc123",
			},
			wantErr: ErrInvalidDatasetID,
		},
		{
			name: "negative index",
			chunk: &Chunk{
				ID:        "chunk-1",
				DatasetID: "dataset-1",
				Index:     -1,
				Hash:      "abc123",
			},
			wantErr: ErrInvalidChunkIndex,
		},
		{
			name: "empty hash",
			chunk: &Chunk{
				ID:        "chunk-1",
				DatasetID: "dataset-1",
				Index:     0,
				Hash:      "",
			},
			wantErr: ErrInvalidHash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.chunk.Validate()
			if err != tt.wantErr {
				t.Errorf("Chunk.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCatalog_HasEntry(t *testing.T) {
	catalog := &Catalog{
		PeerID: "peer-1",
		Entries: []CatalogEntry{
			{Hash: "hash1", DatasetID: "dataset-1"},
			{Hash: "hash2", DatasetID: "dataset-2"},
		},
		LastUpdated: time.Now(),
	}

	if !catalog.HasEntry("hash1") {
		t.Error("expected HasEntry('hash1') to return true")
	}
	if !catalog.HasEntry("hash2") {
		t.Error("expected HasEntry('hash2') to return true")
	}
	if catalog.HasEntry("hash3") {
		t.Error("expected HasEntry('hash3') to return false")
	}
}

func TestCatalog_GetEntry(t *testing.T) {
	catalog := &Catalog{
		PeerID: "peer-1",
		Entries: []CatalogEntry{
			{DatasetID: "dataset-1", DatasetName: "Dataset 1"},
			{DatasetID: "dataset-2", DatasetName: "Dataset 2"},
		},
		LastUpdated: time.Now(),
	}

	entry := catalog.GetEntry("dataset-1")
	if entry == nil {
		t.Fatal("expected entry for dataset-1")
	}
	if entry.DatasetName != "Dataset 1" {
		t.Errorf("expected 'Dataset 1', got %q", entry.DatasetName)
	}

	entry = catalog.GetEntry("dataset-nonexistent")
	if entry != nil {
		t.Error("expected nil for non-existent dataset")
	}
}
