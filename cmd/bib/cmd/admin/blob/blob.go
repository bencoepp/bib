package blob

import (
	"github.com/spf13/cobra"
)

// Cmd represents the blob admin command group
var Cmd = &cobra.Command{
	Use:   "blob",
	Short: "Manage blob storage",
	Long: `Manage blob storage including garbage collection, statistics, and tiering.

Blob storage is used for storing dataset chunks with features like
content-addressed storage, deduplication, compression, and encryption.`,
}

// NewCommand returns the blob command with all subcommands registered
func NewCommand() *cobra.Command {
	Cmd.AddCommand(statsCmd)
	Cmd.AddCommand(gcCmd)
	Cmd.AddCommand(verifyCmd)
	Cmd.AddCommand(tierCmd)

	return Cmd
}

func init() {
	// GC flags
	gcCmd.Flags().BoolVar(&gcForce, "force", false, "Force GC even if conditions not met")
	gcCmd.Flags().BoolVar(&gcPermanent, "permanent", false, "Permanently delete trash contents")
	gcCmd.Flags().BoolVar(&gcEmptyTrash, "empty-trash", false, "Empty trash without running GC")

	// Stats flags
	statsCmd.Flags().StringVar(&statsFormat, "format", "table", "Output format: table, json")

	// Verify flags
	verifyCmd.Flags().StringVar(&verifyDataset, "dataset", "", "Dataset version ID to verify")

	// Tier flags
	tierCmd.Flags().StringVar(&tierCool, "cool", "", "Move blob to cold tier (hash)")
	tierCmd.Flags().StringVar(&tierWarm, "warm", "", "Move blob to hot tier (hash)")
	tierCmd.Flags().BoolVar(&tierApply, "apply", false, "Apply tiering policy to all blobs")
}
