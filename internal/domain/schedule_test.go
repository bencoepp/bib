package domain

import (
	"testing"
	"time"
)

func TestScheduleType_IsValid(t *testing.T) {
	tests := []struct {
		scheduleType ScheduleType
		valid        bool
	}{
		{ScheduleOnce, true},
		{ScheduleCron, true},
		{ScheduleRepeat, true},
		{ScheduleInterval, true},
		{ScheduleType("unknown"), false},
		{ScheduleType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.scheduleType), func(t *testing.T) {
			if got := tt.scheduleType.IsValid(); got != tt.valid {
				t.Errorf("ScheduleType(%q).IsValid() = %v, want %v", tt.scheduleType, got, tt.valid)
			}
		})
	}
}

func TestSchedule_Validate(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name     string
		schedule *Schedule
		wantErr  error
	}{
		{
			name: "valid once schedule",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				Enabled: true,
			},
			wantErr: nil,
		},
		{
			name: "valid cron schedule",
			schedule: &Schedule{
				Type:     ScheduleCron,
				CronExpr: "0 * * * *",
				Enabled:  true,
			},
			wantErr: nil,
		},
		{
			name: "cron without expression",
			schedule: &Schedule{
				Type:    ScheduleCron,
				Enabled: true,
			},
			wantErr: ErrInvalidCronExpr,
		},
		{
			name: "valid repeat schedule",
			schedule: &Schedule{
				Type:        ScheduleRepeat,
				RepeatCount: 5,
				Enabled:     true,
			},
			wantErr: nil,
		},
		{
			name: "repeat with negative count",
			schedule: &Schedule{
				Type:        ScheduleRepeat,
				RepeatCount: -1,
				Enabled:     true,
			},
			wantErr: ErrInvalidRepeatCount,
		},
		{
			name: "valid interval schedule",
			schedule: &Schedule{
				Type:     ScheduleInterval,
				Interval: time.Hour,
				Enabled:  true,
			},
			wantErr: nil,
		},
		{
			name: "interval with zero duration",
			schedule: &Schedule{
				Type:     ScheduleInterval,
				Interval: 0,
				Enabled:  true,
			},
			wantErr: ErrInvalidInterval,
		},
		{
			name: "interval with negative duration",
			schedule: &Schedule{
				Type:     ScheduleInterval,
				Interval: -time.Hour,
				Enabled:  true,
			},
			wantErr: ErrInvalidInterval,
		},
		{
			name: "invalid schedule type",
			schedule: &Schedule{
				Type:    ScheduleType("invalid"),
				Enabled: true,
			},
			wantErr: ErrInvalidScheduleType,
		},
		{
			name: "end time before start time",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				StartAt: &future,
				EndAt:   &past,
				Enabled: true,
			},
			wantErr: ErrInvalidTimeRange,
		},
		{
			name: "valid time range",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				StartAt: &past,
				EndAt:   &future,
				Enabled: true,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schedule.Validate()
			if err != tt.wantErr {
				t.Errorf("Schedule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchedule_IsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name     string
		schedule *Schedule
		expired  bool
	}{
		{
			name: "not expired - no end time",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				Enabled: true,
			},
			expired: false,
		},
		{
			name: "expired - end time passed",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				EndAt:   &past,
				Enabled: true,
			},
			expired: true,
		},
		{
			name: "not expired - end time in future",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				EndAt:   &future,
				Enabled: true,
			},
			expired: false,
		},
		{
			name: "expired - repeat count reached",
			schedule: &Schedule{
				Type:        ScheduleRepeat,
				RepeatCount: 3,
				RunCount:    3,
				Enabled:     true,
			},
			expired: true,
		},
		{
			name: "not expired - repeat count not reached",
			schedule: &Schedule{
				Type:        ScheduleRepeat,
				RepeatCount: 3,
				RunCount:    2,
				Enabled:     true,
			},
			expired: false,
		},
		{
			name: "expired - once schedule already run",
			schedule: &Schedule{
				Type:     ScheduleOnce,
				RunCount: 1,
				Enabled:  true,
			},
			expired: true,
		},
		{
			name: "not expired - once schedule not run",
			schedule: &Schedule{
				Type:     ScheduleOnce,
				RunCount: 0,
				Enabled:  true,
			},
			expired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.schedule.IsExpired(); got != tt.expired {
				t.Errorf("Schedule.IsExpired() = %v, want %v", got, tt.expired)
			}
		})
	}
}

