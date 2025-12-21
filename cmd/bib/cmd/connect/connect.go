// Package connect provides the bib connect command for daemon connection management.
package connect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/certs"
	"bib/internal/config"
	client "bib/internal/grpc/client"

	"github.com/spf13/cobra"
)

var (
	// Flags
	saveFlag       bool
	aliasFlag      string
	testOnly       bool
	timeoutFlag    time.Duration
	trustFirstUse  bool
	skipTrustCheck bool
)

// NewCommand creates the connect command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect [address]",
		Short: "Connect to a bibd daemon",
		Long: `Connect to a bibd daemon and optionally save it as the default.

This command tests the connection to a bibd daemon, authenticates using
your SSH key, and optionally saves the connection as your default node.

TRUST ON FIRST USE (TOFU):
When connecting to a new node for the first time, you will be prompted
to verify and trust the server's certificate. This protects against
man-in-the-middle attacks.

Use --trust-first-use to automatically trust new certificates (less secure).

Examples:
  # Connect to local daemon
  bib connect localhost:4000

  # Connect and save as default
  bib connect --save node1.example.com:4000

  # Connect with alias
  bib connect --save --alias mynode node1.example.com:4000

  # Test connection only (no auth)
  bib connect --test node1.example.com:4000

  # Auto-trust new certificate (use with caution!)
  bib connect --trust-first-use node1.example.com:4000`,
		Args: cobra.MaximumNArgs(1),
		RunE: runConnect,
	}

	cmd.Flags().BoolVar(&saveFlag, "save", false, "Save as default node in config")
	cmd.Flags().StringVar(&aliasFlag, "alias", "", "Alias for the node (used with --save)")
	cmd.Flags().BoolVar(&testOnly, "test", false, "Test connection only, don't authenticate")
	cmd.Flags().DurationVar(&timeoutFlag, "timeout", 10*time.Second, "Connection timeout")
	cmd.Flags().BoolVar(&trustFirstUse, "trust-first-use", false, "Automatically trust new certificates (less secure)")
	cmd.Flags().BoolVar(&skipTrustCheck, "insecure-skip-verify", false, "Skip TLS certificate verification entirely (dangerous!)")

	// Mark dangerous flags
	cmd.Flags().MarkHidden("insecure-skip-verify")

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
		if err == nil && cfg.GetDefaultServerAddress() != "" {
			address = cfg.GetDefaultServerAddress()
		} else {
			address = "localhost:4000"
		}
	}

	fmt.Printf("Connecting to %s...\n", address)

	// Set up trust store for TOFU
	configDir, err := config.UserConfigDir(config.AppBib)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	trustDir := filepath.Join(configDir, "trusted_nodes")
	trustStore, err := certs.NewTrustStore(trustDir)
	if err != nil {
		return fmt.Errorf("failed to open trust store: %w", err)
	}

	// Create TOFU verifier
	tofuVerifier := certs.NewTOFUVerifier(trustStore).WithAutoTrust(trustFirstUse)

	// Build client options
	opts := client.DefaultOptions().
		WithTCPAddress(address).
		WithTimeout(timeoutFlag)

	// Configure TLS options
	if skipTrustCheck {
		fmt.Println("⚠️  WARNING: Skipping TLS verification (insecure)")
		opts = opts.WithInsecureSkipVerify(true)
	}

	// Set TOFU callback for certificate verification
	opts = opts.WithTOFUCallback(func(nodeID string, certPEM []byte) (bool, error) {
		result, err := tofuVerifier.Verify(nodeID, address, certPEM)
		if err != nil {
			if result != nil && result.FingerprintMismatch {
				existingNode, _ := trustStore.Get(nodeID)
				if existingNode != nil {
					info, _ := certs.ParseCertInfo(certPEM)
					if info != nil {
						tofuVerifier.DisplayMismatchWarning(nodeID, address, existingNode.Fingerprint, info.Fingerprint)
					}
				}
			}
			return false, err
		}

		if result.NewTrust {
			tofuVerifier.DisplayNewTrust(result.Node)
		}

		return result.Trusted, nil
	})

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
