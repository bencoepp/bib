package podman

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

// PodConfig contains configuration for Podman pod/compose generation
type PodConfig struct {
	// PodName is the name of the pod
	PodName string

	// BibdImage is the bibd container image
	BibdImage string

	// BibdTag is the bibd container image tag
	BibdTag string

	// P2P configuration
	P2PEnabled bool
	P2PMode    string // proxy, selective, full

	// Storage configuration
	StorageBackend string // sqlite, postgres

	// PostgreSQL configuration (if StorageBackend == "postgres")
	PostgresImage    string
	PostgresTag      string
	PostgresDatabase string
	PostgresUser     string
	PostgresPassword string

	// Network configuration
	APIPort     int
	P2PPort     int
	MetricsPort int

	// Rootless mode configuration
	Rootless bool

	// Host port offset for rootless mode (ports < 1024 not allowed)
	PortOffset int

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

	// Deployment style: "pod" or "compose"
	DeployStyle string

	// Environment variables
	ExtraEnv map[string]string
}

// DefaultPodConfig returns a default pod configuration
func DefaultPodConfig() *PodConfig {
	return &PodConfig{
		PodName:            "bibd",
		BibdImage:          "ghcr.io/bib-project/bibd",
		BibdTag:            "latest",
		P2PEnabled:         true,
		P2PMode:            "proxy",
		StorageBackend:     "sqlite",
		PostgresImage:      "docker.io/library/postgres",
		PostgresTag:        "16-alpine",
		PostgresDatabase:   "bibd",
		PostgresUser:       "bibd",
		PostgresPassword:   "", // Will be auto-generated
		APIPort:            4000,
		P2PPort:            4001,
		MetricsPort:        9090,
		Rootless:           true,
		PortOffset:         0,
		TLSEnabled:         false,
		UsePublicBootstrap: true,
		OutputDir:          ".",
		DeployStyle:        "pod",
		ExtraEnv:           make(map[string]string),
	}
}

// PodGenerator generates Podman pod and compose files
type PodGenerator struct {
	Config *PodConfig
}

// NewPodGenerator creates a new pod generator
func NewPodGenerator(config *PodConfig) *PodGenerator {
	if config == nil {
		config = DefaultPodConfig()
	}
	return &PodGenerator{Config: config}
}

// GeneratePassword generates a random password
func GeneratePassword(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
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

		// Write file with appropriate permissions
		perm := os.FileMode(0644)
		if strings.HasSuffix(filename, ".sh") {
			perm = 0755
		}

		if err := os.WriteFile(path, []byte(content), perm); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}
	return nil
}

// Generate generates all Podman files
func (g *PodGenerator) Generate() (*GeneratedFiles, error) {
	files := &GeneratedFiles{
		Files: make(map[string]string),
	}

	// Generate postgres password if not set
	if g.Config.StorageBackend == "postgres" && g.Config.PostgresPassword == "" {
		g.Config.PostgresPassword = GeneratePassword(32)
	}

	// Adjust ports for rootless if needed
	if g.Config.Rootless && g.Config.PortOffset == 0 {
		// If API port is < 1024, add offset
		if g.Config.APIPort < 1024 {
			g.Config.PortOffset = 8000
		}
	}

	// Generate based on deploy style
	if g.Config.DeployStyle == "pod" {
		// Generate Kubernetes-style pod YAML
		podYaml, err := g.generatePodYaml()
		if err != nil {
			return nil, fmt.Errorf("failed to generate pod.yaml: %w", err)
		}
		files.Files["pod.yaml"] = podYaml
	} else {
		// Generate compose file
		compose, err := g.generateCompose()
		if err != nil {
			return nil, fmt.Errorf("failed to generate podman-compose.yaml: %w", err)
		}
		files.Files["podman-compose.yaml"] = compose
	}

	// Generate .env file
	envFile := g.generateEnvFile()
	files.Files[".env"] = envFile

	// Generate config.yaml
	configYaml, err := g.generateConfigYaml()
	if err != nil {
		return nil, fmt.Errorf("failed to generate config.yaml: %w", err)
	}
	files.Files["config/config.yaml"] = configYaml

	// Generate start.sh script
	startScript := g.generateStartScript()
	files.Files["start.sh"] = startScript

	// Generate stop.sh script
	stopScript := g.generateStopScript()
	files.Files["stop.sh"] = stopScript

	// Generate status.sh script
	statusScript := g.generateStatusScript()
	files.Files["status.sh"] = statusScript

	return files, nil
}

