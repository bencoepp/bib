package grpc

import (
	"testing"
	"time"
)

func TestLogRingBuffer_Add(t *testing.T) {
	buf := NewLogRingBuffer(5)

	// Add entries
	for i := 0; i < 3; i++ {
		buf.Add(LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "test message",
		})
	}

	if buf.count != 3 {
		t.Errorf("expected count 3, got %d", buf.count)
	}
}

func TestLogRingBuffer_Recent(t *testing.T) {
	buf := NewLogRingBuffer(5)

	// Add entries
	for i := 0; i < 7; i++ {
		buf.Add(LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "message " + string(rune('a'+i)),
		})
	}

	// Buffer should only keep 5 entries
	recent := buf.Recent(10)
	if len(recent) != 5 {
		t.Errorf("expected 5 entries, got %d", len(recent))
	}

	// Get 3 most recent
	recent3 := buf.Recent(3)
	if len(recent3) != 3 {
		t.Errorf("expected 3 entries, got %d", len(recent3))
	}
}

func TestLogRingBuffer_Recent_Empty(t *testing.T) {
	buf := NewLogRingBuffer(5)

	recent := buf.Recent(10)
	if recent != nil {
		t.Errorf("expected nil, got %v", recent)
	}
}

func TestLogRingBuffer_Subscribe(t *testing.T) {
	buf := NewLogRingBuffer(10)

	ch, unsubscribe := buf.Subscribe()
	defer unsubscribe()

	// Add entry in goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		buf.Add(LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "test",
		})
	}()

	// Wait for entry
	select {
	case entry := <-ch:
		if entry.Message != "test" {
			t.Errorf("expected 'test', got '%s'", entry.Message)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for log entry")
	}
}

func TestLogRingBuffer_Unsubscribe(t *testing.T) {
	buf := NewLogRingBuffer(10)

	_, unsubscribe := buf.Subscribe()

	// Should have 1 listener
	buf.mu.RLock()
	count := len(buf.listeners)
	buf.mu.RUnlock()
	if count != 1 {
		t.Errorf("expected 1 listener, got %d", count)
	}

	// Unsubscribe
	unsubscribe()

	// Should have 0 listeners
	buf.mu.RLock()
	count = len(buf.listeners)
	buf.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 listeners, got %d", count)
	}
}

func TestMatchesLevel(t *testing.T) {
	tests := []struct {
		entryLevel  string
		filterLevel string
		expected    bool
	}{
		{"info", "info", true},
		{"warn", "info", true},
		{"error", "info", true},
		{"debug", "info", false},
		{"info", "warn", false},
		{"error", "error", true},
		{"INFO", "info", true},    // Case insensitive
		{"unknown", "info", true}, // Unknown levels pass through
	}

	for _, tt := range tests {
		t.Run(tt.entryLevel+"_"+tt.filterLevel, func(t *testing.T) {
			result := matchesLevel(tt.entryLevel, tt.filterLevel)
			if result != tt.expected {
				t.Errorf("matchesLevel(%s, %s) = %v, expected %v",
					tt.entryLevel, tt.filterLevel, result, tt.expected)
			}
		})
	}
}
