package client

import (
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Mode != ConnectionModeSequential {
		t.Errorf("expected sequential mode, got %s", opts.Mode)
	}

	if opts.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", opts.Timeout)
	}

	if opts.RetryAttempts != 3 {
		t.Errorf("expected 3 retry attempts, got %d", opts.RetryAttempts)
	}

	if opts.PoolSize != 5 {
		t.Errorf("expected pool size 5, got %d", opts.PoolSize)
	}

	if !opts.TLS.Enabled {
		t.Error("expected TLS enabled by default")
	}

	if !opts.Auth.UseSSHAgent {
		t.Error("expected SSH agent enabled by default")
	}

	if !opts.Auth.AutoAuth {
		t.Error("expected auto auth enabled by default")
	}
}

func TestOptions_WithBuilders(t *testing.T) {
	opts := DefaultOptions().
		WithUnixSocket("/var/run/bib.sock").
		WithTCPAddress("localhost:4000").
		WithP2PPeerID("QmTest123").
		WithMode(ConnectionModeParallel).
		WithTimeout(5*time.Second).
		WithRetry(5, 2*time.Second).
		WithPoolSize(10)

	if opts.UnixSocket != "/var/run/bib.sock" {
		t.Errorf("unexpected unix socket: %s", opts.UnixSocket)
	}

	if opts.TCPAddress != "localhost:4000" {
		t.Errorf("unexpected tcp address: %s", opts.TCPAddress)
	}

	if opts.P2PPeerID != "QmTest123" {
		t.Errorf("unexpected peer ID: %s", opts.P2PPeerID)
	}

	if opts.Mode != ConnectionModeParallel {
		t.Errorf("unexpected mode: %s", opts.Mode)
	}

	if opts.Timeout != 5*time.Second {
		t.Errorf("unexpected timeout: %v", opts.Timeout)
	}

	if opts.RetryAttempts != 5 {
		t.Errorf("unexpected retry attempts: %d", opts.RetryAttempts)
	}

	if opts.RetryBackoff != 2*time.Second {
		t.Errorf("unexpected retry backoff: %v", opts.RetryBackoff)
	}

	if opts.PoolSize != 10 {
		t.Errorf("unexpected pool size: %d", opts.PoolSize)
	}
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
	}{
		{
			name:    "no targets",
			opts:    Options{Timeout: 30 * time.Second},
			wantErr: true,
		},
		{
			name: "valid with unix socket",
			opts: Options{
				UnixSocket: "/var/run/bib.sock",
				Timeout:    30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid with tcp",
			opts: Options{
				TCPAddress: "localhost:4000",
				Timeout:    30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid with p2p",
			opts: Options{
				P2PPeerID: "QmTest",
				Timeout:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "zero timeout",
			opts: Options{
				TCPAddress: "localhost:4000",
				Timeout:    0,
			},
			wantErr: true,
		},
		{
			name: "negative retry",
			opts: Options{
				TCPAddress:    "localhost:4000",
				Timeout:       30 * time.Second,
				RetryAttempts: -1,
			},
			wantErr: true,
		},
		{
			name: "negative pool size",
			opts: Options{
				TCPAddress: "localhost:4000",
				Timeout:    30 * time.Second,
				PoolSize:   -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOptions_WithInsecure(t *testing.T) {
	opts := DefaultOptions().WithInsecure()

	if opts.TLS.Enabled {
		t.Error("expected TLS disabled after WithInsecure")
	}
}

func TestTLSOptions_BuildTLSConfig_Disabled(t *testing.T) {
	tlsOpts := TLSOptions{Enabled: false}

	cfg, err := tlsOpts.BuildTLSConfig()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil TLS config when disabled")
	}
}

func TestTLSOptions_BuildTLSConfig_InsecureSkipVerify(t *testing.T) {
	tlsOpts := TLSOptions{
		Enabled:            true,
		InsecureSkipVerify: true,
	}

	cfg, err := tlsOpts.BuildTLSConfig()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil TLS config")
	}
	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true")
	}
}

func TestIsLocalTarget(t *testing.T) {
	tests := []struct {
		target   string
		expected bool
	}{
		{"unix:/var/run/bib.sock", true},
		{"pipe:\\\\.\\pipe\\bib", true},
		{"localhost:4000", false},
		{"192.168.1.1:4000", false},
		{"p2p:QmTest", false},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			result := isLocalTarget(tt.target)
			if result != tt.expected {
				t.Errorf("isLocalTarget(%s) = %v, expected %v", tt.target, result, tt.expected)
			}
		})
	}
}
