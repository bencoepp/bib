package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Redactor handles redaction of sensitive values from SQL queries.
type Redactor struct {
	// sensitiveFields are field/column names that should have their values redacted.
	sensitiveFields map[string]struct{}

	// sensitivePatterns are regex patterns for sensitive field names.
	sensitivePatterns []*regexp.Regexp

	// parameterPlaceholder is what to replace parameter values with.
	parameterPlaceholder string
}

// RedactorConfig holds configuration for the redactor.
type RedactorConfig struct {
	// SensitiveFields are field/column names to redact (case-insensitive).
	SensitiveFields []string

	// AdditionalPatterns are additional regex patterns for sensitive fields.
	AdditionalPatterns []string

	// ParameterPlaceholder is what to replace parameter values with.
	// Defaults to "[REDACTED]".
	ParameterPlaceholder string
}

// DefaultRedactorConfig returns the default redactor configuration.
func DefaultRedactorConfig() RedactorConfig {
	return RedactorConfig{
		SensitiveFields: []string{
			"password",
			"token",
			"key",
			"secret",
			"credential",
			"auth",
			"api_key",
			"apikey",
			"access_token",
			"refresh_token",
			"private_key",
			"encryption_key",
			"session",
			"cookie",
			"bearer",
		},
		AdditionalPatterns: []string{
			`(?i)_key$`,
			`(?i)_token$`,
			`(?i)_secret$`,
			`(?i)_password$`,
			`(?i)_credential$`,
		},
		ParameterPlaceholder: "[REDACTED]",
	}
}

// NewRedactor creates a new Redactor with the given configuration.
func NewRedactor(cfg RedactorConfig) *Redactor {
	fields := make(map[string]struct{}, len(cfg.SensitiveFields))
	for _, f := range cfg.SensitiveFields {
		fields[strings.ToLower(f)] = struct{}{}
	}

	var patterns []*regexp.Regexp
	for _, p := range cfg.AdditionalPatterns {
		if re, err := regexp.Compile(p); err == nil {
			patterns = append(patterns, re)
		}
	}

	placeholder := cfg.ParameterPlaceholder
	if placeholder == "" {
		placeholder = "[REDACTED]"
	}

	return &Redactor{
		sensitiveFields:      fields,
		sensitivePatterns:    patterns,
		parameterPlaceholder: placeholder,
	}
}

// IsSensitiveField checks if a field name is sensitive.
func (r *Redactor) IsSensitiveField(fieldName string) bool {
	lower := strings.ToLower(fieldName)

	// Check exact match
	if _, ok := r.sensitiveFields[lower]; ok {
		return true
	}

	// Check if field contains sensitive substring
	for field := range r.sensitiveFields {
		if strings.Contains(lower, field) {
			return true
		}
	}

	// Check patterns
	for _, pattern := range r.sensitivePatterns {
		if pattern.MatchString(fieldName) {
			return true
		}
	}

	return false
}

// RedactQuery redacts sensitive information from a SQL query.
// It handles both the query text and parameter values.
func (r *Redactor) RedactQuery(query string, args []any) (redactedQuery string, redactedArgs []any) {
	redactedQuery = r.redactQueryText(query)
	redactedArgs = r.redactArgs(query, args)
	return
}

// redactQueryText redacts inline sensitive values from the query text.
func (r *Redactor) redactQueryText(query string) string {
	// Remove potential inline credentials
	result := query

	// Redact string literals that might contain sensitive data after sensitive field assignments
	// e.g., password = 'secret123' -> password = '[REDACTED]'
	for field := range r.sensitiveFields {
		// Match field = 'value' or field='value' patterns
		pattern := regexp.MustCompile(
			`(?i)(\b` + regexp.QuoteMeta(field) + `\s*=\s*)('[^']*'|"[^"]*")`,
		)
		result = pattern.ReplaceAllString(result, "${1}'"+r.parameterPlaceholder+"'")
	}

	return result
}

// redactArgs redacts sensitive parameter values based on query analysis.
func (r *Redactor) redactArgs(query string, args []any) []any {
	if len(args) == 0 {
		return args
	}

	redacted := make([]any, len(args))
	copy(redacted, args)

	// Extract field names from query to determine which args are sensitive
	sensitivePositions := r.findSensitivePositions(query)

	for pos := range sensitivePositions {
		if pos >= 0 && pos < len(redacted) {
			redacted[pos] = r.parameterPlaceholder
		}
	}

	return redacted
}

