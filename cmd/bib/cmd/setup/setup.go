package setup

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bib/internal/auth"
	"bib/internal/config"
	"bib/internal/deploy"
	"bib/internal/deploy/local"
	"bib/internal/discovery"
	"bib/internal/tui"
	"bib/internal/tui/component"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// DeploymentTarget represents the target environment for bibd deployment
type DeploymentTarget string

const (
	// TargetLocal runs bibd directly on the host machine
	TargetLocal DeploymentTarget = "local"
	// TargetDocker runs bibd in Docker containers
	TargetDocker DeploymentTarget = "docker"
	// TargetPodman runs bibd in Podman containers
	TargetPodman DeploymentTarget = "podman"
	// TargetKubernetes deploys bibd to a Kubernetes cluster
	TargetKubernetes DeploymentTarget = "kubernetes"
)

// ValidTargets returns all valid deployment targets
func ValidTargets() []DeploymentTarget {
	return []DeploymentTarget{TargetLocal, TargetDocker, TargetPodman, TargetKubernetes}
}

// IsValid checks if a deployment target is valid
func (t DeploymentTarget) IsValid() bool {
	for _, valid := range ValidTargets() {
		if t == valid {
			return true
		}
	}
	return false
}

// ValidReconfigureSections returns valid sections for --reconfigure
func ValidReconfigureSections(isDaemon bool) []string {
	if isDaemon {
		return []string{
			"identity",
			"server",
			"tls",
			"storage",
			"p2p",
			"p2p-mode",
			"bootstrap",
			"logging",
			"cluster",
			"break-glass",
		}
	}
	return []string{
		"identity",
		"output",
		"connection",
		"logging",
	}
}

var (
	setupDaemon      bool
	setupFormat      string
	setupCluster     bool
	setupClusterJoin string

	// New flags for enhanced setup flow
	setupQuick       bool
	setupTarget      string
	setupReconfigure string
	setupFresh       bool
)

// Cmd represents the setup command
var Cmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure bib CLI or bibd daemon",
	Long: `Interactive setup wizard for configuring bib CLI or bibd daemon.

The setup wizard guides you through configuration with sensible defaults.
Use --quick for minimal prompts, or run without flags for full guided setup.

For daemon setup, use --target to specify where bibd will run:
  - local:      Run directly on this machine (default)
  - docker:     Run in Docker containers
  - podman:     Run in Podman containers (rootful or rootless)
  - kubernetes: Deploy to a Kubernetes cluster`,
	Example: `  # Quick CLI setup (minimal prompts)
  bib setup --quick

  # Full interactive CLI setup
  bib setup

  # Quick daemon setup (local, Proxy mode)
  bib setup --daemon --quick

  # Daemon setup for Docker
  bib setup --daemon --target docker

  # Daemon setup for Kubernetes
  bib setup --daemon --target kubernetes

  # Initialize HA cluster
  bib setup --daemon --cluster

  # Join existing cluster
  bib setup --daemon --cluster-join <token>

  # Reconfigure specific section
  bib setup --reconfigure identity
  bib setup --daemon --reconfigure p2p-mode

  # Reset and start fresh
  bib setup --fresh`,
	Annotations: map[string]string{"i18n": "true"},
	RunE:        runSetup,
}

// NewCommand returns the setup command
func NewCommand() *cobra.Command {
	return Cmd
}

func init() {
	// Existing flags
	Cmd.Flags().BoolVarP(&setupDaemon, "daemon", "d", false, "configure bibd daemon instead of bib CLI")
	Cmd.Flags().StringVarP(&setupFormat, "format", "f", "yaml", "config file format (yaml, toml, json)")
	Cmd.Flags().BoolVar(&setupCluster, "cluster", false, "initialize a new HA cluster (outputs join token)")
	Cmd.Flags().StringVar(&setupClusterJoin, "cluster-join", "", "join an existing cluster using this token")

	// New flags for enhanced setup flow
	Cmd.Flags().BoolVarP(&setupQuick, "quick", "q", false, "quick start with minimal prompts and sensible defaults")
	Cmd.Flags().StringVarP(&setupTarget, "target", "t", "local", "deployment target: local, docker, podman, kubernetes (requires --daemon)")
	Cmd.Flags().StringVar(&setupReconfigure, "reconfigure", "", "reconfigure a specific section without full wizard")
	Cmd.Flags().BoolVar(&setupFresh, "fresh", false, "reset configuration and start fresh (deletes existing config)")
}

func runSetup(cmd *cobra.Command, args []string) error {
	// Validate flags
	if err := validateSetupFlags(); err != nil {
		return err
	}

	// Handle --fresh flag: delete existing config before proceeding
	if setupFresh {
		if err := handleFreshSetup(); err != nil {
			return err
		}
	}

	// Handle --reconfigure flag: run partial wizard for specific section
	if setupReconfigure != "" {
		return runReconfigure(setupReconfigure, setupDaemon)
	}

	// Check for partial config and offer resume (unless --fresh was used)
	if !setupFresh && !setupQuick {
		resumed, err := checkAndOfferResume(setupDaemon)
		if err != nil {
			return err
		}
		if resumed {
			return nil // User chose to resume and wizard completed
		}
	}

	// Daemon setup
	if setupDaemon {
		if setupCluster {
			return setupBibdCluster()
		}
		if setupClusterJoin != "" {
			return setupBibdJoinCluster()
		}
		if setupQuick {
			return setupBibdQuick()
		}
		return setupBibdWizard()
	}

	// CLI setup
	if setupQuick {
		return setupBibQuick()
	}
	return setupBibWizard()
}

// validateSetupFlags validates flag combinations and values
func validateSetupFlags() error {
	// Validate --target: only valid with --daemon
	if setupTarget != "local" && !setupDaemon {
		return fmt.Errorf("--target flag requires --daemon flag")
	}

	// Validate --target value
	target := DeploymentTarget(setupTarget)
	if !target.IsValid() {
		validTargets := make([]string, len(ValidTargets()))
		for i, t := range ValidTargets() {
			validTargets[i] = string(t)
		}
		return fmt.Errorf("invalid deployment target %q, must be one of: %s", setupTarget, strings.Join(validTargets, ", "))
	}

	// Validate --cluster and --cluster-join: only valid with --daemon
	if (setupCluster || setupClusterJoin != "") && !setupDaemon {
		return fmt.Errorf("--cluster and --cluster-join flags require --daemon flag")
	}

	// Validate --cluster and --cluster-join are mutually exclusive
	if setupCluster && setupClusterJoin != "" {
		return fmt.Errorf("--cluster and --cluster-join are mutually exclusive")
	}

	// Validate --reconfigure section
	if setupReconfigure != "" {
		validSections := ValidReconfigureSections(setupDaemon)
		isValid := false
		for _, section := range validSections {
			if setupReconfigure == section {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid reconfigure section %q, must be one of: %s", setupReconfigure, strings.Join(validSections, ", "))
		}
	}

	// Validate --format value
	validFormats := []string{"yaml", "toml", "json"}
	isValidFormat := false
	for _, f := range validFormats {
		if setupFormat == f {
			isValidFormat = true
			break
		}
	}
	if !isValidFormat {
		return fmt.Errorf("invalid format %q, must be one of: %s", setupFormat, strings.Join(validFormats, ", "))
	}

	return nil
}

// handleFreshSetup deletes existing configuration to start fresh
func handleFreshSetup() error {
	var appName string
	if setupDaemon {
		appName = config.AppBibd
	} else {
		appName = config.AppBib
	}

	configDir, err := config.UserConfigDir(appName)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	configPath := configDir + "/config.yaml"
	partialPath := configDir + "/config.yaml.partial"

	// Check if config exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("⚠️  This will delete your existing configuration at:\n   %s\n\n", configPath)
		fmt.Print("Are you sure you want to continue? [y/N]: ")

		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			// If user just hits enter, treat as "no"
			response = "n"
		}
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}

		// Delete existing config
		if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete config file: %w", err)
		}
		fmt.Println("✓ Deleted existing configuration")
	}

	// Delete partial config if exists
	if err := os.Remove(partialPath); err == nil {
		fmt.Println("✓ Deleted partial configuration")
	}

	return nil
}

// runReconfigure runs the wizard for a specific section only
func runReconfigure(section string, isDaemon bool) error {
	// TODO: Implement reconfigure for specific sections (Phase 10.4)
	// For now, show a message that this feature is coming
	fmt.Printf("Reconfiguring section: %s\n", section)
	fmt.Println("Note: Full reconfigure support is coming soon. For now, please run the full setup wizard.")
	return nil
}

// checkAndOfferResume checks for partial config and offers to resume
// Returns true if the user chose to resume and the wizard completed
func checkAndOfferResume(isDaemon bool) (bool, error) {
	var appName string
	if isDaemon {
		appName = config.AppBibd
	} else {
		appName = config.AppBib
	}

	// Check for partial config
	progress, err := config.DetectPartialConfig(appName)
	if err != nil {
		// Log warning but don't fail - just continue with fresh setup
		fmt.Printf("Warning: could not check for partial config: %v\n", err)
		return false, nil
	}

	if progress == nil {
		// No partial config found
		return false, nil
	}

	// Show resume prompt
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────────────┐")
	fmt.Println("│              Partial Configuration Detected                  │")
	fmt.Println("├─────────────────────────────────────────────────────────────┤")
	fmt.Println("│                                                              │")
	fmt.Printf("│  A previous setup was interrupted at step %d of %d:          │\n",
		progress.CurrentStepIndex+1, progress.TotalSteps)
	if progress.CurrentStepID != "" {
		fmt.Printf("│  \"%s\"%-45s│\n", progress.CurrentStepID, "")
	}
	fmt.Println("│                                                              │")
	fmt.Printf("│  Started: %s%-35s│\n", progress.StartedAt.Format("2006-01-02 15:04"), "")
	fmt.Printf("│  Progress: %d%% complete%-40s│\n", progress.ProgressPercentage(), "")
	fmt.Println("│                                                              │")
	fmt.Println("└─────────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("What would you like to do?")
	fmt.Println("  [R] Resume from where you left off")
	fmt.Println("  [S] Start over (delete partial config)")
	fmt.Println("  [C] Cancel")
	fmt.Println()
	fmt.Print("Choice [R/s/c]: ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		response = "r" // Default to resume
	}
	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "r", "":
		// Resume - load the saved data and continue wizard
		fmt.Println("\nResuming setup...")
		return resumeSetup(progress, isDaemon)

	case "s":
		// Start over - delete partial config
		if err := config.DeletePartialConfig(appName); err != nil {
			fmt.Printf("Warning: failed to delete partial config: %v\n", err)
		} else {
			fmt.Println("✓ Deleted partial configuration")
		}
		return false, nil

	case "c":
		// Cancel
		fmt.Println("Cancelled.")
		os.Exit(0)
		return false, nil

	default:
		fmt.Println("Invalid choice. Resuming...")
		return resumeSetup(progress, isDaemon)
	}
}

