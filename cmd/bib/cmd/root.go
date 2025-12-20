package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"bib/cmd/bib/cmd/admin"
	certcmd "bib/cmd/bib/cmd/cert"
	configcmd "bib/cmd/bib/cmd/config"
	connectcmd "bib/cmd/bib/cmd/connect"
	"bib/cmd/bib/cmd/demo"
	"bib/cmd/bib/cmd/setup"
	trustcmd "bib/cmd/bib/cmd/trust"
	"bib/cmd/bib/cmd/tui"
	"bib/cmd/bib/cmd/version"
	clii18n "bib/internal/cli/i18n"
	"bib/internal/config"
	"bib/internal/logger"
	"bib/internal/tui/i18n"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// cfgFile is the path to the config file (set via --config flag)
	cfgFile string

	// cfg holds the loaded configuration
	cfg *config.BibConfig

	// log is the logger instance
	log *logger.Logger

	// auditLog is the audit logger instance
	auditLog *logger.AuditLogger

	// cmdStartTime tracks when command execution started
	cmdStartTime time.Time

	// cmdCtx is the command context with logger and command context
	cmdCtx context.Context

	// Global output flags
	outputFormat string
	verboseMode  bool
	localeFlag   string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:         "bib",
	Short:       "bib.short",
	Long:        "bib.long",
	Annotations: map[string]string{"i18n": "true"},
	// Allow flags before or after subcommand
	TraverseChildren: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for setup command when no config exists
		if cmd.Name() == "setup" {
			return nil
		}

		if err := loadConfig(cmd); err != nil {
			return err
		}

		// Initialize logger
		var err error
		log, err = logger.New(cfg.Log)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}

		// Initialize audit logger if configured
		if cfg.Log.AuditPath != "" {
			auditLog, err = logger.NewAuditLogger(cfg.Log.AuditPath, cfg.Log.AuditMaxAgeDays)
			if err != nil {
				log.Warn("failed to initialize audit logger", "error", err)
			}
		}

		// Create command context
		cc := logger.NewCommandContext(cmd, args)
		cmdCtx = logger.WithCommandContext(context.Background(), cc)
		cmdCtx = logger.WithLogger(cmdCtx, log)

		// Track start time for duration logging
		cmdStartTime = time.Now()

		// Log command start
		log.Debug("command started",
			"command", cc.Command,
			"args", cc.Args,
			"request_id", cc.RequestID,
			"user", cc.User,
			"hostname", cc.Hostname,
			"working_dir", cc.WorkingDir,
		)

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if log == nil {
			return nil
		}

		duration := time.Since(cmdStartTime)
		cc := logger.CommandContextFrom(cmdCtx)

		log.Debug("command completed",
			"command", cc.Command,
			"duration_ms", duration.Milliseconds(),
			"request_id", cc.RequestID,
		)

		// Log to audit if configured
		if auditLog != nil {
			auditLog.LogCommand(cmdCtx, cc.Command, logger.AuditOutcomeSuccess, map[string]any{
				"duration_ms": duration.Milliseconds(),
				"args":        cc.Args,
			})
		}

		// Cleanup
		if auditLog != nil {
			auditLog.Close()
		}
		log.Close()

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(onInitialize)

	// Global persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/bib/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "output format (json, yaml, table, quiet)")
	rootCmd.PersistentFlags().BoolVarP(&verboseMode, "verbose", "v", false, "verbose output (includes full log output)")
	rootCmd.PersistentFlags().StringVarP(&localeFlag, "locale", "L", "", "UI locale (en, de, fr, ru, zh-tw). Overrides config and system locale")
	rootCmd.PersistentFlags().StringVar(GetNodeFlag(), "node", "", "daemon address to connect to (overrides config)")

	// Bind flags to viper
	viper.BindPFlag("output.format", rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("locale", rootCmd.PersistentFlags().Lookup("locale"))

	// Add subcommands from subdirectories
	rootCmd.AddCommand(admin.NewCommand())
	rootCmd.AddCommand(certcmd.NewCommand())
	rootCmd.AddCommand(configcmd.NewCommand())
	rootCmd.AddCommand(connectcmd.NewCommand())
	rootCmd.AddCommand(demo.NewCommand())
	rootCmd.AddCommand(setup.NewCommand())
	rootCmd.AddCommand(trustcmd.NewCommand())
	rootCmd.AddCommand(tui.NewCommand())
	rootCmd.AddCommand(version.NewCommand())

	// Initialize i18n early for help text translation
	// This happens before flags are parsed, so we use config + system locale only
	// The locale flag will re-translate in PersistentPreRunE if provided
	initI18n()
}

