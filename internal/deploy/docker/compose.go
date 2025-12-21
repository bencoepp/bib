package docker

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ComposeConfig contains configuration for Docker Compose generation
type ComposeConfig struct {
	// ProjectName is the Docker Compose project name
	ProjectName string

	// BibdImage is the bibd Docker image
	BibdImage string

	// BibdTag is the bibd Docker image tag
	BibdTag string

	// P2P configuration
	P2PEnabled bool
	P2PMode    string // proxy, selective, full

	// Storage configuration
	StorageBackend string // sqlite, postgres

	// PostgreSQL configuration (if StorageBackend == "postgres")
	PostgresImage        string
	PostgresTag          string
	PostgresDatabase     string
	PostgresUser         string
	PostgresPassword     string
	PostgresSSLMode      string // disable, require, verify-ca, verify-full
	PostgresMaxConns     int    // Maximum connections
	PostgresSharedBufs   string // shared_buffers setting
	PostgresWorkMem      string // work_mem setting
	PostgresExposePort   bool   // Expose postgres port externally
	PostgresExternalPort int    // External port for postgres (default 5432)

	// Network configuration
	APIPort     int
	P2PPort     int
	MetricsPort int

	// TLS configuration
	TLSEnabled bool

	// Bootstrap configuration
	UsePublicBootstrap   bool
	CustomBootstrapPeers []string

	// Identity
	Name  string
	Email string

	// Output directory
	OutputDir string

	// Environment variables
	ExtraEnv map[string]string

	// Deployment options
	AutoStart  bool // Automatically start containers after generation
	PullImages bool // Pull latest images before starting
}

// DefaultComposeConfig returns a default compose configuration
func DefaultComposeConfig() *ComposeConfig {
	return &ComposeConfig{
		ProjectName:          "bibd",
		BibdImage:            "ghcr.io/bib-project/bibd",
		BibdTag:              "latest",
		P2PEnabled:           true,
		P2PMode:              "proxy",
		StorageBackend:       "sqlite",
		PostgresImage:        "postgres",
		PostgresTag:          "16-alpine",
		PostgresDatabase:     "bibd",
		PostgresUser:         "bibd",
		PostgresPassword:     "", // Will be auto-generated
		PostgresSSLMode:      "disable",
		PostgresMaxConns:     100,
		PostgresSharedBufs:   "128MB",
		PostgresWorkMem:      "4MB",
		PostgresExposePort:   false,
		PostgresExternalPort: 5432,
		APIPort:              4000,
		P2PPort:              4001,
		MetricsPort:          9090,
		TLSEnabled:           false,
		UsePublicBootstrap:   true,
		OutputDir:            ".",
		ExtraEnv:             make(map[string]string),
		AutoStart:            false,
		PullImages:           true,
	}
}

// ComposeGenerator generates Docker Compose files
type ComposeGenerator struct {
	Config *ComposeConfig
}

// NewComposeGenerator creates a new compose generator
func NewComposeGenerator(config *ComposeConfig) *ComposeGenerator {
	if config == nil {
		config = DefaultComposeConfig()
	}
	return &ComposeGenerator{Config: config}
}

// GeneratePassword generates a random password
func GeneratePassword(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// Generate generates all Docker Compose files
func (g *ComposeGenerator) Generate() (*GeneratedFiles, error) {
	files := &GeneratedFiles{
		Files: make(map[string]string),
	}

	// Generate postgres password if not set
	if g.Config.StorageBackend == "postgres" && g.Config.PostgresPassword == "" {
		g.Config.PostgresPassword = GeneratePassword(32)
	}

	// Generate docker-compose.yaml
	compose, err := g.generateCompose()
	if err != nil {
		return nil, fmt.Errorf("failed to generate docker-compose.yaml: %w", err)
	}
	files.Files["docker-compose.yaml"] = compose

	// Generate .env file
	envFile, err := g.generateEnvFile()
	if err != nil {
		return nil, fmt.Errorf("failed to generate .env: %w", err)
	}
	files.Files[".env"] = envFile

	// Generate config.yaml
	configYaml, err := g.generateConfigYaml()
	if err != nil {
		return nil, fmt.Errorf("failed to generate config.yaml: %w", err)
	}
	files.Files["config/config.yaml"] = configYaml

	// Generate PostgreSQL configuration files if using postgres
	if g.Config.StorageBackend == "postgres" {
		// Generate postgres.conf
		postgresConf := g.generatePostgresConf()
		files.Files["config/postgres.conf"] = postgresConf

		// Generate init.sql for database initialization
		initSQL := g.generateInitSQL()
		files.Files["config/init.sql"] = initSQL
	}

	return files, nil
}

// GeneratedFiles contains the generated files
type GeneratedFiles struct {
	Files map[string]string
}

// WriteToDir writes all generated files to a directory
func (f *GeneratedFiles) WriteToDir(dir string) error {
	for filename, content := range f.Files {
		path := filepath.Join(dir, filename)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", filename, err)
		}

		// Write file
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}
	return nil
}