// resumeSetup resumes a wizard from saved progress
func resumeSetup(progress *config.SetupProgress, isDaemon bool) (bool, error) {
	// Load the saved setup data
	data := tui.DefaultSetupData()
	if err := progress.GetData(data); err != nil {
		fmt.Printf("Warning: could not load saved data, starting fresh: %v\n", err)
		return false, nil
	}

	// Run the wizard starting from the saved step
	if isDaemon {
		return runDaemonWizardWithProgress(data, progress)
	}
	return runCLIWizardWithProgress(data, progress)
}

// runCLIWizardWithProgress runs the CLI wizard from saved progress
func runCLIWizardWithProgress(data *tui.SetupData, progress *config.SetupProgress) (bool, error) {
	// TODO: Implement resume for CLI wizard (integrate with SetupWizardModel)
	// For now, just run the normal wizard with the loaded data
	fmt.Printf("Resuming from step: %s\n", progress.CurrentStepID)

	// Delete the partial config since we're resuming
	config.DeletePartialConfig(progress.AppName)

	// Run normal wizard - the data is already loaded
	return false, nil
}

// runDaemonWizardWithProgress runs the daemon wizard from saved progress
func runDaemonWizardWithProgress(data *tui.SetupData, progress *config.SetupProgress) (bool, error) {
	// TODO: Implement resume for daemon wizard (integrate with SetupWizardModel)
	// For now, just run the normal wizard with the loaded data
	fmt.Printf("Resuming from step: %s\n", progress.CurrentStepID)

	// Delete the partial config since we're resuming
	config.DeletePartialConfig(progress.AppName)

	// Run normal wizard - the data is already loaded
	return false, nil
}

// setupBibQuick runs quick CLI setup with minimal prompts
func setupBibQuick() error {
	fmt.Println("🚀 Quick Setup - bib CLI")
	fmt.Println()

	// Create setup data with defaults
	data := tui.DefaultSetupData()

	// Get the huh theme
	theme := huh.ThemeCatppuccin()

	// Step 1: Prompt for name and email only
	var name, email string
	nameEmailForm := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("👤 Quick Setup").
				Description("We just need a few details to get you started.\n\nYour name and email are used for your identity key."),
			huh.NewInput().
				Title("Your Name").
				Description("Display name for your identity").
				Placeholder("John Doe").
				Value(&name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Email").
				Description("Email address for your identity").
				Placeholder("you@example.com").
				Value(&email).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("email is required")
					}
					if !strings.Contains(s, "@") {
						return fmt.Errorf("please enter a valid email address")
					}
					return nil
				}),
		),
	).WithTheme(theme)

	if err := nameEmailForm.Run(); err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nSetup cancelled.")
			return nil
		}
		return err
	}

	data.Name = name
	data.Email = email

	// Step 2: Generate identity key
	fmt.Println("\n🔑 Generating identity key...")
	identityKey, err := auth.GenerateIdentityKey()
	if err != nil {
		return fmt.Errorf("failed to generate identity key: %w", err)
	}

	keyPath, err := auth.DefaultIdentityKeyPath(config.AppBib)
	if err != nil {
		return fmt.Errorf("failed to get identity key path: %w", err)
	}
	if err := identityKey.Save(keyPath); err != nil {
		return fmt.Errorf("failed to save identity key: %w", err)
	}
	data.IdentityKeyPath = keyPath
	fmt.Printf("   ✓ Key saved to %s\n", keyPath)
	fmt.Printf("   ✓ Fingerprint: %s\n", identityKey.Fingerprint())

	// Step 3: Auto-discover local nodes
	fmt.Println("\n🔍 Discovering local nodes...")
	discoverer := discovery.NewWithDefaults()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := discoverer.Discover(ctx)

	localNodes := []discovery.DiscoveredNode{}
	for _, node := range result.Nodes {
		if node.Method == discovery.MethodLocal || node.Method == discovery.MethodMDNS {
			localNodes = append(localNodes, node)
		}
	}

	// Step 4: Decide on bib.dev
	useBibDev := false
	if len(localNodes) == 0 {
		fmt.Println("   No local nodes found.")
		fmt.Println()

		// Prompt for bib.dev confirmation
		bibDevForm := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🌐 Connect to bib.dev?").
					Description("No local bibd instances were found.\n\nbib.dev is the PUBLIC bib network where you can:\n  • Collaborate with users worldwide\n  • Access publicly shared datasets\n  • Contribute to the bib ecosystem\n\n⚠️  Your identity will be visible on the public network."),
				huh.NewConfirm().
					Title("Connect to bib.dev public network?").
					Affirmative("Yes, connect").
					Negative("No, skip for now").
					Value(&useBibDev),
			),
		).WithTheme(theme)

		if err := bibDevForm.Run(); err != nil {
			if err == huh.ErrUserAborted {
				fmt.Println("\nSetup cancelled.")
				return nil
			}
			return err
		}

		data.BibDevConfirmed = useBibDev
	} else {
		fmt.Printf("   ✓ Found %d local node(s)\n", len(localNodes))
	}

	// Step 5: Configure selected nodes
	if len(localNodes) > 0 {
		// Add local nodes
		for i, node := range localNodes {
			alias := fmt.Sprintf("Local (%s)", node.Address)
			if node.NodeInfo != nil && node.NodeInfo.Name != "" {
				alias = node.NodeInfo.Name
			}
			data.AddSelectedNode(node.Address, alias, string(node.Method), i == 0)
		}
		data.ServerAddr = localNodes[0].Address
	}

	if useBibDev {
		// Add bib.dev
		data.AddSelectedNode("bib.dev:4000", "bib.dev (Public Network)", "public", len(localNodes) == 0)
		if len(localNodes) == 0 {
			data.ServerAddr = "bib.dev:4000"
		}
	}

	// Step 6: Test connections
	if len(data.SelectedNodes) > 0 {
		fmt.Println("\n🔌 Testing connections...")
		tester := discovery.NewConnectionTester().WithTimeout(5 * time.Second)
		addresses := make([]string, len(data.SelectedNodes))
		for i, n := range data.SelectedNodes {
			addresses[i] = n.Address
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		results := tester.TestConnections(ctx, addresses)
		cancel()

		connected := 0
		for _, r := range results {
			if r.Status == discovery.StatusConnected {
				connected++
				fmt.Printf("   ✓ %s connected (%s)\n", r.Address, r.Latency.Round(time.Millisecond))
			} else {
				fmt.Printf("   ✗ %s failed: %s\n", r.Address, r.Status)
			}
		}

		if connected == 0 {
			fmt.Println("\n⚠️  No nodes could be connected. You can configure manually later with 'bib setup'.")
		}
	} else {
		fmt.Println("\n⚠️  No nodes configured. You can add nodes later with 'bib connect'.")
	}

	// Step 7: Set default preferences
	data.OutputFormat = "table"
	data.ColorEnabled = true
	data.LogLevel = "info"

	// Step 8: Generate and save config
	fmt.Println("\n💾 Saving configuration...")
	cfg := data.ToBibConfig()
	configDir, err := config.UserConfigDir(config.AppBib)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}
	configPath := filepath.Join(configDir, "config.yaml")

	if err := config.SaveBib(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Printf("   ✓ Config saved to %s\n", configPath)

	// Step 9: Show summary and next steps
	fmt.Println("\n" + strings.Repeat("─", 50))
	fmt.Println("✅ Quick setup complete!")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Println()
	fmt.Printf("  Identity: %s <%s>\n", data.Name, data.Email)
	fmt.Printf("  Key:      %s\n", identityKey.Fingerprint())
	if len(data.SelectedNodes) > 0 {
		fmt.Printf("  Nodes:    %d configured\n", len(data.SelectedNodes))
		fmt.Printf("  Default:  %s\n", data.ServerAddr)
	} else {
		fmt.Println("  Nodes:    None configured")
	}
	fmt.Println()
	fmt.Println("Next steps:")
	if len(data.SelectedNodes) == 0 {
		fmt.Println("  • Run 'bib connect <address>' to connect to a node")
		fmt.Println("  • Run 'bib setup' for full configuration")
	} else {
		fmt.Println("  • Run 'bib status' to check connection status")
		fmt.Println("  • Run 'bib topic list' to see available topics")
	}
	fmt.Println("  • Run 'bib help' for more commands")
	fmt.Println()

	return nil
}

// setupBibdQuick runs quick daemon setup with minimal prompts
func setupBibdQuick() error {
	// TODO: Implement quick daemon setup (Phase 3.5, 4.5, 5.6, 6.7)
	// For now, fall back to full wizard
	fmt.Printf("Running quick setup for target: %s\n", setupTarget)
	return setupBibdWizard()
}

// GetDeploymentTarget returns the current deployment target
func GetDeploymentTarget() DeploymentTarget {
	return DeploymentTarget(setupTarget)
}

// SetupWizardModel wraps the wizard and huh form for a step-by-step setup
type SetupWizardModel struct {
	wizard      *tui.Wizard
	data        *tui.SetupData
	isDaemon    bool
	currentForm *huh.Form
	width       int
	height      int
	done        bool
	cancelled   bool
	configPath  string
	err         error

	// Progress tracking for partial config save
	progress *config.SetupProgress

	// Identity key for authentication
	identityKey    *auth.IdentityKey
	identityKeyNew bool // true if key was newly generated

	// Node discovery and selection (CLI only)
	discoveryResult      *discovery.DiscoveryResult
	nodeSelector         *component.NodeSelector
	discoveryDone        bool
	bibDevConfirmed      bool                              // User has explicitly confirmed bib.dev connection
	connectionResults    []*discovery.ConnectionTestResult // Results of connection tests
	connectionTested     bool                              // True if connection tests have been run
	authResults          []*discovery.AuthTestResult       // Results of auth tests
	authTested           bool                              // True if auth tests have been run
	networkHealthResults []*discovery.NetworkHealthResult  // Results of network health checks
	networkHealthChecked bool                              // True if network health has been checked

	// Deployment target selection (daemon only)
	targetSelector *component.TargetSelector
	targetDetected bool // True if target detection has been run

	// PostgreSQL connection test (daemon only)
	postgresTestResult *PostgresTestResult
	postgresTestDone   bool
	postgresPortStr    string // Temporary string for port input in forms

	// Bootstrap configuration (daemon only)
	customBootstrapInput string // Temporary input for adding custom bootstrap peers

	// Service installation (local daemon only)
	serviceInstaller *local.ServiceInstaller
	installService   bool
	userService      bool
}

// PostgresTestResult contains the result of a PostgreSQL connection test
type PostgresTestResult struct {
	Success       bool
	ServerVersion string
	Database      string
	User          string
	Duration      time.Duration
	Error         string
}

