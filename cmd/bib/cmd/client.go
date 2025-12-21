// Package cmd provides CLI commands for the bib client.
package cmd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"bib/internal/config"
	client "bib/internal/grpc/client"
)

var (
	// daemonClient is the shared gRPC client for daemon communication.
	daemonClient *client.Client
	clientOnce   sync.Once
	clientErr    error

	// nodeFlag allows explicit node selection via --node flag
	nodeFlag string
)

// localOnlyCommands lists commands that don't need daemon connection.
var localOnlyCommands = map[string]bool{
	"setup":      true,
	"config":     true,
	"cert":       true,
	"version":    true,
	"help":       true,
	"completion": true,
}

// IsLocalOnlyCommand returns true if the command doesn't need daemon connection.
func IsLocalOnlyCommand(cmdName string) bool {
	return localOnlyCommands[cmdName]
}

// GetClient returns the shared daemon client, initializing it if needed.
// This provides lazy connection - the client is only created on first use.
func GetClient(ctx context.Context) (*client.Client, error) {
	clientOnce.Do(func() {
		daemonClient, clientErr = initClient(ctx)
	})
	return daemonClient, clientErr
}

// initClient initializes the daemon client from configuration.
func initClient(ctx context.Context) (*client.Client, error) {
	// Build client options from config
	opts, err := buildClientOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to build client options: %w", err)
	}

	// Create client
	c, err := client.New(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Connect
	if err := c.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	// Ensure authenticated
	if err := c.EnsureAuthenticated(ctx); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	return c, nil
}

// buildClientOptions builds client options from configuration.
func buildClientOptions() (client.Options, error) {
	opts := client.DefaultOptions()

	// Load config
	bibCfg, err := config.LoadBib("")
	if err != nil {
		// Config might not exist yet, use defaults
		return opts, nil
	}

	// Apply connection settings
	connCfg := bibCfg.Connection

	// Connection mode
	if connCfg.Mode == "parallel" {
		opts.Mode = client.ConnectionModeParallel
	} else {
		opts.Mode = client.ConnectionModeSequential
	}

	// Timeout
	if connCfg.Timeout != "" {
		timeout, err := time.ParseDuration(connCfg.Timeout)
		if err == nil {
			opts.Timeout = timeout
		}
	}

	// Retry
	if connCfg.RetryAttempts > 0 {
		opts.RetryAttempts = connCfg.RetryAttempts
	}

	// Pool size
	if connCfg.PoolSize > 0 {
		opts.PoolSize = connCfg.PoolSize
	}

	// TLS settings
	opts.TLS.InsecureSkipVerify = connCfg.TLS.SkipVerify
	opts.TLS.CAFile = connCfg.TLS.CAFile
	opts.TLS.CertFile = connCfg.TLS.CertFile
	opts.TLS.KeyFile = connCfg.TLS.KeyFile

	// Determine connection target
	target, err := resolveConnectionTarget(bibCfg)
	if err != nil {
		return opts, err
	}

	// Apply target
	if target.UnixSocket != "" {
		opts.UnixSocket = target.UnixSocket
	}
	if target.TCPAddress != "" {
		opts.TCPAddress = target.TCPAddress
	}
	if target.P2PPeerID != "" {
		opts.P2PPeerID = target.P2PPeerID
	}

	// SSH key settings from identity config
	if bibCfg.Identity.Key != "" {
		opts.Auth.SSHKeyPath = bibCfg.Identity.Key
	}

	return opts, nil
}

// connectionTarget holds resolved connection target info.
type connectionTarget struct {
	UnixSocket string
	TCPAddress string
	P2PPeerID  string
}

// resolveConnectionTarget determines the connection target from config and flags.
func resolveConnectionTarget(cfg *config.BibConfig) (connectionTarget, error) {
	var target connectionTarget

	// 1. Explicit --node flag takes precedence
	if nodeFlag != "" {
		target.TCPAddress = nodeFlag
		return target, nil
	}

	// 2. Default node from config
	if cfg.Connection.DefaultNode != "" {
		// Look up in favorite nodes
		for _, node := range cfg.Connection.FavoriteNodes {
			if node.Alias == cfg.Connection.DefaultNode || node.ID == cfg.Connection.DefaultNode {
				if node.UnixSocket != "" {
					target.UnixSocket = node.UnixSocket
				}
				if node.Address != "" {
					target.TCPAddress = node.Address
				}
				if node.ID != "" {
					target.P2PPeerID = node.ID
				}
				return target, nil
			}
		}
		// Treat as direct address
		target.TCPAddress = cfg.Connection.DefaultNode
		return target, nil
	}

	// 3. First favorite node
	if len(cfg.Connection.FavoriteNodes) > 0 {
		node := cfg.Connection.FavoriteNodes[0]
		if node.UnixSocket != "" {
			target.UnixSocket = node.UnixSocket
		}
		if node.Address != "" {
			target.TCPAddress = node.Address
		}
		if node.ID != "" {
			target.P2PPeerID = node.ID
		}
		return target, nil
	}

	// 4. Auto-detect via mDNS if enabled
	if cfg.Connection.AutoDetect {
		// TODO: Implement mDNS discovery
		// For now, try localhost
		target.TCPAddress = "localhost:4000"
		return target, nil
	}

	// 6. Default to localhost
	target.TCPAddress = "localhost:4000"
	return target, nil
}

// CloseClient closes the shared client connection.
func CloseClient() error {
	if daemonClient != nil {
		return daemonClient.Close()
	}
	return nil
}

// ResetClient resets the client so the next GetClient call will reinitialize.
func ResetClient() {
	if daemonClient != nil {
		_ = daemonClient.Close()
	}
	daemonClient = nil
	clientOnce = sync.Once{}
	clientErr = nil
}

// GetNodeFlag returns the --node flag for adding to commands.
func GetNodeFlag() *string {
	return &nodeFlag
}
