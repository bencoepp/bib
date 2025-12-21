package trust

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"bib/internal/certs"
	"bib/internal/config"

	"github.com/spf13/cobra"
)

func newAddCommand() *cobra.Command {
	var (
		nodeID      string
		fingerprint string
		alias       string
		address     string
		certFile    string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a trusted node",
		Long: `Manually add a node to the trusted nodes list.

This bypasses Trust-On-First-Use (TOFU) for security-sensitive deployments
where you want to pre-configure trusted nodes.

Example:
  bib trust add --node-id 12D3KooW... --fingerprint abc123...
  bib trust add --node-id 12D3KooW... --cert server.crt --alias production`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if nodeID == "" {
				return fmt.Errorf("--node-id is required")
			}
			if fingerprint == "" && certFile == "" {
				return fmt.Errorf("--fingerprint or --cert is required")
			}

			// If cert file provided, extract fingerprint
			var certPEM string
			if certFile != "" {
				data, err := os.ReadFile(certFile)
				if err != nil {
					return fmt.Errorf("failed to read certificate: %w", err)
				}
				certPEM = string(data)

				fp, err := certs.Fingerprint(data)
				if err != nil {
					return fmt.Errorf("failed to calculate fingerprint: %w", err)
				}
				fingerprint = fp
			}

			// Get trust store
			configDir, err := config.UserConfigDir(config.AppBib)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			trustDir := filepath.Join(configDir, "trusted_nodes")
			ts, err := certs.NewTrustStore(trustDir)
			if err != nil {
				return fmt.Errorf("failed to open trust store: %w", err)
			}

			// Check if already trusted
			if existing, ok := ts.Get(nodeID); ok {
				if existing.Fingerprint == fingerprint {
					fmt.Println("Node is already trusted with the same certificate.")
					return nil
				}
				return fmt.Errorf("node already trusted with different certificate. Use 'bib trust remove' first")
			}

			// Add to trust store
			node := &certs.TrustedNode{
				NodeID:      nodeID,
				Fingerprint: fingerprint,
				Certificate: certPEM,
				Alias:       alias,
				Address:     address,
				TrustMethod: certs.TrustMethodManual,
				Verified:    true, // Manual adds are considered verified
			}

			if err := ts.Add(node); err != nil {
				return fmt.Errorf("failed to add trusted node: %w", err)
			}

			fmt.Printf("✓ Node added to trusted list\n\n")
			fmt.Printf("  Node ID:     %s\n", nodeID)
			fmt.Printf("  Fingerprint: %s\n", fingerprint)
			if alias != "" {
				fmt.Printf("  Alias:       %s\n", alias)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&nodeID, "node-id", "", "Node ID (peer ID) to trust (required)")
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "Certificate fingerprint")
	cmd.Flags().StringVar(&certFile, "cert", "", "Path to certificate file")
	cmd.Flags().StringVar(&alias, "alias", "", "Friendly alias for the node")
	cmd.Flags().StringVar(&address, "address", "", "Node address (multiaddr or host:port)")

	cmd.MarkFlagRequired("node-id")

	return cmd
}

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List trusted nodes",
		Long: `List all trusted nodes.

Example:
  bib trust list`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get trust store
			configDir, err := config.UserConfigDir(config.AppBib)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			trustDir := filepath.Join(configDir, "trusted_nodes")
			ts, err := certs.NewTrustStore(trustDir)
			if err != nil {
				return fmt.Errorf("failed to open trust store: %w", err)
			}

			nodes := ts.List()
			if len(nodes) == 0 {
				fmt.Println("No trusted nodes.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NODE ID\tALIAS\tFINGERPRINT\tMETHOD\tVERIFIED\tLAST SEEN")
			fmt.Fprintln(w, "-------\t-----\t-----------\t------\t--------\t---------")

			for _, node := range nodes {
				nodeID := node.NodeID
				if len(nodeID) > 16 {
					nodeID = nodeID[:8] + "..." + nodeID[len(nodeID)-4:]
				}

				fp := node.Fingerprint
				if len(fp) > 16 {
					fp = fp[:16] + "..."
				}

				verified := "No"
				if node.Verified {
					verified = "Yes"
				}
				if node.PinnedAt != nil {
					verified = "Pinned"
				}

				lastSeen := "Never"
				if !node.LastSeen.IsZero() {
					lastSeen = node.LastSeen.Format("2006-01-02")
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					nodeID, node.Alias, fp, node.TrustMethod, verified, lastSeen)
			}

			w.Flush()
			return nil
		},
	}

	return cmd
}

func newRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <node-id>",
		Short: "Remove a trusted node",
		Long: `Remove a node from the trusted list.

Example:
  bib trust remove 12D3KooW...`,
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeID := args[0]

			// Get trust store
			configDir, err := config.UserConfigDir(config.AppBib)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			trustDir := filepath.Join(configDir, "trusted_nodes")
			ts, err := certs.NewTrustStore(trustDir)
			if err != nil {
				return fmt.Errorf("failed to open trust store: %w", err)
			}

			if err := ts.Remove(nodeID); err != nil {
				return fmt.Errorf("failed to remove node: %w", err)
			}

			fmt.Printf("✓ Node removed from trusted list: %s\n", nodeID)
			return nil
		},
	}

	return cmd
}

func newPinCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pin <node-id>",
		Short: "Pin a node's certificate",
		Long: `Pin a trusted node's certificate.

Pinning marks a certificate as explicitly trusted and will warn
if the certificate ever changes (stronger than TOFU).

Example:
  bib trust pin 12D3KooW...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeID := args[0]

			// Get trust store
			configDir, err := config.UserConfigDir(config.AppBib)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			trustDir := filepath.Join(configDir, "trusted_nodes")
			ts, err := certs.NewTrustStore(trustDir)
			if err != nil {
				return fmt.Errorf("failed to open trust store: %w", err)
			}

			node, ok := ts.Get(nodeID)
			if !ok {
				return fmt.Errorf("node not found in trusted list")
			}

			if err := ts.Pin(nodeID); err != nil {
				return fmt.Errorf("failed to pin certificate: %w", err)
			}

			fmt.Printf("✓ Certificate pinned for node: %s\n", nodeID)
			fmt.Printf("  Fingerprint: %s\n", node.Fingerprint)
			fmt.Printf("  Pinned At:   %s\n", time.Now().Format(time.RFC3339))
			return nil
		},
	}

	return cmd
}

func newShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <node-id>",
		Short: "Show detailed info about a trusted node",
		Long: `Show detailed information about a trusted node.

Displays all stored information including certificate details,
trust method, and verification status.

