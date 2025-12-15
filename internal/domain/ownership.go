package domain

import (
	"time"
)

// ResourceType represents the type of resource for ownership.
type ResourceType string

const (
	ResourceTypeTopic   ResourceType = "topic"
	ResourceTypeDataset ResourceType = "dataset"
	ResourceTypeTask    ResourceType = "task"
)

// OwnershipRole represents the role/permission level.
type OwnershipRole string

const (
	// RoleOwner has full control including deletion and ownership transfer.
	RoleOwner OwnershipRole = "owner"

	// RoleAdmin can modify but not delete or transfer ownership.
	RoleAdmin OwnershipRole = "admin"

	// RoleContributor can add data but not modify structure.
	RoleContributor OwnershipRole = "contributor"

	// RoleReader can only read/query data.
	RoleReader OwnershipRole = "reader"
)

// Ownership represents ownership or access grant for a resource.
type Ownership struct {
	// ResourceType is the type of resource (topic, dataset, task).
	ResourceType ResourceType `json:"resource_type"`

	// ResourceID is the ID of the resource.
	ResourceID string `json:"resource_id"`

	// UserID is the user who has this ownership/access.
	UserID UserID `json:"user_id"`

	// Role is the permission level.
	Role OwnershipRole `json:"role"`

	// GrantedAt is when this ownership was granted.
	GrantedAt time.Time `json:"granted_at"`

	// GrantedBy is the user who granted this ownership.
	GrantedBy UserID `json:"granted_by"`

	// ExpiresAt is optional expiration for temporary access.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Validate validates the ownership.
func (o *Ownership) Validate() error {
	if o.ResourceType == "" {
		return ErrInvalidResourceType
	}
	if o.ResourceID == "" {
		return ErrInvalidResourceID
	}
	if o.UserID == "" {
		return ErrInvalidUserID
	}
	if !o.Role.IsValid() {
		return ErrInvalidOwnershipRole
	}
	return nil
}

// IsValid checks if the role is a valid value.
func (r OwnershipRole) IsValid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleContributor, RoleReader:
		return true
	default:
		return false
	}
}

// IsExpired checks if the ownership has expired.
func (o *Ownership) IsExpired() bool {
	if o.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*o.ExpiresAt)
}

// CanModify returns true if the role allows modification.
func (r OwnershipRole) CanModify() bool {
	return r == RoleOwner || r == RoleAdmin
}

// CanContribute returns true if the role allows adding data.
func (r OwnershipRole) CanContribute() bool {
	return r == RoleOwner || r == RoleAdmin || r == RoleContributor
}

// CanTransferOwnership returns true if the role allows ownership transfer.
func (r OwnershipRole) CanTransferOwnership() bool {
	return r == RoleOwner
}

// CanDelete returns true if the role allows deletion.
func (r OwnershipRole) CanDelete() bool {
	return r == RoleOwner
}

// OwnershipTransfer represents a request to transfer ownership.
type OwnershipTransfer struct {
	// ResourceType is the type of resource.
	ResourceType ResourceType `json:"resource_type"`

	// ResourceID is the ID of the resource.
	ResourceID string `json:"resource_id"`

	// FromUserID is the current owner.
	FromUserID UserID `json:"from_user_id"`

	// ToUserID is the new owner.
	ToUserID UserID `json:"to_user_id"`

	// RequestedAt is when the transfer was requested.
	RequestedAt time.Time `json:"requested_at"`

	// Signature is the signature from the current owner.
	Signature []byte `json:"signature"`
}

// Validate validates the ownership transfer.
func (t *OwnershipTransfer) Validate() error {
	if t.ResourceType == "" {
		return ErrInvalidResourceType
	}
	if t.ResourceID == "" {
		return ErrInvalidResourceID
	}
	if t.FromUserID == "" || t.ToUserID == "" {
		return ErrInvalidUserID
	}
	if t.FromUserID == t.ToUserID {
		return ErrSelfTransfer
	}
	return nil
}
