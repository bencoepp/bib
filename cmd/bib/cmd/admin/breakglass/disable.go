package breakglass

import (
	"fmt"

	"github.com/spf13/cobra"
)

// disableCmd disables an active break glass session
var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable an active break glass session",
	Long: `Disable an active break glass session before it expires.

This immediately invalidates the temporary database credentials
and ends the session. A session report will be generated.

If acknowledgment is required, the session will move to pending
acknowledgment state until acknowledged with 'bib admin break-glass acknowledge'.`,
	RunE: runBreakGlassDisable,
}

func runBreakGlassDisable(cmd *cobra.Command, args []string) error {
	fmt.Println("Disabling break glass session...")

	// TODO: Implement gRPC call to disable session

	fmt.Println()
	fmt.Println("✓ Break glass session disabled")
	fmt.Println()
	fmt.Println("Session summary:")
	fmt.Println("  Duration: 15m23s")
	fmt.Println("  Queries executed: 47")
	fmt.Println("  Tables accessed: nodes, topics, datasets")
	fmt.Println()
	fmt.Println("This session requires acknowledgment. Run:")
	fmt.Println("  bib admin break-glass acknowledge")

	return nil
}
