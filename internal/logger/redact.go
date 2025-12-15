package logger

import (
	"context"
	"log/slog"
	"strings"
)

// RedactingHandler wraps an slog.Handler to redact sensitive fields.
type RedactingHandler struct {
	handler      slog.Handler
	redactFields map[string]struct{}
}

// NewRedactingHandler creates a handler that redacts specified fields.
func NewRedactingHandler(handler slog.Handler, fields []string) *RedactingHandler {
	redactFields := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		redactFields[strings.ToLower(f)] = struct{}{}
	}
	return &RedactingHandler{
		handler:      handler,
		redactFields: redactFields,
	}
}

// Enabled implements slog.Handler.
func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Create a new record with redacted attributes
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.handler.Handle(ctx, newRecord)
}

// WithAttrs implements slog.Handler.
func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = h.redactAttr(a)
	}
	return &RedactingHandler{
		handler:      h.handler.WithAttrs(redacted),
		redactFields: h.redactFields,
	}
}

// WithGroup implements slog.Handler.
func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		handler:      h.handler.WithGroup(name),
		redactFields: h.redactFields,
	}
}

// redactAttr redacts an attribute if its key matches a redact field.
func (h *RedactingHandler) redactAttr(a slog.Attr) slog.Attr {
	key := strings.ToLower(a.Key)

	// Check if this key should be redacted
	if h.shouldRedact(key) {
		return slog.String(a.Key, "[REDACTED]")
	}

	// Recursively handle groups
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		redacted := make([]any, 0, len(attrs)*2)
		for _, ga := range attrs {
			ra := h.redactAttr(ga)
			redacted = append(redacted, ra)
		}
		return slog.Group(a.Key, redacted...)
	}

	return a
}

// shouldRedact checks if a key should be redacted.
func (h *RedactingHandler) shouldRedact(key string) bool {
	// Exact match
	if _, ok := h.redactFields[key]; ok {
		return true
	}

	// Check if key contains any redact field as substring
	for field := range h.redactFields {
		if strings.Contains(key, field) {
			return true
		}
	}

	return false
}

// RedactedValue is a placeholder for redacted values.
const RedactedValue = "[REDACTED]"
