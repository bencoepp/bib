package breakglass

import (
	"github.com/spf13/cobra"
)

// Cmd represents the break-glass command group
var Cmd = &cobra.Command{
	Use:   "break-glass",
	Short: "Emergency database access commands",
	Long: `Break glass provides controlled emergency access to the database
for disaster recovery and debugging scenarios.

Break glass access is:
- Disabled by default and requires explicit configuration
- Time-limited with automatic expiration
- Fully audited with no query redaction
- Requires administrator acknowledgment after use

IMPORTANT: Break glass access bypasses normal security controls.
Only use when absolutely necessary for disaster recovery or debugging.`,
}

// NewCommand returns the break-glass command with all subcommands registered
func NewCommand() *cobra.Command {
	Cmd.AddCommand(enableCmd)
	Cmd.AddCommand(disableCmd)
	Cmd.AddCommand(statusCmd)
	Cmd.AddCommand(acknowledgeCmd)
	Cmd.AddCommand(reportCmd)

	return Cmd
}

func init() {
	// Flags for enable command
	enableCmd.Flags().StringVar(&bgReason, "reason", "", "Reason for break glass access (required)")
	enableCmd.Flags().StringVar(&bgDuration, "duration", "1h", "Session duration (e.g., 30m, 1h, 2h)")
	enableCmd.Flags().StringVar(&bgAccessLevel, "access-level", "", "Access level: readonly or readwrite (default: configured default)")
	enableCmd.Flags().StringVar(&bgUsername, "user", "", "Break glass username (if multiple users configured)")
	enableCmd.Flags().StringVar(&bgKeyPath, "key", "", "Path to Ed25519 private key for authentication")
	_ = enableCmd.MarkFlagRequired("reason")

	// Flags for acknowledge command
	acknowledgeCmd.Flags().StringVar(&bgSessionID, "session", "", "Session ID to acknowledge (optional if only one pending)")
}
