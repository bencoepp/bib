package kubernetes

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ManifestConfig contains configuration for Kubernetes manifest generation
type ManifestConfig struct {
	// Namespace is the Kubernetes namespace
	Namespace string

	// BibdImage is the bibd container image
	BibdImage string

	// BibdTag is the bibd container image tag
	BibdTag string

	// Replicas is the number of bibd replicas
	Replicas int

	// P2P configuration
	P2PEnabled bool
	P2PMode    string // proxy, selective, full

	// Storage configuration
	StorageBackend string // sqlite, postgres

	// PostgreSQL configuration
	PostgresMode     string // statefulset, cloudnativepg, external
	PostgresImage    string
	PostgresTag      string
	PostgresDatabase string
	PostgresUser     string
	PostgresPassword string
	PostgresHost     string // for external postgres
	PostgresPort     int    // for external postgres

	// Storage class configuration
	StorageClass string
	PVCSize      string

	// Network configuration
	ServiceType   string // ClusterIP, LoadBalancer, NodePort
	NodePort      int    // for NodePort service
	APIPort       int
	P2PPort       int
	MetricsPort   int
	IngressHost   string // hostname for ingress
	IngressClass  string // ingress class name
	IngressTLS    bool   // enable TLS on ingress
	TLSSecretName string // secret name for TLS

	// TLS configuration for bibd
	TLSEnabled bool

	// Bootstrap configuration
	UsePublicBootstrap   bool
	CustomBootstrapPeers []string

	// Identity
	Name  string
	Email string

	// Output directory
	OutputDir string

	// Output mode: "apply", "generate", "helm"
	OutputMode string

	// Labels to apply to all resources
	Labels map[string]string

	// Annotations to apply to all resources
	Annotations map[string]string
}

// DefaultManifestConfig returns a default manifest configuration
func DefaultManifestConfig() *ManifestConfig {
	return &ManifestConfig{
		Namespace:          "bibd",
		BibdImage:          "ghcr.io/bib-project/bibd",
		BibdTag:            "latest",
		Replicas:           1,
		P2PEnabled:         true,
		P2PMode:            "proxy",
		StorageBackend:     "sqlite",
		PostgresMode:       "statefulset",
		PostgresImage:      "postgres",
		PostgresTag:        "16-alpine",
		PostgresDatabase:   "bibd",
		PostgresUser:       "bibd",
		PostgresPassword:   "", // Will be auto-generated
		PostgresPort:       5432,
		StorageClass:       "",
		PVCSize:            "10Gi",
		ServiceType:        "ClusterIP",
		APIPort:            4000,
		P2PPort:            4001,
		MetricsPort:        9090,
		IngressClass:       "",
		IngressTLS:         false,
		TLSEnabled:         false,
		UsePublicBootstrap: true,
		OutputDir:          ".",
		OutputMode:         "generate",
		Labels: map[string]string{
			"app.kubernetes.io/name":       "bibd",
			"app.kubernetes.io/managed-by": "bib-setup",
		},
		Annotations: make(map[string]string),
	}
}

// ManifestGenerator generates Kubernetes manifests
type ManifestGenerator struct {
	Config *ManifestConfig
}

// NewManifestGenerator creates a new manifest generator
func NewManifestGenerator(config *ManifestConfig) *ManifestGenerator {
	if config == nil {
		config = DefaultManifestConfig()
	}
	return &ManifestGenerator{Config: config}
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

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}
	return nil
}

