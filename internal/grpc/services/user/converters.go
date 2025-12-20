// Package user implements the UserService gRPC service.
package user

import (
	"bib/internal/domain"
	"bib/internal/storage"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// userToProto converts a domain user to proto User.
func userToProto(u *domain.User) *services.User {
	if u == nil {
		return nil
	}

	proto := &services.User{
		Id:                   string(u.ID),
		PublicKey:            u.PublicKey,
		KeyType:              string(u.KeyType),
		PublicKeyFingerprint: u.PublicKeyFingerprint,
		Name:                 u.Name,
		Email:                u.Email,
		Status:               domainUserStatusToProto(u.Status),
		Role:                 domainUserRoleToProto(u.Role),
		Locale:               u.Locale,
		CreatedAt:            timestamppb.New(u.CreatedAt),
		UpdatedAt:            timestamppb.New(u.UpdatedAt),
		Metadata:             u.Metadata,
	}

	if u.LastLoginAt != nil {
		proto.LastLoginAt = timestamppb.New(*u.LastLoginAt)
	}

	return proto
}

func domainUserStatusToProto(s domain.UserStatus) services.UserStatus {
	switch s {
	case domain.UserStatusActive:
		return services.UserStatus_USER_STATUS_ACTIVE
	case domain.UserStatusPending:
		return services.UserStatus_USER_STATUS_PENDING
	case domain.UserStatusSuspended:
		return services.UserStatus_USER_STATUS_SUSPENDED
	case domain.UserStatusDeleted:
		return services.UserStatus_USER_STATUS_DELETED
	default:
		return services.UserStatus_USER_STATUS_UNSPECIFIED
	}
}

func protoUserStatusToDomain(s services.UserStatus) domain.UserStatus {
	switch s {
	case services.UserStatus_USER_STATUS_ACTIVE:
		return domain.UserStatusActive
	case services.UserStatus_USER_STATUS_PENDING:
		return domain.UserStatusPending
	case services.UserStatus_USER_STATUS_SUSPENDED:
		return domain.UserStatusSuspended
	case services.UserStatus_USER_STATUS_DELETED:
		return domain.UserStatusDeleted
	default:
		return ""
	}
}

func domainUserRoleToProto(r domain.UserRole) services.UserRole {
	switch r {
	case domain.UserRoleAdmin:
		return services.UserRole_USER_ROLE_ADMIN
	case domain.UserRoleUser:
		return services.UserRole_USER_ROLE_USER
	case domain.UserRoleReadonly:
		return services.UserRole_USER_ROLE_READONLY
	default:
		return services.UserRole_USER_ROLE_UNSPECIFIED
	}
}

func protoUserRoleToDomain(r services.UserRole) domain.UserRole {
	switch r {
	case services.UserRole_USER_ROLE_ADMIN:
		return domain.UserRoleAdmin
	case services.UserRole_USER_ROLE_USER:
		return domain.UserRoleUser
	case services.UserRole_USER_ROLE_READONLY:
		return domain.UserRoleReadonly
	default:
		return ""
	}
}

func prefsToProto(p *storage.UserPreferences) *services.UserPreferences {
	if p == nil {
		return nil
	}

	return &services.UserPreferences{
		UserId:               string(p.UserID),
		Theme:                p.Theme,
		Locale:               p.Locale,
		Timezone:             p.Timezone,
		DateFormat:           p.DateFormat,
		NotificationsEnabled: p.NotificationsEnabled,
		EmailNotifications:   p.EmailNotifications,
		Custom:               p.Custom,
	}
}

func sessionToProto(s *storage.Session) *services.Session {
	if s == nil {
		return nil
	}

	proto := &services.Session{
		Id:                   s.ID,
		UserId:               string(s.UserID),
		Type:                 sessionTypeToProto(s.Type),
		ClientIp:             s.ClientIP,
		ClientAgent:          s.ClientAgent,
		PublicKeyFingerprint: s.PublicKeyFingerprint,
		NodeId:               s.NodeID,
		StartedAt:            timestamppb.New(s.StartedAt),
		LastActivityAt:       timestamppb.New(s.LastActivityAt),
		Metadata:             s.Metadata,
	}

	if s.EndedAt != nil {
		proto.EndedAt = timestamppb.New(*s.EndedAt)
	}

	return proto
}

func sessionTypeToProto(t storage.SessionType) services.SessionType {
	switch t {
	case storage.SessionTypeSSH:
		return services.SessionType_SESSION_TYPE_SSH
	case storage.SessionTypeGRPC:
		return services.SessionType_SESSION_TYPE_GRPC
	case storage.SessionTypeAPI:
		return services.SessionType_SESSION_TYPE_API
	default:
		return services.SessionType_SESSION_TYPE_UNSPECIFIED
	}
}
