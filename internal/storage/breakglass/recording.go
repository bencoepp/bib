package breakglass

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// RecordingEvent represents a single event in a session recording.
type RecordingEvent struct {
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Type is the event type: "input", "output", "query", "result", "error".
	Type string `json:"type"`

	// Data is the event data (input/output bytes, query text, etc.).
	Data string `json:"data"`

	// Duration is the time since the last event (for playback).
	DurationMS int64 `json:"duration_ms,omitempty"`

	// Metadata contains additional context.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// RecordingHeader contains metadata about a session recording.
type RecordingHeader struct {
	// Version is the recording format version.
	Version int `json:"version"`

	// SessionID is the break glass session ID.
	SessionID string `json:"session_id"`

	// Username is the break glass user.
	Username string `json:"username"`

	// StartedAt is when the session started.
	StartedAt time.Time `json:"started_at"`

	// NodeID is the node where the session ran.
	NodeID string `json:"node_id"`

	// AccessLevel is the session access level.
	AccessLevel string `json:"access_level"`

	// Reason is the stated reason for the break glass.
	Reason string `json:"reason"`
}

// RecordingFooter contains summary information about the recording.
type RecordingFooter struct {
	// EndedAt is when the session ended.
	EndedAt time.Time `json:"ended_at"`

	// Duration is the total session duration.
	Duration time.Duration `json:"duration"`

	// EventCount is the total number of events.
	EventCount int64 `json:"event_count"`

	// QueryCount is the number of queries executed.
	QueryCount int64 `json:"query_count"`
}

// SessionRecorder records break glass session activity.
type SessionRecorder struct {
	path       string
	header     *RecordingHeader
	file       *os.File
	gzWriter   *gzip.Writer
	encoder    *json.Encoder
	lastEvent  time.Time
	eventCount int64
	queryCount int64
	mu         sync.Mutex
	closed     bool
}

// NewSessionRecorder creates a new session recorder.
func NewSessionRecorder(path string, session *Session) (*SessionRecorder, error) {
	// Create the recording file
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create recording file: %w", err)
	}

	// Use gzip compression
	gzWriter := gzip.NewWriter(file)

	recorder := &SessionRecorder{
		path:      path,
		file:      file,
		gzWriter:  gzWriter,
		encoder:   json.NewEncoder(gzWriter),
		lastEvent: time.Now(),
		header: &RecordingHeader{
			Version:     1,
			SessionID:   session.ID,
			Username:    session.User.Name,
			StartedAt:   session.StartedAt,
			NodeID:      session.NodeID,
			AccessLevel: session.AccessLevel.String(),
			Reason:      session.Reason,
		},
	}

	// Write header
	if err := recorder.encoder.Encode(recorder.header); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to write recording header: %w", err)
	}

	return recorder, nil
}

// RecordInput records input (e.g., a query from the user).
func (r *SessionRecorder) RecordInput(data string) error {
	return r.record("input", data, nil)
}

// RecordOutput records output (e.g., query results).
func (r *SessionRecorder) RecordOutput(data string) error {
	return r.record("output", data, nil)
}

// RecordQuery records a SQL query.
func (r *SessionRecorder) RecordQuery(query string, metadata map[string]any) error {
	r.mu.Lock()
	r.queryCount++
	r.mu.Unlock()
	return r.record("query", query, metadata)
}

// RecordResult records query results.
func (r *SessionRecorder) RecordResult(result string, rowCount int) error {
	return r.record("result", result, map[string]any{"row_count": rowCount})
}

// RecordError records an error.
func (r *SessionRecorder) RecordError(errMsg string) error {
	return r.record("error", errMsg, nil)
}

// record records a single event.
func (r *SessionRecorder) record(eventType, data string, metadata map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("recorder is closed")
	}

	now := time.Now()
	event := RecordingEvent{
		Timestamp:  now,
		Type:       eventType,
		Data:       data,
		DurationMS: now.Sub(r.lastEvent).Milliseconds(),
		Metadata:   metadata,
	}

	r.lastEvent = now
	r.eventCount++

	return r.encoder.Encode(event)
}

// Close finalizes and closes the recording.
func (r *SessionRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	// Write footer
	footer := RecordingFooter{
		EndedAt:    time.Now(),
		Duration:   time.Since(r.header.StartedAt),
		EventCount: r.eventCount,
		QueryCount: r.queryCount,
	}
	if err := r.encoder.Encode(footer); err != nil {
		// Try to close files anyway
		_ = r.gzWriter.Close()
		_ = r.file.Close()
		return fmt.Errorf("failed to write recording footer: %w", err)
	}

	if err := r.gzWriter.Close(); err != nil {
		_ = r.file.Close()
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return r.file.Close()
}

// Path returns the recording file path.
func (r *SessionRecorder) Path() string {
	return r.path
}

// Stats returns the current recording statistics.
func (r *SessionRecorder) Stats() (eventCount, queryCount int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.eventCount, r.queryCount
}

// RecordingReader reads a session recording file.
type RecordingReader struct {
	file     *os.File
	gzReader *gzip.Reader
	decoder  *json.Decoder
	Header   *RecordingHeader
}

// OpenRecording opens a session recording file for reading.
func OpenRecording(path string) (*RecordingReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open recording file: %w", err)
	}

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	reader := &RecordingReader{
		file:     file,
		gzReader: gzReader,
		decoder:  json.NewDecoder(gzReader),
	}

	// Read header
	reader.Header = &RecordingHeader{}
	if err := reader.decoder.Decode(reader.Header); err != nil {
		_ = reader.Close()
		return nil, fmt.Errorf("failed to read recording header: %w", err)
	}

	return reader, nil
}

// NextEvent reads the next event from the recording.
// Returns io.EOF when there are no more events.
func (r *RecordingReader) NextEvent() (*RecordingEvent, error) {
	event := &RecordingEvent{}
	if err := r.decoder.Decode(event); err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to read event: %w", err)
	}

	// Check if this is actually the footer
	if event.Type == "" && event.Timestamp.IsZero() {
		return nil, io.EOF
	}

	return event, nil
}

// Close closes the recording reader.
func (r *RecordingReader) Close() error {
	if r.gzReader != nil {
		_ = r.gzReader.Close()
	}
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