// GeneratePassword generates a random password
func GeneratePassword(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// Generate generates all Kubernetes manifests
func (g *ManifestGenerator) Generate() (*GeneratedFiles, error) {
	files := &GeneratedFiles{
		Files: make(map[string]string),
	}

	// Generate postgres password if not set
	if g.Config.StorageBackend == "postgres" && g.Config.PostgresPassword == "" {
		g.Config.PostgresPassword = GeneratePassword(32)
	}

	// Generate namespace
	files.Files["namespace.yaml"] = g.generateNamespace()

	// Generate ConfigMap
	configYaml, err := g.generateConfigMap()
	if err != nil {
		return nil, fmt.Errorf("failed to generate configmap: %w", err)
	}
	files.Files["configmap.yaml"] = configYaml

	// Generate Secrets
	files.Files["secrets.yaml"] = g.generateSecrets()

	// Generate bibd Deployment/StatefulSet
	bibdManifest, err := g.generateBibdDeployment()
	if err != nil {
		return nil, fmt.Errorf("failed to generate bibd deployment: %w", err)
	}
	files.Files["bibd-deployment.yaml"] = bibdManifest

	// Generate bibd Service
	files.Files["bibd-service.yaml"] = g.generateBibdService()

	// Generate Ingress if configured
	if g.Config.IngressHost != "" {
		files.Files["bibd-ingress.yaml"] = g.generateIngress()
	}

	// Generate PostgreSQL manifests if using statefulset mode
	if g.Config.StorageBackend == "postgres" && g.Config.PostgresMode == "statefulset" {
		files.Files["postgres-statefulset.yaml"] = g.generatePostgresStatefulSet()
		files.Files["postgres-service.yaml"] = g.generatePostgresService()
	}

	// Generate CloudNativePG cluster if using that mode
	if g.Config.StorageBackend == "postgres" && g.Config.PostgresMode == "cloudnativepg" {
		cnpgManifest, err := g.generateCloudNativePGCluster()
		if err != nil {
			return nil, fmt.Errorf("failed to generate cloudnativepg cluster: %w", err)
		}
		files.Files["cloudnativepg-cluster.yaml"] = cnpgManifest
	}

	// Generate Kustomization file
	files.Files["kustomization.yaml"] = g.generateKustomization()

	// Generate apply script
	files.Files["apply.sh"] = g.generateApplyScript()

	// Generate delete script
	files.Files["delete.sh"] = g.generateDeleteScript()

	return files, nil
}

// generateNamespace generates the namespace manifest
func (g *ManifestGenerator) generateNamespace() string {
	return fmt.Sprintf(`# Namespace for bibd
# Generated by bib setup
apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    app.kubernetes.io/name: bibd
    app.kubernetes.io/managed-by: bib-setup
`, g.Config.Namespace)
}

// generateConfigMap generates the ConfigMap with bibd configuration
func (g *ManifestGenerator) generateConfigMap() (string, error) {
	tmpl := `# ConfigMap for bibd configuration
# Generated by bib setup
apiVersion: v1
kind: ConfigMap
metadata:
  name: bibd-config
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: bibd
    app.kubernetes.io/component: config
data:
  config.yaml: |
    log:
      level: info
      format: json
      output: stdout

    identity:
      name: "{{ .Name }}"
      email: "{{ .Email }}"
      key: /etc/bibd/identity/identity.pem

    server:
      host: 0.0.0.0
      port: 4000
      data_dir: /var/lib/bibd
{{- if .TLSEnabled }}
      tls:
        enabled: true
        cert_file: /etc/bibd/tls/tls.crt
        key_file: /etc/bibd/tls/tls.key
{{- end }}

    database:
{{- if eq .StorageBackend "sqlite" }}
      backend: sqlite
      path: /var/lib/bibd/db/bibd.db
{{- else }}
      backend: postgres
      # Connection from environment variable
{{- end }}

    p2p:
      enabled: {{ .P2PEnabled }}
      mode: {{ .P2PMode }}
      listen_addr: /ip4/0.0.0.0/tcp/4001
{{- if .UsePublicBootstrap }}
      bootstrap:
        - /dns4/bootstrap.bib.dev/tcp/4001/p2p/12D3KooWBib...
{{- end }}
`

	t, err := template.New("configmap").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, g.Config); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateSecrets generates the Secrets manifest
func (g *ManifestGenerator) generateSecrets() string {
	var sb strings.Builder

	sb.WriteString(`# Secrets for bibd
# Generated by bib setup
apiVersion: v1
kind: Secret
metadata:
  name: bibd-secrets
  namespace: ` + g.Config.Namespace + `
  labels:
    app.kubernetes.io/name: bibd
    app.kubernetes.io/component: secrets
type: Opaque
data:
`)

	// Add postgres password if using postgres
	if g.Config.StorageBackend == "postgres" {
		encodedPassword := base64.StdEncoding.EncodeToString([]byte(g.Config.PostgresPassword))
		sb.WriteString(fmt.Sprintf("  POSTGRES_PASSWORD: %s\n", encodedPassword))

		// Build connection string
		var host string
		if g.Config.PostgresMode == "external" {
			host = g.Config.PostgresHost
		} else {
			host = "postgres"
		}
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			g.Config.PostgresUser, g.Config.PostgresPassword, host, g.Config.PostgresPort, g.Config.PostgresDatabase)
		encodedConnStr := base64.StdEncoding.EncodeToString([]byte(connStr))
		sb.WriteString(fmt.Sprintf("  DATABASE_URL: %s\n", encodedConnStr))
	}

	return sb.String()
}

