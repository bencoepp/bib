package domain

import (
	"time"
)

// ScheduleType represents the type of schedule.
type ScheduleType string

const (
	// ScheduleOnce runs the job exactly once.
	ScheduleOnce ScheduleType = "once"

	// ScheduleCron runs the job on a cron schedule.
	ScheduleCron ScheduleType = "cron"

	// ScheduleRepeat runs the job a specific number of times.
	ScheduleRepeat ScheduleType = "repeat"

	// ScheduleInterval runs the job at fixed intervals.
	ScheduleInterval ScheduleType = "interval"
)

// IsValid checks if the schedule type is valid.
func (s ScheduleType) IsValid() bool {
	switch s {
	case ScheduleOnce, ScheduleCron, ScheduleRepeat, ScheduleInterval:
		return true
	default:
		return false
	}
}

// Schedule defines when and how often a job runs.
type Schedule struct {
	// Type is the schedule type.
	Type ScheduleType `json:"type"`

	// CronExpr is the cron expression (for ScheduleCron).
	// Standard 5-field cron: minute hour day-of-month month day-of-week
	CronExpr string `json:"cron_expr,omitempty"`

	// RepeatCount is the number of times to repeat (for ScheduleRepeat).
	// 0 means infinite (use with caution).
	RepeatCount int `json:"repeat_count,omitempty"`

	// Interval is the duration between runs (for ScheduleInterval).
	Interval time.Duration `json:"interval,omitempty"`

	// StartAt is when the schedule becomes active.
	StartAt *time.Time `json:"start_at,omitempty"`

	// EndAt is when the schedule expires (optional).
	EndAt *time.Time `json:"end_at,omitempty"`

	// Timezone is the timezone for cron expressions.
	Timezone string `json:"timezone,omitempty"`

	// RunCount is the current number of completed runs.
	RunCount int `json:"run_count"`

	// NextRunAt is the calculated next run time.
	NextRunAt *time.Time `json:"next_run_at,omitempty"`

	// LastRunAt is when the job last ran.
	LastRunAt *time.Time `json:"last_run_at,omitempty"`

	// Enabled indicates if the schedule is active.
	Enabled bool `json:"enabled"`
}

// Validate validates the schedule.
func (s *Schedule) Validate() error {
	if !s.Type.IsValid() {
		return ErrInvalidScheduleType
	}

	switch s.Type {
	case ScheduleCron:
		if s.CronExpr == "" {
			return ErrInvalidCronExpr
		}
	case ScheduleRepeat:
		if s.RepeatCount < 0 {
			return ErrInvalidRepeatCount
		}
	case ScheduleInterval:
		if s.Interval <= 0 {
			return ErrInvalidInterval
		}
	}

	// Validate time range
	if s.StartAt != nil && s.EndAt != nil && s.EndAt.Before(*s.StartAt) {
		return ErrInvalidTimeRange
	}

	return nil
}

// IsExpired checks if the schedule has expired.
func (s *Schedule) IsExpired() bool {
	if s.EndAt != nil && time.Now().After(*s.EndAt) {
		return true
	}
	if s.Type == ScheduleRepeat && s.RepeatCount > 0 && s.RunCount >= s.RepeatCount {
		return true
	}
	if s.Type == ScheduleOnce && s.RunCount > 0 {
		return true
	}
	return false
}

// ShouldRun checks if the schedule should run now.
func (s *Schedule) ShouldRun() bool {
	if !s.Enabled {
		return false
	}
	if s.IsExpired() {
		return false
	}
	if s.StartAt != nil && time.Now().Before(*s.StartAt) {
		return false
	}
	if s.NextRunAt != nil && time.Now().Before(*s.NextRunAt) {
		return false
	}
	return true
}

// IncrementRun updates the schedule after a run completes.
func (s *Schedule) IncrementRun() {
	now := time.Now()
	s.RunCount++
	s.LastRunAt = &now

	// Calculate next run time based on type
	switch s.Type {
	case ScheduleOnce:
		s.NextRunAt = nil
		s.Enabled = false
	case ScheduleRepeat:
		if s.RepeatCount > 0 && s.RunCount >= s.RepeatCount {
			s.NextRunAt = nil
			s.Enabled = false
		}
	case ScheduleInterval:
		next := now.Add(s.Interval)
		s.NextRunAt = &next
	case ScheduleCron:
		// Cron next time calculation would be done by a cron parser
		// This is a placeholder - actual implementation needs a cron library
	}
}
