package configcmd

import (
	"bib/internal/config"

	"github.com/spf13/cobra"
)

// configPathCmd shows config file path
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Long:  `Display the path to the configuration file being used.`,
	RunE:  runConfigPath,
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	out := NewOutputWriter()

	var appName string
	if configDaemon {
		appName = config.AppBibd
	} else {
		appName = config.AppBib
	}

	cfgFile := ConfigFile()
	if cfgFile != "" && !configDaemon {
		out.Write(cfgFile)
		return nil
	}

	if path := config.ConfigFileUsed(appName); path != "" {
		out.Write(path)
		return nil
	}

	out.Write("No config file found, using defaults")
	return nil
}