func newSetupWizardModel(isDaemon bool) *SetupWizardModel {
	data := tui.DefaultSetupData()

	// Set default identity key path based on app type
	var appName string
	if isDaemon {
		appName = config.AppBibd
	} else {
		appName = config.AppBib
	}
	if keyPath, err := auth.DefaultIdentityKeyPath(appName); err == nil {
		data.IdentityKeyPath = keyPath
	}

	steps := []tui.WizardStep{
		{
			ID:          "welcome",
			Title:       "Welcome",
			Description: "Let's get started!",
			HelpText:    "This wizard will guide you through configuring bib. You can press Esc to go back or Ctrl+C to cancel at any time.",
		},
		{
			ID:          "identity",
			Title:       "Identity",
			Description: "Configure your identity",
			HelpText:    "Your identity is used for signing and attributing changes. This information may be visible to others in a collaborative environment.",
		},
		{
			ID:          "identity-key",
			Title:       "Identity Key",
			Description: "Generate authentication key",
			HelpText:    "An Ed25519 keypair is generated for authenticating with bibd nodes. This key is separate from your SSH keys.",
		},
		{
			ID:          "deployment-target",
			Title:       "Deployment Target",
			Description: "Select where to deploy bibd",
			HelpText:    "Choose how you want to deploy bibd. Local installation runs directly on this machine. Container options (Docker/Podman) provide isolation. Kubernetes is for cluster deployments.",
			ShouldSkip:  func() bool { return !isDaemon },
		},
		{
			ID:          "output",
			Title:       "Output",
			Description: "Configure output settings",
			HelpText:    "These settings control how bib displays information. Table format is recommended for interactive use, JSON/YAML for scripting.",
			ShouldSkip:  func() bool { return isDaemon },
		},
		{
			ID:          "node-discovery",
			Title:       "Node Discovery",
			Description: "Discovering bibd nodes",
			HelpText:    "Scanning for local and network bibd nodes. This includes localhost ports, mDNS discovery, and nearby P2P peers.",
			ShouldSkip:  func() bool { return isDaemon },
		},
		{
			ID:          "node-selection",
			Title:       "Node Selection",
			Description: "Select nodes to connect to",
			HelpText:    "Select one or more bibd nodes to connect to. You can also add the public bib.dev network or enter custom addresses.",
			ShouldSkip:  func() bool { return isDaemon },
		},
		{
			ID:          "bib-dev-confirm",
			Title:       "Public Network",
			Description: "Confirm bib.dev connection",
			HelpText:    "bib.dev is a public network. Your public identity and published data will be visible to other users.",
			ShouldSkip: func() bool {
				// Skip if daemon setup OR if bib.dev is not selected
				if isDaemon {
					return true
				}
				// Check if node selector exists and bib.dev is selected
				// This will be evaluated when the wizard navigates to this step
				return false // Will be checked dynamically in the form
			},
		},
		{
			ID:          "connection",
			Title:       "Connection",
			Description: "Configure connection to bibd",
			HelpText:    "Review and confirm your node selections. The first selected node will be used as the default.",
			ShouldSkip:  func() bool { return isDaemon },
		},
		{
			ID:          "connection-test",
			Title:       "Connection Test",
			Description: "Testing node connections",
			HelpText:    "Testing connectivity to all selected nodes. This verifies network access and retrieves node information.",
			ShouldSkip:  func() bool { return isDaemon },
		},
		{
			ID:          "auth-test",
			Title:       "Authentication Test",
			Description: "Testing authentication",
			HelpText:    "Testing authentication with your identity key against connected nodes. This verifies your key is accepted.",
			ShouldSkip: func() bool {
				if isDaemon {
					return true
				}
				// Skip if no connection tests succeeded
				return false
			},
		},
		{
			ID:          "network-health",
			Title:       "Network Health",
			Description: "Checking network status",
			HelpText:    "Querying peer count, bootstrap connection status, and DHT status from connected nodes.",
			ShouldSkip: func() bool {
				if isDaemon {
					return true
				}
				return false
			},
		},
		{
			ID:          "server",
			Title:       "Server",
			Description: "Configure server settings",
			HelpText:    "These settings control how the daemon listens for connections. Use 0.0.0.0 to accept connections from any interface.",
			ShouldSkip:  func() bool { return !isDaemon },
		},
		{
			ID:          "tls",
			Title:       "TLS",
			Description: "Configure TLS encryption",
			HelpText:    "TLS encrypts connections between clients and the daemon. Recommended for production use.",
			ShouldSkip:  func() bool { return !isDaemon },
		},
		{
			ID:          "tls-certs",
			Title:       "TLS Certificates",
			Description: "Provide TLS certificate files",
			HelpText:    "Provide paths to your TLS certificate and private key. You can generate self-signed certificates with: openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes",
			ShouldSkip:  func() bool { return !isDaemon || !data.TLSEnabled },
		},
		{
			ID:          "storage",
			Title:       "Storage",
			Description: "Configure storage backend",
			HelpText:    "SQLite is lightweight and requires no setup. PostgreSQL is recommended for production and enables full data replication.",
			ShouldSkip:  func() bool { return !isDaemon },
		},
		{
			ID:          "postgres-config",
			Title:       "PostgreSQL Configuration",
			Description: "Configure PostgreSQL connection",
			HelpText:    "Configure how PostgreSQL should be deployed and connected.",
			ShouldSkip:  func() bool { return !isDaemon || data.StorageBackend != "postgres" },
		},
		{
			ID:          "postgres-test",
			Title:       "PostgreSQL Test",
			Description: "Testing PostgreSQL connection",
			HelpText:    "Testing the connection to PostgreSQL to ensure it's properly configured.",
			ShouldSkip:  func() bool { return !isDaemon || data.StorageBackend != "postgres" },
		},
		{
			ID:          "p2p",
			Title:       "P2P",
			Description: "Enable P2P networking",
			HelpText:    "P2P networking allows nodes to discover each other and share data without a central server.",
			ShouldSkip:  func() bool { return !isDaemon },
		},
		{
			ID:          "p2p-mode",
			Title:       "P2P Mode",
			Description: "Select P2P mode",
			HelpText:    "Proxy: Forwards requests, minimal resources.\nSelective: Subscribe to specific topics.\nFull: Replicate all data (requires PostgreSQL).",
			ShouldSkip:  func() bool { return !isDaemon || !data.P2PEnabled },
		},
		{
			ID:          "bootstrap-peers",
			Title:       "Bootstrap Peers",
			Description: "Configure bootstrap peers",
			HelpText:    "Bootstrap peers help your node discover other peers on the network. bib.dev is the public network bootstrap.",
			ShouldSkip:  func() bool { return !isDaemon || !data.P2PEnabled },
		},
		{
			ID:          "bootstrap-confirm",
			Title:       "Public Network",
			Description: "Confirm public network connection",
			HelpText:    "Connecting to the public network makes your node discoverable by other users worldwide.",
			ShouldSkip: func() bool {
				return !isDaemon || !data.P2PEnabled || !data.UsePublicBootstrap
			},
		},
		{
			ID:          "custom-bootstrap",
			Title:       "Custom Bootstrap",
			Description: "Add custom bootstrap peers",
			HelpText:    "Add additional bootstrap peers using multiaddr format, e.g., /ip4/1.2.3.4/tcp/4001/p2p/Qm...",
			ShouldSkip:  func() bool { return !isDaemon || !data.P2PEnabled },
		},
		{
			ID:          "logging",
			Title:       "Logging",
			Description: "Configure logging",
			HelpText:    "Debug level shows detailed information useful for troubleshooting. Info is recommended for normal operation.",
		},
		{
			ID:          "cluster",
			Title:       "Cluster",
			Description: "Enable clustering",
			HelpText:    "Clustering provides high availability through Raft consensus. Requires at least 3 voting nodes for quorum.",
			ShouldSkip:  func() bool { return !isDaemon },
		},
		{
			ID:          "cluster-settings",
			Title:       "Cluster Settings",
			Description: "Configure cluster",
			HelpText:    "Cluster name must be unique. The Raft address is used for inter-node communication (separate from the API port).",
			ShouldSkip:  func() bool { return !isDaemon || !data.ClusterEnabled },
		},
		{
			ID:          "break-glass",
			Title:       "Break Glass",
			Description: "Emergency database access",
			HelpText:    "Break glass provides controlled emergency access to the database for disaster recovery. Disabled by default for security.",
			ShouldSkip:  func() bool { return !isDaemon },
		},
		{
			ID:          "break-glass-user",
			Title:       "Break Glass User",
			Description: "Configure emergency user",
			HelpText:    "Create an emergency access user with an Ed25519 SSH key. This user can enable break glass sessions when needed.",
			ShouldSkip:  func() bool { return !isDaemon || !data.BreakGlassEnabled },
		},
		{
			ID:          "service-install",
			Title:       "Service Installation",
			Description: "Install as system service",
			HelpText:    "Install bibd as a system service to run automatically at startup. You can choose user or system-level installation.",
			ShouldSkip: func() bool {
				// Only show for local daemon deployments
				return !isDaemon || data.DeploymentTarget != tui.DeployTargetLocal
			},
		},
		{
			ID:          "confirm",
			Title:       "Confirm",
			Description: "Review and save",
			HelpText:    "Review your settings and save the configuration.",
		},
	}

	m := &SetupWizardModel{
		data:     data,
		isDaemon: isDaemon,
	}

	// Initialize progress tracking for partial config save
	m.progress = config.NewSetupProgress(appName, isDaemon, len(steps))

	m.wizard = tui.NewWizard(
		getWizardTitle(isDaemon),
		getWizardDescription(isDaemon),
		steps,
		func() error { return m.saveConfig() },
		tui.WithCardWidth(65),
		tui.WithHelpPanel(35),
		tui.WithCentering(true),
	)

	return m
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// validatePort validates a port number string
func validatePort(s string) error {
	if s == "" {
		return nil // Will use default
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("port must be a number")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func getWizardTitle(isDaemon bool) string {
	if isDaemon {
		return "◆ bibd Daemon Setup"
	}
	return "◆ bib CLI Setup"
}

func getWizardDescription(isDaemon bool) string {
	if isDaemon {
		return "Configure the bibd daemon"
	}
	return "Configure the bib CLI"
}

func (m *SetupWizardModel) Init() tea.Cmd {
	// Initialize with welcome form
	m.updateFormForCurrentStep()
	cmds := []tea.Cmd{m.wizard.Init()}
	if m.currentForm != nil {
		cmds = append(cmds, m.currentForm.Init())
	}
	return tea.Batch(cmds...)
}

func (m *SetupWizardModel) updateFormForCurrentStep() {
	step := m.wizard.CurrentStep()
	if step == nil {
		m.currentForm = nil
		return
	}

	theme := tui.HuhTheme()

	switch step.ID {
	case "welcome":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title(tui.Banner()).
					Description(m.getWelcomeText()),
			),
		).WithTheme(theme).WithShowHelp(false)

	case "identity":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Name").
					Description("Your display name").
					Placeholder("John Doe").
					Value(&m.data.Name),
				huh.NewInput().
					Title("Email").
					Description("Your email address").
					Placeholder("john@example.com").
					Value(&m.data.Email),
			),
		).WithTheme(theme)

	case "identity-key":
		// Generate or load identity key if not already done
		if m.identityKey == nil {
			key, isNew, err := auth.LoadOrGenerateIdentityKey(m.data.IdentityKeyPath)
			if err != nil {
				m.err = err
				m.currentForm = huh.NewForm(
					huh.NewGroup(
						huh.NewNote().
							Title("❌ Key Generation Failed").
							Description(fmt.Sprintf("Failed to generate identity key: %v\n\nPlease check permissions and try again.", err)),
					),
				).WithTheme(theme)
				return
			}
			m.identityKey = key
			m.identityKeyNew = isNew
		}

		// Build key info display
		var statusMsg string
		if m.identityKeyNew {
			statusMsg = "✓ Generated new Ed25519 identity key"
		} else {
			statusMsg = "✓ Loaded existing identity key"
		}

		keyInfo := m.identityKey.Info()
		keyDisplay := fmt.Sprintf(`%s

Location:    %s
Fingerprint: %s
Public Key:  %s...

⚠️  Keep your identity key secure! It authenticates you
   to all bib nodes.`,
			statusMsg,
			keyInfo.Path,
			keyInfo.Fingerprint,
			truncateString(keyInfo.PublicKey, 50))

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🔑 Identity Key").
					Description(keyDisplay),
				huh.NewConfirm().
					Title("Continue?").
					Description("Press Enter to continue").
					Affirmative("Continue").
					Negative("Regenerate").
					Value(new(bool)),
			),
		).WithTheme(theme)

	case "deployment-target":
		// Run target detection if not already done
		if !m.targetDetected {
			m.runTargetDetection()
		}

		// Build target selection display
		var targetDisplay string
		var selectedTarget string

		if m.targetSelector != nil && m.targetSelector.DetectionDone {
			// Format detected targets
			var sb strings.Builder
			sb.WriteString("Detected deployment targets:\n\n")

			for i, target := range m.targetSelector.Targets {
				var icon, status string
				switch target.Type {
				case deploy.TargetLocal:
					icon = "🖥️ "
				case deploy.TargetDocker:
					icon = "🐳"
				case deploy.TargetPodman:
					icon = "🦭"
				case deploy.TargetKubernetes:
					icon = "☸️ "
				}

				if target.Available {
					status = "✓"
				} else {
					status = "✗"
				}

				cursor := "  "
				if i == m.targetSelector.Selected {
					cursor = "▸ "
					if target.Available {
						selectedTarget = string(target.Type)
					}
				}

				sb.WriteString(fmt.Sprintf("%s%s %s %s - %s\n",
					cursor, icon, status, deploy.TargetDisplayName(target.Type), target.Status))
			}

			// Show summary
			available := len(m.targetSelector.GetAvailableTargets())
			sb.WriteString(fmt.Sprintf("\n%d of %d targets available", available, len(m.targetSelector.Targets)))

			targetDisplay = sb.String()
		} else {
			targetDisplay = "Detecting available deployment targets..."
		}

		// Get target options for select
		targetOptions := []huh.Option[string]{
			huh.NewOption("Local Installation", "local"),
			huh.NewOption("Docker", "docker"),
			huh.NewOption("Podman", "podman"),
			huh.NewOption("Kubernetes", "kubernetes"),
		}

		// Default to local if nothing selected
		if selectedTarget == "" {
			selectedTarget = "local"
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🎯 Deployment Target").
					Description(targetDisplay),
				huh.NewSelect[string]().
					Title("Select deployment target").
					Description("Choose where to deploy bibd").
					Options(targetOptions...).
					Value(&selectedTarget),
			),
		).WithTheme(theme)

		// Store selection in data
		m.data.DeploymentTarget = selectedTarget

	case "output":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Output Format").
					Description("Default output format for commands").
					Options(
						huh.NewOption("Table", "table"),
						huh.NewOption("JSON", "json"),
						huh.NewOption("YAML", "yaml"),
						huh.NewOption("Text", "text"),
					).
					Value(&m.data.OutputFormat),
				huh.NewConfirm().
					Title("Enable Colors").
					Description("Use colored output in the terminal").
					Affirmative("Yes").
					Negative("No").
					Value(&m.data.ColorEnabled),
			),
		).WithTheme(theme)

	case "node-discovery":
		// Run discovery if not already done
		if !m.discoveryDone {
			m.runNodeDiscovery()
		}

		// Build discovery result display
		var resultDisplay string
		if m.discoveryResult != nil {
			resultDisplay = fmt.Sprintf("Discovery completed in %s\n\n%s",
				m.discoveryResult.Duration.Round(time.Millisecond),
				discovery.FormatDiscoveryResult(m.discoveryResult))
		} else {
			resultDisplay = "Running discovery..."
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🔍 Node Discovery").
					Description(resultDisplay),
				huh.NewConfirm().
					Title("Continue to node selection?").
					Affirmative("Continue").
					Negative("Retry").
					Value(new(bool)),
			),
		).WithTheme(theme)

	case "node-selection":
		// Initialize node selector if not already done
		if m.nodeSelector == nil {
			m.nodeSelector = component.NewNodeSelector().
				WithBibDev(true).
				WithAddCustom(true).
				WithMultiSelect(true).
				WithLatency(true)

			if m.discoveryResult != nil {
				m.nodeSelector.WithNodes(m.discoveryResult.Nodes)
			}

			// Auto-select first local node if available
			m.nodeSelector.SelectFirst()
		}

		// Build selection summary
		selectedNodes := m.nodeSelector.SelectedItems()
		var selectionSummary string
		if len(selectedNodes) == 0 {
			selectionSummary = "No nodes selected. Please select at least one node."
		} else {
			selectionSummary = fmt.Sprintf("%d node(s) selected", len(selectedNodes))
			if m.nodeSelector.IsBibDevSelected() {
				selectionSummary += " (including bib.dev public network)"
			}
		}

		// Show the node selector view
		selectorView := m.nodeSelector.ViewWidth(55)

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("📡 Select Nodes").
					Description(selectorView+"\n\n"+selectionSummary),
				huh.NewConfirm().
					Title("Confirm selection?").
					Affirmative("Confirm").
					Negative("Back").
					Value(new(bool)),
			),
		).WithTheme(theme)

	case "bib-dev-confirm":
		// Check if bib.dev is selected - if not, this step should be auto-skipped
		if m.nodeSelector == nil || !m.nodeSelector.IsBibDevSelected() {
			// Not selected, show a simple "skipping" form that auto-proceeds
			m.currentForm = huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("Public Network").
						Description("bib.dev not selected, skipping confirmation."),
				),
			).WithTheme(theme)
			return
		}

		// Build the confirmation dialog
		bibDevWarning := `☁️  You have selected to connect to bib.dev

bib.dev is the PUBLIC bib network. By connecting, you agree that:

  ⚠️  Your public identity (name, email) will be visible to others
  ⚠️  Any data you publish will be accessible to the public network
  ⚠️  Your IP address may be logged by public infrastructure
  ⚠️  You are subject to the bib.dev Terms of Service

This is recommended if you want to:
  ✓  Collaborate with users outside your local network
  ✓  Access publicly shared datasets and topics
  ✓  Contribute to the public bib ecosystem

If you only need local/private access, go back and deselect bib.dev.`

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🌐 Public Network Confirmation").
					Description(bibDevWarning),
				huh.NewConfirm().
					Title("Connect to bib.dev public network?").
					Description("This requires explicit confirmation").
					Affirmative("Yes, Connect to Public Network").
					Negative("No, Go Back").
					Value(&m.bibDevConfirmed),
			),
		).WithTheme(theme)

	case "connection":
		// Summarize selected nodes from the node selector
		var connectionSummary string
		if m.nodeSelector != nil && m.nodeSelector.HasSelection() {
			selectedItems := m.nodeSelector.SelectedItems()
			connectionSummary = fmt.Sprintf("Selected %d node(s):\n", len(selectedItems))
			for i, item := range selectedItems {
				defaultMarker := ""
				if item.IsDefault {
					defaultMarker = " (default)"
				}
				connectionSummary += fmt.Sprintf("  %d. %s%s\n", i+1, item.Alias, defaultMarker)
			}

			// Update the data with the first/default node
			if defaultNode := m.nodeSelector.GetDefaultNode(); defaultNode != nil {
				m.data.ServerAddr = defaultNode.Node.Address
			}

			// Update SelectedNodes in data
			m.data.SelectedNodes = make([]tui.NodeSelection, len(selectedItems))
			for i, item := range selectedItems {
				m.data.SelectedNodes[i] = tui.NodeSelection{
					Address:         item.Node.Address,
					Alias:           item.Alias,
					IsDefault:       item.IsDefault,
					DiscoveryMethod: string(item.Node.Method),
				}
			}

			// Track bib.dev confirmation
			m.data.BibDevConfirmed = m.nodeSelector.IsBibDevSelected()
		} else {
			connectionSummary = "No nodes selected. Using manual configuration."
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🔗 Connection Summary").
					Description(connectionSummary),
				huh.NewInput().
					Title("Default Server Address").
					Description("Primary bibd daemon address").
					Placeholder("localhost:4000").
					Value(&m.data.ServerAddr),
			),
		).WithTheme(theme)

	case "connection-test":
		// Run connection tests if not already done
		if !m.connectionTested {
			m.runConnectionTests()
		}

		// Format results
		var testResultsDisplay string
		if len(m.connectionResults) > 0 {
			testResultsDisplay = discovery.FormatConnectionResults(m.connectionResults)

			// Count connected/failed
			connected := 0
			failed := 0
			for _, r := range m.connectionResults {
				if r.Status == discovery.StatusConnected {
					connected++
				} else {
					failed++
				}
			}

			// Update node statuses in the selector if available
			if m.nodeSelector != nil {
				for _, result := range m.connectionResults {
					// Could update node status here for display
					_ = result
				}
			}

			if failed > 0 {
				testResultsDisplay += fmt.Sprintf("\n⚠️  %d node(s) failed connection test", failed)
			}
		} else {
			testResultsDisplay = "No nodes to test."
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🔌 Connection Test Results").
					Description(testResultsDisplay),
				huh.NewConfirm().
					Title("Continue?").
					Description("Press Enter to continue with setup").
					Affirmative("Continue").
					Negative("Retry Tests").
					Value(new(bool)),
			),
		).WithTheme(theme)

	case "auth-test":
		// Run auth tests if not already done
		if !m.authTested {
			m.runAuthTests()
		}

		// Format results
		var authResultsDisplay string
		if len(m.authResults) > 0 {
			authResultsDisplay = discovery.FormatAuthResults(m.authResults)

			// Count success/failed
			success := 0
			autoReg := 0
			failed := 0
			for _, r := range m.authResults {
				switch r.Status {
				case discovery.AuthStatusSuccess:
					success++
				case discovery.AuthStatusAutoRegistered:
					autoReg++
				default:
					failed++
				}
			}

			if autoReg > 0 {
				authResultsDisplay += fmt.Sprintf("\n✓ %d node(s) auto-registered your identity", autoReg)
			}
			if failed > 0 {
				authResultsDisplay += fmt.Sprintf("\n⚠️  %d node(s) failed authentication", failed)
				authResultsDisplay += "\n   You may need to register on these nodes first."
			}
		} else {
			authResultsDisplay = "No nodes to authenticate with.\n\nSkipping authentication test."
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🔐 Authentication Test Results").
					Description(authResultsDisplay),
				huh.NewConfirm().
					Title("Continue?").
					Description("Press Enter to continue with setup").
					Affirmative("Continue").
					Negative("Retry Tests").
					Value(new(bool)),
			),
		).WithTheme(theme)

	case "network-health":
		// Run network health check if not already done
		if !m.networkHealthChecked {
			m.runNetworkHealthCheck()
		}

		// Format results
		var healthDisplay string
		if len(m.networkHealthResults) > 0 {
			healthDisplay = discovery.FormatNetworkHealthResults(m.networkHealthResults)

			// Get summary
			checker := discovery.NewNetworkHealthChecker()
			summary := checker.GetSummary(m.networkHealthResults)

			// Add summary at the end
			healthDisplay += "\n" + discovery.FormatNetworkHealthSummary(summary)

			// Add recommendations based on status
			switch summary.OverallStatus {
			case discovery.NetworkHealthGood:
				healthDisplay += "\n✓ Network health is good. You're ready to go!"
			case discovery.NetworkHealthDegraded:
				healthDisplay += "\n⚠️  Some nodes have degraded connectivity."
				if !summary.BootstrapConnected {
					healthDisplay += "\n   Consider checking bootstrap node configuration."
				}
			case discovery.NetworkHealthPoor:
				healthDisplay += "\n⚠️  Network connectivity is poor."
				healthDisplay += "\n   Check firewall settings and network configuration."
			case discovery.NetworkHealthOffline:
				healthDisplay += "\n✗ Unable to retrieve network health information."
			}
		} else {
			healthDisplay = "No connected nodes to check network health.\n\nSkipping network health check."
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🌐 Network Health Check").
					Description(healthDisplay),
				huh.NewConfirm().
					Title("Continue?").
					Description("Press Enter to continue with setup").
					Affirmative("Continue").
					Negative("Retry Check").
					Value(new(bool)),
			),
		).WithTheme(theme)

	case "server":
		portStr := fmt.Sprintf("%d", m.data.Port)
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Listen Host").
					Description("Host address to bind to").
					Placeholder("0.0.0.0").
					Value(&m.data.Host),
				huh.NewInput().
					Title("Listen Port").
					Description("Port number for API").
					Placeholder("8080").
					Value(&portStr).
					Validate(func(s string) error {
						if s == "" {
							return nil
						}
						var p int
						_, err := fmt.Sscanf(s, "%d", &p)
						if err != nil || p < 1 || p > 65535 {
							return fmt.Errorf("port must be 1-65535")
						}
						m.data.Port = p
						return nil
					}),
				huh.NewInput().
					Title("Data Directory").
					Description("Where to store data").
					Placeholder("~/.local/share/bibd").
					Value(&m.data.DataDir),
			),
		).WithTheme(theme)

	case "tls":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Enable TLS").
					Description("Encrypt connections with TLS").
					Affirmative("Yes").
					Negative("No").
					Value(&m.data.TLSEnabled),
			),
		).WithTheme(theme)

	case "tls-certs":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Certificate File").
					Description("Path to TLS certificate").
					Placeholder("/etc/bibd/cert.pem").
					Value(&m.data.CertFile),
				huh.NewInput().
					Title("Key File").
					Description("Path to TLS private key").
					Placeholder("/etc/bibd/key.pem").
					Value(&m.data.KeyFile),
			),
		).WithTheme(theme)

	case "storage":
		// Build storage options based on deployment target
		var storageOptions []huh.Option[string]

		// SQLite is always available for proxy/selective modes
		storageOptions = append(storageOptions,
			huh.NewOption("SQLite (lightweight, local cache)", "sqlite"),
		)

		// PostgreSQL options depend on deployment target
		switch m.data.DeploymentTarget {
		case tui.DeployTargetDocker, tui.DeployTargetPodman:
			storageOptions = append(storageOptions,
				huh.NewOption("PostgreSQL - Managed Container (recommended)", "postgres"),
			)
		case tui.DeployTargetKubernetes:
			storageOptions = append(storageOptions,
				huh.NewOption("PostgreSQL - StatefulSet", "postgres"),
				huh.NewOption("PostgreSQL - CloudNativePG Operator", "postgres-cnpg"),
				huh.NewOption("PostgreSQL - External Server", "postgres-external"),
			)
		default: // local
			storageOptions = append(storageOptions,
				huh.NewOption("PostgreSQL - Managed Container (Docker/Podman)", "postgres-container"),
				huh.NewOption("PostgreSQL - Local Installation", "postgres-local"),
				huh.NewOption("PostgreSQL - Remote Server", "postgres-remote"),
			)
		}

		// Build description based on deployment target
		var storageDesc string
		switch m.data.DeploymentTarget {
		case tui.DeployTargetDocker, tui.DeployTargetPodman:
			storageDesc = "SQLite is lightweight. PostgreSQL will be deployed as a container alongside bibd."
		case tui.DeployTargetKubernetes:
			storageDesc = "SQLite is lightweight. PostgreSQL can be deployed as StatefulSet, CloudNativePG, or use external server."
		default:
			storageDesc = "SQLite is lightweight. PostgreSQL can be a local installation, container, or remote server."
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("💾 Storage Backend").
					Description(storageDesc),
				huh.NewSelect[string]().
					Title("Storage Backend").
					Description("Database to use for storage").
					Options(storageOptions...).
					Value(&m.data.StorageBackend),
			),
		).WithTheme(theme)

	case "postgres-config":
		// Determine what fields to show based on storage backend choice
		var fields []huh.Field

		// Determine PostgreSQL deployment mode from storage backend
		postgresMode := m.getPostgresDeploymentMode()

		// Initialize port string from data if needed
		if m.postgresPortStr == "" && m.data.PostgresPort > 0 {
			m.postgresPortStr = fmt.Sprintf("%d", m.data.PostgresPort)
		}
		if m.postgresPortStr == "" {
			m.postgresPortStr = "5432"
		}

		switch postgresMode {
		case "container": // Managed container (Docker/Podman for local deployment)
			fields = []huh.Field{
				huh.NewNote().
					Title("🐘 PostgreSQL - Managed Container").
					Description("PostgreSQL will be deployed as a container alongside bibd.\n\nWe'll generate a docker-compose.yaml or podman pod with PostgreSQL."),
				huh.NewInput().
					Title("Database Name").
					Description("PostgreSQL database name").
					Placeholder("bibd").
					Value(&m.data.PostgresDatabase),
				huh.NewInput().
					Title("Database User").
					Description("PostgreSQL user").
					Placeholder("bibd").
					Value(&m.data.PostgresUser),
				huh.NewInput().
					Title("Database Password").
					Description("PostgreSQL password (leave blank to auto-generate)").
					Placeholder("").
					Value(&m.data.PostgresPassword).
					EchoMode(huh.EchoModePassword),
			}

		case "local": // Local PostgreSQL installation
			fields = []huh.Field{
				huh.NewNote().
					Title("🐘 PostgreSQL - Local Installation").
					Description("Connect to a locally installed PostgreSQL server.\n\nMake sure PostgreSQL is running on this machine."),
				huh.NewInput().
					Title("Host").
					Description("PostgreSQL host address").
					Placeholder("localhost").
					Value(&m.data.PostgresHost),
				huh.NewInput().
					Title("Port").
					Description("PostgreSQL port").
					Placeholder("5432").
					Value(&m.postgresPortStr).
					Validate(validatePort),
				huh.NewInput().
					Title("Database Name").
					Description("PostgreSQL database name").
					Placeholder("bibd").
					Value(&m.data.PostgresDatabase),
				huh.NewInput().
					Title("User").
					Description("PostgreSQL user").
					Placeholder("bibd").
					Value(&m.data.PostgresUser),
				huh.NewInput().
					Title("Password").
					Description("PostgreSQL password").
					Value(&m.data.PostgresPassword).
					EchoMode(huh.EchoModePassword),
				huh.NewSelect[string]().
					Title("SSL Mode").
					Description("PostgreSQL SSL mode").
					Options(
						huh.NewOption("Disable", "disable"),
						huh.NewOption("Require", "require"),
						huh.NewOption("Verify CA", "verify-ca"),
						huh.NewOption("Verify Full", "verify-full"),
					).
					Value(&m.data.PostgresSSLMode),
			}

		case "remote": // Remote PostgreSQL server
			fields = []huh.Field{
				huh.NewNote().
					Title("🐘 PostgreSQL - Remote Server").
					Description("Connect to a remote PostgreSQL server.\n\nEnsure the server is accessible from this machine."),
				huh.NewInput().
					Title("Host").
					Description("PostgreSQL host address").
					Placeholder("db.example.com").
					Value(&m.data.PostgresHost),
				huh.NewInput().
					Title("Port").
					Description("PostgreSQL port").
					Placeholder("5432").
					Value(&m.postgresPortStr).
					Validate(validatePort),
				huh.NewInput().
					Title("Database Name").
					Description("PostgreSQL database name").
					Placeholder("bibd").
					Value(&m.data.PostgresDatabase),
				huh.NewInput().
					Title("User").
					Description("PostgreSQL user").
					Placeholder("bibd").
					Value(&m.data.PostgresUser),
				huh.NewInput().
					Title("Password").
					Description("PostgreSQL password").
					Value(&m.data.PostgresPassword).
					EchoMode(huh.EchoModePassword),
				huh.NewSelect[string]().
					Title("SSL Mode").
					Description("PostgreSQL SSL mode").
					Options(
						huh.NewOption("Require (recommended)", "require"),
						huh.NewOption("Verify CA", "verify-ca"),
						huh.NewOption("Verify Full", "verify-full"),
						huh.NewOption("Disable (not recommended)", "disable"),
					).
					Value(&m.data.PostgresSSLMode),
			}

		default: // Default to container mode
			fields = []huh.Field{
				huh.NewNote().
					Title("🐘 PostgreSQL Configuration").
					Description("Configure PostgreSQL connection settings."),
				huh.NewInput().
					Title("Database Name").
					Description("PostgreSQL database name").
					Placeholder("bibd").
					Value(&m.data.PostgresDatabase),
			}
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(fields...),
		).WithTheme(theme)

	case "postgres-test":
		// Run PostgreSQL connection test if not already done
		if !m.postgresTestDone {
			m.runPostgresTest()
		}

		// Format test results
		var testDisplay string
		if m.postgresTestResult != nil {
			if m.postgresTestResult.Success {
				testDisplay = fmt.Sprintf(`✓ PostgreSQL connection successful!

Server Version: %s
Database: %s
User: %s
Connection Time: %s`,
					m.postgresTestResult.ServerVersion,
					m.postgresTestResult.Database,
					m.postgresTestResult.User,
					m.postgresTestResult.Duration.Round(time.Millisecond))
			} else {
				testDisplay = fmt.Sprintf(`✗ PostgreSQL connection failed

Error: %s

Troubleshooting:
• Check that PostgreSQL is running
• Verify host, port, and credentials
• Ensure the database exists
• Check firewall settings`,
					m.postgresTestResult.Error)
			}
		} else {
			postgresMode := m.getPostgresDeploymentMode()
			if postgresMode == "container" {
				testDisplay = `⏭ PostgreSQL Container Mode

Connection will be tested after container deployment.
The container will be configured automatically.`
			} else {
				testDisplay = "Testing PostgreSQL connection..."
			}
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🔌 PostgreSQL Connection Test").
					Description(testDisplay),
				huh.NewConfirm().
					Title("Continue?").
					Description("Press Enter to continue").
					Affirmative("Continue").
					Negative("Retry Test").
					Value(new(bool)),
			),
		).WithTheme(theme)

	case "p2p":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Enable P2P").
					Description("Enable peer-to-peer networking").
					Affirmative("Yes").
					Negative("No").
					Value(&m.data.P2PEnabled),
			),
		).WithTheme(theme)

	case "p2p-mode":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("P2P Mode").
					Description("How this node participates in the network").
					Options(
						huh.NewOption("Proxy - Forward requests", "proxy"),
						huh.NewOption("Selective - Subscribe to topics", "selective"),
						huh.NewOption("Full - Replicate all data", "full"),
					).
					Value(&m.data.P2PMode),
			),
		).WithTheme(theme)

	case "bootstrap-peers":
		// Bootstrap peer configuration
		bootstrapDesc := `Bootstrap peers help your node discover other nodes on the network.

Options:
• Public Network (bib.dev) - Connect to the global bib network
• Private Only - Only use custom bootstrap peers

The public network allows your node to participate in the 
global bib ecosystem and discover peers worldwide.`

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🌐 Bootstrap Peers").
					Description(bootstrapDesc),
				huh.NewConfirm().
					Title("Use bib.dev public bootstrap?").
					Description("Connect to the public bib network").
					Affirmative("Yes, use public network").
					Negative("No, private only").
					Value(&m.data.UsePublicBootstrap),
			),
		).WithTheme(theme)

	case "bootstrap-confirm":
		// Confirmation dialog for public network
		confirmDesc := `⚠️  PUBLIC NETWORK CONFIRMATION

You're about to connect to the bib.dev PUBLIC NETWORK.

This means:
• Your node will be discoverable by users worldwide
• Your public identity will be visible on the network
• Published data will be accessible to network participants
• You'll participate in the global P2P mesh

This is required for:
• Collaborating with users outside your local network
• Accessing publicly shared datasets
• Contributing to the bib ecosystem

Your private data remains private - only explicitly 
published data is shared.`

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🌍 Public Network Confirmation").
					Description(confirmDesc),
				huh.NewConfirm().
					Title("Connect to bib.dev public network?").
					Description("This requires explicit confirmation").
					Affirmative("Yes, Connect to Public Network").
					Negative("No, Private Network Only").
					Value(&m.data.BibDevConfirmed),
			),
		).WithTheme(theme)

	case "custom-bootstrap":
		// Show current custom bootstrap peers
		var customPeersDisplay string
		if len(m.data.CustomBootstrapPeers) > 0 {
			customPeersDisplay = "Current custom bootstrap peers:\n"
			for i, peer := range m.data.CustomBootstrapPeers {
				// Truncate long multiaddrs
				displayPeer := peer
				if len(displayPeer) > 60 {
					displayPeer = displayPeer[:57] + "..."
				}
				customPeersDisplay += fmt.Sprintf("  %d. %s\n", i+1, displayPeer)
			}
		} else {
			customPeersDisplay = "No custom bootstrap peers configured."
		}

		// Show status based on public bootstrap setting
		if m.data.UsePublicBootstrap && m.data.BibDevConfirmed {
			customPeersDisplay += "\n\n✓ Using bib.dev public bootstrap"
		} else if m.data.UsePublicBootstrap {
			customPeersDisplay += "\n\n⚠️ Public bootstrap selected but not confirmed"
		} else {
			customPeersDisplay += "\n\n⚠️ Private network only - custom peers required"
		}

		customPeersDisplay += "\n\nYou can add custom peers in multiaddr format:\n/ip4/<ip>/tcp/<port>/p2p/<peerID>"

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🔧 Custom Bootstrap Peers").
					Description(customPeersDisplay),
				huh.NewInput().
					Title("Add Custom Bootstrap Peer").
					Description("Enter multiaddr (leave empty to skip)").
					Placeholder("/ip4/1.2.3.4/tcp/4001/p2p/Qm...").
					Value(&m.customBootstrapInput),
			),
		).WithTheme(theme)

	case "logging":
		fields := []huh.Field{
			huh.NewSelect[string]().
				Title("Log Level").
				Description("Verbosity of log output").
				Options(
					huh.NewOption("Debug", "debug"),
					huh.NewOption("Info", "info"),
					huh.NewOption("Warning", "warn"),
					huh.NewOption("Error", "error"),
				).
				Value(&m.data.LogLevel),
		}
		if m.isDaemon {
			fields = append(fields,
				huh.NewSelect[string]().
					Title("Log Format").
					Description("Format of log messages").
					Options(
						huh.NewOption("Pretty", "pretty"),
						huh.NewOption("Text", "text"),
						huh.NewOption("JSON", "json"),
					).
					Value(&m.data.LogFormat),
			)
		}
		m.currentForm = huh.NewForm(
			huh.NewGroup(fields...),
		).WithTheme(theme)

	case "cluster":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Enable Clustering").
					Description("Join or create an HA cluster").
					Affirmative("Yes").
					Negative("No").
					Value(&m.data.ClusterEnabled),
			),
		).WithTheme(theme)

	case "cluster-settings":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Cluster Name").
					Description("Unique name for this cluster").
					Placeholder("bib-cluster").
					Value(&m.data.ClusterName),
				huh.NewInput().
					Title("Raft Listen Address").
					Description("Address for inter-node communication").
					Placeholder("0.0.0.0:4002").
					Value(&m.data.ClusterAddr),
				huh.NewConfirm().
					Title("Bootstrap New Cluster").
					Description("Initialize as the first node").
					Affirmative("Yes, create new").
					Negative("No, join existing").
					Value(&m.data.Bootstrap),
			),
		).WithTheme(theme)

	case "break-glass":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🔓 Break Glass Emergency Access").
					Description("Break glass provides controlled emergency access to the database for disaster recovery and debugging.\n\n⚠️  This bypasses normal security controls. Only enable if you need emergency access capabilities."),
				huh.NewConfirm().
					Title("Enable Break Glass").
					Description("Allow emergency database access").
					Affirmative("Yes, enable").
					Negative("No, skip").
					Value(&m.data.BreakGlassEnabled),
			),
		).WithTheme(theme)

	case "break-glass-user":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Emergency User Name").
					Description("Username for emergency access").
					Placeholder("emergency_admin").
					Value(&m.data.BreakGlassUserName),
				huh.NewText().
					Title("SSH Public Key").
					Description("Ed25519 SSH public key (ssh-ed25519 AAAA...)").
					Placeholder("ssh-ed25519 AAAA...").
					Value(&m.data.BreakGlassUserKey).
					Lines(3),
				huh.NewSelect[string]().
					Title("Max Session Duration").
					Description("Maximum duration for break glass sessions").
					Options(
						huh.NewOption("30 minutes", "30m"),
						huh.NewOption("1 hour", "1h"),
						huh.NewOption("2 hours", "2h"),
						huh.NewOption("4 hours", "4h"),
					).
					Value(&m.data.BreakGlassMaxDuration),
				huh.NewSelect[string]().
					Title("Default Access Level").
					Description("Default permission level for sessions").
					Options(
						huh.NewOption("Read-only (SELECT only)", "readonly"),
						huh.NewOption("Read-write (full access except audit_log)", "readwrite"),
					).
					Value(&m.data.BreakGlassAccessLevel),
			),
		).WithTheme(theme)

	case "service-install":
		// Initialize service installer if not done
		if m.serviceInstaller == nil {
			serviceConfig := local.DefaultServiceConfig()
			serviceConfig.ConfigPath = m.configPath
			if m.configPath == "" {
				if configDir, err := config.UserConfigDir(config.AppBibd); err == nil {
					serviceConfig.ConfigPath = filepath.Join(configDir, "config.yaml")
					serviceConfig.WorkingDirectory = configDir
				}
			}
			m.serviceInstaller = local.NewServiceInstaller(serviceConfig)
		}

		// Detect service type
		serviceType := local.DetectServiceType()
		var serviceTypeDesc string
		switch serviceType {
		case local.ServiceTypeSystemd:
			serviceTypeDesc = "systemd (Linux)"
		case local.ServiceTypeLaunchd:
			serviceTypeDesc = "launchd (macOS)"
		case local.ServiceTypeWindows:
			serviceTypeDesc = "Windows Service"
		}

		// Build service install description
		var serviceDesc string
		if m.installService {
			// Generate preview of service file
			content, _ := m.serviceInstaller.Generate()
			preview := content
			if len(preview) > 500 {
				preview = preview[:500] + "\n... (truncated)"
			}

			if m.userService {
				serviceDesc = fmt.Sprintf(`🔧 Service Configuration

Type: %s (User Service)
Path: %s

Preview:
%s`, serviceTypeDesc, m.serviceInstaller.GetServiceFilePath(), preview)
			} else {
				serviceDesc = fmt.Sprintf(`🔧 Service Configuration

Type: %s (System Service)
Path: %s

Preview:
%s`, serviceTypeDesc, m.serviceInstaller.GetServiceFilePath(), preview)
			}
		} else {
			serviceDesc = fmt.Sprintf(`🔧 Service Installation

Detected service type: %s

Installing bibd as a service allows it to:
• Start automatically at boot
• Run in the background
• Restart on failure

You can choose:
• System service - Runs as root/system (recommended for servers)
• User service - Runs as your user (no root required)`, serviceTypeDesc)
		}

		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("🛠️  Service Installation").
					Description(serviceDesc),
				huh.NewConfirm().
					Title("Install as service?").
					Description("Install bibd to run automatically").
					Affirmative("Yes, install service").
					Negative("No, I'll run manually").
					Value(&m.installService),
				huh.NewConfirm().
					Title("User-level service?").
					Description("User service doesn't require root/admin").
					Affirmative("Yes, user service").
					Negative("No, system service").
					Value(&m.userService),
			),
		).WithTheme(theme)

	case "confirm":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("Configuration Summary").
					Description(tui.RenderSetupSummary(m.data, m.isDaemon)),
				huh.NewConfirm().
					Title("Save Configuration?").
					Affirmative("Save").
					Negative("Cancel"),
			),
		).WithTheme(theme)

	default:
		m.currentForm = nil
	}
}

