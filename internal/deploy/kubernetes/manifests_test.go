package kubernetes

import (
	"strings"
	"testing"
)

func TestDefaultManifestConfig(t *testing.T) {
	config := DefaultManifestConfig()

	if config == nil {
		t.Fatal("config is nil")
	}

	if config.Namespace != "bibd" {
		t.Errorf("expected namespace 'bibd', got %q", config.Namespace)
	}

	if config.BibdImage != "ghcr.io/bib-project/bibd" {
		t.Errorf("expected bibd image 'ghcr.io/bib-project/bibd', got %q", config.BibdImage)
	}

	if config.Replicas != 1 {
		t.Errorf("expected 1 replica, got %d", config.Replicas)
	}

	if config.ServiceType != "ClusterIP" {
		t.Errorf("expected service type 'ClusterIP', got %q", config.ServiceType)
	}

	if config.StorageBackend != "sqlite" {
		t.Errorf("expected storage backend 'sqlite', got %q", config.StorageBackend)
	}
}

func TestNewManifestGenerator(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &ManifestConfig{Namespace: "test"}
		generator := NewManifestGenerator(config)

		if generator.Config.Namespace != "test" {
			t.Error("config not set correctly")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		generator := NewManifestGenerator(nil)

		if generator.Config == nil {
			t.Error("should use default config when nil")
		}
		if generator.Config.Namespace != "bibd" {
			t.Error("should have default namespace")
		}
	})
}

func TestManifestGenerator_generateNamespace(t *testing.T) {
	config := DefaultManifestConfig()
	config.Namespace = "my-namespace"

	generator := NewManifestGenerator(config)
	ns := generator.generateNamespace()

	checks := []string{
		"apiVersion: v1",
		"kind: Namespace",
		"name: my-namespace",
	}

	for _, check := range checks {
		if !strings.Contains(ns, check) {
			t.Errorf("namespace missing %q", check)
		}
	}
}

func TestManifestGenerator_generateConfigMap(t *testing.T) {
	config := DefaultManifestConfig()
	config.Namespace = "test-ns"
	config.Name = "Test Node"
	config.Email = "test@example.com"
	config.P2PEnabled = true
	config.P2PMode = "selective"

	generator := NewManifestGenerator(config)
	cm, err := generator.generateConfigMap()

	if err != nil {
		t.Fatalf("generateConfigMap failed: %v", err)
	}

	checks := []string{
		"apiVersion: v1",
		"kind: ConfigMap",
		"namespace: test-ns",
		"name: \"Test Node\"",
		"email: \"test@example.com\"",
		"enabled: true",
		"mode: selective",
	}

	for _, check := range checks {
		if !strings.Contains(cm, check) {
			t.Errorf("configmap missing %q", check)
		}
	}
}

func TestManifestGenerator_generateSecrets(t *testing.T) {
	config := DefaultManifestConfig()
	config.Namespace = "test-ns"
	config.StorageBackend = "postgres"
	config.PostgresPassword = "supersecret"
	config.PostgresUser = "myuser"
	config.PostgresDatabase = "mydb"

	generator := NewManifestGenerator(config)
	secrets := generator.generateSecrets()

	checks := []string{
		"apiVersion: v1",
		"kind: Secret",
		"namespace: test-ns",
		"POSTGRES_PASSWORD:",
		"DATABASE_URL:",
	}

	for _, check := range checks {
		if !strings.Contains(secrets, check) {
			t.Errorf("secrets missing %q", check)
		}
	}
}

