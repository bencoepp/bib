package contexts

import (
	"bib/internal/capcheck"
	"bib/internal/capcheck/checks"
	"bib/internal/config"
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

type CapabilityContext struct {
	ContainerRuntime ContainerRuntimeCapability `json:"container_runtime"`
	Kubernetes       KubernetesCapability       `json:"kubernetes"`
	Internet         InternetCapability         `json:"internet"`
	Resources        ResourcesCapability        `json:"resources"`

	LastUpdated time.Time `json:"last_updated"`
}

type ContainerRuntimeCapability struct {
	AvailableRuntimes []string             `json:"available_runtimes,omitempty"`
	ErrorMap          map[string]string    `json:"errors,omitempty"`
	Responsive        bool                 `json:"responsive"`
	LastResult        capcheck.CheckResult `json:"last_result"`
}

type KubernetesCapability struct {
	KubeconfigPaths       []string             `json:"kubeconfig_paths,omitempty"`
	KubeconfigExists      bool                 `json:"kubeconfig_exists"`
	InClusterTokenPath    string               `json:"in_cluster_token_path,omitempty"`
	InClusterTokenExists  bool                 `json:"in_cluster_token_exists"`
	CurrentContext        string               `json:"current_context,omitempty"`
	CurrentContextCluster string               `json:"current_context_cluster,omitempty"`
	AvailableContexts     []string             `json:"available_contexts,omitempty"`
	Supported             bool                 `json:"supported"`
	LastResult            capcheck.CheckResult `json:"last_result"`
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
	CPUCoresEffective       float64 `json:"cpu_cores_effective"`
	CPUCoresDetectionMethod string  `json:"cpu_detection_method,omitempty"`

	CPUQuota         float64 `json:"cpu_quota,omitempty"`
	CPUPeriod        float64 `json:"cpu_period,omitempty"`
	CPUSetsEffective int     `json:"cpusets_effective,omitempty"`
	CGroupVersion    string  `json:"cgroup_version,omitempty"`

	MemoryBytesEffective  uint64 `json:"memory_bytes_effective"`
	MemoryDetectionMethod string `json:"memory_detection_method,omitempty"`

	Supported  bool                 `json:"supported"`
	LastResult capcheck.CheckResult `json:"last_result"`
}

type CapabilityWatcher struct {
	mu        sync.RWMutex
	current   *CapabilityContext
	stopCh    chan struct{}
	updatesCh chan *CapabilityContext
	interval  time.Duration
	runner    *capcheck.Runner
	cfg       *config.BibDaemonConfig
}

func StartCapabilityChecks(cfg *config.BibDaemonConfig) *CapabilityWatcher {
	interval := 5 * time.Minute
	if cfg != nil && cfg.General.CapabilityRefreshInterval > 0 {
		interval = time.Duration(cfg.General.CapabilityRefreshInterval) * time.Minute
	}

	checkers := []capcheck.Checker{
		checks.ContainerRuntimeChecker{},
		checks.KubernetesConfigChecker{},
		checks.InternetAccessChecker{
			HttpUrl: "https://www.google.com/generate_204",
		},
		checks.ResourcesChecker{},
	}

	runner := capcheck.NewRunner(
		checkers,
		capcheck.WithGlobalTimeout(10*time.Second),
		capcheck.WithPerCheckTimeout(2*time.Second),
	)

	w := &CapabilityWatcher{
		current:   &CapabilityContext{},
		stopCh:    make(chan struct{}),
		updatesCh: make(chan *CapabilityContext, 1),
		interval:  interval,
		runner:    runner,
		cfg:       cfg,
	}

	// First run synchronous
	w.runOnce(context.Background(), true)

	// Periodic loop
	go w.loop()

	return w
}

func (w *CapabilityWatcher) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.runOnce(context.Background(), false)
		case <-w.stopCh:
			return
		}
	}
}

// runOnce executes all checks and updates current context.
func (w *CapabilityWatcher) runOnce(ctx context.Context, first bool) {
	if w.cfg != nil && !w.cfg.General.CheckCapabilities {
		if first {
			log.Info("Capability checks disabled, providing empty context.")
		}
		return
	}

	results := w.runner.Run(ctx)
	newCtx := BuildCapabilityContext(results)
	newCtx.LastUpdated = time.Now()

	w.mu.Lock()
	prev := w.current
	w.current = newCtx
	w.mu.Unlock()

	w.logDiff(prev, newCtx)
	select {
	case w.updatesCh <- newCtx:
	default:
		// drop if no listener
	}
}

// logDiff logs changes in supported/error states to reduce noise.
func (w *CapabilityWatcher) logDiff(old, cur *CapabilityContext) {
	if old == nil {
		log.Info("Initial capability check completed.")
		w.logFull(cur)
		return
	}
	type cmp struct {
		name    string
		prevOK  bool
		curOK   bool
		prevErr string
		curErr  string
	}
	items := []cmp{
		{"container_runtime", old.ContainerRuntime.Responsive, cur.ContainerRuntime.Responsive, old.ContainerRuntime.LastResult.Error, cur.ContainerRuntime.LastResult.Error},
		{"kubernetes", old.Kubernetes.Supported, cur.Kubernetes.Supported, old.Kubernetes.LastResult.Error, cur.Kubernetes.LastResult.Error},
		{"internet_access", old.Internet.Supported, cur.Internet.Supported, old.Internet.LastResult.Error, cur.Internet.LastResult.Error},
		{"resources", old.Resources.Supported, cur.Resources.Supported, old.Resources.LastResult.Error, cur.Resources.LastResult.Error},
	}
	for _, it := range items {
		if it.prevOK != it.curOK || it.prevErr != it.curErr {
			if it.curOK {
				log.Infof("✅ capability %s now supported", it.name)
			} else {
				log.Errorf("⛔ capability %s unsupported: %s", it.name, it.curErr)
			}
		}
	}
}

