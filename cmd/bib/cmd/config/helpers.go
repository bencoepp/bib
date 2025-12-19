package configcmd

import (
	"fmt"
	"strings"

	"bib/internal/config"
)

// getNestedValue retrieves a nested value from a map using dot notation
func getNestedValue(m map[string]any, key string) (any, error) {
	parts := strings.Split(key, ".")
	current := any(m)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key not found: %s", key)
			}
			current = val
		default:
			return nil, fmt.Errorf("cannot access key %s in non-object value", part)
		}
	}

	return current, nil
}

// setNestedValue sets a nested value in a map using dot notation
func setNestedValue(m map[string]any, key string, value string) error {
	parts := strings.Split(key, ".")
	current := m

	// Navigate to parent
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			// Create nested map
			current[part] = make(map[string]any)
			next = current[part]
		}
		nextMap, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot set nested key %s: parent is not an object", key)
		}
		current = nextMap
	}

	// Set the value (try to parse as appropriate type)
	lastKey := parts[len(parts)-1]
	current[lastKey] = parseValue(value)
	return nil
}

// parseValue attempts to parse a string value into an appropriate Go type
func parseValue(s string) any {
	// Try bool
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Try int
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
		return i
	}

	// Default to string
	return s
}

// validateBibConfig validates a BibConfig and returns any validation errors
func validateBibConfig(cfg *config.BibConfig) []string {
	var errors []string

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[cfg.Log.Level] {
		errors = append(errors, fmt.Sprintf("invalid log.level: %s (must be debug, info, warn, or error)", cfg.Log.Level))
	}

	// Validate output format
	validFormats := map[string]bool{"text": true, "json": true, "yaml": true, "table": true}
	if cfg.Output.Format != "" && !validFormats[cfg.Output.Format] {
		errors = append(errors, fmt.Sprintf("invalid output.format: %s", cfg.Output.Format))
	}

	return errors
}

// validateBibdConfig validates a BibdConfig and returns any validation errors
func validateBibdConfig(cfg *config.BibdConfig) []string {
	var errors []string

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[cfg.Log.Level] {
		errors = append(errors, fmt.Sprintf("invalid log.level: %s", cfg.Log.Level))
	}

	// Validate database backend
	validBackends := map[string]bool{"sqlite": true, "postgres": true}
	if !validBackends[cfg.Database.Backend] {
		errors = append(errors, fmt.Sprintf("invalid database.backend: %s", cfg.Database.Backend))
	}

	// Validate P2P mode
	validP2PModes := map[string]bool{"proxy": true, "selective": true, "full": true}
	if cfg.P2P.Enabled && !validP2PModes[cfg.P2P.Mode] {
		errors = append(errors, fmt.Sprintf("invalid p2p.mode: %s", cfg.P2P.Mode))
	}

	// Validate server port
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errors = append(errors, fmt.Sprintf("invalid server.port: %d", cfg.Server.Port))
	}

	return errors
}
