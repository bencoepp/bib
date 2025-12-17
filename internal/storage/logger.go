// Package storage provides the data persistence layer.
//
// This file provides logger integration for storage operations.
package storage

import (
	"bib/internal/logger"
)

// log is the package-level logger for storage operations.
// It defaults to the default logger but can be set via SetLogger.
var log = logger.Default()

// SetLogger sets the logger for all storage operations.
// This should be called before creating any storage instances.
func SetLogger(l *logger.Logger) {
	if l != nil {
		log = l.With("component", "storage")
	}
}

// getLogger returns a logger with the given subcomponent.
func getLogger(subcomponent string) *logger.Logger {
	return log.With("subcomponent", subcomponent)
}
