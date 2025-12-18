package domain

import (
	"testing"
	"time"
)

func TestTopicID_String(t *testing.T) {
	id := TopicID("test-topic")
	if id.String() != "test-topic" {
		t.Errorf("expected 'test-topic', got %q", id.String())
	}
}

func TestTopicStatus_IsValid(t *testing.T) {
	tests := []struct {
		status TopicStatus
		valid  bool
	}{
		{TopicStatusActive, true},
		{TopicStatusArchived, true},
		{TopicStatusDeleted, true},
		{TopicStatus("unknown"), false},
		{TopicStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("TopicStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestTopic_Validate(t *testing.T) {
	now := time.Now()
	owner := UserID("owner-123")

	tests := []struct {
		name    string
		topic   *Topic
		wantErr error
	}{
		{
			name: "valid topic",
			topic: &Topic{
				ID:        "topic-1",
				Name:      "Test Topic",
				Status:    TopicStatusActive,
				Owners:    []UserID{owner},
				CreatedAt: now,
				UpdatedAt: now,
			},
			wantErr: nil,
		},
		{
			name: "empty ID",
			topic: &Topic{
				ID:     "",
				Name:   "Test Topic",
				Owners: []UserID{owner},
			},
			wantErr: ErrInvalidTopicID,
		},
		{
			name: "empty name",
			topic: &Topic{
				ID:     "topic-1",
				Name:   "",
				Owners: []UserID{owner},
			},
			wantErr: ErrInvalidTopicName,
		},
		{
			name: "invalid status",
			topic: &Topic{
				ID:     "topic-1",
				Name:   "Test Topic",
				Status: TopicStatus("invalid"),
				Owners: []UserID{owner},
			},
			wantErr: ErrInvalidTopicStatus,
		},
		{
			name: "no owners",
			topic: &Topic{
				ID:     "topic-1",
				Name:   "Test Topic",
				Status: TopicStatusActive,
				Owners: []UserID{},
			},
			wantErr: ErrNoOwners,
		},
		{
			name: "empty status is valid",
			topic: &Topic{
				ID:     "topic-1",
				Name:   "Test Topic",
				Status: "",
				Owners: []UserID{owner},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.topic.Validate()
			if err != tt.wantErr {
				t.Errorf("Topic.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTopic_Path(t *testing.T) {
	topic := &Topic{ID: "weather/precipitation"}
	if got := topic.Path(); got != "weather/precipitation" {
		t.Errorf("Topic.Path() = %q, want %q", got, "weather/precipitation")
	}
}

func TestTopic_IsHierarchical(t *testing.T) {
	tests := []struct {
		name     string
		parentID TopicID
		want     bool
	}{
		{"with parent", "parent-id", true},
		{"without parent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topic := &Topic{ParentID: tt.parentID}
			if got := topic.IsHierarchical(); got != tt.want {
				t.Errorf("Topic.IsHierarchical() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTopic_IsActive(t *testing.T) {
	tests := []struct {
		status TopicStatus
		want   bool
	}{
		{TopicStatusActive, true},
		{TopicStatusArchived, false},
		{TopicStatusDeleted, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			topic := &Topic{Status: tt.status}
			if got := topic.IsActive(); got != tt.want {
				t.Errorf("Topic.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTopic_IsOwner(t *testing.T) {
	owner1 := UserID("owner-1")
	owner2 := UserID("owner-2")
	nonOwner := UserID("non-owner")

	topic := &Topic{
		Owners: []UserID{owner1, owner2},
	}

	tests := []struct {
		userID UserID
		want   bool
	}{
		{owner1, true},
		{owner2, true},
		{nonOwner, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.userID), func(t *testing.T) {
			if got := topic.IsOwner(tt.userID); got != tt.want {
				t.Errorf("Topic.IsOwner(%q) = %v, want %v", tt.userID, got, tt.want)
			}
		})
	}
}

func TestTopic_AddOwner(t *testing.T) {
	owner1 := UserID("owner-1")
	owner2 := UserID("owner-2")

	topic := &Topic{
		Owners:    []UserID{owner1},
		UpdatedAt: time.Now().Add(-time.Hour),
	}
	oldUpdatedAt := topic.UpdatedAt

	// Add new owner
	topic.AddOwner(owner2)
	if len(topic.Owners) != 2 {
		t.Errorf("expected 2 owners, got %d", len(topic.Owners))
	}
	if !topic.IsOwner(owner2) {
		t.Error("owner2 should be an owner")
	}
	if !topic.UpdatedAt.After(oldUpdatedAt) {
		t.Error("UpdatedAt should be updated")
	}

	// Add existing owner (should be no-op)
	prevLen := len(topic.Owners)
	topic.AddOwner(owner1)
	if len(topic.Owners) != prevLen {
		t.Error("adding existing owner should be no-op")
	}
}

func TestTopic_RemoveOwner(t *testing.T) {
	owner1 := UserID("owner-1")
	owner2 := UserID("owner-2")
	nonOwner := UserID("non-owner")

	t.Run("remove existing owner", func(t *testing.T) {
		topic := &Topic{
			Owners:    []UserID{owner1, owner2},
			UpdatedAt: time.Now().Add(-time.Hour),
		}
		oldUpdatedAt := topic.UpdatedAt

		err := topic.RemoveOwner(owner2)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(topic.Owners) != 1 {
			t.Errorf("expected 1 owner, got %d", len(topic.Owners))
		}
		if topic.IsOwner(owner2) {
			t.Error("owner2 should not be an owner")
		}
		if !topic.UpdatedAt.After(oldUpdatedAt) {
			t.Error("UpdatedAt should be updated")
		}
	})

	t.Run("cannot remove last owner", func(t *testing.T) {
		topic := &Topic{
			Owners: []UserID{owner1},
		}

		err := topic.RemoveOwner(owner1)
		if err != ErrCannotRemoveLastOwner {
			t.Errorf("expected ErrCannotRemoveLastOwner, got %v", err)
		}
	})

	t.Run("remove non-existent owner", func(t *testing.T) {
		topic := &Topic{
			Owners: []UserID{owner1, owner2},
		}

		err := topic.RemoveOwner(nonOwner)
		if err != ErrOwnerNotFound {
			t.Errorf("expected ErrOwnerNotFound, got %v", err)
		}
	})
}

func TestParseTopicPath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"weather/precipitation/rainfall", []string{"weather", "precipitation", "rainfall"}},
		{"weather", []string{"weather"}},
		{"/weather/", []string{"weather"}},
		{"", nil},
		{"/", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ParseTopicPath(tt.path)
			if len(got) != len(tt.want) {
				t.Errorf("ParseTopicPath(%q) = %v, want %v", tt.path, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseTopicPath(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
				}
			}
		})
	}
}
