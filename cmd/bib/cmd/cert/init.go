package cert

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bib/internal/certs"
	"bib/internal/config"

	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	var (
		outputDir       string
		caValidityYears int
		force           bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a Certificate Authority",
		Long: `Initialize a Certificate Authority (CA) for self-hosted scenarios.

This command creates a new CA certificate and encrypted private key.
The CA is used to sign server and client certificates.

The CA private key is encrypted using a derived key. For bibd, this is
automatically encrypted using the node's P2P identity key.

Example:
  bib cert init
  bib cert init --output ~/.config/bibd/certs
  bib cert init --ca-validity 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine output directory
			if outputDir == "" {
				configDir, err := config.UserConfigDir(config.AppBibd)
				if err != nil {
					return fmt.Errorf("failed to get config directory: %w", err)
				}
				outputDir = filepath.Join(configDir, "certs")
			}

			// Check if CA already exists
			caCertPath := filepath.Join(outputDir, "ca.crt")
			if _, err := os.Stat(caCertPath); err == nil && !force {
				return fmt.Errorf("CA certificate already exists at %s. Use --force to overwrite", caCertPath)
			}

			// Generate CA
			cfg := certs.DefaultConfig("standalone")
			if caValidityYears > 0 {
				cfg.CAValidDuration = cfg.CAValidDuration * time.Duration(caValidityYears) / 10
			}

			caCert, caKey, err := certs.GenerateCA(cfg)
			if err != nil {
				return fmt.Errorf("failed to generate CA: %w", err)
			}

			// Create output directory
			if err := os.MkdirAll(outputDir, 0700); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			// Save CA cert
			if err := os.WriteFile(caCertPath, caCert, 0644); err != nil {
				return fmt.Errorf("failed to save CA certificate: %w", err)
			}

			// For standalone init, save key unencrypted (user can encrypt manually)
			// When bibd starts, it will re-encrypt with its identity key
			caKeyPath := filepath.Join(outputDir, "ca.key")
			if err := os.WriteFile(caKeyPath, caKey, 0600); err != nil {
				return fmt.Errorf("failed to save CA key: %w", err)
			}

			// Calculate and display fingerprint
			fp, err := certs.Fingerprint(caCert)
			if err != nil {
				return fmt.Errorf("failed to calculate fingerprint: %w", err)
			}

			fmt.Printf("✓ CA certificate initialized\n\n")
			fmt.Printf("  Certificate: %s\n", caCertPath)
			fmt.Printf("  Private Key: %s\n", caKeyPath)
			fmt.Printf("  Fingerprint: %s\n", fp)
			fmt.Printf("\n⚠️  Keep the private key secure. When bibd starts, it will\n")
			fmt.Printf("   encrypt the key using the node's P2P identity.\n")

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for certificates")
	cmd.Flags().IntVar(&caValidityYears, "ca-validity", 10, "CA certificate validity in years")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing CA")

	return cmd
}
