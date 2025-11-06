package main

import (
	"bib/internal/config"
	"bib/internal/config/util"
	"bib/internal/daemon"

	"github.com/charmbracelet/log"
)

func main() {
	configPath, err := util.FindConfigPath(util.Options{AppName: "bibd", FileNames: []string{"config.yaml", "config.yml", "bib-daemon.yaml", "bib-daemon.yml"}, AlsoCheckCWD: true})

	cfg, err := config.LoadBibDaemonConfig(configPath)
	if err != nil {
		log.Error(err)
		log.Info("You need to create the bib daemon configuration file before running the daemon. We recommend you use the cli to create it. Run 'bib setup --daemon' to create a configuration file for the daemon.")
		return
	}

	log.Info("Starting bib daemon...")
	log.Info("It might take a while to index your library the first time. Please be patient.")

	daemon.StartCapabilityChecks(cfg)
	daemon.StartScheduler()
	daemon.StartGRPCServer(cfg)
}
