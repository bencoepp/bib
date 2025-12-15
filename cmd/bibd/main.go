package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"bib/internal/config"
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
			log.Printf("Created default config at: %s", path)
			log.Printf("Run 'bib setup --daemon' to customize your configuration.")
		}
	}

	// Load configuration
	cfg, err := config.LoadBibd(cfgFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Start daemon
	log.Printf("Starting bibd on %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Log level: %s, format: %s", cfg.Log.Level, cfg.Log.Format)
	log.Printf("Data directory: %s", cfg.Server.DataDir)

	if cfg.Server.TLS.Enabled {
		log.Printf("TLS enabled with cert: %s", cfg.Server.TLS.CertFile)
	}

	// TODO: Implement actual daemon logic
	select {}
}
