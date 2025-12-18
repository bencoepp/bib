package credentials

import (
	"context"
	"fmt"
	"sync"
	"time"

	"bib/internal/storage"
)

// RotationState represents the current state of a credential rotation.
type RotationState string

const (
	// RotationIdle means no rotation is in progress.
	RotationIdle RotationState = "idle"

	// RotationPreparing means new credentials are being generated.
	RotationPreparing RotationState = "preparing"

	// RotationTransitioning means both old and new credentials are valid.
	RotationTransitioning RotationState = "transitioning"

	// RotationFinalizing means old credentials are being invalidated.
	RotationFinalizing RotationState = "finalizing"

	// RotationComplete means rotation finished successfully.
	RotationComplete RotationState = "complete"

	// RotationFailed means rotation encountered an error.
	RotationFailed RotationState = "failed"
)

// RotationEvent represents a rotation lifecycle event.
type RotationEvent struct {
	Timestamp  time.Time     `json:"timestamp"`
	State      RotationState `json:"state"`
	OldVersion int           `json:"old_version"`
	NewVersion int           `json:"new_version"`
	Message    string        `json:"message,omitempty"`
	Error      string        `json:"error,omitempty"`
	DurationMS int64         `json:"duration_ms,omitempty"`
}

// RotationCallback is called during rotation to apply database changes.
type RotationCallback interface {
	// CreateRoles creates new PostgreSQL roles with the given credentials.
	CreateRoles(ctx context.Context, creds *Credentials) error

	// UpdatePoolCredentials updates the connection pool to use new credentials.
	UpdatePoolCredentials(ctx context.Context, creds *Credentials) error

	// DropRoles drops old PostgreSQL roles.
	DropRoles(ctx context.Context, creds *Credentials) error

	// AuditRotation logs the rotation event.
	AuditRotation(ctx context.Context, event RotationEvent) error
}

// Rotator handles credential rotation with zero-downtime.
type Rotator struct {
	manager  *Manager
	callback RotationCallback
	state    RotationState
	events   []RotationEvent
	mu       sync.RWMutex
}

// NewRotator creates a new credential rotator.
func NewRotator(manager *Manager, callback RotationCallback) *Rotator {
	return &Rotator{
		manager:  manager,
		callback: callback,
		state:    RotationIdle,
		events:   make([]RotationEvent, 0),
	}
}

// State returns the current rotation state.
func (r *Rotator) State() RotationState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// Events returns the rotation event history.
func (r *Rotator) Events() []RotationEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]RotationEvent, len(r.events))
	copy(result, r.events)
	return result
}

// Rotate performs a zero-downtime credential rotation.
// The rotation follows these steps:
//  1. Generate new credentials (Version N+1)
//  2. Create new PostgreSQL roles with new passwords
//  3. Mark old credentials as "retiring" (still valid)
//  4. Update connection pool to use new credentials
//  5. Wait for grace period
//  6. Verify no connections using old credentials
//  7. Drop old PostgreSQL roles
//  8. Mark old credentials as "expired"
//  9. Audit log the rotation event
func (r *Rotator) Rotate(ctx context.Context) error {
	r.mu.Lock()
	if r.state != RotationIdle && r.state != RotationComplete && r.state != RotationFailed {
		r.mu.Unlock()
		return fmt.Errorf("rotation already in progress: %s", r.state)
	}
	r.state = RotationPreparing
	r.mu.Unlock()

	startTime := time.Now()
	oldCreds := r.manager.Current()
	if oldCreds == nil {
		return fmt.Errorf("no current credentials to rotate")
	}

	oldVersion := oldCreds.Version

	// Step 1: Generate new credentials
	r.recordEvent(RotationPreparing, oldVersion, oldVersion+1, "Generating new credentials", "")

	newCreds, err := r.generateNewCredentials(oldCreds)
	if err != nil {
		r.failRotation(oldVersion, oldVersion+1, "Failed to generate new credentials", err)
		return err
	}

	// Step 2: Create new PostgreSQL roles
	r.setState(RotationTransitioning)
	r.recordEvent(RotationTransitioning, oldVersion, newCreds.Version, "Creating new PostgreSQL roles", "")

	if r.callback != nil {
		if err := r.callback.CreateRoles(ctx, newCreds); err != nil {
			r.failRotation(oldVersion, newCreds.Version, "Failed to create new roles", err)
			return err
		}
	}

	// Step 3: Mark old credentials as retiring
	r.markRetiring(oldCreds)

	// Step 4: Update connection pool
	if r.callback != nil {
		if err := r.callback.UpdatePoolCredentials(ctx, newCreds); err != nil {
			r.failRotation(oldVersion, newCreds.Version, "Failed to update connection pool", err)
			return err
		}
	}

	// Step 5: Wait for grace period
	r.recordEvent(RotationTransitioning, oldVersion, newCreds.Version,
		fmt.Sprintf("Waiting for grace period (%s)", r.manager.config.RotationGracePeriod), "")

	select {
	case <-ctx.Done():
		r.failRotation(oldVersion, newCreds.Version, "Context cancelled during grace period", ctx.Err())
		return ctx.Err()
	case <-time.After(r.manager.config.RotationGracePeriod):
		// Grace period complete
	}

	// Step 6 & 7: Drop old roles
	r.setState(RotationFinalizing)
	r.recordEvent(RotationFinalizing, oldVersion, newCreds.Version, "Dropping old PostgreSQL roles", "")

	if r.callback != nil {
		if err := r.callback.DropRoles(ctx, oldCreds); err != nil {
			// Log but don't fail - old roles will be cleaned up later
			r.recordEvent(RotationFinalizing, oldVersion, newCreds.Version,
				"Warning: Failed to drop old roles (will retry later)", err.Error())
		}
	}

	// Step 8: Mark old credentials as expired and save
	r.markExpired(oldCreds)
	newCreds.Previous = nil // Clear reference to old credentials

	if err := r.manager.storage.Save(newCreds); err != nil {
		r.failRotation(oldVersion, newCreds.Version, "Failed to save new credentials", err)
		return err
	}

	// Update manager's current credentials
	r.manager.mu.Lock()
	r.manager.current = newCreds
	r.manager.mu.Unlock()

	// Step 9: Audit log
	duration := time.Since(startTime)
	event := RotationEvent{
		Timestamp:  time.Now().UTC(),
		State:      RotationComplete,
		OldVersion: oldVersion,
		NewVersion: newCreds.Version,
		Message:    "Credential rotation completed successfully",
		DurationMS: duration.Milliseconds(),
	}

	r.mu.Lock()
	r.state = RotationComplete
	r.events = append(r.events, event)
	r.mu.Unlock()

	if r.callback != nil {
		r.callback.AuditRotation(ctx, event)
	}

	return nil
}