// generatePodYaml generates a Kubernetes-style pod YAML for podman kube play
func (g *PodGenerator) generatePodYaml() (string, error) {
	apiPort := g.Config.APIPort + g.Config.PortOffset
	p2pPort := g.Config.P2PPort + g.Config.PortOffset
	metricsPort := g.Config.MetricsPort + g.Config.PortOffset

	tmpl := `# Podman Pod specification for bibd
# Generated by bib setup
# Use: podman kube play pod.yaml

apiVersion: v1
kind: Pod
metadata:
  name: {{ .PodName }}
  labels:
    app: bibd
spec:
  containers:
    - name: bibd
      image: {{ .BibdImage }}:{{ .BibdTag }}
      ports:
        - containerPort: 4000
          hostPort: {{ .APIPortHost }}
          protocol: TCP
{{- if .P2PEnabled }}
        - containerPort: 4001
          hostPort: {{ .P2PPortHost }}
          protocol: TCP
{{- end }}
        - containerPort: 9090
          hostPort: {{ .MetricsPortHost }}
          protocol: TCP
      env:
        - name: BIBD_LOG_LEVEL
          value: "info"
        - name: BIBD_LOG_FORMAT
          value: "json"
{{- if eq .StorageBackend "postgres" }}
        - name: BIBD_DATABASE_URL
          value: "postgres://{{ .PostgresUser }}:{{ .PostgresPassword }}@127.0.0.1:5432/{{ .PostgresDatabase }}?sslmode=disable"
{{- end }}
      volumeMounts:
        - name: config
          mountPath: /etc/bibd
          readOnly: true
        - name: data
          mountPath: /var/lib/bibd
{{- if eq .StorageBackend "sqlite" }}
        - name: sqlite
          mountPath: /var/lib/bibd/db
{{- end }}
      resources:
        limits:
          memory: "512Mi"
          cpu: "500m"

{{- if eq .StorageBackend "postgres" }}
    - name: postgres
      image: {{ .PostgresImage }}:{{ .PostgresTag }}
      env:
        - name: POSTGRES_DB
          value: "{{ .PostgresDatabase }}"
        - name: POSTGRES_USER
          value: "{{ .PostgresUser }}"
        - name: POSTGRES_PASSWORD
          value: "{{ .PostgresPassword }}"
      volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
      resources:
        limits:
          memory: "256Mi"
          cpu: "250m"
{{- end }}

  volumes:
    - name: config
      hostPath:
        path: {{ .OutputDir }}/config
        type: Directory
    - name: data
      persistentVolumeClaim:
        claimName: bibd-data
{{- if eq .StorageBackend "sqlite" }}
    - name: sqlite
      persistentVolumeClaim:
        claimName: bibd-sqlite
{{- end }}
{{- if eq .StorageBackend "postgres" }}
    - name: postgres-data
      persistentVolumeClaim:
        claimName: postgres-data
{{- end }}

  restartPolicy: Always
`

	data := struct {
		*PodConfig
		APIPortHost     int
		P2PPortHost     int
		MetricsPortHost int
	}{
		PodConfig:       g.Config,
		APIPortHost:     apiPort,
		P2PPortHost:     p2pPort,
		MetricsPortHost: metricsPort,
	}

	t, err := template.New("pod").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateCompose generates a podman-compose.yaml file
func (g *PodGenerator) generateCompose() (string, error) {
	apiPort := g.Config.APIPort + g.Config.PortOffset
	p2pPort := g.Config.P2PPort + g.Config.PortOffset
	metricsPort := g.Config.MetricsPort + g.Config.PortOffset

	tmpl := `# Podman Compose configuration for bibd
# Generated by bib setup

services:
  bibd:
    image: {{ .BibdImage }}:{{ .BibdTag }}
    container_name: {{ .PodName }}-bibd
    restart: unless-stopped
    ports:
      - "{{ .APIPortHost }}:4000"
{{- if .P2PEnabled }}
      - "{{ .P2PPortHost }}:4001"
{{- end }}
      - "{{ .MetricsPortHost }}:9090"
    volumes:
      - ./config:/etc/bibd:ro,Z
      - bibd-data:/var/lib/bibd:Z
{{- if eq .StorageBackend "sqlite" }}
      - bibd-sqlite:/var/lib/bibd/db:Z
{{- end }}
    environment:
      - BIBD_LOG_LEVEL=info
      - BIBD_LOG_FORMAT=json
{{- if eq .StorageBackend "postgres" }}
      - BIBD_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable
{{- end }}
{{- range $key, $value := .ExtraEnv }}
      - {{ $key }}={{ $value }}
{{- end }}
{{- if eq .StorageBackend "postgres" }}
    depends_on:
      - postgres
{{- end }}
{{- if .Rootless }}
    userns_mode: keep-id
{{- end }}

{{- if eq .StorageBackend "postgres" }}

  postgres:
    image: {{ .PostgresImage }}:{{ .PostgresTag }}
    container_name: {{ .PodName }}-postgres
    restart: unless-stopped
    environment:
      - POSTGRES_DB=${POSTGRES_DB:-{{ .PostgresDatabase }}}
      - POSTGRES_USER=${POSTGRES_USER:-{{ .PostgresUser }}}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data:Z
{{- if .Rootless }}
    userns_mode: keep-id
{{- end }}
{{- end }}

volumes:
  bibd-data:
{{- if eq .StorageBackend "sqlite" }}
  bibd-sqlite:
{{- end }}
{{- if eq .StorageBackend "postgres" }}
  postgres-data:
{{- end }}
`

	data := struct {
		*PodConfig
		APIPortHost     int
		P2PPortHost     int
		MetricsPortHost int
	}{
		PodConfig:       g.Config,
		APIPortHost:     apiPort,
		P2PPortHost:     p2pPort,
		MetricsPortHost: metricsPort,
	}

	t, err := template.New("compose").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateEnvFile generates the .env file
func (g *PodGenerator) generateEnvFile() string {
	var sb strings.Builder

	sb.WriteString("# Environment variables for bibd Podman deployment\n")
	sb.WriteString("# Generated by bib setup\n\n")

	// Ports
	apiPort := g.Config.APIPort + g.Config.PortOffset
	p2pPort := g.Config.P2PPort + g.Config.PortOffset
	metricsPort := g.Config.MetricsPort + g.Config.PortOffset

	sb.WriteString("# bibd Configuration\n")
	sb.WriteString(fmt.Sprintf("BIBD_API_PORT=%d\n", apiPort))
	if g.Config.P2PEnabled {
		sb.WriteString(fmt.Sprintf("BIBD_P2P_PORT=%d\n", p2pPort))
	}
	sb.WriteString(fmt.Sprintf("BIBD_METRICS_PORT=%d\n", metricsPort))
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

	return sb.String()
}

// generateConfigYaml generates the bibd config.yaml
func (g *PodGenerator) generateConfigYaml() (string, error) {
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

// generateStartScript generates the start.sh convenience script
func (g *PodGenerator) generateStartScript() string {
	var sb strings.Builder

	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# Start script for bibd Podman deployment\n")
	sb.WriteString("# Generated by bib setup\n\n")

	sb.WriteString("set -e\n")
	sb.WriteString("cd \"$(dirname \"$0\")\"\n\n")

	sb.WriteString("echo \"🦭 Starting bibd...\"\n\n")

	if g.Config.DeployStyle == "pod" {
		sb.WriteString("# Using podman kube play\n")
		sb.WriteString("podman kube play pod.yaml\n")
	} else {
		sb.WriteString("# Using podman-compose\n")
		sb.WriteString("if command -v podman-compose &> /dev/null; then\n")
		sb.WriteString("    podman-compose up -d\n")
		sb.WriteString("elif podman compose version &> /dev/null; then\n")
		sb.WriteString("    podman compose up -d\n")
		sb.WriteString("else\n")
		sb.WriteString("    echo \"Error: Neither podman-compose nor podman compose found\"\n")
		sb.WriteString("    exit 1\n")
		sb.WriteString("fi\n")
	}

	sb.WriteString("\necho \"✓ bibd started\"\n")

	apiPort := g.Config.APIPort + g.Config.PortOffset
	sb.WriteString(fmt.Sprintf("echo \"  API: http://localhost:%d\"\n", apiPort))

	if g.Config.P2PEnabled {
		p2pPort := g.Config.P2PPort + g.Config.PortOffset
		sb.WriteString(fmt.Sprintf("echo \"  P2P: localhost:%d\"\n", p2pPort))
	}

	return sb.String()
}

// generateStopScript generates the stop.sh convenience script
func (g *PodGenerator) generateStopScript() string {
	var sb strings.Builder

	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# Stop script for bibd Podman deployment\n")
	sb.WriteString("# Generated by bib setup\n\n")

	sb.WriteString("set -e\n")
	sb.WriteString("cd \"$(dirname \"$0\")\"\n\n")

	sb.WriteString("echo \"🦭 Stopping bibd...\"\n\n")

	if g.Config.DeployStyle == "pod" {
		sb.WriteString("# Using podman kube down\n")
		sb.WriteString("podman kube down pod.yaml\n")
	} else {
		sb.WriteString("# Using podman-compose\n")
		sb.WriteString("if command -v podman-compose &> /dev/null; then\n")
		sb.WriteString("    podman-compose down\n")
		sb.WriteString("elif podman compose version &> /dev/null; then\n")
		sb.WriteString("    podman compose down\n")
		sb.WriteString("else\n")
		sb.WriteString("    echo \"Error: Neither podman-compose nor podman compose found\"\n")
		sb.WriteString("    exit 1\n")
		sb.WriteString("fi\n")
	}

	sb.WriteString("\necho \"✓ bibd stopped\"\n")

	return sb.String()
}

// generateStatusScript generates the status.sh convenience script
func (g *PodGenerator) generateStatusScript() string {
	var sb strings.Builder

	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# Status script for bibd Podman deployment\n")
	sb.WriteString("# Generated by bib setup\n\n")

	sb.WriteString("cd \"$(dirname \"$0\")\"\n\n")

	sb.WriteString("echo \"🦭 bibd Status\"\n")
	sb.WriteString("echo \"\"\n\n")

	if g.Config.DeployStyle == "pod" {
		sb.WriteString("# Using podman pod\n")
		sb.WriteString(fmt.Sprintf("podman pod ps --filter name=%s\n", g.Config.PodName))
		sb.WriteString("echo \"\"\n")
		sb.WriteString(fmt.Sprintf("podman ps --filter pod=%s\n", g.Config.PodName))
	} else {
		sb.WriteString("# Using podman-compose\n")
		sb.WriteString("if command -v podman-compose &> /dev/null; then\n")
		sb.WriteString("    podman-compose ps\n")
		sb.WriteString("elif podman compose version &> /dev/null; then\n")
		sb.WriteString("    podman compose ps\n")
		sb.WriteString("else\n")
		sb.WriteString("    podman ps --filter label=com.docker.compose.project\n")
		sb.WriteString("fi\n")
	}

	return sb.String()
}

// FormatStartInstructions returns instructions for starting the containers
func (g *PodGenerator) FormatStartInstructions(info *PodmanInfo) string {
	var sb strings.Builder

	sb.WriteString("📋 Podman Deployment\n\n")
	sb.WriteString(fmt.Sprintf("Files generated in: %s\n\n", g.Config.OutputDir))

	sb.WriteString("To start bibd:\n")
	sb.WriteString(fmt.Sprintf("  cd %s\n", g.Config.OutputDir))
	sb.WriteString("  ./start.sh\n\n")

	sb.WriteString("To stop bibd:\n")
	sb.WriteString("  ./stop.sh\n\n")

	sb.WriteString("To check status:\n")
	sb.WriteString("  ./status.sh\n\n")

	if g.Config.DeployStyle == "pod" {
		sb.WriteString("Or manually with podman kube:\n")
		sb.WriteString("  podman kube play pod.yaml\n")
		sb.WriteString("  podman kube down pod.yaml\n")
	} else {
		if info != nil && info.ComposeCommand != "" {
			composeCmd := strings.Join(info.GetComposeCommand(), " ")
			sb.WriteString(fmt.Sprintf("Or manually with %s:\n", composeCmd))
			sb.WriteString(fmt.Sprintf("  %s up -d\n", composeCmd))
			sb.WriteString(fmt.Sprintf("  %s down\n", composeCmd))
		}
	}

	return sb.String()
}
