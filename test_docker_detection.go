package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func main() {
	fmt.Println("Testing Docker detection...")

	// Test 1: Simple docker command
	fmt.Println("\n1. Testing 'docker --version':")
	cmd := exec.Command("docker", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("   ERROR: %v\n", err)
		fmt.Printf("   Output: %s\n", string(output))
	} else {
		fmt.Printf("   SUCCESS: %s\n", string(output))
	}

	// Test 2: Docker info command
	fmt.Println("\n2. Testing 'docker info':")
	cmd2 := exec.Command("docker", "info")
	output2, err2 := cmd2.CombinedOutput()
	if err2 != nil {
		fmt.Printf("   ERROR: %v\n", err2)
		fmt.Printf("   Output length: %d bytes\n", len(output2))
	} else {
		fmt.Printf("   SUCCESS: Docker info returned %d bytes\n", len(output2))
	}

	// Test 3: With context and timeout
	fmt.Println("\n3. Testing 'docker info' with context timeout:")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd3 := exec.CommandContext(ctx, "docker", "info")
	err3 := cmd3.Run()
	if err3 != nil {
		fmt.Printf("   ERROR: %v\n", err3)
	} else {
		fmt.Printf("   SUCCESS: Docker info succeeded\n")
	}

	fmt.Println("\nAll tests complete!")
}
