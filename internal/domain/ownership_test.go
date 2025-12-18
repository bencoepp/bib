package domain

import (
	"testing"
	"time"
)

func TestResourceType_Constants(t *testing.T) {
	if ResourceTypeTopic != "topic" {
		t.Errorf("expected 'topic', got %q", ResourceTypeTopic)
	}
	if ResourceTypeDataset != "dataset" {
		t.Errorf("expected 'dataset', got %q", ResourceTypeDataset)
	}
	if ResourceTypeTask != "task" {
		t.Errorf("expected 'task', got %q", ResourceTypeTask)
	}
}

func TestOwnershipRole_IsValid(t *testing.T) {
	tests := []struct {
		role  OwnershipRole
		valid bool
	}{
		{RoleOwner, true},
		{RoleAdmin, true},
		{RoleContributor, true},
		{RoleReader, true},
		{OwnershipRole("unknown"), false},
		{OwnershipRole(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.IsValid(); got != tt.valid {
				t.Errorf("OwnershipRole(%q).IsValid() = %v, want %v", tt.role, got, tt.valid)
			}
		})
	}
}

func TestOwnershipRole_CanModify(t *testing.T) {
	tests := []struct {
		role      OwnershipRole
		canModify bool
	}{
		{RoleOwner, true},
		{RoleAdmin, true},
		{RoleContributor, false},
		{RoleReader, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.CanModify(); got != tt.canModify {
				t.Errorf("OwnershipRole(%q).CanModify() = %v, want %v", tt.role, got, tt.canModify)
			}
		})
	}
}

func TestOwnershipRole_CanContribute(t *testing.T) {
	tests := []struct {
		role          OwnershipRole
		canContribute bool
	}{
		{RoleOwner, true},
		{RoleAdmin, true},
		{RoleContributor, true},
		{RoleReader, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.CanContribute(); got != tt.canContribute {
				t.Errorf("OwnershipRole(%q).CanContribute() = %v, want %v", tt.role, got, tt.canContribute)
			}
		})
	}
}

func TestOwnership_Validate(t *testing.T) {
	tests := []struct {
		name      string
		ownership *Ownership
		wantErr   error
	}{
		{
			name: "valid ownership",
			ownership: &Ownership{
				ResourceType: ResourceTypeTopic,
				ResourceID:   "topic-1",
				UserID:       "user-123",
				Role:         RoleOwner,
				GrantedAt:    time.Now(),
				GrantedBy:    "admin-1",
			},
			wantErr: nil,
		},
		{
			name: "empty resource type",
			ownership: &Ownership{
				ResourceType: "",
				ResourceID:   "topic-1",
				UserID:       "user-123",
				Role:         RoleOwner,
			},
			wantErr: ErrInvalidResourceType,
		},
		{
			name: "empty resource ID",
			ownership: &Ownership{
				ResourceType: ResourceTypeTopic,
				ResourceID:   "",
				UserID:       "user-123",
				Role:         RoleOwner,
			},
			wantErr: ErrInvalidResourceID,
		},
		{
			name: "empty user ID",
			ownership: &Ownership{
				ResourceType: ResourceTypeTopic,
				ResourceID:   "topic-1",
				UserID:       "",
				Role:         RoleOwner,
			},
			wantErr: ErrInvalidUserID,
		},
		{
			name: "invalid role",
			ownership: &Ownership{
				ResourceType: ResourceTypeTopic,
				ResourceID:   "topic-1",
				UserID:       "user-123",
				Role:         OwnershipRole("invalid"),
			},
			wantErr: ErrInvalidOwnershipRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ownership.Validate()
			if err != tt.wantErr {
				t.Errorf("Ownership.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOwnership_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name      string
		ownership *Ownership
		expired   bool
	}{
		{
			name: "no expiration",
			ownership: &Ownership{
				ExpiresAt: nil,
			},
			expired: false,
		},
		{
			name: "not expired",
			ownership: &Ownership{
				ExpiresAt: &future,
			},
			expired: false,
		},
		{
			name: "expired",
			ownership: &Ownership{
				ExpiresAt: &past,
			},
			expired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ownership.IsExpired(); got != tt.expired {
				t.Errorf("Ownership.IsExpired() = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestOwnership_Fields(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	ownership := Ownership{
		ResourceType: ResourceTypeDataset,
		ResourceID:   "dataset-1",
		UserID:       "user-123",
		Role:         RoleContributor,
		GrantedAt:    now,
		GrantedBy:    "admin-1",
		ExpiresAt:    &expiresAt,
	}

	if ownership.ResourceType != ResourceTypeDataset {
		t.Errorf("expected ResourceTypeDataset, got %q", ownership.ResourceType)
	}
	if ownership.ResourceID != "dataset-1" {
		t.Errorf("expected 'dataset-1', got %q", ownership.ResourceID)
	}
	if ownership.UserID != "user-123" {
		t.Errorf("expected 'user-123', got %q", ownership.UserID)
	}
	if ownership.Role != RoleContributor {
		t.Errorf("expected RoleContributor, got %q", ownership.Role)
	}
	if !ownership.GrantedAt.Equal(now) {
		t.Error("GrantedAt mismatch")
	}
	if ownership.GrantedBy != "admin-1" {
		t.Errorf("expected 'admin-1', got %q", ownership.GrantedBy)
	}
	if ownership.ExpiresAt == nil || !ownership.ExpiresAt.Equal(expiresAt) {
		t.Error("ExpiresAt mismatch")
	}
}
