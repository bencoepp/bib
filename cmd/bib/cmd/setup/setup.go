package setup

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"bib/internal/auth"
	"bib/internal/config"
	"bib/internal/tui"

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
	// TODO: Implement quick CLI setup (Phase 2.9)
	// For now, fall back to full wizard
	fmt.Println("Running quick setup...")
	return setupBibWizard()
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
			ID:          "output",
			Title:       "Output",
			Description: "Configure output settings",
			HelpText:    "These settings control how bib displays information. Table format is recommended for interactive use, JSON/YAML for scripting.",
			ShouldSkip:  func() bool { return isDaemon },
		},
		{
			ID:          "connection",
			Title:       "Connection",
			Description: "Configure connection to bibd",
			HelpText:    "Specify the address of the bibd daemon you want to connect to.",
			ShouldSkip:  func() bool { return isDaemon },
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

	case "connection":
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Server Address").
					Description("Address of the bibd daemon").
					Placeholder("localhost:8080").
					Value(&m.data.ServerAddr),
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
		m.currentForm = huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Storage Backend").
					Description("Database to use for storage").
					Options(
						huh.NewOption("SQLite (lightweight, local cache)", "sqlite"),
						huh.NewOption("PostgreSQL (full replication)", "postgres"),
					).
					Value(&m.data.StorageBackend),
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
