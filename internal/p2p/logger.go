// Package p2p provides libp2p-based peer-to-peer networking for bib.
//
// This file provides logger integration for P2P components.
package p2p

import (
	"bib/internal/logger"
)

// log is the package-level logger for P2P operations.
// It defaults to the default logger but can be set via SetLogger.
var log = logger.Default()

// SetLogger sets the logger for all P2P operations.
// This should be called before creating any P2P components.
func SetLogger(l *logger.Logger) {
	if l != nil {
		log = l.With("component", "p2p")
	}
}

// getLogger returns a logger with the given subcomponent.
func getLogger(subcomponent string) *logger.Logger {
	return log.With("subcomponent", subcomponent)
}