func TestSchedule_ShouldRun(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name      string
		schedule  *Schedule
		shouldRun bool
	}{
		{
			name: "should run - enabled and not expired",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				Enabled: true,
			},
			shouldRun: true,
		},
		{
			name: "should not run - disabled",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				Enabled: false,
			},
			shouldRun: false,
		},
		{
			name: "should not run - expired",
			schedule: &Schedule{
				Type:     ScheduleOnce,
				RunCount: 1,
				Enabled:  true,
			},
			shouldRun: false,
		},
		{
			name: "should not run - start time in future",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				StartAt: &future,
				Enabled: true,
			},
			shouldRun: false,
		},
		{
			name: "should run - start time passed",
			schedule: &Schedule{
				Type:    ScheduleOnce,
				StartAt: &past,
				Enabled: true,
			},
			shouldRun: true,
		},
		{
			name: "should not run - next run time in future",
			schedule: &Schedule{
				Type:      ScheduleInterval,
				NextRunAt: &future,
				Enabled:   true,
			},
			shouldRun: false,
		},
		{
			name: "should run - next run time passed",
			schedule: &Schedule{
				Type:      ScheduleInterval,
				NextRunAt: &past,
				Enabled:   true,
			},
			shouldRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.schedule.ShouldRun(); got != tt.shouldRun {
				t.Errorf("Schedule.ShouldRun() = %v, want %v", got, tt.shouldRun)
			}
		})
	}
}

func TestSchedule_IncrementRun(t *testing.T) {
	t.Run("once schedule", func(t *testing.T) {
		schedule := &Schedule{
			Type:     ScheduleOnce,
			Enabled:  true,
			RunCount: 0,
		}

		schedule.IncrementRun()

		if schedule.RunCount != 1 {
			t.Errorf("expected RunCount 1, got %d", schedule.RunCount)
		}
		if schedule.LastRunAt == nil {
			t.Error("expected LastRunAt to be set")
		}
		if schedule.Enabled {
			t.Error("expected Enabled to be false for once schedule")
		}
		if schedule.NextRunAt != nil {
			t.Error("expected NextRunAt to be nil for once schedule")
		}
	})

	t.Run("repeat schedule - not finished", func(t *testing.T) {
		schedule := &Schedule{
			Type:        ScheduleRepeat,
			RepeatCount: 3,
			RunCount:    1,
			Enabled:     true,
		}

		schedule.IncrementRun()

		if schedule.RunCount != 2 {
			t.Errorf("expected RunCount 2, got %d", schedule.RunCount)
		}
		if !schedule.Enabled {
			t.Error("expected schedule to still be enabled")
		}
	})

	t.Run("repeat schedule - finished", func(t *testing.T) {
		schedule := &Schedule{
			Type:        ScheduleRepeat,
			RepeatCount: 3,
			RunCount:    2,
			Enabled:     true,
		}

		schedule.IncrementRun()

		if schedule.RunCount != 3 {
			t.Errorf("expected RunCount 3, got %d", schedule.RunCount)
		}
		if schedule.Enabled {
			t.Error("expected schedule to be disabled")
		}
	})

	t.Run("interval schedule", func(t *testing.T) {
		schedule := &Schedule{
			Type:     ScheduleInterval,
			Interval: time.Hour,
			RunCount: 0,
			Enabled:  true,
		}
		before := time.Now()

		schedule.IncrementRun()

		if schedule.RunCount != 1 {
			t.Errorf("expected RunCount 1, got %d", schedule.RunCount)
		}
		if schedule.NextRunAt == nil {
			t.Fatal("expected NextRunAt to be set")
		}
		expectedNext := before.Add(time.Hour)
		if schedule.NextRunAt.Before(expectedNext.Add(-time.Second)) || schedule.NextRunAt.After(expectedNext.Add(time.Second)) {
			t.Errorf("expected NextRunAt around %v, got %v", expectedNext, *schedule.NextRunAt)
		}
	})
}
