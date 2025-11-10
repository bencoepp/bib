package cmd

import (
	bibv1 "bib/internal/pb/bibd/v1"
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
)

// runDiscover queries the daemon for peer candidates.
func runDiscover(endpoint string, limit int, region, tagsRaw string, includeSelf bool) error {
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
		IncludeSelf:  includeSelf,
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

// dialWithTimeout is shared; per-platform dialDaemon is provided by build-tagged files.
func dialWithTimeout(endpoint string, timeout time.Duration) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return dialDaemon(ctx, endpoint)
}
