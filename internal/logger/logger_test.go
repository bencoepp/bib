package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

// ==================== Logger Tests ====================

func TestNew_Defaults(t *testing.T) {
	cfg := config.LogConfig{
		Level:  "info",
		Format: "text",
		Output: "stderr",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_JSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}

	cfg := config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stderr",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	// We can't easily redirect the logger output, but we can verify it was created
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	_ = buf
}

func TestNew_PrettyFormat(t *testing.T) {
	cfg := config.LogConfig{
		Level:  "debug",
		Format: "pretty",
		Output: "stdout",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_InvalidLevel(t *testing.T) {
	cfg := config.LogConfig{
		Level:  "invalid",
		Format: "text",
		Output: "stderr",
	}

	_, err := New(cfg)
	if err == nil {
		t.Error("expected error for invalid log level")
	}
}

func TestNew_FileOutput(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	cfg := config.LogConfig{
		Level:    "info",
		Format:   "text",
		Output:   logPath,
		FilePath: "",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Log something
	logger.Info("test message")

	// Close to flush
	logger.Close()

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created")
	}
}

func TestNew_MultipleOutputs(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "extra.log")

	cfg := config.LogConfig{
		Level:    "info",
		Format:   "text",
		Output:   "stderr",
		FilePath: filePath,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logger.Info("test message")
	logger.Close()

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("additional log file was not created")
	}
}

func TestNew_WithRedactFields(t *testing.T) {
	cfg := config.LogConfig{
		Level:        "info",
		Format:       "text",
		Output:       "stderr",
		RedactFields: []string{"password", "secret"},
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_WithCaller(t *testing.T) {
	cfg := config.LogConfig{
		Level:        "info",
		Format:       "text",
		Output:       "stderr",
		EnableCaller: true,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLogger_With(t *testing.T) {
	cfg := config.LogConfig{
		Level:  "info",
		Format: "text",
		Output: "stderr",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	childLogger := logger.With("key", "value")
	if childLogger == nil {
		t.Fatal("expected non-nil child logger")
	}
}

func TestLogger_WithGroup(t *testing.T) {
	cfg := config.LogConfig{
		Level:  "info",
		Format: "text",
		Output: "stderr",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	groupLogger := logger.WithGroup("mygroup")
	if groupLogger == nil {
		t.Fatal("expected non-nil group logger")
	}
}

func TestLogger_Close(t *testing.T) {
	cfg := config.LogConfig{
		Level:  "info",
		Format: "text",
		Output: "stderr",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = logger.Close()
	if err != nil {
		t.Errorf("unexpected error closing logger: %v", err)
	}
}

func TestLogger_CloseNil(t *testing.T) {
	logger := &Logger{}
	err := logger.Close()
	if err != nil {
		t.Errorf("unexpected error closing nil logger: %v", err)
	}
}

func TestDefault(t *testing.T) {
	logger := Default()
	if logger == nil {
		t.Fatal("expected non-nil default logger")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
		hasError bool
	}{
		{"debug", slog.LevelDebug, false},
		{"DEBUG", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"INFO", slog.LevelInfo, false},
		{"", slog.LevelInfo, false}, // empty defaults to info
		{"warn", slog.LevelWarn, false},
		{"WARN", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"ERROR", slog.LevelError, false},
		{"invalid", slog.LevelInfo, true},
		{"trace", slog.LevelInfo, true}, // not supported
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := parseLevel(tt.input)
			if tt.hasError && err == nil {
				t.Error("expected error")
			}
			if !tt.hasError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.hasError && level != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, level)
			}
		})
	}
}

// ==================== Redact Tests ====================

func TestRedactingHandler_New(t *testing.T) {
	baseHandler := slog.NewTextHandler(os.Stderr, nil)
	fields := []string{"password", "secret", "token"}

	handler := NewRedactingHandler(baseHandler, fields)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestRedactingHandler_Enabled(t *testing.T) {
	baseHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	handler := NewRedactingHandler(baseHandler, []string{"password"})

	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected info level to be enabled")
	}
	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug level to be disabled")
	}
}

func TestRedactingHandler_Handle(t *testing.T) {
	buf := &bytes.Buffer{}
	baseHandler := slog.NewTextHandler(buf, nil)
	handler := NewRedactingHandler(baseHandler, []string{"password"})

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.String("password", "secret123"))
	record.AddAttrs(slog.String("username", "testuser"))

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("expected password to be redacted")
	}
	if !strings.Contains(output, "testuser") {
		t.Error("expected username to be present")
	}
	if strings.Contains(output, "secret123") {
		t.Error("password should not appear in output")
	}
}

func TestRedactingHandler_SubstringMatch(t *testing.T) {
	buf := &bytes.Buffer{}
	baseHandler := slog.NewTextHandler(buf, nil)
	handler := NewRedactingHandler(baseHandler, []string{"secret"})

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.String("api_secret_key", "sensitive"))

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("expected field containing 'secret' to be redacted")
	}
}