// generateCompose generates the docker-compose.yaml content
func (g *ComposeGenerator) generateCompose() (string, error) {
	tmpl := `# Docker Compose configuration for bibd
# Generated by bib setup

services:
  bibd:
    image: {{ .BibdImage }}:{{ .BibdTag }}
    container_name: {{ .ProjectName }}-bibd
    restart: unless-stopped
    ports:
      - "${BIBD_API_PORT:-{{ .APIPort }}}:4000"
{{- if .P2PEnabled }}
      - "${BIBD_P2P_PORT:-{{ .P2PPort }}}:4001"
{{- end }}
      - "${BIBD_METRICS_PORT:-{{ .MetricsPort }}}:9090"
    volumes:
      - ./config:/etc/bibd:ro
      - bibd-data:/var/lib/bibd
{{- if eq .StorageBackend "sqlite" }}
      - bibd-sqlite:/var/lib/bibd/db
{{- end }}
    environment:
      - BIBD_LOG_LEVEL=${BIBD_LOG_LEVEL:-info}
      - BIBD_LOG_FORMAT=${BIBD_LOG_FORMAT:-json}
{{- if eq .StorageBackend "postgres" }}
      - BIBD_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable
{{- end }}
{{- range $key, $value := .ExtraEnv }}
      - {{ $key }}={{ $value }}
{{- end }}
{{- if eq .StorageBackend "postgres" }}
    depends_on:
      postgres:
        condition: service_healthy
{{- end }}
    healthcheck:
      test: ["CMD", "bibd", "health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
{{- if .P2PEnabled }}
    networks:
      - bibd-network
{{- end }}

{{- if eq .StorageBackend "postgres" }}

  postgres:
    image: {{ .PostgresImage }}:{{ .PostgresTag }}
    container_name: {{ .ProjectName }}-postgres
    restart: unless-stopped
{{- if .PostgresExposePort }}
    ports:
      - "${POSTGRES_PORT:-{{ .PostgresExternalPort }}}:5432"
{{- end }}
    environment:
      - POSTGRES_DB=${POSTGRES_DB:-{{ .PostgresDatabase }}}
      - POSTGRES_USER=${POSTGRES_USER:-{{ .PostgresUser }}}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_INITDB_ARGS=--encoding=UTF8 --locale=C
    volumes:
      - postgres-data:/var/lib/postgresql/data
{{- if or .PostgresSharedBufs .PostgresWorkMem }}
      - ./config/postgres.conf:/etc/postgresql/postgresql.conf:ro
    command: postgres -c config_file=/etc/postgresql/postgresql.conf
{{- end }}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-{{ .PostgresUser }}} -d ${POSTGRES_DB:-{{ .PostgresDatabase }}}"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 10s
    networks:
      - bibd-network
{{- end }}

volumes:
  bibd-data:
    driver: local
{{- if eq .StorageBackend "sqlite" }}
  bibd-sqlite:
    driver: local
{{- end }}
{{- if eq .StorageBackend "postgres" }}
  postgres-data:
    driver: local
{{- end }}

{{- if .P2PEnabled }}

networks:
  bibd-network:
    driver: bridge
{{- end }}
`

	t, err := template.New("compose").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, g.Config); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateEnvFile generates the .env file content
