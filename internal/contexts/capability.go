package contexts

import (
	"bib/internal/capcheck"
	"time"
)

// CapabilityContext aggregates consolidated, typed sections,
// plus legacy core fields and a raw map of all CheckResults.
type CapabilityContext struct {
	// Consolidated sections
	System      SystemSection      `json:"system"`
	Network     NetworkSection     `json:"network"`
	Storage     StorageSection     `json:"storage"`
	Compute     ComputeSection     `json:"compute"`
	Tooling     ToolingSection     `json:"tooling"`
	Kubernetes  KubernetesSection  `json:"kubernetes"`
	Cloud       CloudSection       `json:"cloud"`
	Secrets     SecretsSection     `json:"secrets"`
	Performance PerformanceSection `json:"performance"`

	// Legacy core typed fields (kept for backward compatibility)
	ContainerRuntime ContainerRuntimeCapability `json:"container_runtime"`
	Internet         InternetCapability         `json:"internet"`
	Resources        ResourcesCapability        `json:"resources"`

	// Generic raw results for every check
	GenericCapabilities map[string]capcheck.CheckResult `json:"generic_capabilities"`

	LastUpdated time.Time `json:"last_updated"`
}

// ----------------- Consolidated Section Types -----------------

// SystemSection groups OS/kernel, virtualization, compliance, security, locale, and time info.
type SystemSection struct {
	OS         string         `json:"os"`
	Arch       string         `json:"arch"`
	Kernel     KernelFeatures `json:"kernel"`
	Virt       Virtualization `json:"virtualization"`
	Compliance ComplianceInfo `json:"compliance"`
	Security   SecurityInfo   `json:"security"`
	Locale     LocaleInfo     `json:"locale"`
	Time       TimeInfo       `json:"time"`
}

type KernelFeatures struct {
	CgroupsPresent  bool   `json:"cgroups_present"`
	SELinuxPresent  bool   `json:"selinux_present"`
	AppArmorPresent bool   `json:"apparmor_present"`
	CGroupVersion   string `json:"cgroup_version,omitempty"`
}

type Virtualization struct {
	InContainer bool `json:"in_container"`
	DockerEnv   bool `json:"docker_env"`
}

type ComplianceInfo struct {
	FIPSEnabledRaw string `json:"fips_enabled_raw,omitempty"`
}

type SecurityInfo struct {
	SELinuxPresent  bool `json:"selinux_present"`
	AppArmorPresent bool `json:"apparmor_present"`
}

type LocaleInfo struct {
	Vars map[string]string `json:"vars,omitempty"` // LANG, LC_ALL, LC_CTYPE, TZ
}

type TimeInfo struct {
	WallTimeRFC3339 string `json:"wall_time_rfc3339,omitempty"`
	MonotonicTestNS int64  `json:"monotonic_test_ns,omitempty"`
}

// NetworkSection consolidates internet/DNS/reachability/proxy/TLS/SCM/VPN.
type NetworkSection struct {
	Internet      InternetCapability `json:"internet"`
	Reachability  ReachabilityInfo   `json:"reachability"`
	DNS           DNSInfo            `json:"dns"`
	Proxy         ProxyInfo          `json:"proxy"`
	TLSPKI        TLSPKIInfo         `json:"tls_pki"`
	SourceControl SCMConnectivity    `json:"source_control"`
	VPN           VPNInfo            `json:"vpn"`
}

type ReachabilityInfo struct {
	Attempts  []NetworkAttempt `json:"attempts,omitempty"`
	IPv6      bool             `json:"ipv6"`
	IPv6Error string           `json:"ipv6_error,omitempty"`
}

type NetworkAttempt struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Success  bool   `json:"success"`
	LatencyM int64  `json:"latency_ms"`
	Error    string `json:"error,omitempty"`
}

type DNSInfo struct {
	Queries []DNSQueryResult `json:"queries,omitempty"`
}

type DNSQueryResult struct {
	Host     string   `json:"host"`
	Success  bool     `json:"success"`
	IPs      []string `json:"ips,omitempty"`
	LatencyM int64    `json:"latency_ms"`
	Error    string   `json:"error,omitempty"`
}

