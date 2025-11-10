//go:build windows

package cmd

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/natefinch/npipe"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of your bib environment (Windows)",
	Long: `Status (Windows). With --discover, queries the local bib daemon for peer candidates.
Supports tcp:// and npipe:// endpoints. Default: tcp://127.0.0.1:50051.`,
	Run: func(cmd *cobra.Command, args []string) {
		discover, _ := cmd.Flags().GetBool("discover")
		limit, _ := cmd.Flags().GetInt("limit")
		preferRegion, _ := cmd.Flags().GetString("prefer-region")
		preferTagsRaw, _ := cmd.Flags().GetString("prefer-tags")
		addr, _ := cmd.Flags().GetString("addr")
		includeSelf, _ := cmd.Flags().GetBool("include-self")

		if !discover {
			fmt.Println("Use --discover to list peer candidates from the local daemon.")
			return
		}
		if err := runDiscover(addr, limit, preferRegion, preferTagsRaw, includeSelf); err != nil {
			log.Fatalf("discover: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("discover", false, "List peer candidates from local daemon")
	statusCmd.Flags().Int("limit", 10, "Max candidates to return when --discover is set")
	statusCmd.Flags().String("prefer-region", "", "Preferred region for candidate scoring")
	statusCmd.Flags().String("prefer-tags", "", "Comma-separated preferred tags")
	statusCmd.Flags().Bool("include-self", true, "Include the local daemon as a candidate")
	statusCmd.Flags().String("addr", "tcp://127.0.0.1:50051", "Daemon endpoint (tcp://host:port or npipe://./pipe/name)")
}

// dialDaemon (Windows) supports tcp://, npipe://, and bare host:port.
func dialDaemon(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("empty endpoint")
	}

	switch {
	case strings.HasPrefix(endpoint, "npipe://"):
		pipePath := strings.TrimPrefix(endpoint, "npipe://")
		if pipePath == "" {
			return nil, fmt.Errorf("npipe endpoint missing pipe name")
		}
		// Windows named pipe full path format: \\.\pipe\<name>
		full := `\\.` + `\` + pipePath
		dialer := func(ctx context.Context, _ string) (net.Conn, error) {
			return npipe.DialTimeout(full, 2*time.Second)
		}
		return grpc.DialContext(
			ctx,
			endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(dialer),
			grpc.WithBlock(),
		)

	case strings.HasPrefix(endpoint, "tcp://"):
		target := strings.TrimPrefix(endpoint, "tcp://")
		if target == "" {
			return nil, fmt.Errorf("tcp endpoint missing host:port")
		}
		return grpc.DialContext(ctx, target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)

	default:
		// Bare host:port
		if strings.Contains(endpoint, ":") {
			return grpc.DialContext(ctx, endpoint,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock(),
			)
		}
		return nil, fmt.Errorf("unsupported endpoint format on Windows: %s", endpoint)
	}
}
