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

func newGenerateCommand() *cobra.Command {
	var (
		name           string
		outputDir      string
		userID         string
		sshFingerprint string
		validityDays   int
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a client certificate",
		Long: `Generate a client certificate signed by the daemon's CA.

The client certificate is used for mTLS authentication to bibd.
It is optionally bound to the user's SSH key fingerprint for identity linking.

Example:
  bib cert generate --name mydevice
  bib cert generate --name laptop --user-id user123
  bib cert generate --name workstation --ssh-fingerprint SHA256:abc...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			// Determine directories
			configDir, err := config.UserConfigDir(config.AppBibd)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			certsDir := filepath.Join(configDir, "certs")
			if outputDir == "" {
				outputDir = filepath.Join(configDir, "client_certs")
			}

			// Load CA certificate and key
			caCertPath := filepath.Join(certsDir, "ca.crt")
			caKeyPath := filepath.Join(certsDir, "ca.key")

			caCert, err := os.ReadFile(caCertPath)
			if err != nil {
				return fmt.Errorf("failed to read CA certificate: %w\nRun 'bib cert init' first or start bibd to auto-generate", err)
			}

			caKey, err := os.ReadFile(caKeyPath)
			if err != nil {
				// Try encrypted key
				encKeyPath := filepath.Join(configDir, "secrets", "ca.key.enc")
				if _, statErr := os.Stat(encKeyPath); statErr == nil {
					return fmt.Errorf("CA key is encrypted. Generate client certs via bibd or decrypt first")
				}
				return fmt.Errorf("failed to read CA key: %w", err)
			}

			// Generate client certificate
			cfg := certs.DefaultConfig("client")
			cfg.ClientCommonName = name
			cfg.UserID = userID
			cfg.SSHKeyFingerprint = sshFingerprint
			if validityDays > 0 {
				cfg.ClientValidDuration = cfg.ClientValidDuration * time.Duration(validityDays) / 90
			}

			clientCert, clientKey, err := certs.GenerateClientCert(caCert, caKey, cfg)
			if err != nil {
				return fmt.Errorf("failed to generate client certificate: %w", err)
			}

			// Create output directory
			clientDir := filepath.Join(outputDir, name)
			if err := os.MkdirAll(clientDir, 0700); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			// Save client cert and key
			certPath := filepath.Join(clientDir, "client.crt")
			keyPath := filepath.Join(clientDir, "client.key")

			if err := os.WriteFile(certPath, clientCert, 0644); err != nil {
				return fmt.Errorf("failed to save client certificate: %w", err)
			}
			if err := os.WriteFile(keyPath, clientKey, 0600); err != nil {
				return fmt.Errorf("failed to save client key: %w", err)
			}

			// Also copy CA cert for convenience
			caCertCopyPath := filepath.Join(clientDir, "ca.crt")
			if err := os.WriteFile(caCertCopyPath, caCert, 0644); err != nil {
				return fmt.Errorf("failed to copy CA certificate: %w", err)
			}

			// Calculate fingerprint
			fp, err := certs.Fingerprint(clientCert)
			if err != nil {
				return fmt.Errorf("failed to calculate fingerprint: %w", err)
			}

			fmt.Printf("✓ Client certificate generated\n\n")
			fmt.Printf("  Name:        %s\n", name)
			fmt.Printf("  Certificate: %s\n", certPath)
			fmt.Printf("  Private Key: %s\n", keyPath)
			fmt.Printf("  CA Cert:     %s\n", caCertCopyPath)
			fmt.Printf("  Fingerprint: %s\n", fp)
			if sshFingerprint != "" {
				fmt.Printf("  SSH Binding: %s\n", sshFingerprint)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the client certificate (required)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for certificates")
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID to embed in certificate")
	cmd.Flags().StringVar(&sshFingerprint, "ssh-fingerprint", "", "SSH key fingerprint to bind to certificate")
	cmd.Flags().IntVar(&validityDays, "validity", 90, "Certificate validity in days")

	cmd.MarkFlagRequired("name")

	return cmd
}
