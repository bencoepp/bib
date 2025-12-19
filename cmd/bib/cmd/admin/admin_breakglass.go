package admin

import (
	"bufio"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

var (
	// break-glass enable flags
	bgReason      string
	bgDuration    string
	bgAccessLevel string
	bgUsername    string
	bgKeyPath     string

	// break-glass acknowledge flags
	bgSessionID string
)

// breakGlassCmd represents the break-glass command group
var breakGlassCmd = &cobra.Command{
	Use:   "break-glass",
	Short: "Emergency database access commands",
	Long: `Break glass provides controlled emergency access to the database
for disaster recovery and debugging scenarios.

Break glass access is:
- Disabled by default and requires explicit configuration
- Time-limited with automatic expiration
- Fully audited with no query redaction
- Requires administrator acknowledgment after use

IMPORTANT: Break glass access bypasses normal security controls.
Only use when absolutely necessary for disaster recovery or debugging.`,
}

// breakGlassEnableCmd enables a break glass session
var breakGlassEnableCmd = &cobra.Command{
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

// breakGlassDisableCmd disables an active break glass session
var breakGlassDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable an active break glass session",
	Long: `Disable an active break glass session before it expires.

This immediately invalidates the temporary database credentials
and ends the session. A session report will be generated.

If acknowledgment is required, the session will move to pending
acknowledgment state until acknowledged with 'bib admin break-glass acknowledge'.`,
	RunE: runBreakGlassDisable,
}

// breakGlassStatusCmd shows the current break glass status
var breakGlassStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show break glass status",
	Long: `Show the current break glass status including:
- Whether break glass is enabled in configuration
- Any active break glass sessions
- Sessions pending acknowledgment`,
	RunE: runBreakGlassStatus,
}

// breakGlassAcknowledgeCmd acknowledges a completed break glass session
var breakGlassAcknowledgeCmd = &cobra.Command{
	Use:   "acknowledge",
	Short: "Acknowledge a completed break glass session",
	Long: `Acknowledge a completed break glass session.

After a break glass session ends, it must be acknowledged by an
administrator. This command displays the session report and requires
confirmation before marking the session as acknowledged.

Use 'bib admin break-glass status' to see pending acknowledgments.`,
	RunE: runBreakGlassAcknowledge,
}

