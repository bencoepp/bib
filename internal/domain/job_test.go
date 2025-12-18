package domain

import (
	"testing"
	"time"
)

func TestJobID_String(t *testing.T) {
	id := JobID("job-123")
	if id.String() != "job-123" {
		t.Errorf("expected 'job-123', got %q", id.String())
	}
}

func TestJobType_IsValid(t *testing.T) {
	tests := []struct {
		jobType JobType
		valid   bool
	}{
		{JobTypeScrape, true},
		{JobTypeTransform, true},
		{JobTypeClean, true},
		{JobTypeAnalyze, true},
		{JobTypeML, true},
		{JobTypeETL, true},
		{JobTypeIngest, true},
		{JobTypeExport, true},
		{JobTypeCustom, true},
		{JobType("unknown"), false},
		{JobType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.jobType), func(t *testing.T) {
			if got := tt.jobType.IsValid(); got != tt.valid {
				t.Errorf("JobType(%q).IsValid() = %v, want %v", tt.jobType, got, tt.valid)
			}
		})
	}
}

func TestJobStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		status   JobStatus
		terminal bool
	}{
		{JobStatusPending, false},
		{JobStatusQueued, false},
		{JobStatusRunning, false},
		{JobStatusWaiting, false},
		{JobStatusRetrying, false},
		{JobStatusCompleted, true},
		{JobStatusFailed, true},
		{JobStatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.terminal {
				t.Errorf("JobStatus(%q).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
			}
		})
	}
}

func TestExecutionMode_IsValid(t *testing.T) {
	tests := []struct {
		mode  ExecutionMode
		valid bool
	}{
		{ExecutionModeGoroutine, true},
		{ExecutionModeContainer, true},
		{ExecutionModePod, true},
		{ExecutionMode("unknown"), false},
		{ExecutionMode(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			if got := tt.mode.IsValid(); got != tt.valid {
				t.Errorf("ExecutionMode(%q).IsValid() = %v, want %v", tt.mode, got, tt.valid)
			}
		})
	}
}

func TestJob_Validate(t *testing.T) {
	tests := []struct {
		name    string
		job     *Job
		wantErr error
	}{
		{
			name: "valid job with task ID",
			job: &Job{
				ID:            "job-1",
				Type:          JobTypeScrape,
				TaskID:        "task-1",
				ExecutionMode: ExecutionModeGoroutine,
			},
			wantErr: nil,
		},
		{
			name: "valid job with inline instructions",
			job: &Job{
				ID:   "job-1",
				Type: JobTypeScrape,
				InlineInstructions: []Instruction{
					{Operation: OpHTTPGet},
				},
				ExecutionMode: ExecutionModeGoroutine,
			},
			wantErr: nil,
		},
		{
			name: "empty ID",
			job: &Job{
				ID:     "",
				Type:   JobTypeScrape,
				TaskID: "task-1",
			},
			wantErr: ErrInvalidJobID,
		},
		{
			name: "invalid job type",
			job: &Job{
				ID:     "job-1",
				Type:   JobType("invalid"),
				TaskID: "task-1",
			},
			wantErr: ErrInvalidJobType,
		},
		{
			name: "no task or instructions",
			job: &Job{
				ID:   "job-1",
				Type: JobTypeScrape,
			},
			wantErr: ErrNoTaskOrInstructions,
		},
		{
			name: "invalid execution mode",
			job: &Job{
				ID:            "job-1",
				Type:          JobTypeScrape,
				TaskID:        "task-1",
				ExecutionMode: ExecutionMode("invalid"),
			},
			wantErr: ErrInvalidExecutionMode,
		},
		{
			name: "empty execution mode is valid",
			job: &Job{
				ID:            "job-1",
				Type:          JobTypeScrape,
				TaskID:        "task-1",
				ExecutionMode: "",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if err != tt.wantErr {
				t.Errorf("Job.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJob_IsComplete(t *testing.T) {
	tests := []struct {
		status   JobStatus
		complete bool
	}{
		{JobStatusPending, false},
		{JobStatusRunning, false},
		{JobStatusCompleted, true},
		{JobStatusFailed, true},
		{JobStatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			job := &Job{Status: tt.status}
			if got := job.IsComplete(); got != tt.complete {
				t.Errorf("Job.IsComplete() = %v, want %v", got, tt.complete)
			}
		})
	}
}

func TestJob_CanRun(t *testing.T) {
	completedJobs := map[JobID]bool{
		"dep-1": true,
		"dep-2": true,
	}

	tests := []struct {
		name         string
		job          *Job
		completedMap map[JobID]bool
		canRun       bool
	}{
		{
			name: "pending with all deps completed",
			job: &Job{
				Status:       JobStatusPending,
				Dependencies: []JobID{"dep-1", "dep-2"},
			},
			completedMap: completedJobs,
			canRun:       true,
		},
		{
			name: "pending with missing dep",
			job: &Job{
				Status:       JobStatusPending,
				Dependencies: []JobID{"dep-1", "dep-3"},
			},
			completedMap: completedJobs,
			canRun:       false,
		},
		{
			name: "queued with all deps",
			job: &Job{
				Status:       JobStatusQueued,
				Dependencies: []JobID{"dep-1"},
			},
			completedMap: completedJobs,
			canRun:       true,
		},
		{
			name: "waiting with all deps",
			job: &Job{
				Status:       JobStatusWaiting,
				Dependencies: []JobID{},
			},
			completedMap: completedJobs,
			canRun:       true,
		},
		{
			name: "running (can't run again)",
			job: &Job{
				Status:       JobStatusRunning,
				Dependencies: []JobID{},
			},
			completedMap: completedJobs,
			canRun:       false,
		},
		{
			name: "completed (can't run again)",
			job: &Job{
				Status:       JobStatusCompleted,
				Dependencies: []JobID{},
			},
			completedMap: completedJobs,
			canRun:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.job.CanRun(tt.completedMap); got != tt.canRun {
				t.Errorf("Job.CanRun() = %v, want %v", got, tt.canRun)
			}
		})
	}
}

func TestJob_Duration(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-time.Hour)
	completedAt := now.Add(-30 * time.Minute)

	t.Run("not started", func(t *testing.T) {
		job := &Job{
			StartedAt: nil,
		}
		if job.Duration() != 0 {
			t.Error("expected 0 duration for not started job")
		}
	})

	t.Run("completed", func(t *testing.T) {
		job := &Job{
			StartedAt:   &startedAt,
			CompletedAt: &completedAt,
		}
		duration := job.Duration()
		expected := 30 * time.Minute
		if duration != expected {
			t.Errorf("expected %v, got %v", expected, duration)
		}
	})

	t.Run("running", func(t *testing.T) {
		job := &Job{
			StartedAt:   &startedAt,
			CompletedAt: nil,
		}
		duration := job.Duration()
		if duration < time.Hour {
			t.Errorf("expected duration >= 1 hour for running job, got %v", duration)
		}
	})
}
