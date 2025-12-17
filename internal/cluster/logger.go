// Package cluster provides Raft-based consensus for HA clusters.
//
// This file provides logger integration for cluster operations.
package cluster

import (
	"bib/internal/logger"
)

// log is the package-level logger for cluster operations.
// It defaults to the default logger but can be set via SetLogger.
var log = logger.Default()

// SetLogger sets the logger for all cluster operations.
// This should be called before creating any cluster instances.
func SetLogger(l *logger.Logger) {
	if l != nil {
		log = l.With("component", "cluster")
	}
}

// getLogger returns a logger with the given subcomponent.
func getLogger(subcomponent string) *logger.Logger {
	return log.With("subcomponent", subcomponent)
}
