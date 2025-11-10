package cmd

import (
	"bib/internal/config"
	"bib/internal/config/util"
	"bib/internal/contexts"
	"bib/internal/ui/models"
	"bib/internal/ui/styles"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	Config     *config.BibConfig
	Identity   *contexts.IdentityContext
	Theme      styles.Theme
	appVersion string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bib",
	Short: "Research & analysis tool for academics",
	Long: `Bib is a command-line tool designed to assist academics
with research and analysis tasks. If you want to know more about the project
and the original idea read our manifesto via:

bib mission`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		if err := ensureConfigAndIdentity(); err != nil {
			var nfErr *notFoundConfigError
			if !errors.As(err, &nfErr) {
				return err
			}
		}

		themeSelected, err := cmd.Flags().GetString("theme")
		if err != nil {
			return fmt.Errorf("failed to get theme flag: %w", err)
		}
		theme, _ := styles.FromMode(themeSelected)
		Theme = theme
		if Identity == nil {
			return enforcePreIdentityGate()
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version string) {
	appVersion = version
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.bib.yaml)")
	rootCmd.PersistentFlags().StringP("theme", "t", "auto", "Color theme for output: auto, dark, light")
	rootCmd.PersistentFlags().Bool("no-tui", false, "Open interactive setup (TUI)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}

func ensureConfigAndIdentity() error {
	// Already loaded
	if Config != nil && Identity != nil {
		return nil
	}

	// 1. Load config if not present
	if Config == nil {
		path, cfgErr := resolveConfigPath()
		if cfgErr != nil {
			// propagate not-found sentinel so caller can decide gate
			return cfgErr
		}
		loadedCfg, err := config.LoadBibConfig(path)
		if err != nil {
			return fmt.Errorf("failed to load config file %s: %w", path, err)
		}
		Config = loadedCfg
	}

	// 2. Load identity if not present
	if Identity == nil && Config != nil {
		pass := ""

		if Config.General.UsePassphrase {
			passphrase, err := models.PromptPassphrase("Enter your identity passphrase: ")
			if err != nil {
				log.Fatal(err)
			}
			pass = passphrase
		} else {
			pass = "example-passphrase"
		}

		ctx, err := contexts.LoadExistingUserIdentity(Config, pass)
		if err != nil {
			// Decide which errors allow fallback to gate vs. fatal
			switch {
			case errors.Is(err, contexts.ErrUserIdentityNotFound):
				// Identity simply does not exist yet; return nil so gating will occur.
				return nil
			case errors.Is(err, contexts.ErrPassphraseRequired):
				return fmt.Errorf("identity requires a passphrase; provide via --passphrase or BIB_PASSPHRASE: %w", err)
			case errors.Is(err, contexts.ErrSecondFactorRequired):
				return fmt.Errorf("second factor required but could not be acquired: %w", err)
			default:
				return fmt.Errorf("failed to load existing user identity: %w", err)
			}
		}
		Identity = ctx
	}

	return nil
}

func resolveConfigPath() (string, error) {
	// Explicit flag path
	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); err == nil {
			return cfgFile, nil
		}
		return "", fmt.Errorf("specified config file not found: %s", cfgFile)
	}

	// Autodiscover
	path, err := util.FindConfigPath(util.Options{
		AppName:      "bib",
		FileNames:    []string{"config.yaml", "config.yml"},
		AlsoCheckCWD: true,
	})
	if err != nil {
		if errors.Is(err, util.ErrConfigNotFound) {
			return "", &notFoundConfigError{}
		}
		return "", err
	}
	return path, nil
}

type notFoundConfigError struct{}

func (e *notFoundConfigError) Error() string { return "config not found" }

func enforcePreIdentityGate() error {
	sub := firstNonFlagArg()
	if sub == "" {
		// No subcommand: allow root (shows help)
		return nil
	}
	allowed := map[string]struct{}{
		"version": {},
		"mission": {},
		"setup":   {},
	}
	if _, ok := allowed[sub]; ok {
		return nil
	}
	return fmt.Errorf("command %q disabled until identity is initialized. Run 'bib setup' first. Allowed: version, mission, setup", sub)
}

func firstNonFlagArg() string {
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return ""
}
