package cmd

import (
	"bib/internal/config"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	Config     *config.BibConfig
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

		// Enforce allowed bootstrap commands before identity exists.
		return enforcePreIdentityGate()
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

// enforcePreIdentityGate returns an error if user tries any command other than:
// version, mission, setup (until identity creation logic is implemented).
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

// firstNonFlagArg finds the first argument that is not a flag (starts without '-')
func firstNonFlagArg() string {
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a
	}
	return ""
}
