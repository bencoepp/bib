package cmd

import (
	bibv1 "bib/internal/pb/bibd/v1"
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/natefinch/npipe"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Chek the status of your bib environment",
	Long: `This command allows you to check the status of your bib environment.

It will check if a local bib daemon is running and provide information about its status.
As well as any close by, in your region or global bib nodes that are reachable. It will
also give you general health information about your bib setup.`,
	Run: func(cmd *cobra.Command, args []string) {

		discover, _ := cmd.Flags().GetBool("discover")
		limit, _ := cmd.Flags().GetInt("limit")
		preferRegion, _ := cmd.Flags().GetString("prefer-region")
		preferTagsRaw, _ := cmd.Flags().GetString("prefer-tags")
		addr, _ := cmd.Flags().GetString("addr")

		if runtime.GOOS == "windows" {
			addr = "tcp://127.0.0.1:50051"
		}

		if !discover {
			fmt.Println("Status command is under development. Use --discover to find peer candidates from the local daemon.")
			return
		}

		if err := runDiscover(addr, limit, preferRegion, preferTagsRaw); err != nil {
			log.Fatalf("discover: %v", err)
		}
		return

	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// statusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	statusCmd.Flags().Bool("discover", false, "List peer candidates from local daemon")
	statusCmd.Flags().Int("limit", 10, "Max candidates to return when --discover is set")
	statusCmd.Flags().String("prefer-region", "", "Preferred region for candidate scoring")
	statusCmd.Flags().String("prefer-tags", "", "Comma-separated preferred tags")
	statusCmd.Flags().String("addr", "unix:///var/run/bibd.sock", "Daemon address (unix:// or tcp://)")
}

func runDiscover(endpoint string, limit int, region, tagsRaw string) error {
	conn, err := dialWithTimeout(endpoint, 5*time.Second)
	if err != nil {
		return fmt.Errorf("dial daemon failed: %w", err)
	}
	defer conn.Close()

	client := bibv1.NewDiscoveryClient(conn)

	var tags []string
	if tagsRaw != "" {
		for _, t := range strings.Split(tagsRaw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	resp, err := client.FindCandidates(context.Background(), &bibv1.FindCandidatesRequest{
		Limit:        uint32(limit),
		PreferRegion: region,
		PreferTags:   tags,
		IncludeSelf:  true,
	})
	if err != nil {
		return fmt.Errorf("FindCandidates RPC failed: %w", err)
	}

	if len(resp.Candidates) == 0 {
		fmt.Println("No candidates found.")
		return nil
	}

	fmt.Printf("Candidates (limit=%d):\n", limit)
	for i, c := range resp.Candidates {
		fmt.Printf("%2d. id=%s score=%.2f rtt=%.1fms load=%.2f region=%s tags=%v src=%s\n",
			i+1, c.PeerId, c.Score, c.LastRttMs, c.Load, c.Region, c.Tags, c.Source)
		for _, a := range c.Multiaddrs {
			fmt.Printf("      %s\n", a)
		}
	}

	return nil
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, trimSpaces(cur))
			cur = ""
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, trimSpaces(cur))
	}
	// final trim
	for i := range out {
		out[i] = trimSpaces(out[i])
	}
	return out
}

func trimSpaces(s string) string {
	return string([]rune(s))
}

// dialDaemon dials:
//
//	unix:///absolute/path         (Unix-like only)
//	tcp://host:port
//	host:port                      (TCP)
//	npipe://./pipe/name            (Windows named pipe)
func dialDaemon(ctx context.Context, endpoint string) (*grpc.ClientConn, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("empty endpoint")
	}

	switch {
	case strings.HasPrefix(endpoint, "unix://"):
		if runtime.GOOS == "windows" {
			return nil, fmt.Errorf("unix sockets unsupported on Windows; use tcp:// or npipe://")
		}
		path := strings.TrimPrefix(endpoint, "unix://")
		if path == "" {
			return nil, fmt.Errorf("unix endpoint missing path")
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("unix socket %s not found (is bibd running?): %w", path, err)
		}
		dialer := func(ctx context.Context, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", path)
		}
		return grpc.DialContext(
			ctx,
			"unix://"+path,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(dialer),
			grpc.WithBlock(),
		)

	case strings.HasPrefix(endpoint, "npipe://"):
		if runtime.GOOS != "windows" {
			return nil, fmt.Errorf("npipe endpoints only valid on Windows")
		}
		// Named pipe path: npipe://./pipe/bibd.grpc
		pipePath := strings.TrimPrefix(endpoint, "npipe://")
		if pipePath == "" {
			return nil, fmt.Errorf("npipe endpoint missing pipe name")
		}
		// Lazy import to avoid platform build issues if you later split files.
		// Requires: github.com/natefinch/npipe
		dialer := func(ctx context.Context, _ string) (net.Conn, error) {
			type pipeConn interface {
				net.Conn
			}
			// Using npipe.DialTimeout for blocking behavior.
			conn, err := npipeDialTimeout("\\\\"+pipePath, 2*time.Second)
			return conn, err
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
		return grpc.DialContext(
			ctx,
			target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)

	default:
		// Bare host:port
		if strings.Contains(endpoint, ":") {
			return grpc.DialContext(
				ctx,
				endpoint,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock(),
			)
		}
		return nil, fmt.Errorf("unsupported endpoint format: %s", endpoint)
	}
}

// Simple wrapper for npipe dial to avoid direct import in non-Windows builds.
// Implemented inline for brevity; in production split to _windows.go.
func npipeDialTimeout(pipePath string, timeout time.Duration) (net.Conn, error) {
	// Import only when on Windows.
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("npipeDialTimeout called on non-Windows")
	}
	type dialer interface {
		DialTimeout(string, time.Duration) (net.Conn, error)
	}
	// We rely on natefinch/npipe; ensure module added in go.mod:
	// require github.com/natefinch/npipe v0.0.0-20220114031633-4a47ec2428ee
	return npipe.DialTimeout(pipePath, timeout)
}

func dialWithTimeout(endpoint string, timeout time.Duration) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return dialDaemon(ctx, endpoint)
}
