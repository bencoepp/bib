package blob

import (
	"fmt"

	"bib/internal/config"
	"bib/internal/storage"
)

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// convertDatabaseConfig converts config.DatabaseConfig to storage.Config
// This is a simplified conversion - in production, you'd want full field mapping
func convertDatabaseConfig(cfg *config.DatabaseConfig) storage.Config {
	storageConfig := storage.DefaultConfig()

	// Map backend type
	if cfg.Backend == "sqlite" {
		storageConfig.Backend = storage.BackendSQLite
	} else if cfg.Backend == "postgres" {
		storageConfig.Backend = storage.BackendPostgres
	}

	// Map SQLite config
	storageConfig.SQLite.Path = cfg.SQLite.Path
	storageConfig.SQLite.MaxOpenConns = cfg.SQLite.MaxOpenConns

	// Map Postgres config
	storageConfig.Postgres.Managed = cfg.Postgres.Managed
	storageConfig.Postgres.DataDir = cfg.Postgres.DataDir

	// Blob config is part of storage.Config by default

	return storageConfig
}