func TestManifestGenerator_generateBibdDeployment(t *testing.T) {
	config := DefaultManifestConfig()
	config.Namespace = "test-ns"
	config.Replicas = 3
	config.P2PEnabled = true
	config.StorageBackend = "postgres"

	generator := NewManifestGenerator(config)
	deployment, err := generator.generateBibdDeployment()

	if err != nil {
		t.Fatalf("generateBibdDeployment failed: %v", err)
	}

	checks := []string{
		"apiVersion: apps/v1",
		"kind: Deployment",
		"namespace: test-ns",
		"replicas: 3",
		"containerPort: 4000",
		"containerPort: 4001",
		"containerPort: 9090",
		"livenessProbe",
		"readinessProbe",
		"BIBD_DATABASE_URL",
	}

	for _, check := range checks {
		if !strings.Contains(deployment, check) {
			t.Errorf("deployment missing %q", check)
		}
	}
}

func TestManifestGenerator_generateBibdService(t *testing.T) {
	t.Run("ClusterIP", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.ServiceType = "ClusterIP"
		config.P2PEnabled = true

		generator := NewManifestGenerator(config)
		svc := generator.generateBibdService()

		checks := []string{
			"apiVersion: v1",
			"kind: Service",
			"type: ClusterIP",
			"port: 4000",
			"port: 4001",
			"port: 9090",
		}

		for _, check := range checks {
			if !strings.Contains(svc, check) {
				t.Errorf("service missing %q", check)
			}
		}
	})

	t.Run("LoadBalancer", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.ServiceType = "LoadBalancer"

		generator := NewManifestGenerator(config)
		svc := generator.generateBibdService()

		if !strings.Contains(svc, "type: LoadBalancer") {
			t.Error("service should be LoadBalancer type")
		}
	})

	t.Run("NodePort", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.ServiceType = "NodePort"
		config.NodePort = 30000

		generator := NewManifestGenerator(config)
		svc := generator.generateBibdService()

		if !strings.Contains(svc, "type: NodePort") {
			t.Error("service should be NodePort type")
		}
		if !strings.Contains(svc, "nodePort: 30000") {
			t.Error("service should have nodePort")
		}
	})
}

func TestManifestGenerator_generateIngress(t *testing.T) {
	config := DefaultManifestConfig()
	config.Namespace = "test-ns"
	config.IngressHost = "bibd.example.com"
	config.IngressClass = "nginx"
	config.IngressTLS = true
	config.TLSSecretName = "bibd-tls"

	generator := NewManifestGenerator(config)
	ingress := generator.generateIngress()

	checks := []string{
		"apiVersion: networking.k8s.io/v1",
		"kind: Ingress",
		"namespace: test-ns",
		"ingressClassName: nginx",
		"host: bibd.example.com",
		"secretName: bibd-tls",
	}

	for _, check := range checks {
		if !strings.Contains(ingress, check) {
			t.Errorf("ingress missing %q", check)
		}
	}
}

func TestManifestGenerator_generatePostgresStatefulSet(t *testing.T) {
	config := DefaultManifestConfig()
	config.Namespace = "test-ns"
	config.StorageBackend = "postgres"
	config.PostgresMode = "statefulset"
	config.PostgresImage = "postgres"
	config.PostgresTag = "16-alpine"
	config.StorageClass = "fast"
	config.PVCSize = "20Gi"

	generator := NewManifestGenerator(config)
	ss := generator.generatePostgresStatefulSet()

	checks := []string{
		"apiVersion: apps/v1",
		"kind: StatefulSet",
		"namespace: test-ns",
		"postgres:16-alpine",
		"storageClassName: fast",
		"storage: 20Gi",
		"pg_isready",
	}

	for _, check := range checks {
		if !strings.Contains(ss, check) {
			t.Errorf("statefulset missing %q", check)
		}
	}
}

func TestManifestGenerator_generatePostgresService(t *testing.T) {
	config := DefaultManifestConfig()
	config.Namespace = "test-ns"

	generator := NewManifestGenerator(config)
	svc := generator.generatePostgresService()

	checks := []string{
		"apiVersion: v1",
		"kind: Service",
		"namespace: test-ns",
		"name: postgres",
		"port: 5432",
	}

	for _, check := range checks {
		if !strings.Contains(svc, check) {
			t.Errorf("postgres service missing %q", check)
		}
	}
}

