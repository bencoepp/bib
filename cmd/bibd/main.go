package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"

	"bib/internal/config"
	"bib/internal/logger"
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
		fmt.Println("bibd version 0.1.0")
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

	// Initialize structured logger
	log, err := logger.New(cfg.Log)
	if err != nil {
		stdlog.Fatalf("Failed to initialize logger: %v", err)
	}
	defer log.Close()

	// Initialize audit logger if configured
	var auditLog *logger.AuditLogger
	if cfg.Log.AuditPath != "" {
		auditLog, err = logger.NewAuditLogger(cfg.Log.AuditPath, cfg.Log.AuditMaxAgeDays)
		if err != nil {
			log.Warn("failed to initialize audit logger", "error", err)
		} else {
			defer auditLog.Close()
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
		"request_id", cc.RequestID,
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

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// TODO: Implement actual daemon logic here
	// For now, wait for shutdown signal

	sig := <-sigChan
	log.Info("received shutdown signal",
		"signal", sig.String(),
		"request_id", cc.RequestID,
	)

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