func (m *SetupWizardModel) getWelcomeText() string {
	if m.isDaemon {
		return "Welcome! This wizard will help you configure the bibd daemon.\n\nWe'll configure identity, server settings, storage, P2P networking, and more.\n\nPress Enter to continue..."
	}
	return "Welcome! This wizard will help you configure the bib CLI.\n\nWe'll configure your identity, output preferences, and connection settings.\n\nPress Enter to continue..."
}

// handleStepCompletion handles step-specific completion logic
// Returns true if we should proceed to next step, false if we should go back
func (m *SetupWizardModel) handleStepCompletion() bool {
	step := m.wizard.CurrentStep()
	if step == nil {
		return true
	}

	switch step.ID {
	case "bib-dev-confirm":
		// Check if bib.dev is actually selected
		if m.nodeSelector == nil || !m.nodeSelector.IsBibDevSelected() {
			// Not selected, auto-proceed (step was skipped anyway)
			return true
		}

		// Check if user confirmed - we need to extract the value from the form
		// The confirm field should have been filled in
		// If user selected "No", we should go back to node selection
		if !m.bibDevConfirmed {
			// User said no - deselect bib.dev and go back
			m.nodeSelector.SetBibDevSelected(false)
			m.data.BibDevConfirmed = false
			return false // Go back to previous step
		}

		// User confirmed
		m.data.BibDevConfirmed = true
		return true

	case "node-selection":
		// After node selection, check if bib.dev is selected
		// If so, the next step (bib-dev-confirm) will handle confirmation
		if m.nodeSelector != nil && m.nodeSelector.IsBibDevSelected() {
			// Reset confirmation state so user must confirm again
			m.bibDevConfirmed = false
		}
		return true

	case "connection-test":
		// Check if user wants to retry tests
		// The confirm value will be false if user selected "Retry Tests"
		// For now, always proceed - retry can be done by going back
		return true

	case "bootstrap-confirm":
		// Handle public network confirmation for daemon setup
		if !m.data.BibDevConfirmed {
			// User declined public network - disable public bootstrap
			m.data.UsePublicBootstrap = false
		}
		return true

	case "custom-bootstrap":
		// Add the custom bootstrap peer if provided
		if m.customBootstrapInput != "" {
			// Basic validation - should start with /
			if strings.HasPrefix(m.customBootstrapInput, "/") {
				m.data.AddCustomBootstrapPeer(m.customBootstrapInput)
			}
			// Clear input for next entry
			m.customBootstrapInput = ""
		}
		return true

	case "service-install":
		// Update service config based on user selection
		if m.serviceInstaller != nil && m.installService {
			m.serviceInstaller.Config.UserService = m.userService
		}
		return true

	default:
		return true
	}
}

