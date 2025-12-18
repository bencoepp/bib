package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
)

// AlertDetector detects suspicious patterns in audit entries.
type AlertDetector struct {
	config         AlertConfig
	thresholdRules []ThresholdRule
	celRules       []CELRule
	counters       map[string]*alertCounter
	mu             sync.RWMutex
	callbacks      []AlertCallback
}

// AlertConfig holds alert detection configuration.
type AlertConfig struct {
	// Enabled controls whether alert detection is active.
	Enabled bool `mapstructure:"enabled"`

	// ThresholdRules are simple threshold-based rules.
	ThresholdRules []ThresholdRule `mapstructure:"threshold_rules"`

	// CELRules are CEL expression-based rules.
	CELRules []CELRuleConfig `mapstructure:"cel_rules"`

	// WindowDuration is the default time window for threshold detection.
	WindowDuration time.Duration `mapstructure:"window_duration"`

	// CleanupInterval is how often to clean up old counters.
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// ThresholdRule defines a simple threshold-based alert rule.
type ThresholdRule struct {
	// Name is the unique rule name.
	Name string `mapstructure:"name"`

	// Description describes what this rule detects.
	Description string `mapstructure:"description"`

	// Enabled controls whether this rule is active.
	Enabled bool `mapstructure:"enabled"`

	// Action filters by action type (empty = all).
	Action Action `mapstructure:"action"`

	// Table filters by table name (empty = all).
	Table string `mapstructure:"table"`

	// Role filters by role (empty = all).
	Role string `mapstructure:"role"`

	// Threshold is the count that triggers an alert.
	Threshold int `mapstructure:"threshold"`

	// Window is the time window for counting.
	Window time.Duration `mapstructure:"window"`

	// GroupBy determines how to group counts (e.g., "actor", "node_id", "role").
	GroupBy string `mapstructure:"group_by"`

	// Severity is the alert severity.
	Severity AlertSeverity `mapstructure:"severity"`

	// TriggerRateLimit indicates if this should trigger rate limiting.
	TriggerRateLimit bool `mapstructure:"trigger_rate_limit"`
}

// CELRuleConfig holds configuration for a CEL-based rule.
type CELRuleConfig struct {
	// Name is the unique rule name.
	Name string `mapstructure:"name"`

	// Description describes what this rule detects.
	Description string `mapstructure:"description"`

	// Enabled controls whether this rule is active.
	Enabled bool `mapstructure:"enabled"`

	// Expression is the CEL expression to evaluate.
	Expression string `mapstructure:"expression"`

	// Severity is the alert severity.
	Severity AlertSeverity `mapstructure:"severity"`

	// TriggerRateLimit indicates if this should trigger rate limiting.
	TriggerRateLimit bool `mapstructure:"trigger_rate_limit"`
}

// CELRule is a compiled CEL rule.
type CELRule struct {
	Config  CELRuleConfig
	Program cel.Program
}

// AlertSeverity represents the severity of an alert.
type AlertSeverity string

const (
	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Alert represents a triggered alert.
type Alert struct {
	// ID is the unique alert ID.
	ID string `json:"id"`

	// RuleName is the name of the rule that triggered.
	RuleName string `json:"rule_name"`

	// Description is the rule description.
	Description string `json:"description"`

	// Severity is the alert severity.
	Severity AlertSeverity `json:"severity"`

	// Timestamp is when the alert was triggered.
	Timestamp time.Time `json:"timestamp"`

	// Entry is the audit entry that triggered the alert.
	Entry *Entry `json:"entry"`

	// Count is the count that triggered the threshold (for threshold rules).
	Count int `json:"count,omitempty"`

	// Threshold is the threshold that was exceeded.
	Threshold int `json:"threshold,omitempty"`

	// TriggerRateLimit indicates if rate limiting should be triggered.
	TriggerRateLimit bool `json:"trigger_rate_limit"`

	// Metadata holds additional context.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AlertCallback is called when an alert is triggered.
type AlertCallback func(ctx context.Context, alert *Alert)

// alertCounter tracks counts for threshold rules.
type alertCounter struct {
	key       string
	counts    []time.Time
	threshold int
	window    time.Duration
}

// DefaultAlertConfig returns the default alert configuration.
func DefaultAlertConfig() AlertConfig {
	return AlertConfig{
		Enabled:         true,
		WindowDuration:  5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		ThresholdRules: []ThresholdRule{
			{
				Name:             "bulk_select",
				Description:      "Large number of SELECT queries in short time",
				Enabled:          true,
				Action:           ActionSelect,
				Threshold:        100,
				Window:           5 * time.Minute,
				GroupBy:          "actor",
				Severity:         AlertSeverityMedium,
				TriggerRateLimit: true,
			},
			{
				Name:             "bulk_delete",
				Description:      "Large number of DELETE queries in short time",
				Enabled:          true,
				Action:           ActionDelete,
				Threshold:        50,
				Window:           5 * time.Minute,
				GroupBy:          "actor",
				Severity:         AlertSeverityHigh,
				TriggerRateLimit: true,
			},
			{
				Name:             "ddl_operations",
				Description:      "Schema modification attempts",
				Enabled:          true,
				Action:           ActionDDL,
				Threshold:        5,
				Window:           1 * time.Minute,
				GroupBy:          "role",
				Severity:         AlertSeverityCritical,
				TriggerRateLimit: true,
			},
			{
				Name:             "auth_failures",
				Description:      "Multiple failed authentication attempts",
				Enabled:          true,
				Table:            "audit_log",
				Threshold:        10,
				Window:           5 * time.Minute,
				GroupBy:          "actor",
				Severity:         AlertSeverityHigh,
				TriggerRateLimit: true,
			},
		},
		CELRules: []CELRuleConfig{
			{
				Name:             "unusual_role_access",
				Description:      "Admin role used for normal operations",
				Enabled:          true,
				Expression:       `entry.role_used == "bibd_admin" && entry.action == "SELECT"`,
				Severity:         AlertSeverityMedium,
				TriggerRateLimit: false,
			},
			{
				Name:             "large_result_set",
				Description:      "Query returned unusually large number of rows",
				Enabled:          true,
				Expression:       `entry.rows_affected > 10000`,
				Severity:         AlertSeverityMedium,
				TriggerRateLimit: true,
			},
			{
				Name:             "slow_query",
				Description:      "Query took unusually long to execute",
				Enabled:          true,
				Expression:       `entry.duration_ms > 30000`,
				Severity:         AlertSeverityLow,
				TriggerRateLimit: false,
			},
		},
	}
}

// NewAlertDetector creates a new alert detector.
func NewAlertDetector(cfg AlertConfig) (*AlertDetector, error) {
	detector := &AlertDetector{
		config:         cfg,
		thresholdRules: cfg.ThresholdRules,
		counters:       make(map[string]*alertCounter),
		callbacks:      make([]AlertCallback, 0),
	}

	// Compile CEL rules
	for _, ruleCfg := range cfg.CELRules {
		if !ruleCfg.Enabled {
			continue
		}

		rule, err := compileCELRule(ruleCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to compile CEL rule %s: %w", ruleCfg.Name, err)
		}
		detector.celRules = append(detector.celRules, *rule)
	}

	// Start cleanup goroutine
	if cfg.CleanupInterval > 0 {
		go detector.cleanupLoop(cfg.CleanupInterval)
	}

	return detector, nil
}

// compileCELRule compiles a CEL rule configuration.
func compileCELRule(cfg CELRuleConfig) (*CELRule, error) {
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("entry", decls.NewMapType(decls.String, decls.Dyn)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	ast, issues := env.Compile(cfg.Expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile expression: %w", issues.Err())
	}

	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to create program: %w", err)
	}

	return &CELRule{
		Config:  cfg,
		Program: program,
	}, nil
}

// OnAlert registers a callback to be called when an alert is triggered.
func (d *AlertDetector) OnAlert(callback AlertCallback) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callbacks = append(d.callbacks, callback)
}

// Check evaluates an audit entry against all rules.
func (d *AlertDetector) Check(ctx context.Context, entry *Entry) []*Alert {
	if d == nil || !d.config.Enabled {
		return nil
	}

	var alerts []*Alert

	// Check threshold rules
	for _, rule := range d.thresholdRules {
		if !rule.Enabled {
			continue
		}

		if alert := d.checkThresholdRule(ctx, entry, rule); alert != nil {
			alerts = append(alerts, alert)
		}
	}

	// Check CEL rules
	for _, rule := range d.celRules {
		if alert := d.checkCELRule(ctx, entry, rule); alert != nil {
			alerts = append(alerts, alert)
		}
	}

	// Trigger callbacks
	for _, alert := range alerts {
		d.triggerCallbacks(ctx, alert)
	}

	return alerts
}

// checkThresholdRule checks an entry against a threshold rule.
func (d *AlertDetector) checkThresholdRule(ctx context.Context, entry *Entry, rule ThresholdRule) *Alert {
	// Check if entry matches rule filters
	if rule.Action != "" && rule.Action != entry.Action {
		return nil
	}
	if rule.Table != "" && rule.Table != entry.TableName {
		return nil
	}
	if rule.Role != "" && rule.Role != entry.RoleUsed {
		return nil
	}

	// Determine grouping key
	groupValue := d.getGroupValue(entry, rule.GroupBy)
	counterKey := fmt.Sprintf("%s:%s:%s", rule.Name, rule.GroupBy, groupValue)

	// Update counter
	count := d.incrementCounter(counterKey, rule.Threshold, rule.Window)

	// Check threshold
	if count >= rule.Threshold {
		return &Alert{
			ID:               GenerateOperationID(),
			RuleName:         rule.Name,
			Description:      rule.Description,
			Severity:         rule.Severity,
			Timestamp:        time.Now().UTC(),
			Entry:            entry,
			Count:            count,
			Threshold:        rule.Threshold,
			TriggerRateLimit: rule.TriggerRateLimit,
			Metadata: map[string]any{
				"group_by":    rule.GroupBy,
				"group_value": groupValue,
				"window":      rule.Window.String(),
			},
		}
	}

	return nil
}

// checkCELRule checks an entry against a CEL rule.
func (d *AlertDetector) checkCELRule(ctx context.Context, entry *Entry, rule CELRule) *Alert {
	// Convert entry to map for CEL evaluation
	entryMap := map[string]any{
		"node_id":          entry.NodeID,
		"job_id":           entry.JobID,
		"operation_id":     entry.OperationID,
		"role_used":        entry.RoleUsed,
		"action":           string(entry.Action),
		"table_name":       entry.TableName,
		"rows_affected":    entry.RowsAffected,
		"duration_ms":      entry.DurationMS,
		"source_component": entry.SourceComponent,
		"actor":            entry.Actor,
		"suspicious":       entry.Flags.Suspicious,
		"break_glass":      entry.Flags.BreakGlass,
	}

	// Evaluate expression
	result, _, err := rule.Program.Eval(map[string]any{
		"entry": entryMap,
	})
	if err != nil {
		// Log error but don't fail
		return nil
	}

	// Check if rule matched
	if matched, ok := result.Value().(bool); ok && matched {
		return &Alert{
			ID:               GenerateOperationID(),
			RuleName:         rule.Config.Name,
			Description:      rule.Config.Description,
			Severity:         rule.Config.Severity,
			Timestamp:        time.Now().UTC(),
			Entry:            entry,
			TriggerRateLimit: rule.Config.TriggerRateLimit,
			Metadata: map[string]any{
				"expression": rule.Config.Expression,
			},
		}
	}

	return nil
}

// getGroupValue extracts the grouping value from an entry.
func (d *AlertDetector) getGroupValue(entry *Entry, groupBy string) string {
	switch groupBy {
	case "actor":
		return entry.Actor
	case "node_id":
		return entry.NodeID
	case "role":
		return entry.RoleUsed
	case "table":
		return entry.TableName
	case "action":
		return string(entry.Action)
	case "source":
		return entry.SourceComponent
	default:
		return "global"
	}
}

// incrementCounter increments a counter and returns the current count.
func (d *AlertDetector) incrementCounter(key string, threshold int, window time.Duration) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	counter, ok := d.counters[key]
	if !ok {
		counter = &alertCounter{
			key:       key,
			counts:    make([]time.Time, 0, threshold*2),
			threshold: threshold,
			window:    window,
		}
		d.counters[key] = counter
	}

	// Remove expired entries
	cutoff := now.Add(-window)
	validCounts := make([]time.Time, 0, len(counter.counts)+1)
	for _, t := range counter.counts {
		if t.After(cutoff) {
			validCounts = append(validCounts, t)
		}
	}

	// Add new entry
	validCounts = append(validCounts, now)
	counter.counts = validCounts

	return len(counter.counts)
}

// triggerCallbacks calls all registered callbacks.
func (d *AlertDetector) triggerCallbacks(ctx context.Context, alert *Alert) {
	d.mu.RLock()
	callbacks := make([]AlertCallback, len(d.callbacks))
	copy(callbacks, d.callbacks)
	d.mu.RUnlock()

	for _, callback := range callbacks {
		callback(ctx, alert)
	}
}

// cleanupLoop periodically cleans up expired counters.
func (d *AlertDetector) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		d.cleanup()
	}
}

// cleanup removes expired counters.
func (d *AlertDetector) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for key, counter := range d.counters {
		cutoff := now.Add(-counter.window)
		hasValidEntries := false
		for _, t := range counter.counts {
			if t.After(cutoff) {
				hasValidEntries = true
				break
			}
		}
		if !hasValidEntries {
			delete(d.counters, key)
		}
	}
}

// GetStats returns current alert detection statistics.
func (d *AlertDetector) GetStats() AlertStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return AlertStats{
		ThresholdRulesCount: len(d.thresholdRules),
		CELRulesCount:       len(d.celRules),
		ActiveCounters:      len(d.counters),
		CallbackCount:       len(d.callbacks),
	}
}

// AlertStats contains alert detection statistics.
type AlertStats struct {
	ThresholdRulesCount int `json:"threshold_rules_count"`
	CELRulesCount       int `json:"cel_rules_count"`
	ActiveCounters      int `json:"active_counters"`
	CallbackCount       int `json:"callback_count"`
}
