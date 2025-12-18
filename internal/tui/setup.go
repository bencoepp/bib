package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"bib/internal/config"
	"bib/internal/tui/component"
	"bib/internal/tui/themes"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// SetupData holds all the data collected during setup
type SetupData struct {
	// Identity
	Name  string
	Email string

	// Server (bibd only)
	Host    string
	Port    int
	DataDir string

	// TLS
	TLSEnabled bool
	CertFile   string
	KeyFile    string

	// Logging
	LogLevel  string
	LogFormat string

	// Output (bib only)
	OutputFormat string
	ColorEnabled bool
	ServerAddr   string

	// Storage (bibd only)
	StorageBackend string

	// P2P (bibd only)
	P2PEnabled     bool
	P2PMode        string
	P2PListenAddrs []string

	// Cluster (bibd only)
	ClusterEnabled bool
	ClusterName    string
	ClusterAddr    string
	AdvertiseAddr  string
	IsVoter        bool
	Bootstrap      bool
	JoinToken      string

	// Break Glass (bibd only)
	BreakGlassEnabled     bool
	BreakGlassMaxDuration string
	BreakGlassAccessLevel string
	BreakGlassUserName    string
	BreakGlassUserKey     string
}

// DefaultSetupData returns setup data with sensible defaults
func DefaultSetupData() *SetupData {
	return &SetupData{
		Host:           "0.0.0.0",
		Port:           8080,
		DataDir:        "~/.local/share/bibd",
		LogLevel:       "info",
		LogFormat:      "pretty",
		OutputFormat:   "table",
		ColorEnabled:   true,
		ServerAddr:     "localhost:8080",
		StorageBackend: "sqlite",
		P2PEnabled:     true,
		P2PMode:        "proxy",
		P2PListenAddrs: []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"},
		ClusterName:    "bib-cluster",
		ClusterAddr:    "0.0.0.0:4002",
		IsVoter:        true,
		// Break Glass defaults
		BreakGlassEnabled:     false,
		BreakGlassMaxDuration: "1h",
		BreakGlassAccessLevel: "readonly",
	}
}

// CreateBibSetupForm creates the setup form for bib CLI
func CreateBibSetupForm(data *SetupData) *huh.Form {
	theme := HuhTheme()

	return huh.NewForm(
		// Identity Group
		huh.NewGroup(
			huh.NewNote().
				Title("👤 Identity").
				Description("Your identity for signing and attribution."),

			huh.NewInput().
				Title("Name").
				Description("Your display name").
				Placeholder("John Doe").
				Value(&data.Name),

			huh.NewInput().
				Title("Email").
				Description("Your email address").
				Placeholder("john@example.com").
				Value(&data.Email),
		).Title("Identity").Description("Configure your identity"),

		// Output Group
		huh.NewGroup(
			huh.NewNote().
				Title("📺 Output Settings").
				Description("How bib displays information."),

			huh.NewSelect[string]().
				Title("Output Format").
				Description("Default output format for commands").
				Options(
					huh.NewOption("Table", "table"),
					huh.NewOption("JSON", "json"),
					huh.NewOption("YAML", "yaml"),
					huh.NewOption("Text", "text"),
				).
				Value(&data.OutputFormat),

			huh.NewConfirm().
				Title("Enable Colors").
				Description("Use colored output in the terminal").
				Affirmative("Yes").
				Negative("No").
				Value(&data.ColorEnabled),
		).Title("Output").Description("Configure output settings"),

		// Connection Group
		huh.NewGroup(
			huh.NewNote().
				Title("🔗 Connection").
				Description("How to connect to bibd."),

			huh.NewInput().
				Title("Server Address").
				Description("Address of the bibd daemon").
				Placeholder("localhost:8080").
				Value(&data.ServerAddr),

			huh.NewSelect[string]().
				Title("Log Level").
				Description("Verbosity of log output").
				Options(
					huh.NewOption("Debug", "debug"),
					huh.NewOption("Info", "info"),
					huh.NewOption("Warning", "warn"),
					huh.NewOption("Error", "error"),
				).
				Value(&data.LogLevel),
		).Title("Connection").Description("Configure connection settings"),

		// Confirmation Group
		huh.NewGroup(
			huh.NewNote().
				Title("✅ Ready to Save").
				Description("Review your settings and save the configuration."),

			huh.NewConfirm().
				Title("Save Configuration?").
				Affirmative("Save").
				Negative("Cancel"),
		).Title("Confirm").Description("Save configuration"),
	).WithTheme(theme)
}