// runNodeDiscovery runs node discovery for CLI setup
func (m *SetupWizardModel) runNodeDiscovery() {
	if m.discoveryDone {
		return
	}

	// Create discoverer with default options
	discoverer := discovery.NewWithDefaults()

	// Run discovery with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m.discoveryResult = discoverer.Discover(ctx)
	m.discoveryDone = true

	// Initialize node selector with results
	m.nodeSelector = component.NewNodeSelector().
		WithNodes(m.discoveryResult.Nodes).
		WithBibDev(true).
		WithAddCustom(true).
		WithMultiSelect(true).
		WithLatency(true)

	// Auto-select first local node if available
	m.nodeSelector.SelectFirst()
}

// runTargetDetection runs deployment target detection for daemon setup
func (m *SetupWizardModel) runTargetDetection() {
	if m.targetDetected {
		return
	}

	// Initialize target selector
	m.targetSelector = component.NewTargetSelector()

	// Create detector and run detection
	detector := deploy.NewTargetDetector().WithTimeout(5 * time.Second)

	// Run detection with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	targets := detector.DetectAll(ctx)

	// Update selector with results
	m.targetSelector.Targets = targets
	m.targetSelector.DetectionDone = true

	// Select first available target
	for i, t := range targets {
		if t.Available {
			m.targetSelector.Selected = i
			break
		}
	}

	m.targetDetected = true

	// Update data with selected target
	if target := m.targetSelector.SelectedTarget(); target != nil && target.Available {
		m.data.DeploymentTarget = string(target.Type)
	}
}

