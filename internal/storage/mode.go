// Package storage provides mode enforcement for storage backends.
package storage

import (
	"fmt"
)

// NodeMode represents the P2P node operation mode.
type NodeMode string

const (
	NodeModeFull      NodeMode = "full"
	NodeModeSelective NodeMode = "selective"
	NodeModeProxy     NodeMode = "proxy"
)

// String returns the string representation.
func (m NodeMode) String() string {
	return string(m)
}

// IsValid checks if the mode is valid.
func (m NodeMode) IsValid() bool {
	switch m {
	case NodeModeFull, NodeModeSelective, NodeModeProxy:
		return true
	default:
		return false
	}
}

// ModeEnforcement holds the result of mode/backend validation.
type ModeEnforcement struct {
	// Valid indicates if the configuration is valid
	Valid bool

	// OriginalMode is the configured mode
	OriginalMode NodeMode

	// EffectiveMode is the mode that will be used (may differ if downgraded)
	EffectiveMode NodeMode

	// Backend is the storage backend
	Backend BackendType

	// Downgraded indicates if the mode was downgraded
	Downgraded bool

	// Warning is a warning message if downgraded
	Warning string

	// Error is an error message if invalid
	Error string

	// IsTrustedStorage indicates if this node can be an authoritative data source
	IsTrustedStorage bool

	// RequiresRestart indicates if a restart is required to change modes
	RequiresRestart bool
}

// EnforceMode validates and enforces mode/backend constraints.
// This implements DB-000 from the TODO.
func EnforceMode(mode NodeMode, backend BackendType) *ModeEnforcement {
	result := &ModeEnforcement{
		OriginalMode:  mode,
		EffectiveMode: mode,
		Backend:       backend,
		Valid:         true,
	}

	switch backend {
	case BackendSQLite:
		// SQLite can only be used with proxy or selective (cache-only) modes
		result.IsTrustedStorage = false

		switch mode {
		case NodeModeFull:
			// Full mode with SQLite is not allowed - downgrade to selective
			result.Downgraded = true
			result.EffectiveMode = NodeModeSelective
			result.Warning = fmt.Sprintf(
				"mode '%s' requires PostgreSQL backend; SQLite cannot be an authoritative data source. "+
					"Downgrading to '%s' mode. The node will operate in cache-only mode and cannot "+
					"distribute data to other peers. To use full mode, configure PostgreSQL backend.",
				NodeModeFull, NodeModeSelective,
			)

		case NodeModeSelective:
			// Selective with SQLite is allowed (cache-only)
			result.Warning = "SQLite backend in selective mode operates as cache-only; " +
				"data is not persisted authoritatively and cannot be distributed to peers"

		case NodeModeProxy:
			// Proxy with SQLite is fine
		}

	case BackendPostgres:
		// PostgreSQL supports all modes
		result.IsTrustedStorage = true

	default:
		result.Valid = false
		result.Error = fmt.Sprintf("unknown storage backend: %s", backend)
	}

	return result
}

// ValidateModeChange validates a runtime mode change request.
// Returns an error if the change is not allowed.
func ValidateModeChange(currentMode, requestedMode NodeMode, backend BackendType) error {
	// Mode changes always require a restart
	if currentMode == requestedMode {
		return nil // No change
	}

	// SQLite cannot change to full mode
	if backend == BackendSQLite && requestedMode == NodeModeFull {
		return fmt.Errorf(
			"cannot change to mode '%s' with SQLite backend; "+
				"SQLite cannot be an authoritative data source. "+
				"Configure PostgreSQL backend to use full mode",
			NodeModeFull,
		)
	}

	// All mode changes require restart
	return &ModeChangeRestartRequired{
		CurrentMode:   currentMode,
		RequestedMode: requestedMode,
	}
}

// ModeChangeRestartRequired indicates that a restart is required for mode change.
type ModeChangeRestartRequired struct {
	CurrentMode   NodeMode
	RequestedMode NodeMode
}

func (e *ModeChangeRestartRequired) Error() string {
	return fmt.Sprintf(
		"mode change from '%s' to '%s' requires a restart; "+
			"update the configuration and restart bibd",
		e.CurrentMode, e.RequestedMode,
	)
}

// IsModeChangeRestartRequired checks if the error indicates a restart is required.
func IsModeChangeRestartRequired(err error) bool {
	_, ok := err.(*ModeChangeRestartRequired)
	return ok
}

// PeerStorageMetadata returns metadata about this node's storage capabilities
// for inclusion in P2P peer metadata.
func PeerStorageMetadata(backend BackendType, mode NodeMode) map[string]string {
	enforcement := EnforceMode(mode, backend)

	return map[string]string{
		"storage_backend": string(backend),
		"storage_mode":    string(enforcement.EffectiveMode),
		"trusted_storage": fmt.Sprintf("%t", enforcement.IsTrustedStorage),
		"authoritative":   fmt.Sprintf("%t", enforcement.IsTrustedStorage && mode == NodeModeFull),
		"can_distribute":  fmt.Sprintf("%t", enforcement.IsTrustedStorage),
		"cache_only":      fmt.Sprintf("%t", backend == BackendSQLite),
	}
}