// CreateBibdSetupForm creates the setup form for bibd daemon
func CreateBibdSetupForm(data *SetupData) *huh.Form {
	theme := HuhTheme()

	var portStr string
	if data.Port > 0 {
		portStr = strconv.Itoa(data.Port)
	} else {
		portStr = "8080"
	}

	return huh.NewForm(
		// Identity Group
		huh.NewGroup(
			huh.NewNote().
				Title("👤 Identity").
				Description("Daemon identity for P2P networking and signatures."),

			huh.NewInput().
				Title("Name").
				Description("Daemon display name").
				Placeholder("My Node").
				Value(&data.Name),

			huh.NewInput().
				Title("Email").
				Description("Contact email (optional)").
				Placeholder("admin@example.com").
				Value(&data.Email),
		).Title("Identity").Description("Configure daemon identity"),

		// Server Group
		huh.NewGroup(
			huh.NewNote().
				Title("🖥️  Server Settings").
				Description("Network configuration for the daemon."),

			huh.NewInput().
				Title("Listen Host").
				Description("Host address to bind to").
				Placeholder("0.0.0.0").
				Value(&data.Host),

			huh.NewInput().
				Title("Listen Port").
				Description("Port number for API").
				Placeholder("8080").
				Value(&portStr).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					p, err := strconv.Atoi(s)
					if err != nil || p < 1 || p > 65535 {
						return fmt.Errorf("port must be 1-65535")
					}
					data.Port = p
					return nil
				}),

			huh.NewInput().
				Title("Data Directory").
				Description("Where to store data").
				Placeholder("~/.local/share/bibd").
				Value(&data.DataDir),
		).Title("Server").Description("Configure server settings"),

		// TLS Enable/Disable Group
		huh.NewGroup(
			huh.NewNote().
				Title("🔒 TLS Settings").
				Description("Secure connections with TLS."),

			huh.NewConfirm().
				Title("Enable TLS").
				Description("Encrypt connections with TLS").
				Affirmative("Yes").
				Negative("No").
				Value(&data.TLSEnabled),
		).Title("TLS").Description("Configure TLS encryption"),

		// TLS Certificate Group - only shown if TLS enabled
		huh.NewGroup(
			huh.NewNote().
				Title("🔒 TLS Certificates").
				Description("Provide your TLS certificate and key files."),

			huh.NewInput().
				Title("Certificate File").
				Description("Path to TLS certificate").
				Placeholder("/etc/bibd/cert.pem").
				Value(&data.CertFile),

			huh.NewInput().
				Title("Key File").
				Description("Path to TLS private key").
				Placeholder("/etc/bibd/key.pem").
				Value(&data.KeyFile),
		).Title("TLS Certificates").
			WithHideFunc(func() bool { return !data.TLSEnabled }),

		// Storage Group
		huh.NewGroup(
			huh.NewNote().
				Title("💾 Storage").
				Description("Database backend for storing data."),

			huh.NewSelect[string]().
				Title("Storage Backend").
				Description("Database to use for storage").
				Options(
					huh.NewOption("SQLite (lightweight, local cache)", "sqlite"),
					huh.NewOption("PostgreSQL (full replication, authoritative)", "postgres"),
				).
				Value(&data.StorageBackend),
		).Title("Storage").Description("Configure storage backend"),

		// P2P Enable Group
		huh.NewGroup(
			huh.NewNote().
				Title("🌐 P2P Networking").
				Description("Peer-to-peer networking settings."),

			huh.NewConfirm().
				Title("Enable P2P").
				Description("Enable peer-to-peer networking").
				Affirmative("Yes").
				Negative("No").
				Value(&data.P2PEnabled),
		).Title("P2P").Description("Configure P2P networking"),

		// P2P Mode Group - only shown if P2P enabled
		huh.NewGroup(
			huh.NewNote().
				Title("🌐 P2P Mode").
				Description("Select how this node participates in the network."),

			huh.NewSelect[string]().
				Title("P2P Mode").
				Description("How this node participates in the network").
				Options(
					huh.NewOption("Proxy - Forward requests, no local storage", "proxy"),
					huh.NewOption("Selective - Subscribe to specific topics", "selective"),
					huh.NewOption("Full - Replicate all data (requires PostgreSQL)", "full"),
				).
				Value(&data.P2PMode),
		).Title("P2P Mode").
			WithHideFunc(func() bool { return !data.P2PEnabled }),

		// Logging Group
		huh.NewGroup(
			huh.NewNote().
				Title("📝 Logging").
				Description("Configure logging output."),

			huh.NewSelect[string]().
				Title("Log Level").
				Description("Verbosity of log output").
				Options(
					huh.NewOption("Debug - Detailed debugging info", "debug"),
					huh.NewOption("Info - General information", "info"),
					huh.NewOption("Warning - Warnings only", "warn"),
					huh.NewOption("Error - Errors only", "error"),
				).
				Value(&data.LogLevel),

			huh.NewSelect[string]().
				Title("Log Format").
				Description("Format of log messages").
				Options(
					huh.NewOption("Pretty - Colored, human-readable", "pretty"),
					huh.NewOption("Text - Plain text", "text"),
					huh.NewOption("JSON - Machine-readable", "json"),
				).
				Value(&data.LogFormat),
		).Title("Logging").Description("Configure logging"),

		// Cluster Enable Group
		huh.NewGroup(
			huh.NewNote().
				Title("🔷 Cluster (Optional)").
				Description("High availability cluster settings."),

			huh.NewConfirm().
				Title("Enable Clustering").
				Description("Join or create an HA cluster").
				Affirmative("Yes").
				Negative("No").
				Value(&data.ClusterEnabled),
		).Title("Cluster").Description("Configure HA clustering"),

		// Cluster Settings Group - only shown if clustering enabled
		huh.NewGroup(
			huh.NewNote().
				Title("🔷 Cluster Settings").
				Description("Configure your cluster settings."),

			huh.NewInput().
				Title("Cluster Name").
				Description("Unique name for this cluster").
				Placeholder("bib-cluster").
				Value(&data.ClusterName),

			huh.NewInput().
				Title("Raft Listen Address").
				Description("Address for inter-node communication").
				Placeholder("0.0.0.0:4002").
				Value(&data.ClusterAddr),

			huh.NewConfirm().
				Title("Bootstrap New Cluster").
				Description("Initialize as the first node in a new cluster").
				Affirmative("Yes, create new cluster").
				Negative("No, join existing").
				Value(&data.Bootstrap),
		).Title("Cluster Settings").
			WithHideFunc(func() bool { return !data.ClusterEnabled }),

		// Confirmation Group
		huh.NewGroup(
			huh.NewNote().
				Title("✅ Ready to Save").
				Description("Review your settings and save the configuration."),

			huh.NewConfirm().
				Title("Save Configuration?").
				Affirmative("Save").
				Negative("Cancel"),
		).Title("Confirm").Description("Save configuration"),
	).WithTheme(theme)
}