// getPostgresDeploymentMode returns the PostgreSQL deployment mode based on storage backend selection
func (m *SetupWizardModel) getPostgresDeploymentMode() string {
	switch m.data.StorageBackend {
	case "postgres-container":
		return "container"
	case "postgres-local":
		return "local"
	case "postgres-remote":
		return "remote"
	case "postgres-cnpg":
		return "cnpg"
	case "postgres-external":
		return "external"
	case "postgres":
		// Default based on deployment target
		switch m.data.DeploymentTarget {
		case tui.DeployTargetDocker, tui.DeployTargetPodman:
			return "container"
		case tui.DeployTargetKubernetes:
			return "statefulset"
		default:
			return "local"
		}
	default:
		return "container"
	}
}

// runPostgresTest tests the PostgreSQL connection
func (m *SetupWizardModel) runPostgresTest() {
	if m.postgresTestDone {
		return
	}

	mode := m.getPostgresDeploymentMode()

	// For container deployments, skip actual test (will be tested after deployment)
	if mode == "container" || mode == "cnpg" || mode == "statefulset" {
		m.postgresTestDone = true
		m.postgresTestResult = nil // Will show "skip" message
		return
	}

	// Set defaults if not provided
	if m.data.PostgresHost == "" {
		m.data.PostgresHost = "localhost"
	}
	// Convert port string to int if needed
	if m.postgresPortStr != "" {
		port, err := strconv.Atoi(m.postgresPortStr)
		if err == nil {
			m.data.PostgresPort = port
		}
	}
	if m.data.PostgresPort == 0 {
		m.data.PostgresPort = 5432
	}
	if m.data.PostgresDatabase == "" {
		m.data.PostgresDatabase = "bibd"
	}
	if m.data.PostgresUser == "" {
		m.data.PostgresUser = "bibd"
	}
	if m.data.PostgresSSLMode == "" {
		m.data.PostgresSSLMode = "disable"
	}

	// Build connection string
	connStr := m.data.GetPostgresConnectionString()

	// Test the connection
	start := time.Now()
	result := testPostgresConnection(connStr)
	result.Duration = time.Since(start)
	result.Database = m.data.PostgresDatabase
	result.User = m.data.PostgresUser

	m.postgresTestResult = result
	m.postgresTestDone = true
}