func TestManifestGenerator_generateCloudNativePGCluster(t *testing.T) {
	config := DefaultManifestConfig()
	config.Namespace = "test-ns"
	config.StorageBackend = "postgres"
	config.PostgresMode = "cloudnativepg"
	config.PostgresDatabase = "mydb"
	config.PostgresUser = "myuser"
	config.PostgresPassword = "mypassword"
	config.PVCSize = "50Gi"

	generator := NewManifestGenerator(config)
	cnpg, err := generator.generateCloudNativePGCluster()

	if err != nil {
		t.Fatalf("generateCloudNativePGCluster failed: %v", err)
	}

	checks := []string{
		"apiVersion: postgresql.cnpg.io/v1",
		"kind: Cluster",
		"namespace: test-ns",
		"instances: 1",
		"database: mydb",
		"owner: myuser",
		"size: 50Gi",
	}

	for _, check := range checks {
		if !strings.Contains(cnpg, check) {
			t.Errorf("cloudnativepg cluster missing %q", check)
		}
	}
}

func TestManifestGenerator_generateKustomization(t *testing.T) {
	t.Run("with postgres statefulset", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.StorageBackend = "postgres"
		config.PostgresMode = "statefulset"

		generator := NewManifestGenerator(config)
		kustomization := generator.generateKustomization()

		checks := []string{
			"apiVersion: kustomize.config.k8s.io/v1beta1",
			"kind: Kustomization",
			"namespace.yaml",
			"configmap.yaml",
			"secrets.yaml",
			"bibd-deployment.yaml",
			"bibd-service.yaml",
			"postgres-statefulset.yaml",
			"postgres-service.yaml",
		}

		for _, check := range checks {
			if !strings.Contains(kustomization, check) {
				t.Errorf("kustomization missing %q", check)
			}
		}
	})

	t.Run("with cloudnativepg", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.StorageBackend = "postgres"
		config.PostgresMode = "cloudnativepg"

		generator := NewManifestGenerator(config)
		kustomization := generator.generateKustomization()

		if !strings.Contains(kustomization, "cloudnativepg-cluster.yaml") {
			t.Error("kustomization should include cloudnativepg-cluster.yaml")
		}
	})

	t.Run("with ingress", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.IngressHost = "bibd.example.com"

		generator := NewManifestGenerator(config)
		kustomization := generator.generateKustomization()

		if !strings.Contains(kustomization, "bibd-ingress.yaml") {
			t.Error("kustomization should include bibd-ingress.yaml")
		}
	})
}

func TestManifestGenerator_Generate(t *testing.T) {
	t.Run("sqlite mode", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.Name = "Test"
		config.Email = "test@example.com"
		config.StorageBackend = "sqlite"

		generator := NewManifestGenerator(config)
		files, err := generator.Generate()

		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		expectedFiles := []string{
			"namespace.yaml",
			"configmap.yaml",
			"secrets.yaml",
			"bibd-deployment.yaml",
			"bibd-service.yaml",
			"kustomization.yaml",
			"apply.sh",
			"delete.sh",
		}

		for _, f := range expectedFiles {
			if _, ok := files.Files[f]; !ok {
				t.Errorf("missing file %q", f)
			}
		}
	})

	t.Run("postgres statefulset mode", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.Name = "Test"
		config.Email = "test@example.com"
		config.StorageBackend = "postgres"
		config.PostgresMode = "statefulset"

		generator := NewManifestGenerator(config)
		files, err := generator.Generate()

		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		expectedFiles := []string{
			"postgres-statefulset.yaml",
			"postgres-service.yaml",
		}

		for _, f := range expectedFiles {
			if _, ok := files.Files[f]; !ok {
				t.Errorf("missing file %q", f)
			}
		}
	})

	t.Run("postgres cloudnativepg mode", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.Name = "Test"
		config.Email = "test@example.com"
		config.StorageBackend = "postgres"
		config.PostgresMode = "cloudnativepg"

		generator := NewManifestGenerator(config)
		files, err := generator.Generate()

		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		if _, ok := files.Files["cloudnativepg-cluster.yaml"]; !ok {
			t.Error("missing cloudnativepg-cluster.yaml")
		}
	})

	t.Run("with ingress", func(t *testing.T) {
		config := DefaultManifestConfig()
		config.Name = "Test"
		config.Email = "test@example.com"
		config.IngressHost = "bibd.example.com"

		generator := NewManifestGenerator(config)
		files, err := generator.Generate()

		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		if _, ok := files.Files["bibd-ingress.yaml"]; !ok {
			t.Error("missing bibd-ingress.yaml")
		}
	})
}