// CreateClusterSetupForm creates the cluster initialization form
func CreateClusterSetupForm(data *SetupData) *huh.Form {
	theme := HuhTheme()

	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("🔷 Cluster Configuration").
				Description("Initialize a new HA cluster."),

			huh.NewInput().
				Title("Cluster Name").
				Description("Unique name for this cluster").
				Placeholder("bib-cluster").
				Value(&data.ClusterName),

			huh.NewInput().
				Title("Raft Listen Address").
				Description("Address for inter-node communication").
				Placeholder("0.0.0.0:4002").
				Value(&data.ClusterAddr),

			huh.NewInput().
				Title("Advertise Address").
				Description("Address other nodes use to reach this node").
				Placeholder("node1.example.com:4002").
				Value(&data.AdvertiseAddr),
		).Title("Cluster").Description("Configure cluster settings"),
	).WithTheme(theme)
}

// CreateClusterJoinForm creates the cluster join form
func CreateClusterJoinForm(data *SetupData, clusterName, leaderAddr string) *huh.Form {
	theme := HuhTheme()

	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("🔷 Join Cluster").
				Description(fmt.Sprintf("Joining cluster: %s\nLeader: %s", clusterName, leaderAddr)),

			huh.NewInput().
				Title("Raft Listen Address").
				Description("Address for inter-node communication").
				Placeholder("0.0.0.0:4002").
				Value(&data.ClusterAddr),

			huh.NewInput().
				Title("Advertise Address").
				Description("Address other nodes use to reach this node").
				Placeholder("node2.example.com:4002").
				Value(&data.AdvertiseAddr),

			huh.NewConfirm().
				Title("Join as Voter").
				Description("Voters participate in leader election").
				Affirmative("Yes").
				Negative("No (Non-voter)").
				Value(&data.IsVoter),
		).Title("Join Cluster").Description("Configure cluster join settings"),
	).WithTheme(theme)
}