// testPostgresConnection tests a PostgreSQL connection
func testPostgresConnection(connStr string) *PostgresTestResult {
	result := &PostgresTestResult{}

	// Try to connect using database/sql with pgx driver
	// Note: This requires the pgx driver to be imported
	// For now, we'll do a simple TCP connection test

	// Parse host and port from connection string
	// Connection string format: postgres://user:pass@host:port/db?sslmode=...
	host := "localhost"
	port := "5432"

	// Simple parsing (production would use proper URL parsing)
	if strings.Contains(connStr, "@") {
		parts := strings.Split(connStr, "@")
		if len(parts) >= 2 {
			hostPart := parts[1]
			if slashIdx := strings.Index(hostPart, "/"); slashIdx > 0 {
				hostPart = hostPart[:slashIdx]
			}
			if colonIdx := strings.Index(hostPart, ":"); colonIdx > 0 {
				host = hostPart[:colonIdx]
				port = hostPart[colonIdx+1:]
			} else {
				host = hostPart
			}
		}
	}

	// Test TCP connection
	addr := fmt.Sprintf("%s:%s", host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Cannot connect to %s: %v", addr, err)
		return result
	}
	conn.Close()

	// For a full implementation, we would use database/sql with pgx
	// to actually authenticate and query server version.
	// For now, we mark success if TCP connection works.
	result.Success = true
	result.ServerVersion = "(TCP connection verified)"

	return result
}

// runConnectionTests tests connections to all selected nodes
func (m *SetupWizardModel) runConnectionTests() {
	if m.connectionTested {
		return
	}

	// Get selected nodes
	var addresses []string
	if m.nodeSelector != nil && m.nodeSelector.HasSelection() {
		for _, item := range m.nodeSelector.SelectedItems() {
			addresses = append(addresses, item.Node.Address)
		}
	} else if m.data.ServerAddr != "" {
		addresses = []string{m.data.ServerAddr}
	}

	if len(addresses) == 0 {
		m.connectionTested = true
		return
	}

	// Create connection tester
	tester := discovery.NewConnectionTester().
		WithTimeout(5 * time.Second)

	// Run tests
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m.connectionResults = tester.TestConnections(ctx, addresses)
	m.connectionTested = true
}

// runAuthTests tests authentication with all connected nodes
func (m *SetupWizardModel) runAuthTests() {
	if m.authTested {
		return
	}

	// Only test against nodes that passed connection test
	var connectedAddresses []string
	for _, r := range m.connectionResults {
		if r.Status == discovery.StatusConnected {
			connectedAddresses = append(connectedAddresses, r.Address)
		}
	}

	if len(connectedAddresses) == 0 {
		m.authTested = true
		return
	}

	// Need identity key for authentication
	if m.identityKey == nil {
		m.authTested = true
		return
	}

	// Create auth tester
	tester := discovery.NewAuthTester(m.identityKey).
		WithTimeout(10*time.Second).
		WithRegistrationInfo(m.data.Name, m.data.Email)

	// Run tests
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	m.authResults = tester.TestAuths(ctx, connectedAddresses)
	m.authTested = true
}

