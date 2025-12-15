package p2p

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"bib/internal/config"
)

func TestSelectiveHandler_Subscriptions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-selective-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.P2PConfig{}
	handler, err := NewSelectiveHandler(nil, nil, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	// Subscribe to a topic
	if err := handler.Subscribe("topic-*"); err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	subs := handler.Subscriptions()
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].TopicPattern != "topic-*" {
		t.Errorf("expected pattern 'topic-*', got %q", subs[0].TopicPattern)
	}

	// Subscribe again (should not duplicate)
	if err := handler.Subscribe("topic-*"); err != nil {
		t.Fatalf("failed to subscribe again: %v", err)
	}
	subs = handler.Subscriptions()
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription after duplicate, got %d", len(subs))
	}

	// Unsubscribe
	if err := handler.Unsubscribe("topic-*"); err != nil {
		t.Fatalf("failed to unsubscribe: %v", err)
	}
	subs = handler.Subscriptions()
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions after unsubscribe, got %d", len(subs))
	}
}

func TestSelectiveHandler_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-selective-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.P2PConfig{}

	// Create first handler and subscribe
	handler1, err := NewSelectiveHandler(nil, nil, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	if err := handler1.Subscribe("persistent-topic"); err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	if err := handler1.Stop(); err != nil {
		t.Fatalf("failed to stop handler: %v", err)
	}

	// Verify file was created
	subPath := filepath.Join(tmpDir, "subscriptions.json")
	if _, err := os.Stat(subPath); os.IsNotExist(err) {
		t.Fatal("subscriptions file was not created")
	}

	// Create second handler and verify subscriptions loaded
	handler2, err := NewSelectiveHandler(nil, nil, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create second handler: %v", err)
	}

	subs := handler2.Subscriptions()
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription after reload, got %d", len(subs))
	}
	if subs[0].TopicPattern != "persistent-topic" {
		t.Errorf("expected pattern 'persistent-topic', got %q", subs[0].TopicPattern)
	}
}

func TestSelectiveHandler_Mode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-selective-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	handler, _ := NewSelectiveHandler(nil, nil, config.P2PConfig{}, tmpDir)
	if handler.Mode() != NodeModeSelective {
		t.Errorf("expected mode %s, got %s", NodeModeSelective, handler.Mode())
	}
}

func TestSelectiveHandler_StartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-selective-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	handler, _ := NewSelectiveHandler(nil, nil, config.P2PConfig{}, tmpDir)

	ctx := context.Background()
	if err := handler.Start(ctx); err != nil {
		t.Fatalf("failed to start handler: %v", err)
	}

	if err := handler.Stop(); err != nil {
		t.Fatalf("failed to stop handler: %v", err)
	}
}

func TestSelectiveHandler_ConfigUpdate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-selective-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	handler, _ := NewSelectiveHandler(nil, nil, config.P2PConfig{}, tmpDir)

	// Update config with subscriptions
	newCfg := config.P2PConfig{
		Selective: config.SelectiveConfig{
			Subscriptions: []string{"config-topic-1", "config-topic-2"},
		},
	}

	if err := handler.OnConfigUpdate(newCfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	subs := handler.Subscriptions()
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions from config, got %d", len(subs))
	}
}