// initI18n initializes i18n with config/system locale for help text.
// This is called in init() before flags are parsed.
func initI18n() {
	// Try to load config for locale setting
	var configLocale string
	if earlyConfig, err := config.LoadBib(""); err == nil {
		configLocale = earlyConfig.Locale
	}

	// Check for --locale or -L flag in os.Args (before cobra parses them)
	flagLocale := parseLocaleFlagFromArgs()

	// Resolve locale: flag > config > system
	resolvedLocale := i18n.ResolveLocale(flagLocale, configLocale)
	_ = i18n.Global().SetLocale(resolvedLocale)

	// Translate all commands
	clii18n.TranslateCommands(rootCmd)
}

// parseLocaleFlagFromArgs scans os.Args for --locale or -L flag value.
// This is needed because we need to translate commands before cobra parses flags.
func parseLocaleFlagFromArgs() string {
	args := os.Args[1:] // skip program name
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle --locale=value or -L=value
		if len(arg) > 9 && arg[:9] == "--locale=" {
			return arg[9:]
		}
		if len(arg) > 3 && arg[:3] == "-L=" {
			return arg[3:]
		}

		// Handle --locale value or -L value
		if (arg == "--locale" || arg == "-L") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// onInitialize is called before any command runs
func onInitialize() {
	// Auto-generate config on first run if setup command is not being used
	if cfgFile == "" {
		path, created, err := config.GenerateConfigIfNotExists(config.AppBib, "yaml")
		if err == nil && created {
			// Can't use logger yet, it's not initialized
			fmt.Fprintf(os.Stderr, "Created default config at %s\n", path)
			fmt.Fprintln(os.Stderr, "Run 'bib setup' to customize your configuration.")
		}
	}
}

// loadConfig loads the configuration
func loadConfig(cmd *cobra.Command) error {
	var err error
	cfg, err = config.LoadBib(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply flag overrides
	if cmd.Flags().Changed("output") {
		cfg.Output.Format = viper.GetString("output.format")
	}

	// Initialize i18n with resolved locale (flag > config > system)
	resolvedLocale := i18n.ResolveLocale(localeFlag, cfg.Locale)
	if err := i18n.Global().SetLocale(resolvedLocale); err != nil {
		// Warn but continue with default locale
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: locale %q not available, using %s\n", resolvedLocale, i18n.DefaultLocale)
	}

	// Translate all commands marked with i18n annotation
	clii18n.TranslateCommands(cmd.Root())

	return nil
}

// Config returns the current configuration (for use by subcommands)
func Config() *config.BibConfig {
	return cfg
}

// ConfigFile returns the config file path (for use by subcommands)
func ConfigFile() string {
	return cfgFile
}

// Log returns the logger instance (for use by subcommands)
func Log() *logger.Logger {
	return log
}

// AuditLog returns the audit logger instance (for use by subcommands)
func AuditLog() *logger.AuditLogger {
	return auditLog
}

// Context returns the command context (for use by subcommands)
func Context() context.Context {
	return cmdCtx
}

// OutputFormat returns the current output format (json, yaml, table, quiet)
func OutputFormat() string {
	return outputFormat
}

// IsVerbose returns whether verbose mode is enabled
func IsVerbose() bool {
	return verboseMode
}

// Locale returns the current locale
func Locale() string {
	return i18n.Global().Locale()
}
