// Package i18n provides internationalization support for the bib TUI.
//
// The package supports:
//   - Loading translations from YAML files
//   - Pluralization rules
//   - Template interpolation with variables
//   - Fallback to default locale
//   - Lazy loading of locale files
package i18n

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Default locale
const DefaultLocale = "en"

// I18n is the main internationalization engine.
type I18n struct {
	mu            sync.RWMutex
	locale        string
	fallback      string
	translations  map[string]map[string]any // locale -> flattened key -> value
	localesDir    string
	embeddedFS    *embed.FS
	embeddedRoot  string
	loadedLocales map[string]bool
}

// Option configures the I18n instance.
type Option func(*I18n)

// WithLocale sets the active locale.
func WithLocale(locale string) Option {
	return func(i *I18n) {
		i.locale = locale
	}
}

// WithFallback sets the fallback locale.
func WithFallback(locale string) Option {
	return func(i *I18n) {
		i.fallback = locale
	}
}

// WithDirectory sets the directory to load locale files from.
func WithDirectory(dir string) Option {
	return func(i *I18n) {
		i.localesDir = dir
	}
}

// WithEmbeddedFS sets an embedded filesystem for locale files.
func WithEmbeddedFS(fs *embed.FS, root string) Option {
	return func(i *I18n) {
		i.embeddedFS = fs
		i.embeddedRoot = root
	}
}

// New creates a new I18n instance.
func New(opts ...Option) *I18n {
	i := &I18n{
		locale:        DefaultLocale,
		fallback:      DefaultLocale,
		translations:  make(map[string]map[string]any),
		loadedLocales: make(map[string]bool),
	}

	for _, opt := range opts {
		opt(i)
	}

	// Load fallback locale
	_ = i.loadLocale(i.fallback)

	// Load active locale if different
	if i.locale != i.fallback {
		_ = i.loadLocale(i.locale)
	}

	return i
}

// SetLocale changes the active locale.
func (i *I18n) SetLocale(locale string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if err := i.loadLocale(locale); err != nil {
		return err
	}
	i.locale = locale
	return nil
}

// Locale returns the current locale.
func (i *I18n) Locale() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.locale
}

// T translates a key with optional arguments.
// Arguments can be a map[string]any or pairs of key, value.
func (i *I18n) T(key string, args ...any) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// Try active locale
	if val := i.lookup(i.locale, key); val != "" {
		return i.interpolate(val, args...)
	}

	// Try fallback
	if i.locale != i.fallback {
		if val := i.lookup(i.fallback, key); val != "" {
			return i.interpolate(val, args...)
		}
	}

	// Return key as fallback
	return key
}

// TPlural translates a key with pluralization.
// count determines which plural form to use.
func (i *I18n) TPlural(key string, count int, args ...any) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	// Build plural key
	pluralKey := i.getPluralKey(key, count)

	// Add count to args
	argsMap := i.argsToMap(args...)
	argsMap["count"] = count

	// Try active locale
	if val := i.lookup(i.locale, pluralKey); val != "" {
		return i.interpolateMap(val, argsMap)
	}

	// Try singular form
	if val := i.lookup(i.locale, key); val != "" {
		return i.interpolateMap(val, argsMap)
	}

	// Try fallback
	if i.locale != i.fallback {
		if val := i.lookup(i.fallback, pluralKey); val != "" {
			return i.interpolateMap(val, argsMap)
		}
		if val := i.lookup(i.fallback, key); val != "" {
			return i.interpolateMap(val, argsMap)
		}
	}

	return key
}

// Has checks if a translation key exists.
func (i *I18n) Has(key string) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.lookup(i.locale, key) != "" {
		return true
	}
	if i.locale != i.fallback {
		return i.lookup(i.fallback, key) != ""
	}
	return false
}

// loadLocale loads a locale file.
func (i *I18n) loadLocale(locale string) error {
	if i.loadedLocales[locale] {
		return nil
	}

	var data []byte
	var err error

	// Try embedded FS first
	if i.embeddedFS != nil {
		path := filepath.Join(i.embeddedRoot, locale+".yaml")
		data, err = i.embeddedFS.ReadFile(path)
		if err != nil {
			// Try .yml extension
			path = filepath.Join(i.embeddedRoot, locale+".yml")
			data, err = i.embeddedFS.ReadFile(path)
		}
	}

	// Try filesystem
	if err != nil && i.localesDir != "" {
		path := filepath.Join(i.localesDir, locale+".yaml")
		data, err = os.ReadFile(path)
		if err != nil {
			path = filepath.Join(i.localesDir, locale+".yml")
			data, err = os.ReadFile(path)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to load locale %s: %w", locale, err)
	}

	// Parse YAML
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse locale %s: %w", locale, err)
	}

	// Flatten into dot notation
	i.translations[locale] = make(map[string]any)
	i.flattenMap("", raw, i.translations[locale])
	i.loadedLocales[locale] = true

	return nil
}

// flattenMap flattens a nested map into dot-notation keys.
func (i *I18n) flattenMap(prefix string, src map[string]any, dst map[string]any) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]any:
			i.flattenMap(key, val, dst)
		default:
			dst[key] = val
		}
	}
}

// lookup finds a translation value.
func (i *I18n) lookup(locale, key string) string {
	translations, ok := i.translations[locale]
	if !ok {
		return ""
	}

	val, ok := translations[key]
	if !ok {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// interpolate replaces template variables in a string.
func (i *I18n) interpolate(s string, args ...any) string {
	if len(args) == 0 {
		return s
	}

	data := i.argsToMap(args...)
	return i.interpolateMap(s, data)
}

// interpolateMap replaces template variables using a map.
func (i *I18n) interpolateMap(s string, data map[string]any) string {
	if len(data) == 0 {
		return s
	}

	// Check if string contains template syntax
	if !strings.Contains(s, "{{") {
		return s
	}

	tmpl, err := template.New("i18n").Parse(s)
	if err != nil {
		return s
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return s
	}

	return buf.String()
}

// argsToMap converts args to a map.
func (i *I18n) argsToMap(args ...any) map[string]any {
	result := make(map[string]any)

	if len(args) == 0 {
		return result
	}

	// Check if first arg is already a map
	if m, ok := args[0].(map[string]any); ok {
		return m
	}

	// Treat as key-value pairs
	for j := 0; j+1 < len(args); j += 2 {
		if key, ok := args[j].(string); ok {
			result[key] = args[j+1]
		}
	}

	return result
}

// getPluralKey returns the pluralized key for a count.
// Uses simple English rules: count == 1 ? "one" : "other"
func (i *I18n) getPluralKey(key string, count int) string {
	if count == 1 {
		return key + ".one"
	}
	return key + ".other"
}

// Global singleton instance
var (
	globalI18n *I18n
	globalOnce sync.Once
)

// Global returns the global I18n instance.
func Global() *I18n {
	globalOnce.Do(func() {
		globalI18n = New()
	})
	return globalI18n
}

// SetGlobal sets the global I18n instance.
func SetGlobal(i *I18n) {
	globalI18n = i
}

// T is a shortcut for Global().T()
func T(key string, args ...any) string {
	return Global().T(key, args...)
}

// TPlural is a shortcut for Global().TPlural()
func TPlural(key string, count int, args ...any) string {
	return Global().TPlural(key, count, args...)
}