// findSensitivePositions finds parameter positions that correspond to sensitive fields.
func (r *Redactor) findSensitivePositions(query string) map[int]struct{} {
	positions := make(map[int]struct{})

	// Handle PostgreSQL $N style parameters in WHERE/SET clauses
	pgPattern := regexp.MustCompile(`(?i)(\w+)\s*=\s*\$(\d+)`)
	matches := pgPattern.FindAllStringSubmatch(query, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			fieldName := match[1]
			if r.IsSensitiveField(fieldName) {
				var pos int
				if _, err := parsePositiveInt(match[2], &pos); err == nil {
					positions[pos-1] = struct{}{} // $1 is index 0
				}
			}
		}
	}

	// Handle INSERT statements: INSERT INTO table (col1, col2, ...) VALUES ($1, $2, ...)
	insertPattern := regexp.MustCompile(`(?i)INSERT\s+INTO\s+\w+\s*\(([^)]+)\)\s*VALUES\s*\(([^)]+)\)`)
	insertMatch := insertPattern.FindStringSubmatch(query)
	if len(insertMatch) >= 3 {
		columns := parseColumnList(insertMatch[1])
		values := parseValueList(insertMatch[2])

		for i, col := range columns {
			if r.IsSensitiveField(col) && i < len(values) {
				// Check if value is a parameter
				if paramPos := extractParamPosition(values[i]); paramPos >= 0 {
					positions[paramPos] = struct{}{}
				}
			}
		}
	}

	// Handle SQLite/MySQL ? style parameters
	// This is trickier - we need to count ? occurrences and match with field names
	if !strings.Contains(query, "$") && strings.Contains(query, "?") {
		r.findQuestionMarkPositions(query, positions)
	}

	return positions
}

// parseColumnList parses a comma-separated column list.
func parseColumnList(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		col := strings.TrimSpace(p)
		col = strings.Trim(col, `"'`)
		if col != "" {
			result = append(result, col)
		}
	}
	return result
}

// parseValueList parses a comma-separated value list.
func parseValueList(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		val := strings.TrimSpace(p)
		if val != "" {
			result = append(result, val)
		}
	}
	return result
}

// extractParamPosition extracts the parameter position from a parameter placeholder.
// Returns -1 if not a parameter.
func extractParamPosition(s string) int {
	s = strings.TrimSpace(s)
	// PostgreSQL $N style
	if strings.HasPrefix(s, "$") {
		var pos int
		if _, err := parsePositiveInt(s[1:], &pos); err == nil {
			return pos - 1 // $1 is index 0
		}
	}
	return -1
}

// findQuestionMarkPositions finds positions of ? parameters that are sensitive.
func (r *Redactor) findQuestionMarkPositions(query string, positions map[int]struct{}) {
	// Parse query to find field = ? patterns
	pattern := regexp.MustCompile(`(?i)(\w+)\s*=\s*\?`)
	matches := pattern.FindAllStringSubmatchIndex(query, -1)

	if len(matches) == 0 {
		return
	}

	// Count ? occurrences to determine position
	pos := 0
	lastEnd := 0
	for _, match := range matches {
		// Count ? marks before this match
		beforeMatch := query[lastEnd:match[0]]
		pos += strings.Count(beforeMatch, "?")

		fieldNameStart := match[2]
		fieldNameEnd := match[3]
		fieldName := query[fieldNameStart:fieldNameEnd]

		if r.IsSensitiveField(fieldName) {
			positions[pos] = struct{}{}
		}
		pos++
		lastEnd = match[1]
	}
}

// parsePositiveInt is a helper to parse a positive integer.
func parsePositiveInt(s string, result *int) (bool, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
		n = n*10 + int(c-'0')
	}
	*result = n
	return true, nil
}

// HashQuery creates a normalized hash of a query for grouping.
// It removes literal values and normalizes whitespace.
func (r *Redactor) HashQuery(query string) string {
	// Normalize whitespace
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	normalized = strings.TrimSpace(normalized)
	normalized = strings.ToUpper(normalized)

	// Replace string literals with placeholder
	normalized = regexp.MustCompile(`'[^']*'`).ReplaceAllString(normalized, "'?'")
	normalized = regexp.MustCompile(`"[^"]*"`).ReplaceAllString(normalized, `"?"`)

	// Replace numeric literals
	normalized = regexp.MustCompile(`\b\d+\b`).ReplaceAllString(normalized, "?")

	// Replace PostgreSQL parameters
	normalized = regexp.MustCompile(`\$\d+`).ReplaceAllString(normalized, "$?")

	// Hash the normalized query
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter hash
}

// RedactMetadata redacts sensitive values from metadata.
func (r *Redactor) RedactMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}

	redacted := make(map[string]any, len(metadata))
	for k, v := range metadata {
		if r.IsSensitiveField(k) {
			redacted[k] = r.parameterPlaceholder
		} else if nested, ok := v.(map[string]any); ok {
			redacted[k] = r.RedactMetadata(nested)
		} else {
			redacted[k] = v
		}
	}
	return redacted
}
