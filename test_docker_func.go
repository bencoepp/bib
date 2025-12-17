package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Simplified version of isDockerAvailable for testing
func isDockerAvailable() bool {
	// Try to run docker command directly
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "info")

	fmt.Printf("Running: docker info\n")
	err := cmd.Run()
	fmt.Printf("Result: err=%v\n", err)

	if err == nil {
		fmt.Println("Docker is available!")
		return true
	}

	fmt.Println("Docker is NOT available (via command)")
	return false
}

func main() {
	fmt.Println("Testing isDockerAvailable() function...")
	result := isDockerAvailable()
	fmt.Printf("\nFinal result: %v\n", result)
}
