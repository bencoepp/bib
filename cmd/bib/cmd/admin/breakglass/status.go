package breakglass

import (
	"fmt"

	"github.com/spf13/cobra"
)

// statusCmd shows the current break glass status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show break glass status",
	Long: `Show the current break glass status including:
- Whether break glass is enabled in configuration
- Any active break glass sessions
- Sessions pending acknowledgment`,
	RunE: runBreakGlassStatus,
}

func runBreakGlassStatus(cmd *cobra.Command, args []string) error {
	// TODO: Implement gRPC call to get status

	fmt.Println("Break Glass Status")
	fmt.Println("==================")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  Enabled: yes")
	fmt.Println("  Max Duration: 1h")
	fmt.Println("  Default Access: readonly")
	fmt.Println("  Require Acknowledgment: yes")
	fmt.Println()
	fmt.Println("Active Sessions: none")
	fmt.Println()
	fmt.Println("Pending Acknowledgments: 0")

	return nil
}