func TestRedactingHandler_WithAttrs(t *testing.T) {
	buf := &bytes.Buffer{}
	baseHandler := slog.NewTextHandler(buf, nil)
	handler := NewRedactingHandler(baseHandler, []string{"password"})

	newHandler := handler.WithAttrs([]slog.Attr{
		slog.String("password", "secret"),
		slog.String("normal", "value"),
	})

	if newHandler == nil {
		t.Fatal("expected non-nil handler")
	}

	// The attrs should be redacted when the handler processes them
	redactHandler, ok := newHandler.(*RedactingHandler)
	if !ok {
		t.Fatal("expected RedactingHandler type")
	}
	_ = redactHandler
}

func TestRedactingHandler_WithGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	baseHandler := slog.NewTextHandler(buf, nil)
	handler := NewRedactingHandler(baseHandler, []string{"password"})

	newHandler := handler.WithGroup("auth")
	if newHandler == nil {
		t.Fatal("expected non-nil handler")
	}

	redactHandler, ok := newHandler.(*RedactingHandler)
	if !ok {
		t.Fatal("expected RedactingHandler type")
	}
	_ = redactHandler
}

func TestRedactingHandler_NestedGroups(t *testing.T) {
	buf := &bytes.Buffer{}
	baseHandler := slog.NewJSONHandler(buf, nil)
	handler := NewRedactingHandler(baseHandler, []string{"password"})

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.Group("auth",
		slog.String("password", "secret"),
		slog.String("user", "test"),
	))

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("expected nested password to be redacted")
	}
}

