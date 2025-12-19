package cert

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"bib/internal/certs"
	"bib/internal/config"

	"github.com/spf13/cobra"
)

func newRevokeCommand() *cobra.Command {
	var (
		fingerprint string
		reason      string
		notes       string
	)

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a certificate",
		Long: `Add a certificate to the revocation list.

Revoked certificates will no longer be accepted for authentication.
The revocation is distributed to cluster nodes via P2P.

Example:
  bib cert revoke --fingerprint abc123...
  bib cert revoke --fingerprint abc123... --reason key_compromise`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fingerprint == "" {
				return fmt.Errorf("--fingerprint is required")
			}

			// Determine revocation list path
			configDir, err := config.UserConfigDir(config.AppBibd)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			rlPath := filepath.Join(configDir, "certs", "revocation.json")

			// Load or create revocation list
			rl, err := certs.NewRevocationList(rlPath)
			if err != nil {
				return fmt.Errorf("failed to load revocation list: %w", err)
			}

			// Check if already revoked
			if rl.IsRevoked(fingerprint) {
				return fmt.Errorf("certificate is already revoked")
			}

			// Add to revocation list
			entry := &certs.RevokedCertificate{
				Fingerprint: fingerprint,
				RevokedAt:   time.Now(),
				Reason:      certs.RevocationReason(reason),
				RevokedBy:   os.Getenv("USER"),
				Notes:       notes,
			}

			if err := rl.Revoke(entry); err != nil {
				return fmt.Errorf("failed to revoke certificate: %w", err)
			}

			fmt.Printf("✓ Certificate revoked\n\n")
			fmt.Printf("  Fingerprint: %s\n", fingerprint)
			fmt.Printf("  Reason:      %s\n", reason)
			fmt.Printf("  Revoked At:  %s\n", entry.RevokedAt.Format(time.RFC3339))
			fmt.Printf("\n")
			fmt.Printf("Note: Restart bibd or trigger a sync for cluster-wide propagation.\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "Certificate fingerprint to revoke (required)")
	cmd.Flags().StringVar(&reason, "reason", "unspecified", "Revocation reason (unspecified, key_compromise, superseded, etc.)")
	cmd.Flags().StringVar(&notes, "notes", "", "Optional notes about the revocation")

	cmd.MarkFlagRequired("fingerprint")

	// Add subcommand to list revocations
	cmd.AddCommand(newRevokeListCommand())

	return cmd
}

func newRevokeListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List revoked certificates",
		Long: `List all revoked certificates.

Example:
  bib cert revoke list`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine revocation list path
			configDir, err := config.UserConfigDir(config.AppBibd)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			rlPath := filepath.Join(configDir, "certs", "revocation.json")

			// Load revocation list
			rl, err := certs.NewRevocationList(rlPath)
			if err != nil {
				return fmt.Errorf("failed to load revocation list: %w", err)
			}

			entries := rl.List()
			if len(entries) == 0 {
				fmt.Println("No revoked certificates.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "FINGERPRINT\tREVOKED AT\tREASON\tSUBJECT")
			fmt.Fprintln(w, "-----------\t----------\t------\t-------")

			for _, entry := range entries {
				fp := entry.Fingerprint
				if len(fp) > 16 {
					fp = fp[:16] + "..."
				}
				revokedAt := entry.RevokedAt.Format("2006-01-02")
				subject := entry.Subject
				if len(subject) > 30 {
					subject = subject[:30] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", fp, revokedAt, entry.Reason, subject)
			}

			w.Flush()
			return nil
		},
	}

	return cmd
}
