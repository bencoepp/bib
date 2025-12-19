package cert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"bib/internal/certs"
	"bib/internal/config"

	"github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
	var (
		certsDir string
		showAll  bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List certificates",
		Long: `List certificates in the certificate directory.

Shows CA, server, and client certificates with their status and expiry.

Example:
  bib cert list
  bib cert list --all`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine certificates directory
			if certsDir == "" {
				configDir, err := config.UserConfigDir(config.AppBibd)
				if err != nil {
					return fmt.Errorf("failed to get config directory: %w", err)
				}
				certsDir = filepath.Join(configDir, "certs")
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TYPE\tNAME\tFINGERPRINT\tEXPIRES\tSTATUS")
			fmt.Fprintln(w, "----\t----\t-----------\t-------\t------")

			// List CA certificate
			caCertPath := filepath.Join(certsDir, "ca.crt")
			if data, err := os.ReadFile(caCertPath); err == nil {
				printCertRow(w, "CA", "ca.crt", data)
			}

			// List server certificate
			serverCertPath := filepath.Join(certsDir, "server.crt")
			if data, err := os.ReadFile(serverCertPath); err == nil {
				printCertRow(w, "Server", "server.crt", data)
			}

			// List client certificates
			clientCertsDir := filepath.Join(filepath.Dir(certsDir), "client_certs")
			if entries, err := os.ReadDir(clientCertsDir); err == nil {
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					certPath := filepath.Join(clientCertsDir, entry.Name(), "client.crt")
					if data, err := os.ReadFile(certPath); err == nil {
						printCertRow(w, "Client", entry.Name(), data)
					}
				}
			}

			w.Flush()
			return nil
		},
	}

	cmd.Flags().StringVar(&certsDir, "dir", "", "Certificates directory")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all certificates including expired")

	return cmd
}

func printCertRow(w *tabwriter.Writer, certType, name string, certPEM []byte) {
	cert, err := certs.ParseCertificate(certPEM)
	if err != nil {
		fmt.Fprintf(w, "%s\t%s\t-\t-\tINVALID\n", certType, name)
		return
	}

	// Format fingerprint (truncated)
	fp := cert.Fingerprint
	if len(fp) > 16 {
		fp = fp[:16] + "..."
	}

	// Format expiry
	daysUntilExpiry := int(time.Until(cert.ExpiresAt).Hours() / 24)
	var expiryStr string
	if daysUntilExpiry < 0 {
		expiryStr = "EXPIRED"
	} else if daysUntilExpiry == 0 {
		expiryStr = "Today"
	} else if daysUntilExpiry == 1 {
		expiryStr = "Tomorrow"
	} else if daysUntilExpiry < 30 {
		expiryStr = fmt.Sprintf("%d days", daysUntilExpiry)
	} else {
		expiryStr = cert.ExpiresAt.Format("2006-01-02")
	}

	// Determine status
	status := "OK"
	if daysUntilExpiry < 0 {
		status = "EXPIRED"
	} else if daysUntilExpiry < 30 {
		status = "RENEW"
	}

	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", certType, name, fp, expiryStr, status)
}

func newInfoCommand() *cobra.Command {
	var showPEM bool

	cmd := &cobra.Command{
		Use:   "info <cert-file>",
		Short: "Show certificate details",
		Long: `Show detailed information about a certificate.

Displays subject, issuer, validity dates, SANs, fingerprint, and more.

Example:
  bib cert info ~/.config/bibd/certs/server.crt
  bib cert info client.crt --pem`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			certPath := args[0]

			data, err := os.ReadFile(certPath)
			if err != nil {
				return fmt.Errorf("failed to read certificate: %w", err)
			}

			cert, err := certs.ParseCertificate(data)
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %w", err)
			}

			fmt.Printf("Certificate Information\n")
			fmt.Printf("=======================\n\n")
			fmt.Printf("Subject:      %s\n", cert.Subject)
			fmt.Printf("Issuer:       %s\n", cert.Issuer)
			fmt.Printf("Serial:       %s\n", cert.SerialHex)
			fmt.Printf("Is CA:        %v\n", cert.IsCA)
			fmt.Printf("\n")
			fmt.Printf("Valid From:   %s\n", cert.Cert.NotBefore.Format(time.RFC3339))
			fmt.Printf("Valid Until:  %s\n", cert.ExpiresAt.Format(time.RFC3339))

			daysUntilExpiry := int(time.Until(cert.ExpiresAt).Hours() / 24)
			if daysUntilExpiry < 0 {
				fmt.Printf("Status:       EXPIRED (%d days ago)\n", -daysUntilExpiry)
			} else if daysUntilExpiry < 30 {
				fmt.Printf("Status:       EXPIRING SOON (%d days)\n", daysUntilExpiry)
			} else {
				fmt.Printf("Status:       Valid (%d days remaining)\n", daysUntilExpiry)
			}

			fmt.Printf("\n")
			fmt.Printf("Fingerprint (SHA256):\n  %s\n", cert.Fingerprint)

			if len(cert.DNSNames) > 0 {
				fmt.Printf("\nDNS Names:\n")
				for _, name := range cert.DNSNames {
					fmt.Printf("  - %s\n", name)
				}
			}

			if len(cert.IPAddresses) > 0 {
				fmt.Printf("\nIP Addresses:\n")
				for _, ip := range cert.IPAddresses {
					fmt.Printf("  - %s\n", ip.String())
				}
			}

			// Show key usages
			if len(cert.Cert.ExtKeyUsage) > 0 {
				fmt.Printf("\nExtended Key Usage:\n")
				for _, usage := range cert.Cert.ExtKeyUsage {
					fmt.Printf("  - %s\n", extKeyUsageName(usage))
				}
			}

			// Check for SSH binding
			if strings.HasPrefix(cert.Cert.Subject.SerialNumber, "ssh-fp:") {
				sshFP := strings.TrimPrefix(cert.Cert.Subject.SerialNumber, "ssh-fp:")
				fmt.Printf("\nSSH Key Binding:\n  %s\n", sshFP)
			}

			// Check for user ID
			for _, ou := range cert.Cert.Subject.OrganizationalUnit {
				if strings.HasPrefix(ou, "user:") {
					userID := strings.TrimPrefix(ou, "user:")
					fmt.Printf("\nUser ID:\n  %s\n", userID)
				}
			}

			if showPEM {
				fmt.Printf("\nPEM:\n%s", string(data))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showPEM, "pem", false, "Also show PEM-encoded certificate")

	return cmd
}

func extKeyUsageName(usage interface{}) string {
	// Convert x509.ExtKeyUsage to readable name
	switch usage {
	case 1: // x509.ExtKeyUsageServerAuth
		return "Server Authentication"
	case 2: // x509.ExtKeyUsageClientAuth
		return "Client Authentication"
	default:
		return fmt.Sprintf("Unknown (%v)", usage)
	}
}
