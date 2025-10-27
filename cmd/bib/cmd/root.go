package cmd

import (
	"bib/internal/config"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	Config     *config.BibConfig
	ConfigPath string
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
		return ensureConfigLoaded()
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

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.bib.yaml)")
	rootCmd.PersistentFlags().StringP("theme", "t", "auto", "Color theme for output: auto, dark, light")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Enable env overrides like GENERAL_THEME via GENERAL_THEME
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}

func ensureConfigLoaded() error {
	if Config != nil {
		return nil
	}

	// Enable env overrides like GENERAL_* via GENERAL_* and UPDATE_* via UPDATE_*
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Always register defaults so they are available even if a file isn't found.
	// Defaults are overridden by file values and env vars.
	config.ApplyBibDefaults(viper.GetViper())

	// 1) If --config provided, use that file directly
	if strings.TrimSpace(cfgFile) != "" {
		path := expandHome(cfgFile)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("config file %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("config path %q is not a file", path)
		}
		return loadFromPath(path)
	}

	// 2) Historical default: $HOME/.bib.yaml or $HOME/.bib.yml
	if p := probeHomeDotBib(); p != "" {
		return loadFromPath(p)
	}

	// 3) Use loader search (supports BIB_CONFIG env, app dir, CWD, XDG, etc.)
	path, err := config.FindConfigPath(config.Options{
		AppName:      "bib",
		FileNames:    []string{"config.yaml", "config.yml", "bib.yaml", "bib.yml"},
		AlsoCheckCWD: true,
	})
	if err != nil {
		// If no config file was found, proceed with defaults + env
		if errors.Is(err, config.ErrConfigNotFound) {
			var cfg config.BibConfig
			if err := viper.Unmarshal(&cfg); err != nil {
				return err
			}
			Config = &cfg
			ConfigPath = ""
			log.Info("No config file found! Run 'bib setup --help' to learn how to create one.")
			return nil
		}
		// Any other error should be returned
		return err
	}

	// Found a file; load and override defaults with file values
	return loadFromPath(path)
}

func loadFromPath(path string) error {
	cfg, err := config.LoadBibConfig(path)
	if err != nil {
		return err
	}
	Config = cfg
	ConfigPath = path
	log.Info("Using config file:", path)
	return nil
}

func probeHomeDotBib() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(home, ".bib.yaml"),
		filepath.Join(home, ".bib.yml"),
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

func expandHome(p string) string {
	if p == "" || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~"))
}
