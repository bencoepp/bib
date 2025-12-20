// Package connect provides the bib connect command for daemon connection management.
package connect

import (
	"context"
	"fmt"
	"os"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/config"
	client "bib/internal/grpc/client"

	"github.com/spf13/cobra"
)

var (
	// Flags
	saveFlag    bool
	aliasFlag   string
	testOnly    bool
	timeoutFlag time.Duration
)

// NewCommand creates the connect command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect [address]",
		Short: "Connect to a bibd daemon",
		Long: `Connect to a bibd daemon and optionally save it as the default.

This command tests the connection to a bibd daemon, authenticates using
your SSH key, and optionally saves the connection as your default node.

Examples:
  # Connect to local daemon
  bib connect localhost:4000

  # Connect and save as default
  bib connect --save node1.example.com:4000

  # Connect with alias
  bib connect --save --alias mynode node1.example.com:4000

  # Test connection only (no auth)
  bib connect --test node1.example.com:4000`,
		Args: cobra.MaximumNArgs(1),
		RunE: runConnect,
	}

	cmd.Flags().BoolVar(&saveFlag, "save", false, "Save as default node in config")
	cmd.Flags().StringVar(&aliasFlag, "alias", "", "Alias for the node (used with --save)")
	cmd.Flags().BoolVar(&testOnly, "test", false, "Test connection only, don't authenticate")
	cmd.Flags().DurationVar(&timeoutFlag, "timeout", 10*time.Second, "Connection timeout")

	return cmd
}

func runConnect(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Determine target address
	var address string
	if len(args) > 0 {
		address = args[0]
	} else {
		// Try to load from config
		cfg, err := config.LoadBib("")
		if err == nil && cfg.Connection.DefaultNode != "" {
			address = cfg.Connection.DefaultNode
		} else if err == nil && cfg.Server != "" {
			address = cfg.Server
		} else {
			address = "localhost:4000"
		}
	}

	fmt.Printf("Connecting to %s...\n", address)

	// Build client options
	opts := client.DefaultOptions().
		WithTCPAddress(address).
		WithTimeout(timeoutFlag)

	// Create client
	c, err := client.New(opts)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer c.Close()

	// Connect
	connectCtx, cancel := context.WithTimeout(ctx, timeoutFlag)
	defer cancel()

	if err := c.Connect(connectCtx); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	fmt.Printf("✓ Connected to %s\n", c.ConnectedTo())

	// Test health
	healthClient, err := c.Health()
	if err != nil {
		return fmt.Errorf("failed to get health client: %w", err)
	}

	healthResp, err := healthClient.Check(ctx, &services.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	fmt.Printf("✓ Health: %s\n", healthResp.Status)

	// Get node info
	nodeInfo, err := healthClient.GetNodeInfo(ctx, &services.GetNodeInfoRequest{})
	if err == nil {
		fmt.Printf("  Node ID: %s\n", nodeInfo.NodeId)
		fmt.Printf("  Version: %s\n", nodeInfo.Version)
		fmt.Printf("  Mode: %s\n", nodeInfo.Mode)
		fmt.Printf("  Uptime: %s\n", nodeInfo.Uptime.AsDuration())
	}

	// Authenticate unless test-only
	if !testOnly {
		fmt.Println("\nAuthenticating...")

		if err := c.Authenticate(ctx); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		fmt.Println("✓ Authenticated successfully")

		// Show public key fingerprint used
		pubKey, err := c.GetPublicKey()
		if err == nil {
			// Truncate for display
			if len(pubKey) > 50 {
				pubKey = pubKey[:47] + "..."
			}
			fmt.Printf("  Key: %s\n", pubKey)
		}
	}

	// Save to config if requested
	if saveFlag {
		if err := saveConnection(address, aliasFlag); err != nil {
			return fmt.Errorf("failed to save connection: %w", err)
		}
		fmt.Printf("\n✓ Saved as default node")
		if aliasFlag != "" {
			fmt.Printf(" (alias: %s)", aliasFlag)
		}
		fmt.Println()
	}

	return nil
}

// saveConnection saves the connection to config.
func saveConnection(address, alias string) error {
	// Load existing config
	cfg, err := config.LoadBib("")
	if err != nil {
		// Create new config
		cfg = &config.BibConfig{}
	}

	// Set default node
	if alias != "" {
		cfg.Connection.DefaultNode = alias
	} else {
		cfg.Connection.DefaultNode = address
	}

	// Add to favorite nodes if alias provided
	if alias != "" {
		// Check if alias already exists
		found := false
		for i, node := range cfg.Connection.FavoriteNodes {
			if node.Alias == alias {
				cfg.Connection.FavoriteNodes[i].Address = address
				found = true
				break
			}
		}
		if !found {
			cfg.Connection.FavoriteNodes = append(cfg.Connection.FavoriteNodes, config.FavoriteNode{
				Alias:    alias,
				Address:  address,
				Priority: len(cfg.Connection.FavoriteNodes),
			})
		}
	}

	// Determine config path
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}
	configPath := configDir + "/bib/config.yaml"

	// Save config
	return config.SaveBib(cfg, configPath)
}
