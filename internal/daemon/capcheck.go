package daemon

import (
	"bib/internal/config"
	"bib/internal/watcher"

	"github.com/charmbracelet/log"
)

func StartCapabilityChecks(cfg *config.BibDaemonConfig) *watcher.CapabilityWatcher {
	checks := watcher.StartCapabilityChecks(cfg)

	for key, check := range checks.Current().GenericCapabilities {
		log.Debugf("Capability Check - %s: %v", key, check)
	}

	return checks
}
