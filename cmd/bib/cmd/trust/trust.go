// Package trust provides TOFU trust management CLI commands.
package trust

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the trust command and subcommands.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage trusted nodes (TOFU)",
		Long: `Manage Trust-On-First-Use (TOFU) for bibd connections.

The trust command provides subcommands for:
- Adding trusted nodes manually
- Listing trusted nodes
- Removing trusted nodes
- Pinning certificates for security-sensitive deployments
- Showing detailed node information
- Verifying node fingerprints`,
	}

	cmd.AddCommand(
		newAddCommand(),
		newListCommand(),
		newRemoveCommand(),
		newPinCommand(),
		newShowCommand(),
		newVerifyCommand(),
	)

	return cmd
}