// generateBibdDeployment generates the bibd Deployment manifest
func (g *ManifestGenerator) generateBibdDeployment() (string, error) {
	tmpl := `# bibd Deployment
# Generated by bib setup
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bibd
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: bibd
    app.kubernetes.io/component: server
spec:
  replicas: {{ .Replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: bibd
      app.kubernetes.io/component: server
  template:
    metadata:
      labels:
        app.kubernetes.io/name: bibd
        app.kubernetes.io/component: server
    spec:
      serviceAccountName: bibd
      containers:
        - name: bibd
          image: {{ .BibdImage }}:{{ .BibdTag }}
          imagePullPolicy: Always
          ports:
            - name: api
              containerPort: 4000
              protocol: TCP
{{- if .P2PEnabled }}
            - name: p2p
              containerPort: 4001
              protocol: TCP
{{- end }}
            - name: metrics
              containerPort: 9090
              protocol: TCP
          env:
{{- if eq .StorageBackend "postgres" }}
            - name: BIBD_DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: bibd-secrets
                  key: DATABASE_URL
{{- end }}
          volumeMounts:
            - name: config
              mountPath: /etc/bibd
              readOnly: true
            - name: data
              mountPath: /var/lib/bibd
          resources:
            requests:
              memory: "256Mi"
              cpu: "100m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: api
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /ready
              port: api
            initialDelaySeconds: 5
            periodSeconds: 10
      volumes:
        - name: config
          configMap:
            name: bibd-config
        - name: data
{{- if .StorageClass }}
          persistentVolumeClaim:
            claimName: bibd-data
{{- else }}
          emptyDir: {}
{{- end }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: bibd
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: bibd
{{- if .StorageClass }}
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: bibd-data
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: bibd
spec:
  accessModes:
    - ReadWriteOnce
{{- if .StorageClass }}
  storageClassName: {{ .StorageClass }}
{{- end }}
  resources:
    requests:
      storage: {{ .PVCSize }}
{{- end }}
`

	t, err := template.New("deployment").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, g.Config); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateBibdService generates the bibd Service manifest
func (g *ManifestGenerator) generateBibdService() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`# bibd Service
# Generated by bib setup
apiVersion: v1
kind: Service
metadata:
  name: bibd
  namespace: %s
  labels:
    app.kubernetes.io/name: bibd
    app.kubernetes.io/component: server
spec:
  type: %s
  selector:
    app.kubernetes.io/name: bibd
    app.kubernetes.io/component: server
  ports:
    - name: api
      port: %d
      targetPort: api
      protocol: TCP
`, g.Config.Namespace, g.Config.ServiceType, g.Config.APIPort))

	if g.Config.ServiceType == "NodePort" && g.Config.NodePort > 0 {
		sb.WriteString(fmt.Sprintf("      nodePort: %d\n", g.Config.NodePort))
	}

	if g.Config.P2PEnabled {
		sb.WriteString(fmt.Sprintf(`    - name: p2p
      port: %d
      targetPort: p2p
      protocol: TCP
`, g.Config.P2PPort))
	}

	sb.WriteString(fmt.Sprintf(`    - name: metrics
      port: %d
      targetPort: metrics
      protocol: TCP
`, g.Config.MetricsPort))

	return sb.String()
}

// generateIngress generates the Ingress manifest
func (g *ManifestGenerator) generateIngress() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`# bibd Ingress
# Generated by bib setup
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: bibd
  namespace: %s
  labels:
    app.kubernetes.io/name: bibd
`, g.Config.Namespace))

	if len(g.Config.Annotations) > 0 {
		sb.WriteString("  annotations:\n")
		for k, v := range g.Config.Annotations {
			sb.WriteString(fmt.Sprintf("    %s: \"%s\"\n", k, v))
		}
	}

	sb.WriteString("spec:\n")

	if g.Config.IngressClass != "" {
		sb.WriteString(fmt.Sprintf("  ingressClassName: %s\n", g.Config.IngressClass))
	}

	if g.Config.IngressTLS && g.Config.TLSSecretName != "" {
		sb.WriteString(fmt.Sprintf(`  tls:
    - hosts:
        - %s
      secretName: %s
`, g.Config.IngressHost, g.Config.TLSSecretName))
	}

	sb.WriteString(fmt.Sprintf(`  rules:
    - host: %s
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: bibd
                port:
                  number: %d
`, g.Config.IngressHost, g.Config.APIPort))

	return sb.String()
}

