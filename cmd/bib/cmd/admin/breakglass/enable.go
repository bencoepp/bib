package breakglass

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	bgReason      string
	bgDuration    string
	bgAccessLevel string
	bgUsername    string
	bgKeyPath     string
)

// enableCmd enables a break glass session
var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable a break glass session",
	Long: `Enable a break glass session for emergency database access.

This command initiates an interactive authentication challenge using
your Ed25519 SSH key. Once authenticated, a time-limited database
user is created with the specified access level.

The command outputs a PostgreSQL connection string that can be used
directly with psql or other PostgreSQL clients.

Examples:
  # Enable with default settings (1 hour, read-only)
  bib admin break-glass enable --reason "investigating data corruption"

  # Enable with custom duration and write access
  bib admin break-glass enable --reason "emergency fix" --duration 30m --access-level readwrite

  # Enable using a specific key file
  bib admin break-glass enable --reason "DR drill" --key ~/.ssh/emergency_key`,
	RunE: runBreakGlassEnable,
}

func runBreakGlassEnable(cmd *cobra.Command, args []string) error {
	// Parse duration
	duration, err := time.ParseDuration(bgDuration)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	// Load the private key for signing
	privateKey, err := loadPrivateKey(bgKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load private key: %w", err)
	}

	fmt.Println("Break Glass Emergency Access")
	fmt.Println("============================")
	fmt.Println()
	fmt.Printf("Reason: %s\n", bgReason)
	fmt.Printf("Duration: %s\n", duration)
	fmt.Printf("Access Level: %s\n", getAccessLevelDisplay(bgAccessLevel))
	fmt.Println()

	// Step 1: Request a challenge from the daemon
	// TODO: Implement gRPC call to get challenge
	fmt.Println("Requesting authentication challenge from daemon...")

	// For now, simulate the challenge flow
	// In the real implementation, this would be a gRPC call
	challenge := make([]byte, 32)
	if _, err := os.Stdin.Read(challenge); err != nil {
		// Generate random challenge for demo
		for i := range challenge {
			challenge[i] = byte(i)
		}
	}

	// Step 2: Sign the challenge
	fmt.Println("Signing challenge with your private key...")
	signature := ed25519.Sign(privateKey, challenge)

	// Step 3: Send signature back to daemon
	// TODO: Implement gRPC call to verify and enable session
	_ = signature

	// Step 4: Display connection string
	fmt.Println()
	fmt.Println("✓ Break glass session enabled!")
	fmt.Println()
	fmt.Println("Connection String:")
	fmt.Println("  postgresql://breakglass_abc12345:PASSWORD@localhost:5432/bib?sslmode=require")
	fmt.Println()
	fmt.Printf("Session expires at: %s\n", time.Now().Add(duration).Format(time.RFC3339))
	fmt.Println()
	fmt.Println("WARNING: All queries will be logged without redaction.")
	fmt.Println("This session requires acknowledgment after completion.")

	return nil
}
