package daemon

import (
	"bib/internal/config"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenDatabase(cfg *config.BibDaemonConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", formatConnectionString(cfg))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.Database.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConnections)
	db.SetConnMaxLifetime(time.Hour)
	return db, err
}

func formatConnectionString(cfg *config.BibDaemonConfig) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode)
}

func CloseDb(db *sql.DB) error {
	return db.Close()
}