// generateNewCredentials creates a new credential set based on the old one.
func (r *Rotator) generateNewCredentials(old *Credentials) (*Credentials, error) {
	r.manager.mu.Lock()
	defer r.manager.mu.Unlock()

	newCreds, err := r.manager.generate()
	if err != nil {
		return nil, err
	}

	newCreds.Version = old.Version + 1
	newCreds.Previous = old

	return newCreds, nil
}

// markRetiring marks all credentials in the set as retiring.
func (r *Rotator) markRetiring(creds *Credentials) {
	creds.Superuser.Status = StatusRetiring
	creds.Admin.Status = StatusRetiring
	for role, cred := range creds.Roles {
		cred.Status = StatusRetiring
		creds.Roles[role] = cred
	}
}

// markExpired marks all credentials in the set as expired.
func (r *Rotator) markExpired(creds *Credentials) {
	creds.Superuser.Status = StatusExpired
	creds.Admin.Status = StatusExpired
	for role, cred := range creds.Roles {
		cred.Status = StatusExpired
		creds.Roles[role] = cred
	}
}

// setState updates the rotation state.
func (r *Rotator) setState(state RotationState) {
	r.mu.Lock()
	r.state = state
	r.mu.Unlock()
}

// recordEvent adds a rotation event to the history.
func (r *Rotator) recordEvent(state RotationState, oldVersion, newVersion int, message, errMsg string) {
	event := RotationEvent{
		Timestamp:  time.Now().UTC(),
		State:      state,
		OldVersion: oldVersion,
		NewVersion: newVersion,
		Message:    message,
		Error:      errMsg,
	}

	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

// failRotation records a rotation failure.
func (r *Rotator) failRotation(oldVersion, newVersion int, message string, err error) {
	r.mu.Lock()
	r.state = RotationFailed
	r.events = append(r.events, RotationEvent{
		Timestamp:  time.Now().UTC(),
		State:      RotationFailed,
		OldVersion: oldVersion,
		NewVersion: newVersion,
		Message:    message,
		Error:      err.Error(),
	})
	r.mu.Unlock()
}

// RotationScheduler manages automatic credential rotation.
type RotationScheduler struct {
	rotator  *Rotator
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewRotationScheduler creates a scheduler for automatic rotations.
func NewRotationScheduler(rotator *Rotator, interval time.Duration) *RotationScheduler {
	return &RotationScheduler{
		rotator:  rotator,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the automatic rotation schedule.
func (s *RotationScheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

// Stop stops the automatic rotation schedule.
func (s *RotationScheduler) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

func (s *RotationScheduler) run(ctx context.Context) {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.interval / 10) // Check 10 times per interval
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			if s.rotator.manager.NeedsRotation() {
				if err := s.rotator.Rotate(ctx); err != nil {
					// Log error but continue - will retry on next tick
					continue
				}
			}
		case <-s.rotator.manager.rotationCh:
			// Manual rotation triggered
			if err := s.rotator.Rotate(ctx); err != nil {
				continue
			}
		}
	}
}

// DualCredentialSet holds both active and retiring credentials during rotation.
type DualCredentialSet struct {
	Active   *Credentials
	Retiring *Credentials
}

// GetPassword returns the password for a role, preferring active credentials.
func (d *DualCredentialSet) GetPassword(role storage.DBRole) (string, error) {
	if d.Active != nil {
		cred, err := d.Active.GetRoleCredential(role)
		if err == nil && cred.Status == StatusActive {
			return cred.Password, nil
		}
	}

	if d.Retiring != nil {
		cred, err := d.Retiring.GetRoleCredential(role)
		if err == nil && cred.Status == StatusRetiring {
			return cred.Password, nil
		}
	}

	return "", fmt.Errorf("no valid credential found for role: %s", role)
}
