package main

import (
	"bib/internal/capcheck"
	"bib/internal/capcheck/checks"
	"bib/internal/config"
	"bib/internal/config/util"
	"context"
	"time"

	"github.com/charmbracelet/log"
)

func main() {
	configPath, err := util.FindConfigPath(util.Options{AppName: "bib-daemon", FileNames: []string{"bib-daemon.yaml", "bib-daemon.yml"}})
	cfg, err := config.LoadBibDaemonConfig(configPath)
	if err != nil {
		log.Error(err)
		log.Info("You need to create the bib daemon configuration file before running the daemon. We recommend you use the cli to create it. Run 'bib setup --daemon' to create a configuration file for the daemon.")
		return
	}

	log.Info("Starting bib daemon...")
	log.Info("It might take a while to index your library the first time. Please be patient.")

	if cfg.General.CheckCapabilities {
		checkers := []capcheck.Checker{
			checks.ContainerRuntimeChecker{},
			checks.KubernetesConfigChecker{},
			checks.InternetAccessChecker{
				// Use a target that works in your environment. Alternatives:
				// "https://www.google.com/generate_204" or a company endpoint.
				HTTPURL: "https://www.google.com/generate_204",
			},
			checks.ResourcesChecker{},
		}

		runner := capcheck.NewRunner(
			checkers,
			capcheck.WithGlobalTimeout(6*time.Second),
			capcheck.WithPerCheckTimeout(1*time.Second),
		)

		log.Info("Checking system capabilities...")
		ctx := context.Background()
		results := runner.Run(ctx)
		for _, result := range results {
			if result.Error != "" {
				log.Errorf("⛔  %s(%s) failed at %s in (%s)", result.Name, result.ID, result.CheckedAt, result.Duration)
				log.Error(result.Error)
			} else {
				log.Infof("✅  %s(%s) passed at %s in (%s)", result.Name, result.ID, result.CheckedAt, result.Duration)
			}
		}
	}

	log.Info("The bib daemon is running.")
}
