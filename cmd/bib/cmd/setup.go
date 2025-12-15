package cmd

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

var (
	setupDaemon      bool
	setupFormat      string
	setupCluster     bool
	setupClusterJoin string
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up configuration interactively",
	Long: `Set up configuration for bib or bibd interactively.

Use --daemon to configure the bibd daemon instead of the bib CLI.
Use --cluster to initialize a new HA cluster (returns a join token).
Use --cluster-join <token> to join an existing cluster.

Examples:
  bib setup                           # Configure bib CLI
  bib setup --daemon                  # Configure bibd daemon
  bib setup --daemon --cluster        # Initialize new cluster (outputs join token)
  bib setup --daemon --cluster-join <token>  # Join existing cluster`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)

	setupCmd.Flags().BoolVarP(&setupDaemon, "daemon", "d", false, "configure bibd daemon instead of bib CLI")
	setupCmd.Flags().StringVarP(&setupFormat, "format", "f", "yaml", "config file format (yaml, toml, json)")
	setupCmd.Flags().BoolVar(&setupCluster, "cluster", false, "initialize a new HA cluster (outputs join token)")
	setupCmd.Flags().StringVar(&setupClusterJoin, "cluster-join", "", "join an existing cluster using this token")
}

func runSetup(cmd *cobra.Command, args []string) error {
	if setupDaemon {
		if setupCluster {
			return setupBibdCluster()
		}
		if setupClusterJoin != "" {
			return setupBibdJoinCluster()
		}
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

// JoinTokenData contains the information encoded in a join token
type JoinTokenData struct {
	ClusterName string `json:"cluster_name"`
	LeaderAddr  string `json:"leader_addr"`
	Token       string `json:"token"`
	ExpiresAt   int64  `json:"expires_at"`
}

// setupBibdCluster initializes a new HA cluster
func setupBibdCluster() error {
	fmt.Println("=== bibd HA Cluster Initialization ===")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// First, run the normal daemon setup
	fmt.Println("First, let's configure the daemon...")
	fmt.Println()

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

	fmt.Println()
	fmt.Println("Now, let's configure the cluster...")
	fmt.Println()

	// Cluster settings
	clusterName := prompt(reader, "Cluster name", "bib-cluster")
	clusterAddr := prompt(reader, "Cluster listen address (for Raft)", "0.0.0.0:4002")
	advertiseAddr := prompt(reader, "Cluster advertise address (accessible by other nodes)", clusterAddr)

	// Parse port
	var portNum int
	fmt.Sscanf(port, "%d", &portNum)
	if portNum == 0 {
		portNum = 8080
	}

	// Generate node ID
	nodeID, err := generateNodeID()
	if err != nil {
		return fmt.Errorf("failed to generate node ID: %w", err)
	}

	// Create config directory
	configDir, err := config.UserConfigDir(config.AppBibd)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
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
		Cluster: config.ClusterConfig{
			Enabled:       true,
			NodeID:        nodeID,
			ClusterName:   clusterName,
			ListenAddr:    clusterAddr,
			AdvertiseAddr: advertiseAddr,
			IsVoter:       true, // First node is always a voter
			Bootstrap:     true, // This is the bootstrap node
		},
	}

	configPath, err := writeConfig(config.AppBibd, setupFormat, cfg)
	if err != nil {
		return err
	}

	// Generate join token for other nodes
	joinToken, err := generateJoinToken(clusterName, advertiseAddr)
	if err != nil {
		return fmt.Errorf("failed to generate join token: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Configuration saved to: %s\n", configPath)
	fmt.Println()
	fmt.Println("=== Cluster Initialized ===")
	fmt.Printf("Node ID: %s\n", nodeID)
	fmt.Printf("Cluster Name: %s\n", clusterName)
	fmt.Println()
	fmt.Println("Share this join token with other nodes to join the cluster:")
	fmt.Println()
	fmt.Printf("  %s\n", joinToken)
	fmt.Println()
	fmt.Println("To join another node to this cluster, run:")
	fmt.Printf("  bib setup --daemon --cluster-join %s\n", joinToken)
	fmt.Println()
	fmt.Println("NOTE: A minimum of 3 voting nodes is required for HA quorum.")

	return nil
}

// setupBibdJoinCluster joins an existing HA cluster
func setupBibdJoinCluster() error {
	fmt.Println("=== bibd Join HA Cluster ===")
	fmt.Println()

	// Decode join token
	tokenData, err := decodeJoinToken(setupClusterJoin)
	if err != nil {
		return fmt.Errorf("invalid join token: %w", err)
	}

	// Check if token is expired
	if time.Now().Unix() > tokenData.ExpiresAt {
		return fmt.Errorf("join token has expired")
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Joining cluster: %s\n", tokenData.ClusterName)
	fmt.Printf("Leader address: %s\n", tokenData.LeaderAddr)
	fmt.Println()

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

	// Cluster settings
	clusterAddr := prompt(reader, "Cluster listen address (for Raft)", "0.0.0.0:4002")
	advertiseAddr := prompt(reader, "Cluster advertise address (accessible by other nodes)", clusterAddr)
	isVoterStr := prompt(reader, "Join as voting member (yes/no)", "yes")
	isVoter := strings.ToLower(isVoterStr) == "yes" || strings.ToLower(isVoterStr) == "y"

	// Parse port
	var portNum int
	fmt.Sscanf(port, "%d", &portNum)
	if portNum == 0 {
		portNum = 8080
	}

	// Generate node ID
	nodeID, err := generateNodeID()
	if err != nil {
		return fmt.Errorf("failed to generate node ID: %w", err)
	}

	// Create config directory
	configDir, err := config.UserConfigDir(config.AppBibd)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
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
		Cluster: config.ClusterConfig{
			Enabled:       true,
			NodeID:        nodeID,
			ClusterName:   tokenData.ClusterName,
			ListenAddr:    clusterAddr,
			AdvertiseAddr: advertiseAddr,
			IsVoter:       isVoter,
			Bootstrap:     false, // Not a bootstrap node
			JoinToken:     setupClusterJoin,
			JoinAddrs:     []string{tokenData.LeaderAddr},
		},
	}

	configPath, err := writeConfig(config.AppBibd, setupFormat, cfg)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("✓ Configuration saved to: %s\n", configPath)
	fmt.Println()
	fmt.Printf("Node ID: %s\n", nodeID)
	fmt.Printf("Cluster: %s\n", tokenData.ClusterName)
	if isVoter {
		fmt.Println("Role: Voter (participates in leader election)")
	} else {
		fmt.Println("Role: Non-Voter (replicates data, no voting)")
	}
	fmt.Println()
	fmt.Println("Start the daemon to complete joining the cluster:")
	fmt.Println("  bibd")
	fmt.Println()
	fmt.Println("NOTE: A minimum of 3 voting nodes is required for HA quorum.")

	return nil
}

// generateNodeID generates a unique node ID
func generateNodeID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// generateJoinToken creates an encoded join token
func generateJoinToken(clusterName, leaderAddr string) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	data := JoinTokenData{
		ClusterName: clusterName,
		LeaderAddr:  leaderAddr,
		Token:       fmt.Sprintf("%x", tokenBytes),
		ExpiresAt:   time.Now().Add(24 * time.Hour).Unix(), // 24 hour expiry
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(jsonData), nil
}

// decodeJoinToken decodes a join token
func decodeJoinToken(token string) (*JoinTokenData, error) {
	jsonData, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	var data JoinTokenData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	return &data, nil
}