// generatePostgresStatefulSet generates the PostgreSQL StatefulSet manifest
func (g *ManifestGenerator) generatePostgresStatefulSet() string {
	var storageClassLine string
	if g.Config.StorageClass != "" {
		storageClassLine = fmt.Sprintf("      storageClassName: %s", g.Config.StorageClass)
	}

	return fmt.Sprintf(`# PostgreSQL StatefulSet
# Generated by bib setup
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: %s
  labels:
    app.kubernetes.io/name: postgres
    app.kubernetes.io/component: database
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: postgres
      app.kubernetes.io/component: database
  template:
    metadata:
      labels:
        app.kubernetes.io/name: postgres
        app.kubernetes.io/component: database
    spec:
      containers:
        - name: postgres
          image: %s:%s
          imagePullPolicy: IfNotPresent
          ports:
            - name: postgres
              containerPort: 5432
              protocol: TCP
          env:
            - name: POSTGRES_DB
              value: "%s"
            - name: POSTGRES_USER
              value: "%s"
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: bibd-secrets
                  key: POSTGRES_PASSWORD
            - name: PGDATA
              value: /var/lib/postgresql/data/pgdata
          volumeMounts:
            - name: postgres-data
              mountPath: /var/lib/postgresql/data
          resources:
            requests:
              memory: "256Mi"
              cpu: "100m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            exec:
              command:
                - pg_isready
                - -U
                - %s
                - -d
                - %s
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            exec:
              command:
                - pg_isready
                - -U
                - %s
                - -d
                - %s
            initialDelaySeconds: 5
            periodSeconds: 5
  volumeClaimTemplates:
    - metadata:
        name: postgres-data
      spec:
        accessModes:
          - ReadWriteOnce
%s
        resources:
          requests:
            storage: %s
`, g.Config.Namespace, g.Config.PostgresImage, g.Config.PostgresTag,
		g.Config.PostgresDatabase, g.Config.PostgresUser,
		g.Config.PostgresUser, g.Config.PostgresDatabase,
		g.Config.PostgresUser, g.Config.PostgresDatabase,
		storageClassLine, g.Config.PVCSize)
}

// generatePostgresService generates the PostgreSQL Service manifest
func (g *ManifestGenerator) generatePostgresService() string {
	return fmt.Sprintf(`# PostgreSQL Service
# Generated by bib setup
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: %s
  labels:
    app.kubernetes.io/name: postgres
    app.kubernetes.io/component: database
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: postgres
    app.kubernetes.io/component: database
  ports:
    - name: postgres
      port: 5432
      targetPort: postgres
      protocol: TCP
`, g.Config.Namespace)
}