// RenderSetupSummary renders a summary of the setup data
func RenderSetupSummary(data *SetupData, isDaemon bool) string {
	theme := themes.Global().Active()
	kv := NewKVRenderer()

	var b strings.Builder

	// Header
	b.WriteString(theme.Title.Render("Configuration Summary"))
	b.WriteString("\n\n")

	// Identity section
	b.WriteString(Header("Identity"))
	b.WriteString("\n")
	b.WriteString(kv.Render("Name", data.Name))
	b.WriteString("\n")
	b.WriteString(kv.Render("Email", data.Email))
	b.WriteString("\n\n")

	if isDaemon {
		// Server section
		b.WriteString(Header("Server"))
		b.WriteString("\n")
		b.WriteString(kv.Render("Host", data.Host))
		b.WriteString("\n")
		b.WriteString(kv.Render("Port", strconv.Itoa(data.Port)))
		b.WriteString("\n")
		b.WriteString(kv.Render("Data Directory", data.DataDir))
		b.WriteString("\n")
		b.WriteString(kv.Render("TLS Enabled", boolToYesNo(data.TLSEnabled)))
		b.WriteString("\n\n")

		// Storage section
		b.WriteString(Header("Storage"))
		b.WriteString("\n")
		b.WriteString(kv.Render("Backend", data.StorageBackend))
		b.WriteString("\n\n")

		// P2P section
		if data.P2PEnabled {
			b.WriteString(Header("P2P Networking"))
			b.WriteString("\n")
			b.WriteString(kv.Render("Enabled", "Yes"))
			b.WriteString("\n")
			b.WriteString(kv.Render("Mode", data.P2PMode))
			b.WriteString("\n\n")
		}

		// Cluster section
		if data.ClusterEnabled {
			b.WriteString(Header("Cluster"))
			b.WriteString("\n")
			b.WriteString(kv.Render("Cluster Name", data.ClusterName))
			b.WriteString("\n")
			b.WriteString(kv.Render("Listen Address", data.ClusterAddr))
			b.WriteString("\n")
			b.WriteString(kv.Render("Is Voter", boolToYesNo(data.IsVoter)))
			b.WriteString("\n\n")
		}
	} else {
		// Output section (bib CLI)
		b.WriteString(Header("Output"))
		b.WriteString("\n")
		b.WriteString(kv.Render("Format", data.OutputFormat))
		b.WriteString("\n")
		b.WriteString(kv.Render("Colors", boolToYesNo(data.ColorEnabled)))
		b.WriteString("\n\n")

		// Connection section
		b.WriteString(Header("Connection"))
		b.WriteString("\n")
		b.WriteString(kv.Render("Server", data.ServerAddr))
		b.WriteString("\n\n")
	}

	// Logging section
	b.WriteString(Header("Logging"))
	b.WriteString("\n")
	b.WriteString(kv.Render("Level", data.LogLevel))
	b.WriteString("\n")
	if isDaemon {
		b.WriteString(kv.Render("Format", data.LogFormat))
		b.WriteString("\n")
	}

	return b.String()
}