func TestManifestGenerator_FormatDeployInstructions(t *testing.T) {
	config := DefaultManifestConfig()
	config.Namespace = "my-namespace"
	config.OutputDir = "/path/to/output"

	generator := NewManifestGenerator(config)
	instructions := generator.FormatDeployInstructions(nil)

	checks := []string{
		"Kubernetes Deployment",
		"/path/to/output",
		"./apply.sh",
		"kubectl -n my-namespace get pods",
		"kubectl -n my-namespace logs",
		"./delete.sh",
	}

	for _, check := range checks {
		if !strings.Contains(instructions, check) {
			t.Errorf("instructions missing %q", check)
		}
	}
}

func TestManifestConfig_Fields(t *testing.T) {
	config := ManifestConfig{
		Namespace:            "custom-ns",
		BibdImage:            "custom/bibd",
		BibdTag:              "v1.0.0",
		Replicas:             3,
		P2PEnabled:           true,
		P2PMode:              "full",
		StorageBackend:       "postgres",
		PostgresMode:         "cloudnativepg",
		PostgresImage:        "postgres",
		PostgresTag:          "15",
		PostgresDatabase:     "mydb",
		PostgresUser:         "myuser",
		PostgresPassword:     "mypass",
		PostgresHost:         "postgres.example.com",
		PostgresPort:         5432,
		StorageClass:         "fast",
		PVCSize:              "100Gi",
		ServiceType:          "LoadBalancer",
		NodePort:             30000,
		APIPort:              8080,
		P2PPort:              8081,
		MetricsPort:          9999,
		IngressHost:          "bibd.example.com",
		IngressClass:         "nginx",
		IngressTLS:           true,
		TLSSecretName:        "bibd-tls",
		TLSEnabled:           true,
		UsePublicBootstrap:   false,
		CustomBootstrapPeers: []string{"/ip4/1.2.3.4/tcp/4001/p2p/Qm..."},
		Name:                 "My Node",
		Email:                "node@example.com",
		OutputDir:            "/custom/output",
		OutputMode:           "apply",
		Labels: map[string]string{
			"env": "production",
		},
		Annotations: map[string]string{
			"cert-manager.io/cluster-issuer": "letsencrypt",
		},
	}

	if config.Namespace != "custom-ns" {
		t.Error("Namespace mismatch")
	}
	if config.Replicas != 3 {
		t.Error("Replicas mismatch")
	}
	if config.ServiceType != "LoadBalancer" {
		t.Error("ServiceType mismatch")
	}
	if config.PostgresMode != "cloudnativepg" {
		t.Error("PostgresMode mismatch")
	}
	if len(config.Labels) != 1 {
		t.Error("Labels mismatch")
	}
}

func TestGeneratePassword(t *testing.T) {
	tests := []struct {
		length int
	}{
		{16},
		{32},
		{64},
	}

	for _, tt := range tests {
		password := GeneratePassword(tt.length)
		if len(password) != tt.length {
			t.Errorf("expected password length %d, got %d", tt.length, len(password))
		}
	}

	// Passwords should be different
	p1 := GeneratePassword(32)
	p2 := GeneratePassword(32)
	if p1 == p2 {
		t.Error("generated passwords should be different")
	}
}