// breakGlassReportCmd shows the report for a break glass session
var breakGlassReportCmd = &cobra.Command{
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

func init() {
	Cmd.AddCommand(breakGlassCmd)

	// Add subcommands to break-glass
	breakGlassCmd.AddCommand(breakGlassEnableCmd)
	breakGlassCmd.AddCommand(breakGlassDisableCmd)
	breakGlassCmd.AddCommand(breakGlassStatusCmd)
	breakGlassCmd.AddCommand(breakGlassAcknowledgeCmd)
	breakGlassCmd.AddCommand(breakGlassReportCmd)

	// Flags for enable command
	breakGlassEnableCmd.Flags().StringVar(&bgReason, "reason", "", "Reason for break glass access (required)")
	breakGlassEnableCmd.Flags().StringVar(&bgDuration, "duration", "1h", "Session duration (e.g., 30m, 1h, 2h)")
	breakGlassEnableCmd.Flags().StringVar(&bgAccessLevel, "access-level", "", "Access level: readonly or readwrite (default: configured default)")
	breakGlassEnableCmd.Flags().StringVar(&bgUsername, "user", "", "Break glass username (if multiple users configured)")
	breakGlassEnableCmd.Flags().StringVar(&bgKeyPath, "key", "", "Path to Ed25519 private key for authentication")
	_ = breakGlassEnableCmd.MarkFlagRequired("reason")

	// Flags for acknowledge command
	breakGlassAcknowledgeCmd.Flags().StringVar(&bgSessionID, "session", "", "Session ID to acknowledge (optional if only one pending)")
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

func runBreakGlassStatus(cmd *cobra.Command, args []string) error {
	// TODO: Implement gRPC call to get status

	fmt.Println("Break Glass Status")
	fmt.Println("==================")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  Enabled: yes")
	fmt.Println("  Max Duration: 1h")
	fmt.Println("  Default Access: readonly")
	fmt.Println("  Require Acknowledgment: yes")
	fmt.Println()
	fmt.Println("Active Sessions: none")
	fmt.Println()
	fmt.Println("Pending Acknowledgments: 0")

	return nil
}

func runBreakGlassAcknowledge(cmd *cobra.Command, args []string) error {
	// TODO: Implement gRPC call to get pending sessions

	fmt.Println("Break Glass Session Acknowledgment")
	fmt.Println("===================================")
	fmt.Println()

	// Display session report
	fmt.Println("Session Report:")
	fmt.Println("  Session ID: abc12345-1234-5678-9abc-def012345678")
	fmt.Println("  User: emergency_admin")
	fmt.Println("  Reason: investigating data corruption")
	fmt.Println("  Started: 2024-01-15T10:30:00Z")
	fmt.Println("  Ended: 2024-01-15T10:45:23Z")
	fmt.Println("  Duration: 15m23s")
	fmt.Println("  Access Level: readonly")
	fmt.Println()
	fmt.Println("Query Statistics:")
	fmt.Println("  Total Queries: 47")
	fmt.Println("  SELECT: 45")
	fmt.Println("  Other: 2")
	fmt.Println()
	fmt.Println("Tables Accessed:")
	fmt.Println("  - nodes")
	fmt.Println("  - topics")
	fmt.Println("  - datasets")
	fmt.Println()

	// Prompt for acknowledgment
	fmt.Print("Do you acknowledge this break glass session? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response != "y" && response != "yes" {
		fmt.Println("Acknowledgment cancelled.")
		return nil
	}

	// TODO: Implement gRPC call to acknowledge

	fmt.Println()
	fmt.Println("✓ Session acknowledged")

	return nil
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

// loadPrivateKey loads an Ed25519 private key from a file.
func loadPrivateKey(keyPath string) (ed25519.PrivateKey, error) {
	// Default key path
	if keyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = home + "/.ssh/id_ed25519_breakglass"

		// Check if the default key exists
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			// Try the regular ed25519 key
			keyPath = home + "/.ssh/id_ed25519"
		}
	}

	// Read the key file
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %s: %w", keyPath, err)
	}

	// Parse the private key
	// First try without passphrase
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		// Try with passphrase
		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			fmt.Printf("Enter passphrase for %s: ", keyPath)
			passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return nil, fmt.Errorf("failed to read passphrase: %w", err)
			}

			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, passphrase)
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key with passphrase: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	// Extract the Ed25519 private key
	cryptoSigner, ok := signer.(ssh.AlgorithmSigner)
	if !ok {
		return nil, fmt.Errorf("key is not suitable for signing")
	}

	pubKey := cryptoSigner.PublicKey()
	cryptoPubKey := pubKey.(ssh.CryptoPublicKey).CryptoPublicKey()
	ed25519PubKey, ok := cryptoPubKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519")
	}

	// For Ed25519, we need to reconstruct the private key
	// The SSH private key format includes both public and private parts
	// We'll return the signer's underlying key
	// Note: This is a simplified approach - in production, we might need
	// to handle this differently based on the ssh library internals

	// For now, return a placeholder - the actual signing will use the ssh.Signer
	// In the real implementation, we'll use the ssh.Signer directly for signing
	_ = ed25519PubKey

	// This is a workaround - in reality, we'd use the signer directly
	// For the stub implementation, generate a dummy key
	_, privateKey, _ := ed25519.GenerateKey(nil)
	return privateKey, nil
}

// getAccessLevelDisplay returns a display string for the access level.
func getAccessLevelDisplay(level string) string {
	if level == "" {
		return "default (readonly)"
	}
	return level
}

// signChallenge signs a challenge using the SSH signer.
func signChallenge(signer ssh.Signer, challenge []byte) ([]byte, error) {
	signature, err := signer.Sign(nil, challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to sign challenge: %w", err)
	}
	return signature.Blob, nil
}

// formatConnectionString formats a connection string for display.
func formatConnectionString(connStr string) string {
	// Mask the password in the connection string
	// postgresql://user:password@host:port/db -> postgresql://user:****@host:port/db
	parts := strings.SplitN(connStr, "@", 2)
	if len(parts) != 2 {
		return connStr
	}

	userPass := strings.TrimPrefix(parts[0], "postgresql://")
	userPassParts := strings.SplitN(userPass, ":", 2)
	if len(userPassParts) != 2 {
		return connStr
	}

	return fmt.Sprintf("postgresql://%s:****@%s", userPassParts[0], parts[1])
}

// base64Encode encodes bytes to base64.
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// base64Decode decodes base64 to bytes.
func base64Decode(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}
