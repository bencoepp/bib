package daemon

import (
	"bib/internal/config"
	"bib/internal/contexts"
)

func StartCapabilityChecks(cfg *config.BibDaemonConfig) *contexts.CapabilityWatcher {
	return contexts.StartCapabilityChecks(cfg)
}