// generateCloudNativePGCluster generates the CloudNativePG Cluster CR
func (g *ManifestGenerator) generateCloudNativePGCluster() (string, error) {
	tmpl := `# CloudNativePG Cluster
# Generated by bib setup
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: bibd-postgres
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/name: postgres
    app.kubernetes.io/component: database
spec:
  instances: 1
  imageName: {{ .PostgresImage }}:{{ .PostgresTag }}
  
  postgresql:
    parameters:
      max_connections: "100"
      shared_buffers: "128MB"
  
  bootstrap:
    initdb:
      database: {{ .PostgresDatabase }}
      owner: {{ .PostgresUser }}
      secret:
        name: bibd-postgres-credentials
  
  storage:
    size: {{ .PVCSize }}
{{- if .StorageClass }}
    storageClass: {{ .StorageClass }}
{{- end }}
  
  monitoring:
    enablePodMonitor: false
---
apiVersion: v1
kind: Secret
metadata:
  name: bibd-postgres-credentials
  namespace: {{ .Namespace }}
type: kubernetes.io/basic-auth
data:
  username: {{ .PostgresUserB64 }}
  password: {{ .PostgresPasswordB64 }}
`

	data := struct {
		*ManifestConfig
		PostgresUserB64     string
		PostgresPasswordB64 string
	}{
		ManifestConfig:      g.Config,
		PostgresUserB64:     base64.StdEncoding.EncodeToString([]byte(g.Config.PostgresUser)),
		PostgresPasswordB64: base64.StdEncoding.EncodeToString([]byte(g.Config.PostgresPassword)),
	}

	t, err := template.New("cnpg").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateKustomization generates the kustomization.yaml file
func (g *ManifestGenerator) generateKustomization() string {
	var sb strings.Builder

	sb.WriteString(`# Kustomization file for bibd
# Generated by bib setup
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: ` + g.Config.Namespace + `

resources:
  - namespace.yaml
  - configmap.yaml
  - secrets.yaml
  - bibd-deployment.yaml
  - bibd-service.yaml
`)

	if g.Config.IngressHost != "" {
		sb.WriteString("  - bibd-ingress.yaml\n")
	}

	if g.Config.StorageBackend == "postgres" {
		if g.Config.PostgresMode == "statefulset" {
			sb.WriteString("  - postgres-statefulset.yaml\n")
			sb.WriteString("  - postgres-service.yaml\n")
		} else if g.Config.PostgresMode == "cloudnativepg" {
			sb.WriteString("  - cloudnativepg-cluster.yaml\n")
		}
	}

	sb.WriteString(`
commonLabels:
  app.kubernetes.io/managed-by: bib-setup
`)

	return sb.String()
}

// generateApplyScript generates the apply.sh convenience script
func (g *ManifestGenerator) generateApplyScript() string {
	return fmt.Sprintf(`#!/bin/bash
# Apply script for bibd Kubernetes deployment
# Generated by bib setup

set -e
cd "$(dirname "$0")"

echo "☸️  Applying bibd manifests to Kubernetes..."
echo "   Namespace: %s"
echo ""

kubectl apply -k .

echo ""
echo "✓ Manifests applied successfully!"
echo ""
echo "To check status:"
echo "  kubectl -n %s get pods"
echo "  kubectl -n %s get svc"
echo ""
echo "To view logs:"
echo "  kubectl -n %s logs -f deployment/bibd"
`, g.Config.Namespace, g.Config.Namespace, g.Config.Namespace, g.Config.Namespace)
}

// generateDeleteScript generates the delete.sh convenience script
func (g *ManifestGenerator) generateDeleteScript() string {
	return fmt.Sprintf(`#!/bin/bash
# Delete script for bibd Kubernetes deployment
# Generated by bib setup

set -e
cd "$(dirname "$0")"

echo "☸️  Deleting bibd from Kubernetes..."
echo "   Namespace: %s"
echo ""

read -p "Are you sure you want to delete bibd? [y/N] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]
then
    echo "Cancelled."
    exit 0
fi

kubectl delete -k .

echo ""
echo "✓ bibd deleted successfully!"
`, g.Config.Namespace)
}

// FormatDeployInstructions returns instructions for deployment
func (g *ManifestGenerator) FormatDeployInstructions(info *KubeInfo) string {
	var sb strings.Builder

	sb.WriteString("📋 Kubernetes Deployment\n\n")
	sb.WriteString(fmt.Sprintf("Files generated in: %s\n\n", g.Config.OutputDir))

	sb.WriteString("To deploy:\n")
	sb.WriteString(fmt.Sprintf("  cd %s\n", g.Config.OutputDir))
	sb.WriteString("  ./apply.sh\n")
	sb.WriteString("  # or: kubectl apply -k .\n\n")

	sb.WriteString("To check status:\n")
	sb.WriteString(fmt.Sprintf("  kubectl -n %s get pods\n", g.Config.Namespace))
	sb.WriteString(fmt.Sprintf("  kubectl -n %s get svc\n\n", g.Config.Namespace))

	sb.WriteString("To view logs:\n")
	sb.WriteString(fmt.Sprintf("  kubectl -n %s logs -f deployment/bibd\n\n", g.Config.Namespace))

	sb.WriteString("To delete:\n")
	sb.WriteString("  ./delete.sh\n")
	sb.WriteString("  # or: kubectl delete -k .\n")

	return sb.String()
}