Example:
  bib trust show 12D3KooW...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeID := args[0]

			// Get trust store
			configDir, err := config.UserConfigDir(config.AppBib)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			trustDir := filepath.Join(configDir, "trusted_nodes")
			ts, err := certs.NewTrustStore(trustDir)
			if err != nil {
				return fmt.Errorf("failed to open trust store: %w", err)
			}

			node, ok := ts.Get(nodeID)
			if !ok {
				return fmt.Errorf("node not found in trusted list")
			}

			fmt.Println("═══════════════════════════════════════════════════════════════")
			fmt.Println("  Trusted Node Details")
			fmt.Println("═══════════════════════════════════════════════════════════════")
			fmt.Println()
			fmt.Printf("  Node ID:      %s\n", node.NodeID)
			if node.Alias != "" {
				fmt.Printf("  Alias:        %s\n", node.Alias)
			}
			if node.Address != "" {
				fmt.Printf("  Address:      %s\n", node.Address)
			}
			fmt.Println()
			fmt.Printf("  Fingerprint:\n")
			fmt.Printf("    %s\n", certs.FormatFingerprint(node.Fingerprint))
			fmt.Println()
			fmt.Printf("  Trust Method: %s\n", node.TrustMethod)
			fmt.Printf("  Verified:     %v\n", node.Verified)
			if node.PinnedAt != nil {
				fmt.Printf("  Pinned At:    %s\n", node.PinnedAt.Format(time.RFC3339))
			}
			fmt.Println()
			fmt.Printf("  First Seen:   %s\n", node.FirstSeen.Format(time.RFC3339))
			fmt.Printf("  Last Seen:    %s\n", node.LastSeen.Format(time.RFC3339))
			if node.Notes != "" {
				fmt.Printf("  Notes:        %s\n", node.Notes)
			}
			fmt.Println()

			// Show certificate info if available
			if node.Certificate != "" {
				certInfo, err := certs.ParseCertInfo([]byte(node.Certificate))
				if err == nil {
					fmt.Println("───────────────────────────────────────────────────────────────")
					fmt.Println("  Certificate Details")
					fmt.Println("───────────────────────────────────────────────────────────────")
					fmt.Println()
					fmt.Printf("  Subject:      %s\n", certInfo.Subject)
					fmt.Printf("  Issuer:       %s\n", certInfo.Issuer)
					fmt.Printf("  Valid From:   %s\n", certInfo.NotBefore.Format(time.RFC3339))
					fmt.Printf("  Valid Until:  %s\n", certInfo.NotAfter.Format(time.RFC3339))
					fmt.Printf("  Is CA:        %v\n", certInfo.IsCA)
					fmt.Println()
				}
			}

			return nil
		},
	}

	return cmd
}

func newVerifyCommand() *cobra.Command {
	var fingerprint string

	cmd := &cobra.Command{
		Use:   "verify <node-id>",
		Short: "Verify a node's fingerprint",
		Long: `Verify that a trusted node's fingerprint matches an expected value.

Use this to confirm out-of-band that you're connected to the correct node.

Example:
  bib trust verify 12D3KooW... --fingerprint AB:CD:EF:...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeID := args[0]

			// Get trust store
			configDir, err := config.UserConfigDir(config.AppBib)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			trustDir := filepath.Join(configDir, "trusted_nodes")
			ts, err := certs.NewTrustStore(trustDir)
			if err != nil {
				return fmt.Errorf("failed to open trust store: %w", err)
			}

			node, ok := ts.Get(nodeID)
			if !ok {
				return fmt.Errorf("node not found in trusted list")
			}

			if fingerprint == "" {
				// Just show the fingerprint
				fmt.Printf("Node: %s\n", nodeID)
				fmt.Printf("Fingerprint: %s\n", certs.FormatFingerprint(node.Fingerprint))
				return nil
			}

			// Normalize fingerprint for comparison (remove colons, lowercase)
			normalizedExpected := normalizeFingerprint(fingerprint)
			normalizedActual := normalizeFingerprint(node.Fingerprint)

			if normalizedExpected == normalizedActual {
				fmt.Println("✓ Fingerprint verified successfully!")
				fmt.Println()
				fmt.Printf("  Node: %s\n", nodeID)
				fmt.Printf("  Fingerprint: %s\n", certs.FormatFingerprint(node.Fingerprint))

				// Mark as verified
				if !node.Verified {
					ts.Verify(nodeID)
					fmt.Println()
					fmt.Println("  Node marked as verified.")
				}
				return nil
			}

			fmt.Println("✗ Fingerprint mismatch!")
			fmt.Println()
			fmt.Printf("  Expected: %s\n", certs.FormatFingerprint(normalizedExpected))
			fmt.Printf("  Actual:   %s\n", certs.FormatFingerprint(normalizedActual))
			fmt.Println()
			fmt.Println("  This could indicate a security issue.")
			return fmt.Errorf("fingerprint verification failed")
		},
	}

	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "Expected fingerprint to verify against")

	return cmd
}

// normalizeFingerprint removes colons and converts to lowercase
func normalizeFingerprint(fp string) string {
	result := ""
	for _, c := range fp {
		if c != ':' && c != ' ' {
			if c >= 'A' && c <= 'Z' {
				result += string(c + 32) // lowercase
			} else {
				result += string(c)
			}
		}
	}
	return result
}
