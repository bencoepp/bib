package storage

import (
	"time"
)

// Config holds the storage configuration.
type Config struct {
	// Backend is the storage backend type: "sqlite" or "postgres"
	Backend BackendType `mapstructure:"backend"`

	// SQLite configuration (used when Backend is "sqlite")
	SQLite SQLiteConfig `mapstructure:"sqlite"`

	// Postgres configuration (used when Backend is "postgres")
	Postgres PostgresConfig `mapstructure:"postgres"`

	// Audit configuration
	Audit AuditConfig `mapstructure:"audit"`

	// BreakGlass emergency access configuration
	BreakGlass BreakGlassConfig `mapstructure:"break_glass"`
}

// SQLiteConfig holds SQLite-specific configuration.
type SQLiteConfig struct {
	// Path is the path to the SQLite database file.
	// Defaults to <data_dir>/cache.db
	Path string `mapstructure:"path"`

	// MaxOpenConns is the maximum number of open connections.
	MaxOpenConns int `mapstructure:"max_open_conns"`

	// CacheTTL is the TTL for cached data.
	CacheTTL time.Duration `mapstructure:"cache_ttl"`

	// MaxCacheSizeMB is the maximum cache size in megabytes.
	MaxCacheSizeMB int `mapstructure:"max_cache_size_mb"`

	// VacuumInterval is how often to run VACUUM.
	VacuumInterval time.Duration `mapstructure:"vacuum_interval"`
}

// PostgresConfig holds PostgreSQL-specific configuration.
// Note: bibd manages the PostgreSQL instance; no external connection strings.
type PostgresConfig struct {
	// Managed indicates whether bibd manages the PostgreSQL lifecycle.
	// When true, bibd provisions and manages a PostgreSQL container.
	Managed bool `mapstructure:"managed"`

	// ContainerRuntime is the container runtime to use: "docker", "podman", or "kubernetes"
	// If empty, auto-detected (docker > podman > kubernetes)
	ContainerRuntime string `mapstructure:"container_runtime"`

	// SocketPath is the path to the container runtime socket
	// Auto-detected if empty
	SocketPath string `mapstructure:"socket_path"`

	// KubeconfigPath is the path to kubeconfig file (for Kubernetes runtime)
	KubeconfigPath string `mapstructure:"kubeconfig_path"`

	// Image is the PostgreSQL container image.
	// Defaults to "postgres:16-alpine"
	Image string `mapstructure:"image"`

	// DataDir is where PostgreSQL data is stored.
	// Defaults to <data_dir>/postgres
	DataDir string `mapstructure:"data_dir"`

	// Port is the PostgreSQL port (internal to the container network).
	// Defaults to 5432 (not exposed externally)
	Port int `mapstructure:"port"`

	// MaxConnections is the maximum number of connections.
	MaxConnections int `mapstructure:"max_connections"`

	// SharedBuffers is the PostgreSQL shared_buffers setting.
	SharedBuffers string `mapstructure:"shared_buffers"`

	// EffectiveCacheSize is the PostgreSQL effective_cache_size setting.
	EffectiveCacheSize string `mapstructure:"effective_cache_size"`

	// CredentialRotationInterval is how often to rotate database credentials.
	CredentialRotationInterval time.Duration `mapstructure:"credential_rotation_interval"`

	// SSLMode is the SSL mode for connections.
	// Options: "disable", "require", "verify-ca", "verify-full"
	SSLMode string `mapstructure:"ssl_mode"`

	// ResourceLimits for the container.
	Resources ContainerResources `mapstructure:"resources"`

	// Network configuration
	Network NetworkConfig `mapstructure:"network"`

	// Health check configuration
	Health HealthConfig `mapstructure:"health"`

	// TLS configuration for PostgreSQL connections
	TLS TLSConfig `mapstructure:"tls"`

	// Kubernetes configuration (when ContainerRuntime is "kubernetes")
	Kubernetes KubernetesConfig `mapstructure:"kubernetes"`

	// Advanced allows manual connection configuration (for debugging only).
	// When set, Managed must be false.
	Advanced *AdvancedPostgresConfig `mapstructure:"advanced,omitempty"`
}

