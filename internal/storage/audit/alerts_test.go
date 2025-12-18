package audit

import (
	"context"
	"testing"
	"time"
)

func TestNewAlertDetector(t *testing.T) {
	cfg := DefaultAlertConfig()
	detector, err := NewAlertDetector(cfg)
	if err != nil {
		t.Fatalf("NewAlertDetector() error = %v", err)
	}

	if detector == nil {
		t.Fatal("NewAlertDetector returned nil")
	}

	stats := detector.GetStats()
	if stats.ThresholdRulesCount == 0 {
		t.Error("Should have threshold rules")
	}
}

func TestAlertDetector_ThresholdRule(t *testing.T) {
	cfg := AlertConfig{
		Enabled:         true,
		WindowDuration:  1 * time.Minute,
		CleanupInterval: 0, // Disable cleanup for test
		ThresholdRules: []ThresholdRule{
			{
				Name:             "test_rule",
				Description:      "Test threshold rule",
				Enabled:          true,
				Action:           ActionSelect,
				Threshold:        3,
				Window:           1 * time.Minute,
				GroupBy:          "actor",
				Severity:         AlertSeverityMedium,
				TriggerRateLimit: true,
			},
		},
	}

	detector, err := NewAlertDetector(cfg)
	if err != nil {
		t.Fatalf("NewAlertDetector() error = %v", err)
	}

	ctx := context.Background()

	// First two entries should not trigger
	for i := 0; i < 2; i++ {
		entry := &Entry{
			NodeID:          "node-1",
			OperationID:     GenerateOperationID(),
			RoleUsed:        "bibd_query",
			Action:          ActionSelect,
			SourceComponent: "test",
			Actor:           "user-1",
		}

		alerts := detector.Check(ctx, entry)
		if len(alerts) > 0 {
			t.Errorf("Entry %d should not trigger alert", i+1)
		}
	}

	// Third entry should trigger
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		Actor:           "user-1",
	}

	alerts := detector.Check(ctx, entry)
	if len(alerts) == 0 {
		t.Error("Third entry should trigger alert")
		return
	}

	alert := alerts[0]
	if alert.RuleName != "test_rule" {
		t.Errorf("Alert rule name = %s, want test_rule", alert.RuleName)
	}
	if alert.Count < 3 {
		t.Errorf("Alert count = %d, want >= 3", alert.Count)
	}
	if !alert.TriggerRateLimit {
		t.Error("Alert should trigger rate limit")
	}
}

func TestAlertDetector_ThresholdRuleGrouping(t *testing.T) {
	cfg := AlertConfig{
		Enabled:         true,
		WindowDuration:  1 * time.Minute,
		CleanupInterval: 0,
		ThresholdRules: []ThresholdRule{
			{
				Name:      "test_rule",
				Enabled:   true,
				Action:    ActionSelect,
				Threshold: 3,
				Window:    1 * time.Minute,
				GroupBy:   "actor",
				Severity:  AlertSeverityMedium,
			},
		},
	}

	detector, err := NewAlertDetector(cfg)
	if err != nil {
		t.Fatalf("NewAlertDetector() error = %v", err)
	}

	ctx := context.Background()

	// Different actors should not cross-trigger
	for i := 0; i < 2; i++ {
		entry := &Entry{
			NodeID:          "node-1",
			OperationID:     GenerateOperationID(),
			RoleUsed:        "bibd_query",
			Action:          ActionSelect,
			SourceComponent: "test",
			Actor:           "user-1",
		}
		detector.Check(ctx, entry)

		entry2 := &Entry{
			NodeID:          "node-1",
			OperationID:     GenerateOperationID(),
			RoleUsed:        "bibd_query",
			Action:          ActionSelect,
			SourceComponent: "test",
			Actor:           "user-2",
		}
		detector.Check(ctx, entry2)
	}

	// Neither should have triggered yet (only 2 each)
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		Actor:           "user-1",
	}

	alerts := detector.Check(ctx, entry)
	if len(alerts) == 0 {
		t.Error("user-1 should trigger alert at 3rd entry")
	}

	// user-2 still shouldn't trigger
	entry2 := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		Actor:           "user-3",
	}

	alerts2 := detector.Check(ctx, entry2)
	if len(alerts2) > 0 {
		t.Error("user-3 should not trigger alert")
	}
}

