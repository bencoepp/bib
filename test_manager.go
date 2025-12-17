package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	pglifecycle "bib/internal/storage/postgres/lifecycle"
)

func main() {
	fmt.Println("Testing PostgreSQL Lifecycle Manager...")

	// Create a temporary directory for testing
	tempDir := filepath.Join(os.TempDir(), "bibd-test")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	// Create default config
	cfg := pglifecycle.DefaultLifecycleConfig()
	fmt.Printf("\nDefault config created:\n")
	fmt.Printf("  Runtime: %q (empty = auto-detect)\n", cfg.Runtime)
	fmt.Printf("  SocketPath: %q\n", cfg.SocketPath)
	fmt.Printf("  Image: %s\n", cfg.Image)

	// Create manager
	fmt.Println("\nCreating lifecycle manager...")
	mgr, err := pglifecycle.NewManager(cfg, "test-node", tempDir)
	if err != nil {
		fmt.Printf("ERROR creating manager: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("SUCCESS! Manager created\n")
	fmt.Printf("  Detected runtime: %s\n", mgr.Runtime())

	// Try to start (this will fail but shows us more)
	fmt.Println("\nAttempting to start PostgreSQL...")
	ctx := context.Background()
	err = mgr.Start(ctx)
	if err != nil {
		fmt.Printf("Start failed (expected): %v\n", err)
	} else {
		fmt.Println("Started successfully!")
		// Stop it
		mgr.Stop(ctx)
	}
}
