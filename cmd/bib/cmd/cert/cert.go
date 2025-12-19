// Package cert provides certificate management CLI commands.
package cert

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the cert command and subcommands.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Manage TLS certificates",
		Long: `Manage TLS certificates for bibd connections.

The cert command provides subcommands for:
- Initializing a Certificate Authority (CA)
- Generating client certificates
- Listing and inspecting certificates
- Exporting certificates in various formats
- Revoking certificates`,
		Aliases: []string{"certs", "certificate"},
	}

	cmd.AddCommand(
		newInitCommand(),
		newGenerateCommand(),
		newListCommand(),
		newInfoCommand(),
		newExportCommand(),
		newRevokeCommand(),
	)

	return cmd
}