type ProxyInfo struct {
	DetectedURL string            `json:"detected_url,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

type TLSPKIInfo struct {
	SystemRootsCount int    `json:"system_roots_count,omitempty"`
	SSLCertFile      string `json:"ssl_cert_file,omitempty"`
}

type SCMConnectivity struct {
	SSHOK        bool  `json:"ssh_ok"`
	HTTPSOK      bool  `json:"https_ok"`
	SSHLatencyMS int64 `json:"ssh_latency_ms,omitempty"`
	HTTPSLatency int64 `json:"https_latency_ms,omitempty"`
}

type VPNInfo struct {
	Present bool   `json:"present"`
	Method  string `json:"method,omitempty"`
}

// StorageSection consolidates disk space, performance, and temp/cache writability.
type StorageSection struct {
	Paths       []DiskPathStat      `json:"paths,omitempty"`
	Performance DiskPerformance     `json:"performance"`
	TempCache   []DirWritableStatus `json:"temp_cache,omitempty"`
}

type DiskPathStat struct {
	Path        string  `json:"path"`
	BytesFree   uint64  `json:"bytes_free"`
	BytesTotal  uint64  `json:"bytes_total"`
	PercentFree float64 `json:"percent_free"`
	Err         string  `json:"err,omitempty"`
}

type DiskPerformance struct {
	WriteNS int64 `json:"write_ns,omitempty"`
	ReadNS  int64 `json:"read_ns,omitempty"`
}

type DirWritableStatus struct {
	Path     string `json:"path"`
	Writable bool   `json:"writable"`
	Error    string `json:"error,omitempty"`
}

// ComputeSection consolidates resources, CPU, GPU, memory, limits.
type ComputeSection struct {
	Resources ResourcesCapability `json:"resources"`
	CPU       CPUInfo             `json:"cpu"`
	GPU       GPUInfo             `json:"gpu"`
	Memory    MemoryInfo          `json:"memory"`
	Limits    LimitsInfo          `json:"limits"`
}

type CPUInfo struct {
	Arch         string `json:"arch"`
	CoresLogical int    `json:"cores_logical"`
}

type GPUInfo struct {
	Present              bool     `json:"present"`
	NVIDIADevices        []string `json:"nvidia_devices,omitempty"`
	CUDAHome             string   `json:"cuda_home,omitempty"`
	NvidiaVisibleDevices string   `json:"nvidia_visible_devices,omitempty"`
}

type MemoryInfo struct {
	AllocBytes uint64 `json:"alloc_bytes,omitempty"`
	SysBytes   uint64 `json:"sys_bytes,omitempty"`
	HeapAlloc  uint64 `json:"heap_alloc,omitempty"`
	HeapSys    uint64 `json:"heap_sys,omitempty"`
	HeapIdle   uint64 `json:"heap_idle,omitempty"`
	HeapInuse  uint64 `json:"heap_inuse,omitempty"`
}

type LimitsInfo struct {
	Implemented bool   `json:"implemented"`
	Note        string `json:"note,omitempty"`
}

// ToolingSection consolidates CLI, language runtimes, container tools.
type ToolingSection struct {
	CLI              CLIToolchain      `json:"cli"`
	LanguageRuntimes map[string]string `json:"language_runtimes,omitempty"`
	ContainerTools   []string          `json:"container_tools,omitempty"`
}

type CLIToolchain struct {
	Found   []string `json:"found,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

// KubernetesSection consolidates kubeconfig and kubectl info.
type KubernetesSection struct {
	KubeconfigPaths       []string `json:"kubeconfig_paths,omitempty"`
	KubeconfigExists      bool     `json:"kubeconfig_exists"`
	InClusterTokenPath    string   `json:"in_cluster_token_path,omitempty"`
	InClusterTokenExists  bool     `json:"in_cluster_token_exists"`
	CurrentContext        string   `json:"current_context,omitempty"`
	CurrentContextCluster string   `json:"current_context_cluster,omitempty"`
	AvailableContexts     []string `json:"available_contexts,omitempty"`
	KubectlPath           string   `json:"kubectl_path,omitempty"`
	KubectlClientVersion  string   `json:"kubectl_client_version,omitempty"`
	Supported             bool     `json:"supported"`
}

// CloudSection consolidates cloud metadata detection.
type CloudSection struct {
	Detected []string `json:"detected,omitempty"`
}

// SecretsSection consolidates discovered secrets/keystores.
type SecretsSection struct {
	Found []string `json:"found,omitempty"`
}

// PerformanceSection exposes quick runtime health signals.
type PerformanceSection struct {
	Goroutines   int    `json:"goroutines"`
	LastGCUnix   uint64 `json:"last_gc_unix"`
	PauseTotalNS uint64 `json:"pause_total_ns"`
	NumGC        uint32 `json:"num_gc"`
}

// ----------------- Legacy Core Types (kept) -----------------

type ContainerRuntimeCapability struct {
	AvailableRuntimes []string             `json:"available_runtimes,omitempty"`
	ErrorMap          map[string]string    `json:"errors,omitempty"`
	Responsive        bool                 `json:"responsive"`
	LastResult        capcheck.CheckResult `json:"last_result"`
}

type InternetCapability struct {
	URL            string               `json:"url"`
	DNSSuccess     bool                 `json:"dns_success"`
	DNSLatencyMS   int64                `json:"dns_latency_ms,omitempty"`
	ResolvedIPs    []string             `json:"resolved_ips,omitempty"`
	HTTPSuccess    bool                 `json:"http_success"`
	HTTPStatusCode int                  `json:"http_status_code,omitempty"`
	HTTPLatencyMS  int64                `json:"http_latency_ms,omitempty"`
	TLSVersion     string               `json:"tls_version,omitempty"`
	Supported      bool                 `json:"supported"`
	LastResult     capcheck.CheckResult `json:"last_result"`
}

type ResourcesCapability struct {
	CPUCoresEffective       float64              `json:"cpu_cores_effective"`
	CPUCoresDetectionMethod string               `json:"cpu_detection_method,omitempty"`
	CPUQuota                float64              `json:"cpu_quota,omitempty"`
	CPUPeriod               float64              `json:"cpu_period,omitempty"`
	CPUSetsEffective        int                  `json:"cpusets_effective,omitempty"`
	CGroupVersion           string               `json:"cgroup_version,omitempty"`
	MemoryBytesEffective    uint64               `json:"memory_bytes_effective"`
	MemoryDetectionMethod   string               `json:"memory_detection_method,omitempty"`
	Supported               bool                 `json:"supported"`
	LastResult              capcheck.CheckResult `json:"last_result"`
}