// RenderWelcome renders the welcome screen for setup
func RenderWelcome(isDaemon bool) string {
	theme := themes.Global().Active()
	var b strings.Builder

	// Banner
	b.WriteString(Banner())
	b.WriteString("\n\n")

	if isDaemon {
		b.WriteString(theme.Title.Render("bibd Daemon Setup"))
		b.WriteString("\n\n")
		b.WriteString(theme.Description.Render("Welcome! This wizard will help you configure the bibd daemon."))
		b.WriteString("\n\n")
		b.WriteString(theme.Base.Render("We'll configure:"))
		b.WriteString("\n")
		b.WriteString(BulletList([]string{
			"Identity - Your daemon's identity",
			"Server - Network and API settings",
			"Storage - Database backend",
			"P2P - Peer-to-peer networking",
			"Logging - Log output settings",
		}))
	} else {
		b.WriteString(theme.Title.Render("bib CLI Setup"))
		b.WriteString("\n\n")
		b.WriteString(theme.Description.Render("Welcome! This wizard will help you configure the bib CLI."))
		b.WriteString("\n\n")
		b.WriteString(theme.Base.Render("We'll configure:"))
		b.WriteString("\n")
		b.WriteString(BulletList([]string{
			"Identity - Your name and email",
			"Output - Display preferences",
			"Connection - How to reach bibd",
		}))
	}

	b.WriteString("\n\n")
	b.WriteString(theme.Help.Render("Press Enter to continue..."))

	return b.String()
}

// RenderSuccess renders the success screen
func RenderSuccess(configPath string, isDaemon bool) string {
	theme := themes.Global().Active()
	status := NewStatusIndicator()

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(status.Success("Configuration saved successfully!"))
	b.WriteString("\n\n")

	b.WriteString(theme.Base.Render("Config file: "))
	b.WriteString(theme.Focused.Render(configPath))
	b.WriteString("\n\n")

	if isDaemon {
		b.WriteString(theme.Base.Render("Start the daemon with:"))
		b.WriteString("\n")
		b.WriteString(theme.Focused.Render("  bibd"))
		b.WriteString("\n")
	} else {
		b.WriteString(theme.Base.Render("You're all set! Try:"))
		b.WriteString("\n")
		b.WriteString(theme.Focused.Render("  bib --help"))
		b.WriteString("\n")
	}

	return b.String()
}

