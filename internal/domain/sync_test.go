package domain

import (
	"testing"
	"time"
)

func TestSyncStatus(t *testing.T) {
	now := time.Now()
	status := SyncStatus{
		InProgress:     true,
		LastSyncTime:   now,
		LastSyncError:  "connection failed",
		PendingEntries: 5,
		SyncedEntries:  100,
	}

	if !status.InProgress {
		t.Error("expected InProgress to be true")
	}
	if !status.LastSyncTime.Equal(now) {
		t.Error("LastSyncTime mismatch")
	}
	if status.LastSyncError != "connection failed" {
		t.Errorf("expected 'connection failed', got %q", status.LastSyncError)
	}
	if status.PendingEntries != 5 {
		t.Errorf("expected 5, got %d", status.PendingEntries)
	}
	if status.SyncedEntries != 100 {
		t.Errorf("expected 100, got %d", status.SyncedEntries)
	}
}

func TestSubscription(t *testing.T) {
	now := time.Now()
	sub := Subscription{
		TopicPattern: "weather/*",
		CreatedAt:    now,
		LastSync:     now.Add(-time.Hour),
	}

	if sub.TopicPattern != "weather/*" {
		t.Errorf("expected 'weather/*', got %q", sub.TopicPattern)
	}
	if !sub.CreatedAt.Equal(now) {
		t.Error("CreatedAt mismatch")
	}
	if !sub.LastSync.Equal(now.Add(-time.Hour)) {
		t.Error("LastSync mismatch")
	}
}

func TestDownloadStatus_Constants(t *testing.T) {
	if DownloadStatusActive != "active" {
		t.Errorf("expected 'active', got %q", DownloadStatusActive)
	}
	if DownloadStatusPaused != "paused" {
		t.Errorf("expected 'paused', got %q", DownloadStatusPaused)
	}
	if DownloadStatusCompleted != "completed" {
		t.Errorf("expected 'completed', got %q", DownloadStatusCompleted)
	}
	if DownloadStatusFailed != "failed" {
		t.Errorf("expected 'failed', got %q", DownloadStatusFailed)
	}
}

func TestDownload(t *testing.T) {
	now := time.Now()
	download := Download{
		ID:              "dl-123",
		DatasetID:       "dataset-1",
		DatasetHash:     "abc123",
		PeerID:          "peer-1",
		TotalChunks:     10,
		CompletedChunks: 5,
		ChunkBitmap:     []byte{0b11111000, 0b00000000},
		Status:          DownloadStatusActive,
		StartedAt:       now,
		UpdatedAt:       now,
		Error:           "",
	}

	if download.ID != "dl-123" {
		t.Errorf("expected 'dl-123', got %q", download.ID)
	}
	if download.DatasetID != "dataset-1" {
		t.Errorf("expected 'dataset-1', got %q", download.DatasetID)
	}
	if download.DatasetHash != "abc123" {
		t.Errorf("expected 'abc123', got %q", download.DatasetHash)
	}
	if download.PeerID != "peer-1" {
		t.Errorf("expected 'peer-1', got %q", download.PeerID)
	}
	if download.TotalChunks != 10 {
		t.Errorf("expected 10, got %d", download.TotalChunks)
	}
	if download.CompletedChunks != 5 {
		t.Errorf("expected 5, got %d", download.CompletedChunks)
	}
	if download.Status != DownloadStatusActive {
		t.Errorf("expected DownloadStatusActive, got %q", download.Status)
	}
	if !download.StartedAt.Equal(now) {
		t.Error("StartedAt mismatch")
	}
	if download.Error != "" {
		t.Errorf("expected empty error, got %q", download.Error)
	}
}

func TestDownload_Progress(t *testing.T) {
	tests := []struct {
		name            string
		totalChunks     int
		completedChunks int
		expectedPercent float64
	}{
		{"half done", 10, 5, 50.0},
		{"not started", 10, 0, 0.0},
		{"complete", 10, 10, 100.0},
		{"zero total", 0, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			download := Download{
				TotalChunks:     tt.totalChunks,
				CompletedChunks: tt.completedChunks,
			}

			var percent float64
			if download.TotalChunks > 0 {
				percent = float64(download.CompletedChunks) / float64(download.TotalChunks) * 100
			}

			if percent != tt.expectedPercent {
				t.Errorf("expected %.1f%%, got %.1f%%", tt.expectedPercent, percent)
			}
		})
	}
}

func TestDownload_WithError(t *testing.T) {
	download := Download{
		ID:     "dl-123",
		Status: DownloadStatusFailed,
		Error:  "connection timeout",
	}

	if download.Status != DownloadStatusFailed {
		t.Errorf("expected DownloadStatusFailed, got %q", download.Status)
	}
	if download.Error != "connection timeout" {
		t.Errorf("expected 'connection timeout', got %q", download.Error)
	}
}
