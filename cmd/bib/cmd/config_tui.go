package cmd

import (
	"fmt"
	"os"
	"strconv"

	"bib/internal/config"
	"bib/internal/tui"

	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"
)

// runConfigTUI launches the interactive config editor
func runConfigTUI(isDaemon bool) error {
	var title string
	var cfgPath string
	var appName string

	theme := tui.GetTheme()

	if isDaemon {
		title = "bibd Daemon Configuration"
		appName = config.AppBibd
		cfgPath = config.ConfigFileUsed(appName)

		// Load existing config or use defaults
		cfg, _ := config.LoadBibd("")
		if cfg == nil {
			defaultCfg := config.DefaultBibdConfig()
			cfg = &defaultCfg
		}

		// Ensure config file path exists
		if cfgPath == "" {
			path, _, err := config.GenerateConfigIfNotExists(appName, "yaml")
			if err != nil {
				return fmt.Errorf("failed to create config file: %w", err)
			}
			cfgPath = path
		}

		// Clear screen and show title
		fmt.Print("\033[H\033[2J")
		fmt.Println(theme.Title.Render(title))
		fmt.Println(theme.Description.Render("Edit your bibd configuration"))
		fmt.Println()

		return runBibdConfigForm(cfg, cfgPath)
	}

	title = "bib CLI Configuration"
	appName = config.AppBib
	cfgPath = config.ConfigFileUsed(appName)

	// Load existing config or use defaults
	cfg, _ := config.LoadBib("")
	if cfg == nil {
		cfg = config.DefaultBibConfig()
	}

	// Ensure config file path exists
	if cfgPath == "" {
		path, _, err := config.GenerateConfigIfNotExists(appName, "yaml")
		if err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
		cfgPath = path
	}

	// Clear screen and show title
	fmt.Print("\033[H\033[2J")
	fmt.Println(theme.Title.Render(title))
	fmt.Println(theme.Description.Render("Edit your bib configuration"))
	fmt.Println()

	return runBibConfigForm(cfg, cfgPath)
}

func runBibConfigForm(cfg *config.BibConfig, cfgPath string) error {
	theme := tui.HuhTheme()

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("👤 Identity"),

			huh.NewInput().
				Title("Name").
				Description("Your display name").
				Value(&cfg.Identity.Name),

			huh.NewInput().
				Title("Email").
				Description("Your email address").
				Value(&cfg.Identity.Email),
		),

		huh.NewGroup(
			huh.NewNote().
				Title("📺 Output"),

			huh.NewSelect[string]().
				Title("Format").
				Options(
					huh.NewOption("Table", "table"),
					huh.NewOption("JSON", "json"),
					huh.NewOption("YAML", "yaml"),
					huh.NewOption("Text", "text"),
				).
				Value(&cfg.Output.Format),

			huh.NewConfirm().
				Title("Color Output").
				Value(&cfg.Output.Color),
		),

		huh.NewGroup(
			huh.NewNote().
				Title("📝 Logging"),

			huh.NewSelect[string]().
				Title("Log Level").
				Options(
					huh.NewOption("Debug", "debug"),
					huh.NewOption("Info", "info"),
					huh.NewOption("Warning", "warn"),
					huh.NewOption("Error", "error"),
				).
				Value(&cfg.Log.Level),
		),

		huh.NewGroup(
			huh.NewNote().
				Title("🔗 Server"),

			huh.NewInput().
				Title("Server Address").
				Description("bibd server address (host:port)").
				Value(&cfg.Server),
		),
	).WithTheme(theme)

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nCancelled.")
			return nil
		}
		return err
	}

	// Save config
	if err := saveConfig(cfgPath, cfg); err != nil {
		return err
	}

	status := tui.NewStatusIndicator()
	fmt.Println()
	fmt.Println(status.Success("Configuration saved!"))
	fmt.Println(tui.NewKVRenderer().Render("Path", cfgPath))
	return nil
}

func runBibdConfigForm(cfg *config.BibdConfig, cfgPath string) error {
	theme := tui.HuhTheme()

	portStr := strconv.Itoa(cfg.Server.Port)
	p2pEnabled := cfg.P2P.Enabled
	clusterEnabled := cfg.Cluster.Enabled

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("🖥️  Server"),

			huh.NewInput().
				Title("Host").
				Value(&cfg.Server.Host),

			huh.NewInput().
				Title("Port").
				Value(&portStr).
				Validate(func(s string) error {
					p, err := strconv.Atoi(s)
					if err != nil || p < 1 || p > 65535 {
						return fmt.Errorf("port must be 1-65535")
					}
					cfg.Server.Port = p
					return nil
				}),

			huh.NewInput().
				Title("Data Directory").
				Value(&cfg.Server.DataDir),
		),

		huh.NewGroup(
			huh.NewNote().
				Title("💾 Database"),

			huh.NewSelect[string]().
				Title("Backend").
				Options(
					huh.NewOption("SQLite", "sqlite"),
					huh.NewOption("PostgreSQL", "postgres"),
				).
				Value(&cfg.Database.Backend),
		),

		huh.NewGroup(
			huh.NewNote().
				Title("🌐 P2P Networking"),

			huh.NewConfirm().
				Title("Enabled").
				Value(&p2pEnabled),

			huh.NewSelect[string]().
				Title("Mode").
				Options(
					huh.NewOption("Proxy", "proxy"),
					huh.NewOption("Selective", "selective"),
					huh.NewOption("Full", "full"),
				).
				Value(&cfg.P2P.Mode),
		),

		huh.NewGroup(
			huh.NewNote().
				Title("📝 Logging"),

			huh.NewSelect[string]().
				Title("Log Level").
				Options(
					huh.NewOption("Debug", "debug"),
					huh.NewOption("Info", "info"),
					huh.NewOption("Warning", "warn"),
					huh.NewOption("Error", "error"),
				).
				Value(&cfg.Log.Level),

			huh.NewSelect[string]().
				Title("Log Format").
				Options(
					huh.NewOption("Pretty", "pretty"),
					huh.NewOption("Text", "text"),
					huh.NewOption("JSON", "json"),
				).
				Value(&cfg.Log.Format),
		),

		huh.NewGroup(
			huh.NewNote().
				Title("🔷 Cluster"),

			huh.NewConfirm().
				Title("Enabled").
				Value(&clusterEnabled),

			huh.NewInput().
				Title("Node ID").
				Description("Auto-generated if empty").
				Value(&cfg.Cluster.NodeID),
		),
	).WithTheme(theme)

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nCancelled.")
			return nil
		}
		return err
	}

	// Apply boolean values
	cfg.P2P.Enabled = p2pEnabled
	cfg.Cluster.Enabled = clusterEnabled

	// Save config
	if err := saveConfig(cfgPath, cfg); err != nil {
		return err
	}

	status := tui.NewStatusIndicator()
	fmt.Println()
	fmt.Println(status.Success("Configuration saved!"))
	fmt.Println(tui.NewKVRenderer().Render("Path", cfgPath))
	return nil
}

func saveConfig(cfgPath string, cfg interface{}) error {
	output, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cfgPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
