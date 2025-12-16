package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"bib/internal/config"
	"bib/internal/logger"

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
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bib",
	Short: "bib is a CLI client",
	Long:  `bib is a command-line interface client that communicates with the bibd daemon.`,
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

	// Bind flags to viper
	viper.BindPFlag("output.format", rootCmd.PersistentFlags().Lookup("output"))
}

// onInitialize is called before any command runs
func onInitialize() {
	// Auto-generate config on first run if setup command is not being used
	if cfgFile == "" {
		path, created, err := config.GenerateConfigIfNotExists(config.AppBib, "yaml")
		if err == nil && created {
			fmt.Fprintf(os.Stderr, "Created default config at: %s\n", path)
			fmt.Fprintf(os.Stderr, "Run 'bib setup' to customize your configuration.\n")
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
