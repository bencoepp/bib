package breakglass

import (
	"fmt"

	"github.com/spf13/cobra"
)

// reportCmd shows the report for a break glass session
var reportCmd = &cobra.Command{
	Use:   "report [session-id]",
	Short: "Show the report for a break glass session",
	Long: `Display the detailed report for a break glass session.

The report includes:
- Session details (user, reason, duration, access level)
- Query statistics (count, types, tables accessed)
- Session recording path (if enabled)`,
	Args: cobra.ExactArgs(1),
	RunE: runBreakGlassReport,
}

func runBreakGlassReport(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	// TODO: Implement gRPC call to get report

	fmt.Printf("Break Glass Session Report: %s\n", sessionID)
	fmt.Println("==========================================")
	fmt.Println()
	fmt.Println("Session Details:")
	fmt.Println("  User: emergency_admin")
	fmt.Println("  Reason: investigating data corruption")
	fmt.Println("  Started: 2024-01-15T10:30:00Z")
	fmt.Println("  Ended: 2024-01-15T10:45:23Z")
	fmt.Println("  Duration: 15m23s")
	fmt.Println("  Access Level: readonly")
	fmt.Println("  Node: QmNode123...")
	fmt.Println()
	fmt.Println("Query Statistics:")
	fmt.Println("  Total Queries: 47")
	fmt.Println("  SELECT: 45")
	fmt.Println("  Other: 2")
	fmt.Println()
	fmt.Println("Tables Accessed:")
	fmt.Println("  - nodes (32 queries)")
	fmt.Println("  - topics (10 queries)")
	fmt.Println("  - datasets (5 queries)")
	fmt.Println()
	fmt.Println("Recording: ~/.local/share/bibd/audit/breakglass_abc12345.rec.gz")
	fmt.Println()
	fmt.Println("Acknowledgment:")
	fmt.Println("  Status: acknowledged")
	fmt.Println("  By: admin@example.com")
	fmt.Println("  At: 2024-01-15T11:00:00Z")

	return nil
}