// runNetworkHealthCheck checks network health of all connected nodes
func (m *SetupWizardModel) runNetworkHealthCheck() {
	if m.networkHealthChecked {
		return
	}

	// Only check nodes that passed connection test
	var connectedAddresses []string
	for _, r := range m.connectionResults {
		if r.Status == discovery.StatusConnected {
			connectedAddresses = append(connectedAddresses, r.Address)
		}
	}

	if len(connectedAddresses) == 0 {
		m.networkHealthChecked = true
		return
	}

	// Create health checker
	checker := discovery.NewNetworkHealthChecker().
		WithTimeout(10 * time.Second)

	// Run checks
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	m.networkHealthResults = checker.CheckHealthMultiple(ctx, connectedAddresses)
	m.networkHealthChecked = true
}

func (m *SetupWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Pass size to wizard
		m.wizard.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Offer to save progress before quitting
			m.saveProgressOnExit()
			m.cancelled = true
			m.done = true
			return m, tea.Quit

		case "esc":
			if m.wizard.CurrentStepIndex() > 0 {
				m.wizard.PrevStep()
				m.updateProgressTracking()
				m.updateFormForCurrentStep()
				if m.currentForm != nil {
					return m, m.currentForm.Init()
				}
			} else {
				m.saveProgressOnExit()
				m.cancelled = true
				m.done = true
				return m, tea.Quit
			}
			return m, nil

		case "enter":
			// Check if form is complete
			if m.currentForm != nil {
				// Update form first
				var cmd tea.Cmd
				formModel, cmd := m.currentForm.Update(msg)
				m.currentForm = formModel.(*huh.Form)

				// If form completed, move to next step
				if m.currentForm.State == huh.StateCompleted {
					// Handle step-specific completion logic
					if !m.handleStepCompletion() {
						// Step completion returned false, meaning we should go back
						m.wizard.PrevStep()
						m.updateFormForCurrentStep()
						if m.currentForm != nil {
							return m, m.currentForm.Init()
						}
						return m, nil
					}

					// Mark current step as completed and update progress
					if step := m.wizard.CurrentStep(); step != nil {
						m.progress.MarkStepCompleted(step.ID)
					}

					nextCmd := m.wizard.NextStep()
					if m.wizard.IsDone() {
						// Clean up partial config on successful completion
						config.DeletePartialConfig(m.progress.AppName)
						m.done = true
						return m, tea.Quit
					}
					m.updateProgressTracking()
					m.updateFormForCurrentStep()
					if m.currentForm != nil {
						return m, tea.Batch(nextCmd, m.currentForm.Init())
					}
					return m, nextCmd
				}
				return m, cmd
			}
		}
	}

	// Update the current form
	if m.currentForm != nil {
		formModel, cmd := m.currentForm.Update(msg)
		m.currentForm = formModel.(*huh.Form)

		// Check if form was aborted
		if m.currentForm.State == huh.StateAborted {
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}

		return m, cmd
	}

	return m, nil
}

// updateProgressTracking updates the progress tracking with current step info
func (m *SetupWizardModel) updateProgressTracking() {
	if m.progress == nil {
		return
	}

	step := m.wizard.CurrentStep()
	if step != nil {
		m.progress.SetCurrentStep(step.ID, m.wizard.CurrentStepIndex())
	}
}

// saveProgressOnExit saves progress when the user cancels/exits
func (m *SetupWizardModel) saveProgressOnExit() {
	if m.progress == nil {
		return
	}

	// Update progress with current state
	m.updateProgressTracking()

	// Store the current setup data
	if err := m.progress.SetData(m.data); err != nil {
		// Log but don't fail - user is exiting anyway
		fmt.Printf("\nWarning: could not save setup data: %v\n", err)
	}

	// Only save if we have made some progress (not on first step)
	if m.wizard.CurrentStepIndex() > 0 || len(m.progress.CompletedSteps) > 0 {
		if err := config.SavePartialConfig(m.progress); err != nil {
			fmt.Printf("\nWarning: could not save progress: %v\n", err)
		} else {
			fmt.Printf("\n✓ Progress saved. Run 'bib setup' to resume.\n")
		}
	}
}

func (m *SetupWizardModel) View() string {
	if m.done {
		return ""
	}

	// Set step view function to render the form
	step := m.wizard.CurrentStep()
	if step != nil && m.currentForm != nil {
		step.View = func(width int) string {
			return m.currentForm.View()
		}
	}

	return m.wizard.View()
}

func (m *SetupWizardModel) saveConfig() error {
	var appName string
	if m.isDaemon {
		appName = config.AppBibd
	} else {
		appName = config.AppBib
	}

	configDir, err := config.UserConfigDir(appName)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var cfg interface{}
	if m.isDaemon {
		cfg = m.data.ToBibdConfig()
	} else {
		cfg = m.data.ToBibConfig()
	}

	path, err := writeConfig(appName, setupFormat, cfg)
	if err != nil {
		return err
	}

	m.configPath = path
	return nil
}

func setupBibWizard() error {
	m := newSetupWizardModel(false)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	result := finalModel.(*SetupWizardModel)
	if result.cancelled {
		fmt.Println("\nSetup cancelled.")
		return nil
	}

	if result.configPath != "" {
		fmt.Println(tui.RenderSuccess(result.configPath, false))
	}
	return result.err
}

func setupBibdWizard() error {
	m := newSetupWizardModel(true)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	result := finalModel.(*SetupWizardModel)
	if result.cancelled {
		fmt.Println("\nSetup cancelled.")
		return nil
	}

	if result.configPath != "" {
		fmt.Println(tui.RenderSuccess(result.configPath, true))
	}
	return result.err
}

func setupBibdCluster() error {
	// Show welcome screen
	fmt.Print("\033[H\033[2J") // Clear screen

	theme := tui.GetTheme()
	fmt.Println(tui.Banner())
	fmt.Println()
	fmt.Println(theme.Title.Render("bibd HA Cluster Initialization"))
	fmt.Println()
	fmt.Println(theme.Description.Render("This wizard will initialize a new HA cluster and generate a join token."))
	fmt.Println()
	fmt.Println("Press Enter to continue...")
	_, _ = fmt.Scanln()

	// Create setup data with defaults
	data := tui.DefaultSetupData()
	data.ClusterEnabled = true
	data.Bootstrap = true

	// First run the daemon setup form
	form := tui.CreateBibdSetupForm(data)

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nSetup cancelled.")
			return nil
		}
		return err
	}

	// Then run the cluster setup form
	clusterForm := tui.CreateClusterSetupForm(data)

	err = clusterForm.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nSetup cancelled.")
			return nil
		}
		return err
	}

	// Generate node ID
	nodeID, err := generateNodeID()
	if err != nil {
		return fmt.Errorf("failed to generate node ID: %w", err)
	}

	// Set advertise address if not set
	if data.AdvertiseAddr == "" {
		data.AdvertiseAddr = data.ClusterAddr
	}

	// Create config directory
	configDir, err := config.UserConfigDir(config.AppBibd)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Build config
	cfg := data.ToBibdConfig()
	cfg.Cluster.NodeID = nodeID
	cfg.Cluster.Bootstrap = true

	configPath, err := writeConfig(config.AppBibd, setupFormat, cfg)
	if err != nil {
		return err
	}

	// Generate join token
	joinToken, err := generateJoinToken(data.ClusterName, data.AdvertiseAddr)
	if err != nil {
		return fmt.Errorf("failed to generate join token: %w", err)
	}

	// Show success
	fmt.Println(tui.RenderClusterSuccess(configPath, nodeID, data.ClusterName, joinToken))
	return nil
}

func setupBibdJoinCluster() error {
	// Decode join token
	tokenData, err := decodeJoinToken(setupClusterJoin)
	if err != nil {
		return fmt.Errorf("invalid join token: %w", err)
	}

	// Check if token is expired
	if time.Now().Unix() > tokenData.ExpiresAt {
		return fmt.Errorf("join token has expired")
	}

	// Show welcome screen
	fmt.Print("\033[H\033[2J") // Clear screen

	theme := tui.GetTheme()
	status := tui.NewStatusIndicator()

	fmt.Println(tui.Banner())
	fmt.Println()
	fmt.Println(theme.Title.Render("Join HA Cluster"))
	fmt.Println()
	fmt.Println(status.Info(fmt.Sprintf("Cluster: %s", tokenData.ClusterName)))
	fmt.Println(status.Info(fmt.Sprintf("Leader: %s", tokenData.LeaderAddr)))
	fmt.Println()
	fmt.Println("Press Enter to continue...")
	_, _ = fmt.Scanln()

	// Create setup data with defaults
	data := tui.DefaultSetupData()
	data.ClusterEnabled = true
	data.Bootstrap = false
	data.JoinToken = setupClusterJoin
	data.ClusterName = tokenData.ClusterName

	// First run the daemon setup form
	form := tui.CreateBibdSetupForm(data)

	err = form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nSetup cancelled.")
			return nil
		}
		return err
	}

	// Then run the cluster join form
	joinForm := tui.CreateClusterJoinForm(data, tokenData.ClusterName, tokenData.LeaderAddr)

	err = joinForm.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nSetup cancelled.")
			return nil
		}
		return err
	}

	// Generate node ID
	nodeID, err := generateNodeID()
	if err != nil {
		return fmt.Errorf("failed to generate node ID: %w", err)
	}

	// Set advertise address if not set
	if data.AdvertiseAddr == "" {
		data.AdvertiseAddr = data.ClusterAddr
	}

	// Create config directory
	configDir, err := config.UserConfigDir(config.AppBibd)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Build config
	cfg := data.ToBibdConfig()
	cfg.Cluster.NodeID = nodeID
	cfg.Cluster.Bootstrap = false
	cfg.Cluster.JoinAddrs = []string{tokenData.LeaderAddr}

	configPath, err := writeConfig(config.AppBibd, setupFormat, cfg)
	if err != nil {
		return err
	}

	// Show success
	fmt.Println()
	fmt.Println(status.Success("Configuration saved successfully!"))
	fmt.Println()
	fmt.Println(theme.Base.Render("Config file: "))
	fmt.Println(theme.Focused.Render("  " + configPath))
	fmt.Println()
	fmt.Println(tui.NewKVRenderer().Render("Node ID", nodeID))
	fmt.Println(tui.NewKVRenderer().Render("Cluster", tokenData.ClusterName))
	if data.IsVoter {
		fmt.Println(tui.NewKVRenderer().Render("Role", "Voter"))
	} else {
		fmt.Println(tui.NewKVRenderer().Render("Role", "Non-Voter"))
	}
	fmt.Println()
	fmt.Println(theme.Base.Render("Start the daemon to complete joining:"))
	fmt.Println(theme.Focused.Render("  bibd"))
	fmt.Println()
	fmt.Println(theme.Warning.Render("⚠ Minimum 3 voting nodes required for HA quorum."))

	return nil
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