// NetworkConfig holds network configuration for managed PostgreSQL.
type NetworkConfig struct {
	// UseBridgeNetwork creates a private bridge network for isolation
	UseBridgeNetwork bool `mapstructure:"use_bridge_network"`

	// BridgeNetworkName is the name of the bridge network
	BridgeNetworkName string `mapstructure:"bridge_network_name"`

	// UseUnixSocket uses Unix socket only (no TCP)
	UseUnixSocket bool `mapstructure:"use_unix_socket"`

	// BindAddress is the address to bind to (default: 127.0.0.1)
	BindAddress string `mapstructure:"bind_address"`
}

// HealthConfig holds health check configuration for managed PostgreSQL.
type HealthConfig struct {
	// Interval is how often to check health
	Interval time.Duration `mapstructure:"interval"`

	// Timeout is the timeout for each health check
	Timeout time.Duration `mapstructure:"timeout"`

	// StartupTimeout is how long to wait for initial startup
	StartupTimeout time.Duration `mapstructure:"startup_timeout"`

	// Action defines what happens on repeated failures: "shutdown", "retry_always", "retry_limit"
	Action string `mapstructure:"action"`

	// MaxRetries is the maximum retries (for "retry_limit" action)
	MaxRetries int `mapstructure:"max_retries"`

	// RetryBackoff is the backoff duration between retries
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`
}

// TLSConfig holds TLS configuration for PostgreSQL connections.
type TLSConfig struct {
	// Enabled controls whether mTLS is enabled (always true for managed)
	Enabled bool `mapstructure:"enabled"`

	// CertDir is where certificates are stored
	CertDir string `mapstructure:"cert_dir"`

	// AutoGenerate automatically generates certificates from node identity
	AutoGenerate bool `mapstructure:"auto_generate"`
}

// ContainerResources defines resource limits for the PostgreSQL container.
type ContainerResources struct {
	// MemoryMB is the memory limit in megabytes.
	MemoryMB int `mapstructure:"memory_mb"`

	// CPUCores is the CPU limit (can be fractional).
	CPUCores float64 `mapstructure:"cpu_cores"`
}

// KubernetesConfig holds Kubernetes-specific configuration for PostgreSQL deployment.
type KubernetesConfig struct {
	// Namespace is the Kubernetes namespace to deploy PostgreSQL into.
	// Defaults to the same namespace as bibd (if in-cluster) or "default".
	Namespace string `mapstructure:"namespace"`

	// UseCNPG enables CloudNativePG operator for PostgreSQL management.
	// If true, creates a CNPG Cluster resource instead of StatefulSet.
	// Requires CNPG operator to be installed in the cluster.
	UseCNPG bool `mapstructure:"use_cnpg"`

	// CNPGClusterVersion is the CNPG cluster version to use.
	CNPGClusterVersion string `mapstructure:"cnpg_cluster_version"`

	// StorageClassName is the StorageClass for PersistentVolumeClaims.
	// Empty string uses the cluster's default StorageClass.
	StorageClassName string `mapstructure:"storage_class_name"`

	// StorageSize is the size of the PostgreSQL data volume.
	// Examples: "10Gi", "50Gi", "100Gi"
	StorageSize string `mapstructure:"storage_size"`

	// BackupEnabled enables automatic backup CronJob creation.
	BackupEnabled bool `mapstructure:"backup_enabled"`

	// BackupSchedule is the cron schedule for backups.
	// Example: "0 2 * * *" for 2 AM daily
	BackupSchedule string `mapstructure:"backup_schedule"`

	// BackupRetention is how many backups to keep.
	BackupRetention int `mapstructure:"backup_retention"`

	// BackupStorageSize is the size of the backup PVC.
	BackupStorageSize string `mapstructure:"backup_storage_size"`

	// BackupToS3 enables backup to S3-compatible storage instead of PVC.
	BackupToS3 bool `mapstructure:"backup_to_s3"`

	// BackupS3Config holds S3 backup configuration.
	BackupS3 S3BackupConfig `mapstructure:"backup_s3"`

	// NetworkPolicyEnabled creates a NetworkPolicy restricting access.
	NetworkPolicyEnabled bool `mapstructure:"network_policy_enabled"`

	// NetworkPolicyAllowedLabels are pod labels allowed to connect.
	// Default: app=bibd
	NetworkPolicyAllowedLabels map[string]string `mapstructure:"network_policy_allowed_labels"`

	// ServiceType is the Kubernetes Service type.
	// Options: "ClusterIP" (in-cluster), "NodePort" (external access)
	// Default: "ClusterIP" if in-cluster, "NodePort" if out-of-cluster
	ServiceType string `mapstructure:"service_type"`

	// NodePort is the specific NodePort to use (if ServiceType is NodePort).
	// Leave 0 for automatic assignment.
	NodePort int `mapstructure:"node_port"`

	// PodAntiAffinity enables anti-affinity with bibd pods.
	PodAntiAffinity bool `mapstructure:"pod_anti_affinity"`

	// PodAntiAffinityLabels are the labels to use for anti-affinity rules.
	// Default: app=bibd
	PodAntiAffinityLabels map[string]string `mapstructure:"pod_anti_affinity_labels"`

	// SecurityContext holds pod security context configuration.
	SecurityContext PodSecurityContext `mapstructure:"security_context"`

	// ServiceAccountName is the ServiceAccount for the PostgreSQL pod.
	// If empty, bibd creates a dedicated ServiceAccount.
	ServiceAccountName string `mapstructure:"service_account_name"`

	// CreateRBAC creates necessary RBAC resources (ServiceAccount, Role, RoleBinding).
	CreateRBAC bool `mapstructure:"create_rbac"`

	// ImagePullSecrets are secrets for pulling private images.
	ImagePullSecrets []string `mapstructure:"image_pull_secrets"`

	// Tolerations for pod scheduling.
	Tolerations []Toleration `mapstructure:"tolerations"`

	// NodeSelector for pod scheduling.
	NodeSelector map[string]string `mapstructure:"node_selector"`

	// PriorityClassName for pod priority.
	PriorityClassName string `mapstructure:"priority_class_name"`

	// Resources are the resource requests and limits for PostgreSQL pod.
	Resources KubernetesResources `mapstructure:"resources"`

	// LivenessProbe configuration.
	LivenessProbe ProbeConfig `mapstructure:"liveness_probe"`

	// ReadinessProbe configuration.
	ReadinessProbe ProbeConfig `mapstructure:"readiness_probe"`

	// StartupProbe configuration.
	StartupProbe ProbeConfig `mapstructure:"startup_probe"`

	// UpdateStrategy for StatefulSet updates.
	// Options: "RollingUpdate", "OnDelete"
	UpdateStrategy string `mapstructure:"update_strategy"`

	// DeleteOnCleanup determines if resources are deleted on `bibd cleanup`.
	// If false, StatefulSet is scaled to 0 but not deleted.
	DeleteOnCleanup bool `mapstructure:"delete_on_cleanup"`

	// Labels are additional labels to apply to all Kubernetes resources.
	Labels map[string]string `mapstructure:"labels"`

	// Annotations are additional annotations to apply to all Kubernetes resources.
	Annotations map[string]string `mapstructure:"annotations"`
}

// S3BackupConfig holds S3 backup configuration.
type S3BackupConfig struct {
	// Endpoint is the S3 endpoint (e.g., s3.amazonaws.com or minio.example.com)
	Endpoint string `mapstructure:"endpoint"`

	// Region is the S3 region.
	Region string `mapstructure:"region"`

	// Bucket is the S3 bucket name.
	Bucket string `mapstructure:"bucket"`

	// Prefix is the key prefix for backups.
	Prefix string `mapstructure:"prefix"`

	// AccessKeyID is the AWS access key ID (stored in Secret).
	AccessKeyID string `mapstructure:"access_key_id"`

	// SecretAccessKey is the AWS secret access key (stored in Secret).
	SecretAccessKey string `mapstructure:"secret_access_key"`

	// UseIRSA enables IAM Roles for Service Accounts (AWS EKS).
	UseIRSA bool `mapstructure:"use_irsa"`

	// IAMRole is the IAM role ARN for IRSA.
	IAMRole string `mapstructure:"iam_role"`
}

// PodSecurityContext holds pod security context configuration.
type PodSecurityContext struct {
	// RunAsNonRoot forces the container to run as a non-root user.
	RunAsNonRoot bool `mapstructure:"run_as_non_root"`

	// RunAsUser is the UID to run the container as.
	RunAsUser int64 `mapstructure:"run_as_user"`

	// RunAsGroup is the GID to run the container as.
	RunAsGroup int64 `mapstructure:"run_as_group"`

	// FSGroup is the group ID for volume ownership.
	FSGroup int64 `mapstructure:"fs_group"`

	// FSGroupChangePolicy controls how volume ownership is changed.
	// Options: "OnRootMismatch", "Always"
	FSGroupChangePolicy string `mapstructure:"fs_group_change_policy"`

	// SeccompProfile is the seccomp profile to use.
	SeccompProfile string `mapstructure:"seccomp_profile"`

	// SELinuxOptions holds SELinux options.
	SELinuxOptions SELinuxOptions `mapstructure:"selinux_options"`
}

// SELinuxOptions holds SELinux configuration.
type SELinuxOptions struct {
	// User is the SELinux user label.
	User string `mapstructure:"user"`

	// Role is the SELinux role label.
	Role string `mapstructure:"role"`

	// Type is the SELinux type label.
	Type string `mapstructure:"type"`

	// Level is the SELinux level label.
	Level string `mapstructure:"level"`
}

// Toleration represents a Kubernetes toleration.
type Toleration struct {
	// Key is the taint key.
	Key string `mapstructure:"key"`

	// Operator is the operator (Exists or Equal).
	Operator string `mapstructure:"operator"`

	// Value is the taint value.
	Value string `mapstructure:"value"`

	// Effect is the taint effect (NoSchedule, PreferNoSchedule, NoExecute).
	Effect string `mapstructure:"effect"`

	// TolerationSeconds is the period before eviction.
	TolerationSeconds *int64 `mapstructure:"toleration_seconds,omitempty"`
}

// KubernetesResources holds Kubernetes resource requests and limits.
type KubernetesResources struct {
	// Requests are the resource requests.
	Requests ResourceQuantity `mapstructure:"requests"`

	// Limits are the resource limits.
	Limits ResourceQuantity `mapstructure:"limits"`
}

// ResourceQuantity holds resource quantity specifications.
type ResourceQuantity struct {
	// CPU in cores (e.g., "1", "500m").
	CPU string `mapstructure:"cpu"`

	// Memory in bytes (e.g., "1Gi", "512Mi").
	Memory string `mapstructure:"memory"`

	// EphemeralStorage in bytes (e.g., "10Gi").
	EphemeralStorage string `mapstructure:"ephemeral_storage"`
}

// ProbeConfig holds Kubernetes probe configuration.
type ProbeConfig struct {
	// Enabled controls whether the probe is configured.
	Enabled bool `mapstructure:"enabled"`

	// InitialDelaySeconds is the delay before the first probe.
	InitialDelaySeconds int32 `mapstructure:"initial_delay_seconds"`

	// PeriodSeconds is how often to perform the probe.
	PeriodSeconds int32 `mapstructure:"period_seconds"`

	// TimeoutSeconds is the probe timeout.
	TimeoutSeconds int32 `mapstructure:"timeout_seconds"`

	// SuccessThreshold is the number of successes required.
	SuccessThreshold int32 `mapstructure:"success_threshold"`

	// FailureThreshold is the number of failures before taking action.
	FailureThreshold int32 `mapstructure:"failure_threshold"`
}

// AdvancedPostgresConfig allows manual PostgreSQL configuration.
// This should only be used for debugging and testing.
type AdvancedPostgresConfig struct {
	// Host is the PostgreSQL host.
	Host string `mapstructure:"host"`

	// Port is the PostgreSQL port.
	Port int `mapstructure:"port"`

	// Database is the database name.
	Database string `mapstructure:"database"`

	// User is the database user.
	User string `mapstructure:"user"`

	// Password is the database password.
	// WARNING: This is stored in plaintext. Use only for testing.
	Password string `mapstructure:"password"`

	// SSLMode is the SSL mode.
	SSLMode string `mapstructure:"ssl_mode"`
}

// AuditConfig holds audit logging configuration.
type AuditConfig struct {
	// Enabled controls whether audit logging is active.
	Enabled bool `mapstructure:"enabled"`

	// RetentionDays is how long to keep audit logs.
	RetentionDays int `mapstructure:"retention_days"`

	// HashChain enables hash chain for tamper detection.
	HashChain bool `mapstructure:"hash_chain"`

	// StreamToExternal enables streaming to external SIEM.
	StreamToExternal bool `mapstructure:"stream_to_external"`

	// ExternalEndpoint is the endpoint for external streaming.
	ExternalEndpoint string `mapstructure:"external_endpoint,omitempty"`
}

// BreakGlassConfig holds emergency access configuration.
type BreakGlassConfig struct {
	// Enabled allows break glass access to be activated.
	// Must be explicitly enabled in config.
	Enabled bool `mapstructure:"enabled"`

	// RequireRestart requires bibd restart to enable break glass.
	RequireRestart bool `mapstructure:"require_restart"`

	// MaxDuration is the maximum duration for a break glass session.
	MaxDuration time.Duration `mapstructure:"max_duration"`

	// AllowedUsers are pre-configured emergency access users.
	AllowedUsers []BreakGlassUser `mapstructure:"allowed_users"`

	// AuditLevel is the audit level during break glass ("normal" or "paranoid").
	AuditLevel string `mapstructure:"audit_level"`

	// NotificationWebhook is called when break glass is activated.
	NotificationWebhook string `mapstructure:"notification_webhook,omitempty"`

	// NotificationEmail is emailed when break glass is activated.
	NotificationEmail string `mapstructure:"notification_email,omitempty"`
}

// BreakGlassUser represents an allowed emergency access user.
type BreakGlassUser struct {
	// Name is the user name.
	Name string `mapstructure:"name"`

	// PublicKey is the SSH public key for authentication.
	PublicKey string `mapstructure:"public_key"`
}

// DefaultConfig returns the default storage configuration.
func DefaultConfig() Config {
	return Config{
		Backend: BackendSQLite, // Default to SQLite for easy onboarding
		SQLite: SQLiteConfig{
			Path:           "", // Defaults to <data_dir>/cache.db
			MaxOpenConns:   10,
			CacheTTL:       5 * time.Minute,
			MaxCacheSizeMB: 500,
			VacuumInterval: 24 * time.Hour,
		},
		Postgres: PostgresConfig{
			Managed:                    true,
			ContainerRuntime:           "", // Auto-detect
			SocketPath:                 "", // Auto-detect
			Image:                      "postgres:16-alpine",
			DataDir:                    "", // Defaults to <data_dir>/postgres
			Port:                       5432,
			MaxConnections:             100,
			SharedBuffers:              "256MB",
			EffectiveCacheSize:         "1GB",
			CredentialRotationInterval: 7 * 24 * time.Hour, // 7 days
			SSLMode:                    "require",
			Resources: ContainerResources{
				MemoryMB: 512,
				CPUCores: 1.0,
			},
			Network: NetworkConfig{
				UseBridgeNetwork:  true,
				BridgeNetworkName: "bibd-network",
				UseUnixSocket:     true,
				BindAddress:       "127.0.0.1",
			},
			Health: HealthConfig{
				Interval:       5 * time.Second,
				Timeout:        5 * time.Second,
				StartupTimeout: 60 * time.Second,
				Action:         "retry_limit",
				MaxRetries:     5,
				RetryBackoff:   10 * time.Second,
			},
			TLS: TLSConfig{
				Enabled:      true,
				AutoGenerate: true,
			},
			Kubernetes: KubernetesConfig{
				Namespace:            "",    // Auto-detect
				UseCNPG:              false, // Use vanilla StatefulSet by default
				CNPGClusterVersion:   "16",
				StorageClassName:     "",     // Use cluster default
				StorageSize:          "10Gi", // 10GB default
				BackupEnabled:        true,
				BackupSchedule:       "0 2 * * *", // 2 AM daily
				BackupRetention:      7,           // Keep 7 backups
				BackupStorageSize:    "20Gi",      // 20GB for backups
				BackupToS3:           false,
				NetworkPolicyEnabled: true,
				NetworkPolicyAllowedLabels: map[string]string{
					"app": "bibd",
				},
				ServiceType:     "", // Auto-detect based on in-cluster vs out-of-cluster
				NodePort:        0,  // Auto-assign if NodePort
				PodAntiAffinity: true,
				PodAntiAffinityLabels: map[string]string{
					"app": "bibd",
				},
				SecurityContext: PodSecurityContext{
					RunAsNonRoot:        true,
					RunAsUser:           999, // postgres user
					RunAsGroup:          999, // postgres group
					FSGroup:             999,
					FSGroupChangePolicy: "OnRootMismatch",
					SeccompProfile:      "runtime/default",
				},
				CreateRBAC:      true,
				UpdateStrategy:  "RollingUpdate",
				DeleteOnCleanup: true,
				Resources: KubernetesResources{
					Requests: ResourceQuantity{
						CPU:    "500m",
						Memory: "512Mi",
					},
					Limits: ResourceQuantity{
						CPU:    "2",
						Memory: "2Gi",
					},
				},
				LivenessProbe: ProbeConfig{
					Enabled:             true,
					InitialDelaySeconds: 30,
					PeriodSeconds:       10,
					TimeoutSeconds:      5,
					SuccessThreshold:    1,
					FailureThreshold:    3,
				},
				ReadinessProbe: ProbeConfig{
					Enabled:             true,
					InitialDelaySeconds: 5,
					PeriodSeconds:       10,
					TimeoutSeconds:      5,
					SuccessThreshold:    1,
					FailureThreshold:    3,
				},
				StartupProbe: ProbeConfig{
					Enabled:             true,
					InitialDelaySeconds: 0,
					PeriodSeconds:       10,
					TimeoutSeconds:      5,
					SuccessThreshold:    1,
					FailureThreshold:    30, // Give 5 minutes for startup
				},
			},
		},
		Audit: AuditConfig{
			Enabled:          true,
			RetentionDays:    90,
			HashChain:        true,
			StreamToExternal: false,
		},
		BreakGlass: BreakGlassConfig{
			Enabled:        false,
			RequireRestart: true,
			MaxDuration:    1 * time.Hour,
			AuditLevel:     "paranoid",
		},
	}
}

// Validate validates the storage configuration.
func (c *Config) Validate() error {
	switch c.Backend {
	case BackendSQLite:
		// SQLite config validation
	case BackendPostgres:
		if c.Postgres.Advanced != nil && c.Postgres.Managed {
			return ErrInvalidInput
		}
	default:
		return ErrInvalidInput
	}
	return nil
}
