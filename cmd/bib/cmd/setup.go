package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

var (
	setupDaemon bool
	setupFormat string
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up configuration interactively",
	Long: `Set up configuration for bib or bibd interactively.

Use --daemon to configure the bibd daemon instead of the bib CLI.

Examples:
  bib setup           # Configure bib CLI
  bib setup --daemon  # Configure bibd daemon`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)

	setupCmd.Flags().BoolVarP(&setupDaemon, "daemon", "d", false, "configure bibd daemon instead of bib CLI")
	setupCmd.Flags().StringVarP(&setupFormat, "format", "f", "yaml", "config file format (yaml, toml, json)")
}

func runSetup(cmd *cobra.Command, args []string) error {
	if setupDaemon {
		return setupBibd()
	}
	return setupBib()
}

func setupBib() error {
	fmt.Println("=== bib CLI Configuration Setup ===")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Get identity info
	name := prompt(reader, "Your name", "")
	email := prompt(reader, "Your email", "")

	// Get output preferences
	outputFormat := prompt(reader, "Default output format (text/json/yaml/table)", "text")
	colorStr := prompt(reader, "Enable colored output (yes/no)", "yes")
	color := strings.ToLower(colorStr) == "yes" || strings.ToLower(colorStr) == "y"

	// Get server address
	server := prompt(reader, "bibd server address", "localhost:8080")

	// Get log level
	logLevel := prompt(reader, "Log level (debug/info/warn/error)", "info")

	// Create config directory
	configDir, err := config.UserConfigDir(config.AppBib)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Generate config file with user values
	cfg := &config.BibConfig{
		Log: config.LogConfig{
			Level:  logLevel,
			Format: "text",
			Output: "stderr",
		},
		Identity: config.IdentityConfig{
			Name:  name,
			Email: email,
		},
		Output: config.OutputConfig{
			Format: outputFormat,
			Color:  color,
		},
		Server: server,
	}

	configPath, err := writeConfig(config.AppBib, setupFormat, cfg)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("✓ Configuration saved to: %s\n", configPath)
	return nil
}

func setupBibd() error {
	fmt.Println("=== bibd Daemon Configuration Setup ===")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Get identity info
	name := prompt(reader, "Daemon identity name", "")
	email := prompt(reader, "Daemon identity email", "")

	// Get server settings
	host := prompt(reader, "Listen host", "0.0.0.0")
	port := prompt(reader, "Listen port", "8080")

	// TLS settings
	tlsStr := prompt(reader, "Enable TLS (yes/no)", "no")
	tlsEnabled := strings.ToLower(tlsStr) == "yes" || strings.ToLower(tlsStr) == "y"

	var certFile, keyFile string
	if tlsEnabled {
		certFile = prompt(reader, "TLS certificate file path", "")
		keyFile = prompt(reader, "TLS key file path", "")
	}

	// Data directory
	homeDir, _ := os.UserHomeDir()
	defaultDataDir := homeDir + "/.local/share/bibd"
	dataDir := prompt(reader, "Data directory", defaultDataDir)

	// Log level
	logLevel := prompt(reader, "Log level (debug/info/warn/error)", "info")
	logFormat := prompt(reader, "Log format (text/json)", "json")

	// Create config directory
	configDir, err := config.UserConfigDir(config.AppBibd)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Parse port
	var portNum int
	fmt.Sscanf(port, "%d", &portNum)
	if portNum == 0 {
		portNum = 8080
	}

	// Generate config file with user values
	cfg := &config.BibdConfig{
		Log: config.LogConfig{
			Level:  logLevel,
			Format: logFormat,
			Output: "stdout",
		},
		Identity: config.IdentityConfig{
			Name:  name,
			Email: email,
		},
		Server: config.ServerConfig{
			Host:    host,
			Port:    portNum,
			DataDir: dataDir,
			PIDFile: "/var/run/bibd.pid",
			TLS: config.TLSConfig{
				Enabled:  tlsEnabled,
				CertFile: certFile,
				KeyFile:  keyFile,
			},
		},
	}

	configPath, err := writeConfig(config.AppBibd, setupFormat, cfg)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("✓ Configuration saved to: %s\n", configPath)
	return nil
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func writeConfig(appName, format string, cfg interface{}) (string, error) {
	configDir, err := config.UserConfigDir(appName)
	if err != nil {
		return "", err
	}

	configPath := fmt.Sprintf("%s/config.%s", configDir, format)

	// Use viper to write the config
	v := config.NewViperFromConfig(appName, cfg)
	v.SetConfigType(format)

	if err := v.WriteConfigAs(configPath); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	return configPath, nil
}
