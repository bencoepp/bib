package cert

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"software.sslmate.com/src/go-pkcs12"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

func newExportCommand() *cobra.Command {
	var (
		format   string
		output   string
		password string
		certName string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export certificates",
		Long: `Export certificates in various formats.

Supported formats:
  pem   - PEM encoded (default)
  p12   - PKCS#12 bundle (for browsers/Windows)
  der   - DER encoded

Example:
  bib cert export --name mydevice --format p12 --output mydevice.p12
  bib cert export --name mydevice --format pem`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if certName == "" {
				return fmt.Errorf("--name is required")
			}

			// Determine paths
			configDir, err := config.UserConfigDir(config.AppBibd)
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}

			clientDir := filepath.Join(configDir, "client_certs", certName)
			certPath := filepath.Join(clientDir, "client.crt")
			keyPath := filepath.Join(clientDir, "client.key")
			caPath := filepath.Join(clientDir, "ca.crt")

			// Read certificate and key
			certPEM, err := os.ReadFile(certPath)
			if err != nil {
				return fmt.Errorf("failed to read certificate: %w", err)
			}

			keyPEM, err := os.ReadFile(keyPath)
			if err != nil {
				return fmt.Errorf("failed to read private key: %w", err)
			}

			caPEM, err := os.ReadFile(caPath)
			if err != nil {
				// CA is optional for export
				caPEM = nil
			}

			switch format {
			case "pem":
				return exportPEM(certPEM, keyPEM, caPEM, output)
			case "p12", "pkcs12":
				if password == "" {
					return fmt.Errorf("--password is required for PKCS#12 export")
				}
				return exportPKCS12(certPEM, keyPEM, caPEM, password, output)
			case "der":
				return exportDER(certPEM, output)
			default:
				return fmt.Errorf("unsupported format: %s", format)
			}
		},
	}

	cmd.Flags().StringVar(&certName, "name", "", "Client certificate name to export (required)")
	cmd.Flags().StringVarP(&format, "format", "f", "pem", "Export format (pem, p12, der)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout for PEM)")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Password for PKCS#12 export")

	cmd.MarkFlagRequired("name")

	return cmd
}

func exportPEM(certPEM, keyPEM, caPEM []byte, output string) error {
	var data []byte
	data = append(data, certPEM...)
	data = append(data, keyPEM...)
	if caPEM != nil {
		data = append(data, caPEM...)
	}

	if output == "" {
		fmt.Print(string(data))
		return nil
	}

	if err := os.WriteFile(output, data, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Exported to %s\n", output)
	return nil
}

func exportPKCS12(certPEM, keyPEM, caPEM []byte, password, output string) error {
	// Parse certificate
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Parse private key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode key PEM")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Parse CA cert if provided
	var caCerts []*x509.Certificate
	if caPEM != nil {
		caBlock, _ := pem.Decode(caPEM)
		if caBlock != nil {
			caCert, err := x509.ParseCertificate(caBlock.Bytes)
			if err == nil {
				caCerts = append(caCerts, caCert)
			}
		}
	}

	// Create PKCS#12 bundle
	p12Data, err := pkcs12.Modern.Encode(key, cert, caCerts, password)
	if err != nil {
		return fmt.Errorf("failed to create PKCS#12 bundle: %w", err)
	}

	if output == "" {
		output = "client.p12"
	}

	if err := os.WriteFile(output, p12Data, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Exported to %s\n", output)
	return nil
}

func exportDER(certPEM []byte, output string) error {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode PEM")
	}

	if output == "" {
		output = "client.der"
	}

	if err := os.WriteFile(output, block.Bytes, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Exported to %s\n", output)
	return nil
}