// RenderClusterSuccess renders the cluster initialization success screen
func RenderClusterSuccess(configPath, nodeID, clusterName, joinToken string) string {
	theme := themes.Global().Active()
	status := NewStatusIndicator()

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(status.Success("Cluster initialized successfully!"))
	b.WriteString("\n\n")

	b.WriteString(theme.Base.Render("Config file: "))
	b.WriteString(theme.Focused.Render(configPath))
	b.WriteString("\n\n")

	b.WriteString(theme.SectionTitle.Render("Cluster Details"))
	b.WriteString("\n")
	b.WriteString(NewKVRenderer().Render("Node ID", nodeID))
	b.WriteString("\n")
	b.WriteString(NewKVRenderer().Render("Cluster", clusterName))
	b.WriteString("\n\n")

	b.WriteString(theme.SectionTitle.Render("Join Token"))
	b.WriteString("\n")
	b.WriteString(theme.Description.Render("Share this token with other nodes to join the cluster:"))
	b.WriteString("\n\n")

	// Box around join token
	tokenBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Render(joinToken)
	b.WriteString(tokenBox)
	b.WriteString("\n\n")

	b.WriteString(theme.Base.Render("To join another node:"))
	b.WriteString("\n")
	b.WriteString(theme.Focused.Render(fmt.Sprintf("  bib setup --daemon --cluster-join %s", joinToken)))
	b.WriteString("\n\n")

	b.WriteString(theme.Warning.Render("⚠ Minimum 3 voting nodes required for HA quorum."))
	b.WriteString("\n")

	return b.String()
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// ToBibConfig converts setup data to a BibConfig
func (d *SetupData) ToBibConfig() *config.BibConfig {
	return &config.BibConfig{
		Log: config.LogConfig{
			Level:  d.LogLevel,
			Format: "text",
			Output: "stderr",
		},
		Identity: config.IdentityConfig{
			Name:  d.Name,
			Email: d.Email,
		},
		Output: config.OutputConfig{
			Format: d.OutputFormat,
			Color:  d.ColorEnabled,
		},
		Server: d.ServerAddr,
	}
}

// ToBibdConfig converts setup data to a BibdConfig
func (d *SetupData) ToBibdConfig() *config.BibdConfig {
	cfg := &config.BibdConfig{
		Log: config.LogConfig{
			Level:  d.LogLevel,
			Format: d.LogFormat,
			Output: "stdout",
		},
		Identity: config.IdentityConfig{
			Name:  d.Name,
			Email: d.Email,
		},
		Server: config.ServerConfig{
			Host:    d.Host,
			Port:    d.Port,
			DataDir: d.DataDir,
			PIDFile: "/var/run/bibd.pid",
			TLS: config.TLSConfig{
				Enabled:  d.TLSEnabled,
				CertFile: d.CertFile,
				KeyFile:  d.KeyFile,
			},
		},
		Database: config.DatabaseConfig{
			Backend: d.StorageBackend,
		},
		P2P: config.P2PConfig{
			Enabled: d.P2PEnabled,
			Mode:    d.P2PMode,
		},
	}

	if d.ClusterEnabled {
		cfg.Cluster = config.ClusterConfig{
			Enabled:       true,
			ClusterName:   d.ClusterName,
			ListenAddr:    d.ClusterAddr,
			AdvertiseAddr: d.AdvertiseAddr,
			IsVoter:       d.IsVoter,
			Bootstrap:     d.Bootstrap,
			JoinToken:     d.JoinToken,
		}
	}

	if d.BreakGlassEnabled {
		// Parse duration
		maxDuration, err := time.ParseDuration(d.BreakGlassMaxDuration)
		if err != nil {
			maxDuration = 1 * time.Hour
		}

		cfg.Database.BreakGlass = config.BreakGlassConfig{
			Enabled:               true,
			RequireRestart:        true,
			MaxDuration:           maxDuration,
			DefaultAccessLevel:    d.BreakGlassAccessLevel,
			AuditLevel:            "paranoid",
			RequireAcknowledgment: true,
			SessionRecording:      true,
		}

		// Add configured user if provided
		if d.BreakGlassUserName != "" && d.BreakGlassUserKey != "" {
			cfg.Database.BreakGlass.AllowedUsers = []config.BreakGlassUser{
				{
					Name:      d.BreakGlassUserName,
					PublicKey: d.BreakGlassUserKey,
				},
			}
		}
	}

	return cfg
}

// Backward compatibility helpers for setup.go

// KVRenderer is a helper for rendering key-value pairs
type KVRenderer struct {
	theme *themes.Theme
}

// NewKVRenderer creates a new key-value renderer (backward compat)
func NewKVRenderer() *KVRenderer {
	return &KVRenderer{theme: themes.Global().Active()}
}

// Render renders a key-value pair
func (kv *KVRenderer) Render(key, value string) string {
	return component.NewKeyValue(key, value).View(60)
}

// Header renders a section header (backward compat)
func Header(title string) string {
	return themes.Global().Active().SectionTitle.Render(title)
}

// HuhTheme returns a huh theme (backward compat)
func HuhTheme() *huh.Theme {
	return themes.Global().Active().HuhTheme()
}

// ColorPrimary for backward compat
var ColorPrimary = themes.Global().Active().Palette.Primary

// Banner returns the ASCII art banner for bib (backward compat)
func Banner() string {
	banner := `
 ██████╗ ██╗██████╗ 
 ██╔══██╗██║██╔══██╗
 ██████╔╝██║██████╔╝
 ██╔══██╗██║██╔══██╗
 ██████╔╝██║██████╔╝
 ╚═════╝ ╚═╝╚═════╝ `

	return lipgloss.NewStyle().
		Foreground(themes.Global().Active().Palette.Primary).
		Bold(true).
		Render(banner)
}

// BulletList renders a list of bullet points (backward compat)
func BulletList(items []string) string {
	theme := themes.Global().Active()
	var lines []string
	for _, item := range items {
		lines = append(lines, theme.Base.Render("  "+themes.IconBullet+" "+item))
	}
	return strings.Join(lines, "\n")
}

// StatusIndicator for backward compat
type StatusIndicator struct {
	theme *themes.Theme
}

// NewStatusIndicator creates a new status indicator (backward compat)
func NewStatusIndicator() *StatusIndicator {
	return &StatusIndicator{theme: themes.Global().Active()}
}

// Success renders a success message
func (s *StatusIndicator) Success(message string) string {
	return component.Success(message).View(0)
}

// Error renders an error message
func (s *StatusIndicator) Error(message string) string {
	return component.Error(message).View(0)
}

// Warning renders a warning message
func (s *StatusIndicator) Warning(message string) string {
	return component.Warning(message).View(0)
}

// Info renders an info message
func (s *StatusIndicator) Info(message string) string {
	return component.Info(message).View(0)
}