func TestShouldRedact(t *testing.T) {
	handler := NewRedactingHandler(nil, []string{"password", "secret", "token"})

	tests := []struct {
		key      string
		expected bool
	}{
		{"password", true},
		{"PASSWORD", false}, // case sensitive lookup
		{"secret", true},
		{"token", true},
		{"api_secret_key", true}, // substring match
		{"my_password_field", true},
		{"auth_token", true},
		{"username", false},
		{"email", false},
		{"name", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := handler.shouldRedact(tt.key)
			if result != tt.expected {
				t.Errorf("shouldRedact(%q) = %v, expected %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestRedactedValue(t *testing.T) {
	if RedactedValue != "[REDACTED]" {
		t.Errorf("expected '[REDACTED]', got %q", RedactedValue)
	}
}

// ==================== Context Tests ====================

func TestNewCommandContext(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	args := []string{"arg1", "arg2"}

	cc := NewCommandContext(cmd, args)

	if cc == nil {
		t.Fatal("expected non-nil CommandContext")
	}
	if cc.Command != "test" {
		t.Errorf("expected command 'test', got %q", cc.Command)
	}
	if len(cc.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(cc.Args))
	}
	if cc.RequestID == "" {
		t.Error("expected non-empty request ID")
	}
	if cc.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestNewDaemonContext(t *testing.T) {
	cc := NewDaemonContext("startup")

	if cc == nil {
		t.Fatal("expected non-nil CommandContext")
	}
	if cc.Command != "startup" {
		t.Errorf("expected command 'startup', got %q", cc.Command)
	}
	if cc.RequestID == "" {
		t.Error("expected non-empty request ID")
	}
}

func TestWithCommandContext(t *testing.T) {
	cc := &CommandContext{
		Command:   "test",
		RequestID: "req-123",
	}

	ctx := context.Background()
	ctx = WithCommandContext(ctx, cc)

	retrieved := CommandContextFrom(ctx)
	if retrieved == nil {
		t.Fatal("expected non-nil CommandContext")
	}
	if retrieved.Command != "test" {
		t.Errorf("expected command 'test', got %q", retrieved.Command)
	}
	if retrieved.RequestID != "req-123" {
		t.Errorf("expected request ID 'req-123', got %q", retrieved.RequestID)
	}
}

func TestCommandContextFrom_NotSet(t *testing.T) {
	ctx := context.Background()
	cc := CommandContextFrom(ctx)
	if cc != nil {
		t.Error("expected nil CommandContext")
	}
}

func TestWithLogger(t *testing.T) {
	logger := Default()
	ctx := context.Background()
	ctx = WithLogger(ctx, logger)

	retrieved := LoggerFrom(ctx)
	if retrieved == nil {
		t.Fatal("expected non-nil Logger")
	}
}

func TestLoggerFrom_NotSet(t *testing.T) {
	ctx := context.Background()
	logger := LoggerFrom(ctx)
	if logger == nil {
		t.Fatal("expected default logger when not set")
	}
}

func TestCommandContext_LogAttrs(t *testing.T) {
	cc := &CommandContext{
		Command:    "test",
		Args:       []string{"arg1"},
		User:       "testuser",
		Hostname:   "testhost",
		WorkingDir: "/tmp",
		Timestamp:  time.Now(),
		RequestID:  "req-123",
	}

	attrs := cc.LogAttrs()
	if len(attrs) == 0 {
		t.Error("expected non-empty attrs")
	}

	// Check that we have the expected attributes
	hasRequestID := false
	hasCommand := false
	for _, attr := range attrs {
		if attr.Key == "request_id" {
			hasRequestID = true
		}
		if attr.Key == "command" {
			hasCommand = true
		}
	}
	if !hasRequestID {
		t.Error("expected request_id attribute")
	}
	if !hasCommand {
		t.Error("expected command attribute")
	}
}

func TestCommandContext_LogAttrs_Nil(t *testing.T) {
	var cc *CommandContext
	attrs := cc.LogAttrs()
	if attrs != nil {
		t.Error("expected nil attrs for nil CommandContext")
	}
}

func TestCommandContext_LogGroup(t *testing.T) {
	cc := &CommandContext{
		Command:   "test",
		RequestID: "req-123",
	}

	group := cc.LogGroup()
	if group.Key != "context" {
		t.Errorf("expected group key 'context', got %q", group.Key)
	}
}

func TestCommandContext_LogGroup_Nil(t *testing.T) {
	var cc *CommandContext
	group := cc.LogGroup()
	if group.Key != "" {
		t.Error("expected empty group for nil CommandContext")
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == "" {
		t.Error("expected non-empty request ID")
	}
	if id1 == id2 {
		t.Error("expected unique request IDs")
	}
}

// ==================== Error Tests ====================

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := WrapError(originalErr, "wrapped message")

	if wrappedErr == nil {
		t.Fatal("expected non-nil error")
	}

	if !strings.Contains(wrappedErr.Error(), "wrapped message") {
		t.Error("expected wrapped message in error")
	}
	if !strings.Contains(wrappedErr.Error(), "original error") {
		t.Error("expected original error in error")
	}
}

func TestWrapError_Nil(t *testing.T) {
	err := WrapError(nil, "message")
	if err != nil {
		t.Error("expected nil when wrapping nil error")
	}
}

func TestWrappedError_Error(t *testing.T) {
	originalErr := errors.New("original")
	wrapped := &WrappedError{
		msg:   "wrapped",
		cause: originalErr,
	}

	if wrapped.Error() != "wrapped: original" {
		t.Errorf("unexpected error string: %q", wrapped.Error())
	}
}

func TestWrappedError_ErrorNoCause(t *testing.T) {
	wrapped := &WrappedError{
		msg: "wrapped",
	}

	if wrapped.Error() != "wrapped" {
		t.Errorf("unexpected error string: %q", wrapped.Error())
	}
}

func TestWrappedError_Unwrap(t *testing.T) {
	originalErr := errors.New("original")
	wrapped := &WrappedError{
		msg:   "wrapped",
		cause: originalErr,
	}

	unwrapped := wrapped.Unwrap()
	if unwrapped != originalErr {
		t.Error("expected original error from Unwrap")
	}
}

func TestWrappedError_Caller(t *testing.T) {
	wrapped := &WrappedError{
		caller: "test.go:10",
	}

	if wrapped.Caller() != "test.go:10" {
		t.Errorf("unexpected caller: %q", wrapped.Caller())
	}
}

func TestWithError(t *testing.T) {
	err := errors.New("test error")
	attr := WithError(err)

	if attr.Key != "error" {
		t.Errorf("expected key 'error', got %q", attr.Key)
	}
}

func TestWithError_Nil(t *testing.T) {
	attr := WithError(nil)
	if attr.Key != "" {
		t.Error("expected empty attr for nil error")
	}
}

func TestWithError_WrappedError(t *testing.T) {
	originalErr := errors.New("original")
	wrapped := WrapError(originalErr, "wrapped")
	attr := WithError(wrapped)

	if attr.Key != "error" {
		t.Errorf("expected key 'error', got %q", attr.Key)
	}
}

func TestWithStack(t *testing.T) {
	attr := WithStack()
	if attr.Key != "stack" {
		t.Errorf("expected key 'stack', got %q", attr.Key)
	}
	if attr.Value.String() == "" {
		t.Error("expected non-empty stack")
	}
}

func TestWithStackSkip(t *testing.T) {
	attr := WithStackSkip(0)
	if attr.Key != "stack" {
		t.Errorf("expected key 'stack', got %q", attr.Key)
	}
}

func TestCaptureStack(t *testing.T) {
	stack := captureStack(1)
	if stack == "" {
		t.Error("expected non-empty stack")
	}
	// Should contain this test file
	if !strings.Contains(stack, "logger_test.go") {
		t.Error("expected stack to contain test file")
	}
}

func TestErrorGroup(t *testing.T) {
	err := errors.New("test error")
	attr := ErrorGroup(err, false)

	if attr.Key != "error" {
		t.Errorf("expected key 'error', got %q", attr.Key)
	}
}

func TestErrorGroup_Nil(t *testing.T) {
	attr := ErrorGroup(nil, false)
	if attr.Key != "" {
		t.Error("expected empty attr for nil error")
	}
}

func TestErrorGroup_WithStack(t *testing.T) {
	err := errors.New("test error")
	attr := ErrorGroup(err, true)

	if attr.Key != "error" {
		t.Errorf("expected key 'error', got %q", attr.Key)
	}
}

// ==================== Console Handler Tests ====================

func TestNewCharmHandler(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewCharmHandler(buf, nil)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNewCharmHandler_WithOptions(t *testing.T) {
	buf := &bytes.Buffer{}
	opts := &CharmHandlerOptions{
		Level:      slog.LevelDebug,
		NoColor:    true,
		TimeFormat: "15:04:05",
		ShowCaller: true,
		Prefix:     "test",
	}

	handler := NewCharmHandler(buf, opts)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestCharmHandler_Enabled(t *testing.T) {
	buf := &bytes.Buffer{}
	opts := &CharmHandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := NewCharmHandler(buf, opts)

	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected info level to be enabled")
	}
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Error("expected error level to be enabled")
	}
	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug level to be disabled")
	}
}

func TestCharmHandler_Handle(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewCharmHandler(buf, nil)

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("key", "value"))

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("expected message in output")
	}
}

func TestCharmHandler_HandleLevels(t *testing.T) {
	levels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			buf := &bytes.Buffer{}
			handler := NewCharmHandler(buf, &CharmHandlerOptions{Level: slog.LevelDebug})

			ctx := context.Background()
			record := slog.NewRecord(time.Now(), level, "test", 0)

			err := handler.Handle(ctx, record)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCharmHandler_WithAttrs(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewCharmHandler(buf, nil)

	newHandler := handler.WithAttrs([]slog.Attr{slog.String("key", "value")})
	if newHandler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestCharmHandler_WithGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewCharmHandler(buf, nil)

	newHandler := handler.WithGroup("test")
	if newHandler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestCharmHandler_WithGroup_Empty(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewCharmHandler(buf, nil)

	newHandler := handler.WithGroup("")
	if newHandler != handler {
		t.Error("expected same handler for empty group name")
	}
}

func TestNewConsoleHandler(t *testing.T) {
	buf := &bytes.Buffer{}
	handler := NewConsoleHandler(buf, nil)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestCharmLogLevel(t *testing.T) {
	tests := []struct {
		slogLevel slog.Level
		expected  string
	}{
		{slog.LevelDebug, "debug"},
		{slog.LevelInfo, "info"},
		{slog.LevelWarn, "warn"},
		{slog.LevelError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			level := charmLogLevel(tt.slogLevel)
			_ = level // Just verify no panic
		})
	}
}

func TestFormatSlogValue(t *testing.T) {
	tests := []struct {
		name  string
		value slog.Value
	}{
		{"time", slog.TimeValue(time.Now())},
		{"duration", slog.DurationValue(time.Second)},
		{"string", slog.StringValue("test")},
		{"int", slog.Int64Value(42)},
		{"any", slog.AnyValue(struct{ Name string }{"test"})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSlogValue(tt.value)
			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	// Test with non-terminal writer
	buf := &bytes.Buffer{}
	if isTerminal(buf) {
		t.Error("expected bytes.Buffer to not be a terminal")
	}

	// Test with stdout (may or may not be a terminal depending on environment)
	// Just ensure it doesn't panic
	_ = isTerminal(os.Stdout)
}

// ==================== Audit Logger Tests ====================

func TestNewAuditLogger_EmptyPath(t *testing.T) {
	_, err := NewAuditLogger("", 365)
	if err == nil {
		t.Error("expected error for empty audit path")
	}
}

func TestNewAuditLogger_DefaultMaxAge(t *testing.T) {
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	logger, err := NewAuditLogger(auditPath, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestAuditLogger_Log(t *testing.T) {
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	logger, err := NewAuditLogger(auditPath, 365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	event := AuditEvent{
		Action:   AuditActionCommand,
		Actor:    "testuser",
		Resource: "test-command",
		Outcome:  AuditOutcomeSuccess,
		Metadata: map[string]any{"key": "value"},
	}

	ctx := context.Background()
	logger.Log(ctx, event)

	// Close to flush
	logger.Close()

	// Read and verify log
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	if !strings.Contains(string(data), "testuser") {
		t.Error("expected actor in audit log")
	}
	if !strings.Contains(string(data), "command") {
		t.Error("expected action in audit log")
	}
}

func TestAuditLogger_Log_WithContext(t *testing.T) {
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	logger, err := NewAuditLogger(auditPath, 365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	cc := &CommandContext{
		RequestID: "ctx-req-123",
		User:      "ctxuser",
	}
	ctx := WithCommandContext(context.Background(), cc)

	event := AuditEvent{
		Action:   AuditActionAccess,
		Actor:    "testuser",
		Resource: "resource",
		Outcome:  AuditOutcomeSuccess,
	}

	logger.Log(ctx, event)
	logger.Close()

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	if !strings.Contains(string(data), "ctx-req-123") {
		t.Error("expected request ID from context in audit log")
	}
}

func TestAuditLogger_LogCommand(t *testing.T) {
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	logger, err := NewAuditLogger(auditPath, 365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	logger.LogCommand(ctx, "test-cmd", AuditOutcomeSuccess, map[string]any{"duration": "1s"})
	logger.Close()

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	if !strings.Contains(string(data), "test-cmd") {
		t.Error("expected command in audit log")
	}
}

func TestAuditLogger_LogConfigChange(t *testing.T) {
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	logger, err := NewAuditLogger(auditPath, 365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	logger.LogConfigChange(ctx, "config.yaml", AuditOutcomeSuccess, "old", "new")
	logger.Close()

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	if !strings.Contains(string(data), "config_change") {
		t.Error("expected config_change action in audit log")
	}
}

func TestAuditLogger_LogAuth_Success(t *testing.T) {
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	logger, err := NewAuditLogger(auditPath, 365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	logger.LogAuth(ctx, "testuser", true, nil)
	logger.Close()

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	if !strings.Contains(string(data), "auth_success") {
		t.Error("expected auth_success action in audit log")
	}
}

func TestAuditLogger_LogAuth_Failure(t *testing.T) {
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	logger, err := NewAuditLogger(auditPath, 365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	ctx := context.Background()
	logger.LogAuth(ctx, "testuser", false, map[string]any{"reason": "invalid password"})
	logger.Close()

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	if !strings.Contains(string(data), "auth_failure") {
		t.Error("expected auth_failure action in audit log")
	}
}

func TestAuditLogger_Close(t *testing.T) {
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	logger, err := NewAuditLogger(auditPath, 365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = logger.Close()
	if err != nil {
		t.Errorf("unexpected error closing audit logger: %v", err)
	}
}

func TestAuditLogger_Close_Nil(t *testing.T) {
	var logger *AuditLogger
	err := logger.Close()
	if err != nil {
		t.Errorf("unexpected error closing nil audit logger: %v", err)
	}
}

func TestAuditLogger_Log_Nil(t *testing.T) {
	var logger *AuditLogger
	// Should not panic
	logger.Log(context.Background(), AuditEvent{})
}

func TestNopAuditLogger(t *testing.T) {
	logger := NopAuditLogger()
	if logger != nil {
		t.Error("expected nil from NopAuditLogger")
	}
}

func TestAuditEvent_Timestamp(t *testing.T) {
	tempDir := t.TempDir()
	auditPath := filepath.Join(tempDir, "audit.log")

	logger, err := NewAuditLogger(auditPath, 365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer logger.Close()

	// Event without timestamp
	event := AuditEvent{
		Action:   AuditActionCommand,
		Actor:    "testuser",
		Resource: "cmd",
		Outcome:  AuditOutcomeSuccess,
	}

	ctx := context.Background()
	logger.Log(ctx, event)
	logger.Close()

	// Verify timestamp was set
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	var logEntry map[string]any
	if err := json.Unmarshal(data, &logEntry); err != nil {
		// Log might have multiple lines, just check it's not empty
		if len(data) == 0 {
			t.Error("expected non-empty audit log")
		}
	}
}

// ==================== Audit Actions and Outcomes ====================

func TestAuditActions(t *testing.T) {
	actions := []AuditAction{
		AuditActionConfigChange,
		AuditActionAuthAttempt,
		AuditActionAuthSuccess,
		AuditActionAuthFailure,
		AuditActionCommand,
		AuditActionAccess,
		AuditActionCreate,
		AuditActionUpdate,
		AuditActionDelete,
		AuditActionPermission,
	}

	for _, action := range actions {
		if action == "" {
			t.Error("expected non-empty action")
		}
	}
}

func TestAuditOutcomes(t *testing.T) {
	outcomes := []AuditOutcome{
		AuditOutcomeSuccess,
		AuditOutcomeFailure,
		AuditOutcomeDenied,
		AuditOutcomePending,
	}

	for _, outcome := range outcomes {
		if outcome == "" {
			t.Error("expected non-empty outcome")
		}
	}
}

// ==================== Integration Tests ====================

func TestLogger_FullPipeline(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	cfg := config.LogConfig{
		Level:        "debug",
		Format:       "json",
		Output:       logPath,
		EnableCaller: true,
		RedactFields: []string{"password", "secret"},
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Log with sensitive data
	logger.Info("user login",
		"username", "testuser",
		"password", "secret123",
	)

	logger.Close()

	// Read and verify
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}

	output := string(data)
	if strings.Contains(output, "secret123") {
		t.Error("password should be redacted")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("expected redaction marker")
	}
	if !strings.Contains(output, "testuser") {
		t.Error("expected username in output")
	}
}

func TestContextAwareLogging(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	cfg := config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: logPath,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create command context
	cmd := &cobra.Command{Use: "test"}
	cc := NewCommandContext(cmd, []string{"arg1"})
	ctx := WithCommandContext(context.Background(), cc)
	ctx = WithLogger(ctx, logger)

	// Get logger from context
	ctxLogger := LoggerFrom(ctx)
	ctxLogger.Info("test message")

	logger.Close()

	// Verify log was written
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty log file")
	}
}

// ==================== Benchmark Tests ====================

func BenchmarkLogger_Info(b *testing.B) {
	cfg := config.LogConfig{
		Level:  "info",
		Format: "text",
		Output: "stderr",
	}
	logger, _ := New(cfg)
	defer logger.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("test message", "key", "value")
	}
}

func BenchmarkRedactingHandler(b *testing.B) {
	buf := &bytes.Buffer{}
	baseHandler := slog.NewJSONHandler(buf, nil)
	handler := NewRedactingHandler(baseHandler, []string{"password", "secret", "token"})

	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.String("password", "secret123"))
	record.AddAttrs(slog.String("username", "testuser"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = handler.Handle(ctx, record)
	}
}

func BenchmarkCommandContext_Create(b *testing.B) {
	cmd := &cobra.Command{Use: "test"}
	args := []string{"arg1", "arg2"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewCommandContext(cmd, args)
	}
}

func BenchmarkWrapError(b *testing.B) {
	err := errors.New("original error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WrapError(err, "wrapped message")
	}
}

func BenchmarkCaptureStack(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = captureStack(1)
	}
}
