package pool

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxConns != 25 {
		t.Errorf("expected MaxConns 25, got %d", cfg.MaxConns)
	}
	if cfg.MinConns != 5 {
		t.Errorf("expected MinConns 5, got %d", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != time.Hour {
		t.Errorf("expected MaxConnLifetime 1h, got %v", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 30*time.Minute {
		t.Errorf("expected MaxConnIdleTime 30m, got %v", cfg.MaxConnIdleTime)
	}
	if cfg.HealthCheckPeriod != time.Minute {
		t.Errorf("expected HealthCheckPeriod 1m, got %v", cfg.HealthCheckPeriod)
	}
	if cfg.ConnectTimeout != 5*time.Second {
		t.Errorf("expected ConnectTimeout 5s, got %v", cfg.ConnectTimeout)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		message string
	}{
		{"ErrPoolClosed", ErrPoolClosed, "connection pool is closed"},
		{"ErrInvalidRole", ErrInvalidRole, "invalid database role"},
		{"ErrNoOperationContext", ErrNoOperationContext, "OperationContext required for database operations"},
		{"ErrRoleSwitchFailed", ErrRoleSwitchFailed, "failed to switch database role"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s is nil", tt.name)
			}
			if tt.err.Error() != tt.message {
				t.Errorf("expected %q, got %q", tt.message, tt.err.Error())
			}
		})
	}
}

func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		MaxConns:          50,
		MinConns:          10,
		MaxConnLifetime:   2 * time.Hour,
		MaxConnIdleTime:   15 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    10 * time.Second,
	}

	if cfg.MaxConns != 50 {
		t.Errorf("MaxConns mismatch")
	}
	if cfg.MinConns != 10 {
		t.Errorf("MinConns mismatch")
	}
	if cfg.MaxConnLifetime != 2*time.Hour {
		t.Errorf("MaxConnLifetime mismatch")
	}
	if cfg.MaxConnIdleTime != 15*time.Minute {
		t.Errorf("MaxConnIdleTime mismatch")
	}
	if cfg.HealthCheckPeriod != 30*time.Second {
		t.Errorf("HealthCheckPeriod mismatch")
	}
	if cfg.ConnectTimeout != 10*time.Second {
		t.Errorf("ConnectTimeout mismatch")
	}
}