func TestAlertDetector_ActionFilter(t *testing.T) {
	cfg := AlertConfig{
		Enabled:         true,
		CleanupInterval: 0,
		ThresholdRules: []ThresholdRule{
			{
				Name:      "delete_rule",
				Enabled:   true,
				Action:    ActionDelete,
				Threshold: 2,
				Window:    1 * time.Minute,
				GroupBy:   "global",
				Severity:  AlertSeverityHigh,
			},
		},
	}

	detector, err := NewAlertDetector(cfg)
	if err != nil {
		t.Fatalf("NewAlertDetector() error = %v", err)
	}

	ctx := context.Background()

	// SELECT should not count
	for i := 0; i < 5; i++ {
		entry := &Entry{
			NodeID:          "node-1",
			OperationID:     GenerateOperationID(),
			RoleUsed:        "bibd_query",
			Action:          ActionSelect,
			SourceComponent: "test",
		}
		alerts := detector.Check(ctx, entry)
		if len(alerts) > 0 {
			t.Error("SELECT should not trigger DELETE rule")
		}
	}

	// DELETE should count
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_transform",
		Action:          ActionDelete,
		SourceComponent: "test",
	}
	detector.Check(ctx, entry)

	entry2 := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_transform",
		Action:          ActionDelete,
		SourceComponent: "test",
	}
	alerts := detector.Check(ctx, entry2)

	if len(alerts) == 0 {
		t.Error("Second DELETE should trigger alert")
	}
}

func TestAlertDetector_CELRule(t *testing.T) {
	cfg := AlertConfig{
		Enabled:         true,
		CleanupInterval: 0,
		CELRules: []CELRuleConfig{
			{
				Name:             "large_result",
				Description:      "Large result set",
				Enabled:          true,
				Expression:       `entry.rows_affected > 1000`,
				Severity:         AlertSeverityMedium,
				TriggerRateLimit: true,
			},
		},
	}

	detector, err := NewAlertDetector(cfg)
	if err != nil {
		t.Fatalf("NewAlertDetector() error = %v", err)
	}

	ctx := context.Background()

	// Small result should not trigger
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		RowsAffected:    100,
	}

	alerts := detector.Check(ctx, entry)
	if len(alerts) > 0 {
		t.Error("Small result should not trigger")
	}

	// Large result should trigger
	entry2 := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		RowsAffected:    5000,
	}

	alerts = detector.Check(ctx, entry2)
	if len(alerts) == 0 {
		t.Error("Large result should trigger alert")
		return
	}

	if alerts[0].RuleName != "large_result" {
		t.Errorf("Alert rule = %s, want large_result", alerts[0].RuleName)
	}
}

func TestAlertDetector_OnAlert(t *testing.T) {
	cfg := AlertConfig{
		Enabled:         true,
		CleanupInterval: 0,
		ThresholdRules: []ThresholdRule{
			{
				Name:      "test_rule",
				Enabled:   true,
				Threshold: 1, // Trigger immediately
				Window:    1 * time.Minute,
				GroupBy:   "global",
				Severity:  AlertSeverityMedium,
			},
		},
	}

	detector, err := NewAlertDetector(cfg)
	if err != nil {
		t.Fatalf("NewAlertDetector() error = %v", err)
	}

	ctx := context.Background()

	// Register callback
	alertReceived := make(chan *Alert, 1)
	detector.OnAlert(func(ctx context.Context, alert *Alert) {
		alertReceived <- alert
	})

	// Trigger alert
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
	}

	detector.Check(ctx, entry)

	// Check callback was called
	select {
	case alert := <-alertReceived:
		if alert.RuleName != "test_rule" {
			t.Errorf("Callback received wrong alert: %s", alert.RuleName)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Callback not called")
	}
}

func TestAlertDetector_Disabled(t *testing.T) {
	cfg := AlertConfig{
		Enabled: false,
		ThresholdRules: []ThresholdRule{
			{
				Name:      "test_rule",
				Enabled:   true,
				Threshold: 1,
				Window:    1 * time.Minute,
				GroupBy:   "global",
			},
		},
	}

	detector, err := NewAlertDetector(cfg)
	if err != nil {
		t.Fatalf("NewAlertDetector() error = %v", err)
	}

	ctx := context.Background()

	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
	}

	alerts := detector.Check(ctx, entry)
	if len(alerts) > 0 {
		t.Error("Disabled detector should not trigger alerts")
	}
}