func (g *ComposeGenerator) generateEnvFile() (string, error) {
	var sb strings.Builder

	sb.WriteString("# Environment variables for bibd Docker deployment\n")
	sb.WriteString("# Generated by bib setup\n\n")

	// bibd settings
	sb.WriteString("# bibd Configuration\n")
	sb.WriteString(fmt.Sprintf("BIBD_API_PORT=%d\n", g.Config.APIPort))
	if g.Config.P2PEnabled {
		sb.WriteString(fmt.Sprintf("BIBD_P2P_PORT=%d\n", g.Config.P2PPort))
	}
	sb.WriteString(fmt.Sprintf("BIBD_METRICS_PORT=%d\n", g.Config.MetricsPort))
	sb.WriteString("BIBD_LOG_LEVEL=info\n")
	sb.WriteString("BIBD_LOG_FORMAT=json\n")
	sb.WriteString("\n")

	// PostgreSQL settings
	if g.Config.StorageBackend == "postgres" {
		sb.WriteString("# PostgreSQL Configuration\n")
		sb.WriteString(fmt.Sprintf("POSTGRES_DB=%s\n", g.Config.PostgresDatabase))
		sb.WriteString(fmt.Sprintf("POSTGRES_USER=%s\n", g.Config.PostgresUser))
		sb.WriteString(fmt.Sprintf("POSTGRES_PASSWORD=%s\n", g.Config.PostgresPassword))
		sb.WriteString("\n")
	}

	// Extra environment variables
	if len(g.Config.ExtraEnv) > 0 {
		sb.WriteString("# Additional Environment Variables\n")
		for key, value := range g.Config.ExtraEnv {
			sb.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
	}

	return sb.String(), nil
}

// generateConfigYaml generates the bibd config.yaml content
func (g *ComposeGenerator) generateConfigYaml() (string, error) {
	tmpl := `# bibd configuration
# Generated by bib setup

log:
  level: info
  format: json
  output: stdout

identity:
  name: "{{ .Name }}"
  email: "{{ .Email }}"
  key: /etc/bibd/identity.pem

server:
  host: 0.0.0.0
  port: 4000
  data_dir: /var/lib/bibd
{{- if .TLSEnabled }}
  tls:
    enabled: true
    cert_file: /etc/bibd/tls/cert.pem
    key_file: /etc/bibd/tls/key.pem
{{- end }}

database:
{{- if eq .StorageBackend "sqlite" }}
  backend: sqlite
  path: /var/lib/bibd/db/bibd.db
{{- else }}
  backend: postgres
  # Connection string from environment variable BIBD_DATABASE_URL
{{- end }}

p2p:
  enabled: {{ .P2PEnabled }}
  mode: {{ .P2PMode }}
  listen_addr: /ip4/0.0.0.0/tcp/4001
{{- if .UsePublicBootstrap }}
  bootstrap:
    - /dns4/bootstrap.bib.dev/tcp/4001/p2p/12D3KooWBib...
{{- end }}
{{- if .CustomBootstrapPeers }}
  bootstrap:
{{- range .CustomBootstrapPeers }}
    - {{ . }}
{{- end }}
{{- end }}
`

	t, err := template.New("config").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, g.Config); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetComposeUpCommand returns the command to start the containers
func (g *ComposeGenerator) GetComposeUpCommand(info *DockerInfo) []string {
	cmd := info.GetComposeCommand()
	return append(cmd, "up", "-d")
}

// GetComposeDownCommand returns the command to stop the containers
func (g *ComposeGenerator) GetComposeDownCommand(info *DockerInfo) []string {
	cmd := info.GetComposeCommand()
	return append(cmd, "down")
}

// GetComposeLogsCommand returns the command to view logs
func (g *ComposeGenerator) GetComposeLogsCommand(info *DockerInfo) []string {
	cmd := info.GetComposeCommand()
	return append(cmd, "logs", "-f", "bibd")
}

// FormatStartInstructions returns instructions for starting the containers
func (g *ComposeGenerator) FormatStartInstructions(info *DockerInfo) string {
	var sb strings.Builder

	composeCmd := strings.Join(info.GetComposeCommand(), " ")

	sb.WriteString("📋 Docker Compose Deployment\n\n")
	sb.WriteString(fmt.Sprintf("Files generated in: %s\n\n", g.Config.OutputDir))

	sb.WriteString("To start bibd:\n")
	sb.WriteString(fmt.Sprintf("  cd %s\n", g.Config.OutputDir))
	sb.WriteString(fmt.Sprintf("  %s up -d\n\n", composeCmd))

	sb.WriteString("To view logs:\n")
	sb.WriteString(fmt.Sprintf("  %s logs -f bibd\n\n", composeCmd))

	sb.WriteString("To stop bibd:\n")
	sb.WriteString(fmt.Sprintf("  %s down\n\n", composeCmd))

	sb.WriteString("To check status:\n")
	sb.WriteString(fmt.Sprintf("  %s ps\n", composeCmd))

	return sb.String()
}

// generatePostgresConf generates PostgreSQL configuration file
func (g *ComposeGenerator) generatePostgresConf() string {
	var sb strings.Builder

	sb.WriteString("# PostgreSQL configuration for bibd\n")
	sb.WriteString("# Generated by bib setup\n\n")

	sb.WriteString("# Connection settings\n")
	sb.WriteString("listen_addresses = '*'\n")
	sb.WriteString(fmt.Sprintf("max_connections = %d\n", g.Config.PostgresMaxConns))
	sb.WriteString("\n")

	sb.WriteString("# Memory settings\n")
	sb.WriteString(fmt.Sprintf("shared_buffers = %s\n", g.Config.PostgresSharedBufs))
	sb.WriteString(fmt.Sprintf("work_mem = %s\n", g.Config.PostgresWorkMem))
	sb.WriteString("maintenance_work_mem = 64MB\n")
	sb.WriteString("effective_cache_size = 256MB\n")
	sb.WriteString("\n")

	sb.WriteString("# WAL settings\n")
	sb.WriteString("wal_level = replica\n")
	sb.WriteString("max_wal_size = 1GB\n")
	sb.WriteString("min_wal_size = 80MB\n")
	sb.WriteString("\n")

	sb.WriteString("# Logging\n")
	sb.WriteString("log_destination = 'stderr'\n")
	sb.WriteString("logging_collector = off\n")
	sb.WriteString("log_min_messages = warning\n")
	sb.WriteString("log_min_error_statement = error\n")
	sb.WriteString("log_timezone = 'UTC'\n")
	sb.WriteString("\n")

	sb.WriteString("# Locale settings\n")
	sb.WriteString("datestyle = 'iso, mdy'\n")
	sb.WriteString("timezone = 'UTC'\n")
	sb.WriteString("lc_messages = 'C'\n")
	sb.WriteString("lc_monetary = 'C'\n")
	sb.WriteString("lc_numeric = 'C'\n")
	sb.WriteString("lc_time = 'C'\n")
	sb.WriteString("default_text_search_config = 'pg_catalog.english'\n")

	return sb.String()
}

// generateInitSQL generates PostgreSQL initialization SQL
func (g *ComposeGenerator) generateInitSQL() string {
	var sb strings.Builder

	sb.WriteString("-- PostgreSQL initialization for bibd\n")
	sb.WriteString("-- Generated by bib setup\n\n")

	sb.WriteString("-- Enable required extensions\n")
	sb.WriteString("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";\n")
	sb.WriteString("CREATE EXTENSION IF NOT EXISTS \"pgcrypto\";\n")
	sb.WriteString("\n")

	sb.WriteString("-- Create schemas\n")
	sb.WriteString("CREATE SCHEMA IF NOT EXISTS bib;\n")
	sb.WriteString("\n")

	sb.WriteString("-- Grant permissions\n")
	sb.WriteString(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s;\n",
		g.Config.PostgresDatabase, g.Config.PostgresUser))
	sb.WriteString(fmt.Sprintf("GRANT ALL PRIVILEGES ON SCHEMA bib TO %s;\n", g.Config.PostgresUser))
	sb.WriteString(fmt.Sprintf("GRANT ALL PRIVILEGES ON SCHEMA public TO %s;\n", g.Config.PostgresUser))
	sb.WriteString("\n")

	sb.WriteString("-- Set default search path\n")
	sb.WriteString(fmt.Sprintf("ALTER USER %s SET search_path TO bib, public;\n", g.Config.PostgresUser))

	return sb.String()
}
