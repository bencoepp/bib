package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bib/internal/config"
	"bib/internal/logger"
	"bib/internal/version"
)

var (
	cfgFile     string
	showVersion bool
)

func init() {
	flag.StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/bibd/config.yaml)")
	flag.BoolVar(&showVersion, "version", false, "show version")
}

func main() {
	flag.Parse()

	if showVersion {
		info := version.Get()
		fmt.Printf("bibd %s\n", info.String())
		fmt.Println(info.Full())
		os.Exit(0)
	}

	// Auto-generate config on first run
	if cfgFile == "" {
		path, created, err := config.GenerateConfigIfNotExists(config.AppBibd, "yaml")
		if err == nil && created {
			stdlog.Printf("Created default config at: %s", path)
			stdlog.Printf("Run 'bib setup --daemon' to customize your configuration.")
		}
	}

	// Load configuration
	cfg, err := config.LoadBibd(cfgFile)
	if err != nil {
		stdlog.Fatalf("Failed to load config: %v", err)
	}

	// Get config directory for component paths
	configDir, err := config.UserConfigDir(config.AppBibd)
	if err != nil {
		stdlog.Fatalf("Failed to get config directory: %v", err)
	}

	// Expand data directory
	dataDir := expandPath(cfg.Server.DataDir)
	cfg.Server.DataDir = dataDir

	// Debug: log what we're trying to create
	stdlog.Printf("Creating data directory: %q", dataDir)

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		stdlog.Fatalf("Failed to create data directory %q: %v", dataDir, err)
	}

	// Initialize structured logger
	log, err := logger.New(cfg.Log)
	if err != nil {
		stdlog.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() { _ = log.Close() }()

	// Initialize audit logger if configured
	var auditLog *logger.AuditLogger
	if cfg.Log.AuditPath != "" {
		auditLog, err = logger.NewAuditLogger(cfg.Log.AuditPath, cfg.Log.AuditMaxAgeDays)
		if err != nil {
			log.Warn("failed to initialize audit logger", "error", err)
		} else {
			defer func() { _ = auditLog.Close() }()
		}
	}

	// Create daemon context
	cc := logger.NewDaemonContext("bibd")
	ctx := logger.WithCommandContext(context.Background(), cc)
	ctx = logger.WithLogger(ctx, log)

	// Log startup
	log.Info("starting bibd",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"log_level", cfg.Log.Level,
		"log_format", cfg.Log.Format,
		"data_dir", cfg.Server.DataDir,
		"config_dir", configDir,
		"request_id", cc.RequestID,
	)

	// Debug log configuration details
	log.Debug("storage configuration",
		"backend", cfg.Database.Backend,
		"sqlite_path", cfg.Database.SQLite.Path,
		"postgres_managed", cfg.Database.Postgres.Managed,
	)

	log.Debug("P2P configuration",
		"enabled", cfg.P2P.Enabled,
		"mode", cfg.P2P.Mode,
		"listen_addresses", cfg.P2P.ListenAddresses,
		"mdns_enabled", cfg.P2P.MDNS.Enabled,
		"dht_enabled", cfg.P2P.DHT.Enabled,
	)

	log.Debug("cluster configuration",
		"enabled", cfg.Cluster.Enabled,
		"cluster_name", cfg.Cluster.ClusterName,
		"is_voter", cfg.Cluster.IsVoter,
	)

	if cfg.Server.TLS.Enabled {
		log.Info("TLS enabled",
			"cert_file", cfg.Server.TLS.CertFile,
		)
	}

	// Log startup audit event
	if auditLog != nil {
		auditLog.Log(ctx, logger.AuditEvent{
			Action:   logger.AuditActionCommand,
			Actor:    cc.User,
			Resource: "bibd",
			Outcome:  logger.AuditOutcomeSuccess,
			Metadata: map[string]any{
				"event":    "startup",
				"host":     cfg.Server.Host,
				"port":     cfg.Server.Port,
				"data_dir": cfg.Server.DataDir,
			},
		})
	}

	// Create and start daemon
	daemon := NewDaemon(cfg, configDir, log, auditLog)

	if err := daemon.Start(ctx); err != nil {
		log.Error("failed to start daemon", "error", err)
		if auditLog != nil {
			auditLog.Log(ctx, logger.AuditEvent{
				Action:   logger.AuditActionCommand,
				Actor:    cc.User,
				Resource: "bibd",
				Outcome:  logger.AuditOutcomeFailure,
				Metadata: map[string]any{
					"event": "startup_failed",
					"error": err.Error(),
				},
			})
		}
		os.Exit(1)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigChan
	log.Info("received shutdown signal",
		"signal", sig.String(),
		"request_id", cc.RequestID,
	)

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop daemon
	if err := daemon.Stop(shutdownCtx); err != nil {
		log.Error("error during shutdown", "error", err)
	}

	// Log shutdown audit event
	if auditLog != nil {
		auditLog.Log(ctx, logger.AuditEvent{
			Action:   logger.AuditActionCommand,
			Actor:    cc.User,
			Resource: "bibd",
			Outcome:  logger.AuditOutcomeSuccess,
			Metadata: map[string]any{
				"event":  "shutdown",
				"signal": sig.String(),
			},
		})
	}

	log.Info("bibd stopped", "request_id", cc.RequestID)
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) string {
	if len(path) == 0 {
		return path
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home + path[1:]
	}
	return path
}