// logFull can be used on first run.
func (w *CapabilityWatcher) logFull(cur *CapabilityContext) {
	log.Infof("Container runtimes: %v responsive=%v", cur.ContainerRuntime.AvailableRuntimes, cur.ContainerRuntime.Responsive)
	log.Infof("Kubernetes: supported=%v current-context=%s cluster=%s kubeconfigExists=%v inClusterToken=%v",
		cur.Kubernetes.Supported, cur.Kubernetes.CurrentContext, cur.Kubernetes.CurrentContextCluster,
		cur.Kubernetes.KubeconfigExists, cur.Kubernetes.InClusterTokenExists)
	log.Infof("Internet: dns=%v http=%v url=%s status=%d",
		cur.Internet.DNSSuccess, cur.Internet.HTTPSuccess, cur.Internet.URL, cur.Internet.HTTPStatusCode)
	log.Infof("Resources: cpu=%.2f mem=%d method(cpu=%s mem=%s)",
		cur.Resources.CPUCoresEffective, cur.Resources.MemoryBytesEffective,
		cur.Resources.CPUCoresDetectionMethod, cur.Resources.MemoryDetectionMethod)
}

// Current returns a snapshot copy.
func (w *CapabilityWatcher) Current() *CapabilityContext {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// Return pointer (read-only by convention)
	return w.current
}

// Updates returns a channel of context updates.
func (w *CapabilityWatcher) Updates() <-chan *CapabilityContext {
	return w.updatesCh
}

// Stop terminates periodic checks.
func (w *CapabilityWatcher) Stop() {
	close(w.stopCh)
}

// BuildCapabilityContext builds a CapabilityContext from raw results.
func BuildCapabilityContext(results []capcheck.CheckResult) *CapabilityContext {
	ctx := &CapabilityContext{}
	for _, r := range results {
		switch r.ID {
		case "container_runtime":
			cr := ContainerRuntimeCapability{
				Responsive: r.Supported,
				LastResult: r,
			}
			if avail, ok := r.Details["available"].([]string); ok {
				cr.AvailableRuntimes = avail
			} else if rawIface, ok := r.Details["available"]; ok {
				cr.AvailableRuntimes = anyStringSlice(rawIface)
			}
			if errs, ok := r.Details["errors"].(map[string]string); ok {
				cr.ErrorMap = errs
			} else if rawErrs, ok := r.Details["errors"]; ok {
				cr.ErrorMap = anyStringMap(rawErrs)
			}
			ctx.ContainerRuntime = cr

		case "kubernetes_config":
			kc := KubernetesCapability{
				Supported:  r.Supported,
				LastResult: r,
			}
			kc.KubeconfigPaths = anyStringSlice(r.Details["kubeconfig_paths"])
			if p, ok := r.Details["in_cluster_token_path"].(string); ok {
				kc.InClusterTokenPath = p
			}
			if b, ok := r.Details["kubeconfig_exists"].(bool); ok {
				kc.KubeconfigExists = b
			}
			if b, ok := r.Details["in_cluster_token_exists"].(bool); ok {
				kc.InClusterTokenExists = b
			}
			if cc, ok := r.Details["current_context"].(string); ok {
				kc.CurrentContext = cc
			}
			if cl, ok := r.Details["current_context_cluster"].(string); ok {
				kc.CurrentContextCluster = cl
			}
			kc.AvailableContexts = anyStringSlice(r.Details["available_contexts"])
			ctx.Kubernetes = kc

		case "internet_access":
			netc := InternetCapability{
				URL:            asString(r.Details["url"]),
				DNSSuccess:     asBool(r.Details["dns_success"]),
				DNSLatencyMS:   asInt64(r.Details["dns_latency_ms"]),
				ResolvedIPs:    anyStringSlice(r.Details["resolved_ips"]),
				HTTPSuccess:    asBool(r.Details["http_success"]),
				HTTPStatusCode: int(asInt64(r.Details["http_status_code"])),
				HTTPLatencyMS:  asInt64(r.Details["http_latency_ms"]),
				TLSVersion:     asString(r.Details["tls_version"]),
				Supported:      r.Supported,
				LastResult:     r,
			}
			ctx.Internet = netc

		case "resources":
			res := ResourcesCapability{
				CPUCoresEffective:       asFloat64(r.Details["cpu_cores_effective"]),
				CPUCoresDetectionMethod: asString(r.Details["cpu_detection_method"]),
				CPUQuota:                asFloat64(r.Details["cpu_quota"]),
				CPUPeriod:               asFloat64(r.Details["cpu_period"]),
				CPUSetsEffective:        int(asInt64(r.Details["cpusets_effective"])),
				CGroupVersion:           asString(r.Details["cgroup_version"]),
				MemoryBytesEffective:    uint64(asInt64(r.Details["memory_bytes_effective"])),
				MemoryDetectionMethod:   asString(r.Details["memory_detection_method"]),
				Supported:               r.Supported,
				LastResult:              r,
			}
			ctx.Resources = res
		}
	}
	return ctx
}

// --- helper conversion functions ---

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
	case uint64:
		return int64(n)
	case int32:
		return int64(n)
	case float32:
		return int64(n)
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
	default:
		return nil
	}
}
func anyStringMap(v any) map[string]string {
	out := map[string]string{}
	switch m := v.(type) {
	case map[string]string:
		return m
	case map[string]any:
		for k, val := range m {
			if str, ok := val.(string); ok {
				out[k] = str
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
