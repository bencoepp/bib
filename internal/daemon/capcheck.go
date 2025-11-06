package daemon

import (
	"bib/internal/capcheck"
	"bib/internal/capcheck/checks"
	"bib/internal/config"
	"context"
	"time"

	"github.com/charmbracelet/log"
)

func StartCapabilityChecks(config *config.BibDaemonConfig) {
	if config.General.CheckCapabilities {
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
}
