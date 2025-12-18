package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"bib/internal/config"
	"bib/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
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
		return setupBibdWizard()
	}
	return setupBibWizard()
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
}

func newSetupWizardModel(isDaemon bool) *SetupWizardModel {
	data := tui.DefaultSetupData()

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
			m.cancelled = true
			m.done = true
			return m, tea.Quit

		case "esc":
			if m.wizard.CurrentStepIndex() > 0 {
				m.wizard.PrevStep()
				m.updateFormForCurrentStep()
				if m.currentForm != nil {
					return m, m.currentForm.Init()
				}
			} else {
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
					nextCmd := m.wizard.NextStep()
					if m.wizard.IsDone() {
						m.done = true
						return m, tea.Quit
					}
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
