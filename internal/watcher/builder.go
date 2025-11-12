package watcher

import (
	"bib/internal/capcheck"
	"bib/internal/contexts"
	"encoding/json"
	"runtime"
)

// BuildCapabilityContext updates/creates a CapabilityContext from results.
// - Keeps GenericCapabilities raw
// - Populates consolidated sections
// - Maintains legacy core fields for compatibility
func BuildCapabilityContext(existing *contexts.CapabilityContext, results []capcheck.CheckResult) *contexts.CapabilityContext {
	if existing == nil {
		existing = &contexts.CapabilityContext{
			GenericCapabilities: make(map[string]capcheck.CheckResult),
		}
	}
	if existing.GenericCapabilities == nil {
		existing.GenericCapabilities = make(map[string]capcheck.CheckResult)
	}

	// Always refresh system OS/Arch
	existing.System.OS = runtime.GOOS
	existing.System.Arch = runtime.GOARCH

	for _, r := range results {
		// store raw result
		existing.GenericCapabilities[string(r.ID)] = r

		switch r.ID {
		// ---- core/legacy mappings (also feed consolidated sections) ----
		case "container_runtime":
			// legacy
			cr := existing.ContainerRuntime
			cr.Responsive = r.Supported
			cr.LastResult = r
			cr.AvailableRuntimes = anyStringSlice(r.Details["available"])
			if em, ok := asStringMap(r.Details["errors"]); ok {
				cr.ErrorMap = em
			}
			existing.ContainerRuntime = cr

			// consolidated: nothing more specific beyond legacy

		case "kubernetes_config":
			// consolidated Kubernetes basics
			k := existing.Kubernetes
			k.KubeconfigPaths = anyStringSlice(r.Details["kubeconfig_paths"])
			k.KubeconfigExists = asBool(r.Details["kubeconfig_exists"])
			k.InClusterTokenPath = asString(r.Details["in_cluster_token_path"])
			k.InClusterTokenExists = asBool(r.Details["in_cluster_token_exists"])
			k.CurrentContext = asString(r.Details["current_context"])
			k.CurrentContextCluster = asString(r.Details["current_context_cluster"])
			k.AvailableContexts = anyStringSlice(r.Details["available_contexts"])
			k.Supported = r.Supported
			existing.Kubernetes = k

		case "internet_access":
			// legacy
			net := existing.Internet
			net.Supported = r.Supported
			net.LastResult = r
			net.URL = asString(r.Details["url"])
			net.DNSSuccess = asBool(r.Details["dns_success"])
			net.HTTPSuccess = asBool(r.Details["http_success"])
			net.DNSLatencyMS = asInt64(r.Details["dns_latency_ms"])
			net.HTTPLatencyMS = asInt64(r.Details["http_latency_ms"])
			net.HTTPStatusCode = int(asInt64(r.Details["http_status_code"]))
			net.TLSVersion = asString(r.Details["tls_version"])
			net.ResolvedIPs = anyStringSlice(r.Details["resolved_ips"])
			existing.Internet = net

			// consolidated network
			existing.Network.Internet = net

		case "resources":
			// legacy
			rs := existing.Resources
			rs.Supported = r.Supported
			rs.LastResult = r
			rs.CPUCoresEffective = asFloat64(r.Details["cpu_cores_effective"])
			rs.CPUCoresDetectionMethod = asString(r.Details["cpu_detection_method"])
			rs.CPUQuota = asFloat64(r.Details["cpu_quota"])
			rs.CPUPeriod = asFloat64(r.Details["cpu_period"])
			rs.CPUSetsEffective = int(asInt64(r.Details["cpusets_effective"]))
			rs.CGroupVersion = asString(r.Details["cgroup_version"])
			rs.MemoryBytesEffective = uint64(asInt64(r.Details["memory_bytes_effective"]))
			rs.MemoryDetectionMethod = asString(r.Details["memory_detection_method"])
			existing.Resources = rs

			// consolidated compute
			existing.Compute.Resources = rs

		// ---- consolidated-only mappings for extended checks ----
		case "gpu":
			g := existing.Compute.GPU
			g.Present = r.Supported
			g.NVIDIADevices = anyStringSlice(r.Details["nvidia_devices"])
			if s := asString(r.Details["cuda_home"]); s != "" {
				g.CUDAHome = s
			}
			if s := asString(r.Details["nvidia_visible_devices"]); s != "" {
				g.NvidiaVisibleDevices = s
			}
			// if any indicator present, mark present even if r.Supported false
			if len(g.NVIDIADevices) > 0 || g.CUDAHome != "" || g.NvidiaVisibleDevices != "" {
				g.Present = true
			}
			existing.Compute.GPU = g

		case "disk_storage":
			var paths []contexts.DiskPathStat
			if ok := decodeInto(r.Details["paths"], &paths); ok {
				existing.Storage.Paths = paths
			}

		case "disk_performance":
			existing.Storage.Performance = contexts.DiskPerformance{
				WriteNS: asInt64(r.Details["write_ns"]),
				ReadNS:  asInt64(r.Details["read_ns"]),
			}

		case "network_reachability":
			var atts []contexts.NetworkAttempt
			_ = decodeInto(r.Details["attempts"], &atts)
			existing.Network.Reachability.Attempts = atts
			existing.Network.Reachability.IPv6 = asBool(r.Details["ipv6"])
			existing.Network.Reachability.IPv6Error = asString(r.Details["ipv6_error"])

		case "dns_resolver":
			var qs []contexts.DNSQueryResult
			_ = decodeInto(r.Details["queries"], &qs)
			existing.Network.DNS.Queries = qs

		case "proxy":
			env := map[string]string{}
			for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"} {
				if s, ok := r.Details[k].(string); ok && s != "" {
					env[k] = s
				}
			}
			existing.Network.Proxy.Env = env
			existing.Network.Proxy.DetectedURL = asString(r.Details["proxy_detected"])

		case "tls_pki":
			existing.Network.TLSPKI.SystemRootsCount = int(asInt64(r.Details["system_roots"]))
			existing.Network.TLSPKI.SSLCertFile = asString(r.Details["ssl_cert_file"])

		case "time_ntp":
			existing.System.Time.WallTimeRFC3339 = asString(r.Details["wall_time"])
			existing.System.Time.MonotonicTestNS = asInt64(r.Details["monotonic_test_ns"])

		case "os_kernel_features":
			existing.System.Kernel.CgroupsPresent = asBool(r.Details["cgroups"])
			existing.System.Kernel.SELinuxPresent = asBool(r.Details["selinux"])
			existing.System.Kernel.AppArmorPresent = asBool(r.Details["apparmor"])
			// CGroupVersion is also filled by resources, prefer that where present

		case "virtualization":
			existing.System.Virt.InContainer = asBool(r.Details["container"])
			existing.System.Virt.DockerEnv = asBool(r.Details["docker_env"])

		case "cpu_features":
			existing.Compute.CPU.Arch = asString(r.Details["arch"])
			if existing.Compute.CPU.Arch == "" {
				existing.Compute.CPU.Arch = runtime.GOARCH
			}
			if v := asInt64(r.Details["cores_logical"]); v > 0 {
				existing.Compute.CPU.CoresLogical = int(v)
			}

		case "memory_characteristics":
			existing.Compute.Memory.AllocBytes = uint64(asInt64(r.Details["alloc_bytes"]))
			existing.Compute.Memory.SysBytes = uint64(asInt64(r.Details["sys_bytes"]))
			existing.Compute.Memory.HeapAlloc = uint64(asInt64(r.Details["heap_alloc"]))
			existing.Compute.Memory.HeapSys = uint64(asInt64(r.Details["heap_sys"]))
			existing.Compute.Memory.HeapIdle = uint64(asInt64(r.Details["heap_idle"]))
			existing.Compute.Memory.HeapInuse = uint64(asInt64(r.Details["heap_inuse"]))

		case "system_limits":
			existing.Compute.Limits.Implemented = r.Supported
			if !r.Supported && r.Error != "" {
				existing.Compute.Limits.Note = r.Error
			}

		case "security_posture":
			existing.System.Security.SELinuxPresent = asBool(r.Details["selinux_present"])
			existing.System.Security.AppArmorPresent = asBool(r.Details["apparmor_present"])

		case "cli_toolchain":
			existing.Tooling.CLI.Found = anyStringSlice(r.Details["found"])
			existing.Tooling.CLI.Missing = anyStringSlice(r.Details["missing"])

		case "language_runtimes":
			langs := map[string]string{}
			for k, v := range r.Details {
				if s, ok := v.(string); ok && s != "" {
					langs[k] = s
				}
			}
			existing.Tooling.LanguageRuntimes = langs

		case "container_ecosystem":
			existing.Tooling.ContainerTools = anyStringSlice(r.Details["found"])

		case "kubernetes_deep":
			k := existing.Kubernetes
			if s := asString(r.Details["kubectl_path"]); s != "" {
				k.KubectlPath = s
			}
			if s := asString(r.Details["kubectl_client_version"]); s != "" {
				k.KubectlClientVersion = s
			}
			existing.Kubernetes = k

		case "cloud_environment":
			existing.Cloud.Detected = anyStringSlice(r.Details["detected"])

		case "source_control_connectivity":
			existing.Network.SourceControl.SSHOK = asBool(r.Details["ssh_ok"])
			existing.Network.SourceControl.HTTPSOK = asBool(r.Details["https_ok"])
			existing.Network.SourceControl.SSHLatencyMS = asInt64(r.Details["ssh_latency_ms"])
			existing.Network.SourceControl.HTTPSLatency = asInt64(r.Details["https_latency_ms"])

		case "vpn":
			existing.Network.VPN.Present = asBool(r.Details["env_vpn"])
			if existing.Network.VPN.Present {
				existing.Network.VPN.Method = "env"
			}

		case "temp_cache_dirs":
			var dirs []contexts.DirWritableStatus
			_ = decodeInto(r.Details["dirs"], &dirs)
			existing.Storage.TempCache = dirs

		case "locale_environment":
			vars := map[string]string{}
			for _, k := range []string{"LANG", "LC_ALL", "LC_CTYPE", "TZ"} {
				if s, ok := r.Details[k].(string); ok && s != "" {
					vars[k] = s
				}
			}
			existing.System.Locale.Vars = vars

		case "compliance":
			existing.System.Compliance.FIPSEnabledRaw = asString(r.Details["fips_enabled_raw"])
		}
	}

	// cross-source preference: CGroupVersion from resources if available
	if v := existing.Resources.CGroupVersion; v != "" {
		existing.System.Kernel.CGroupVersion = v
	}

	return existing
}

// ---------- helpers ----------

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case uint64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}
func asFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}
func anyStringSlice(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			if str, ok := e.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case nil:
		return nil
	default:
		// best-effort: try JSON roundtrip
		var out []string
		if decodeInto(v, &out) {
			return out
		}
		return nil
	}
}
func asStringMap(v any) (map[string]string, bool) {
	switch m := v.(type) {
	case map[string]string:
		return m, true
	case map[string]any:
		out := map[string]string{}
		for k, val := range m {
			if s, ok := val.(string); ok {
				out[k] = s
			}
		}
		return out, true
	default:
		// JSON roundtrip fallback
		out := map[string]string{}
		if decodeInto(v, &out) {
			return out, true
		}
		return nil, false
	}
}

// decodeInto uses JSON roundtrip to map arbitrary struct/map/slice into a typed destination.
func decodeInto[T any](v any, dst *T) bool {
	if v == nil {
		return false
	}
	b, err := json.Marshal(v)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return false
	}
	return true
}
