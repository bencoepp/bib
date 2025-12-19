package i18n

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	i := New()
	if i.Locale() != DefaultLocale {
		t.Errorf("expected locale %s, got %s", DefaultLocale, i.Locale())
	}
}

func TestSupportedLocales(t *testing.T) {
	expected := []string{"en", "de", "fr", "ru", "zh-tw"}
	if len(SupportedLocales) != len(expected) {
		t.Errorf("expected %d locales, got %d", len(expected), len(SupportedLocales))
	}
	for i, locale := range expected {
		if SupportedLocales[i] != locale {
			t.Errorf("expected locale %s at index %d, got %s", locale, i, SupportedLocales[i])
		}
	}
}

func TestSetLocale(t *testing.T) {
	tests := []struct {
		locale    string
		expectErr bool
	}{
		{"en", false},
		{"de", false},
		{"fr", false},
		{"ru", false},
		{"zh-tw", false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			i := New()
			err := i.SetLocale(tt.locale)
			if tt.expectErr && err == nil {
				t.Errorf("expected error for locale %s", tt.locale)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error for locale %s: %v", tt.locale, err)
			}
			if !tt.expectErr && i.Locale() != tt.locale {
				t.Errorf("expected locale %s, got %s", tt.locale, i.Locale())
			}
		})
	}
}

func TestTranslation(t *testing.T) {
	tests := []struct {
		locale   string
		key      string
		expected string
	}{
		{"en", "common.ok", "OK"},
		{"de", "common.ok", "OK"},
		{"fr", "common.ok", "OK"},
		{"ru", "common.ok", "OK"},
		{"zh-tw", "common.ok", "確定"},
		{"en", "common.cancel", "Cancel"},
		{"de", "common.cancel", "Abbrechen"},
		{"fr", "common.cancel", "Annuler"},
		{"ru", "common.cancel", "Отмена"},
		{"zh-tw", "common.cancel", "取消"},
		{"en", "common.yes", "Yes"},
		{"de", "common.yes", "Ja"},
		{"fr", "common.yes", "Oui"},
		{"ru", "common.yes", "Да"},
		{"zh-tw", "common.yes", "是"},
	}

	for _, tt := range tests {
		t.Run(tt.locale+"_"+tt.key, func(t *testing.T) {
			i := New()
			if err := i.SetLocale(tt.locale); err != nil {
				t.Fatalf("failed to set locale: %v", err)
			}
			result := i.T(tt.key)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTranslationFallback(t *testing.T) {
	i := New()
	// Non-existent key should return the key itself
	result := i.T("nonexistent.key")
	if result != "nonexistent.key" {
		t.Errorf("expected key to be returned for missing translation, got %q", result)
	}
}

func TestInterpolation(t *testing.T) {
	i := New()

	// Test with map args
	result := i.T("wizard.step", map[string]any{"current": 1, "total": 5})
	if result != "Step 1 of 5" {
		t.Errorf("expected 'Step 1 of 5', got %q", result)
	}

	// Test with key-value pairs
	result = i.T("wizard.step", "current", 2, "total", 10)
	if result != "Step 2 of 10" {
		t.Errorf("expected 'Step 2 of 10', got %q", result)
	}
}

func TestPluralRussian(t *testing.T) {
	i := New()
	_ = i.SetLocale("ru")

	tests := []struct {
		count    int
		expected string
	}{
		{0, "many"},
		{1, "one"},
		{2, "few"},
		{3, "few"},
		{4, "few"},
		{5, "many"},
		{10, "many"},
		{11, "many"},
		{12, "many"},
		{13, "many"},
		{14, "many"},
		{20, "many"},
		{21, "one"},
		{22, "few"},
		{23, "few"},
		{24, "few"},
		{25, "many"},
		{100, "many"},
		{101, "one"},
		{102, "few"},
		{111, "many"},
		{112, "many"},
	}

	for _, tt := range tests {
		t.Run("count_"+string(rune(tt.count)), func(t *testing.T) {
			result := i.getPluralFormRussian(tt.count)
			if result != tt.expected {
				t.Errorf("count %d: expected %q, got %q", tt.count, tt.expected, result)
			}
		})
	}
}

func TestIsValidLocale(t *testing.T) {
	tests := []struct {
		locale string
		valid  bool
	}{
		{"en", true},
		{"de", true},
		{"fr", true},
		{"ru", true},
		{"zh-tw", true},
		{"es", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			result := IsValidLocale(tt.locale)
			if result != tt.valid {
				t.Errorf("expected %v for locale %q, got %v", tt.valid, tt.locale, result)
			}
		})
	}
}

func TestLocaleDisplayName(t *testing.T) {
	tests := []struct {
		locale   string
		expected string
	}{
		{"en", "English"},
		{"de", "Deutsch"},
		{"fr", "Français"},
		{"ru", "Русский"},
		{"zh-tw", "繁體中文"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.locale, func(t *testing.T) {
			result := LocaleDisplayName(tt.locale)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestAvailableLocales(t *testing.T) {
	locales := AvailableLocales()
	if len(locales) != 5 {
		t.Errorf("expected 5 locales, got %d", len(locales))
	}
}

func TestHas(t *testing.T) {
	i := New()

	if !i.Has("common.ok") {
		t.Error("expected Has('common.ok') to return true")
	}

	if i.Has("nonexistent.key") {
		t.Error("expected Has('nonexistent.key') to return false")
	}
}

func TestMatchLocale(t *testing.T) {
	tests := []struct {
		sysLocale string
		expected  string
	}{
		{"en", "en"},
		{"en_US", "en"},
		{"en_US.UTF-8", "en"},
		{"de", "de"},
		{"de_DE", "de"},
		{"de_DE.UTF-8", "de"},
		{"fr", "fr"},
		{"fr_FR.UTF-8", "fr"},
		{"ru", "ru"},
		{"ru_RU.UTF-8", "ru"},
		{"zh_TW", "zh-tw"},
		{"zh_TW.UTF-8", "zh-tw"},
		{"zh-tw", "zh-tw"},
		{"es_ES.UTF-8", ""},   // Spanish not supported
		{"invalid", ""},       // Invalid locale
		{"C", ""},             // POSIX C locale
		{"POSIX", ""},         // POSIX locale
		{"en_GB.UTF-8", "en"}, // British English -> en
		{"de_AT.UTF-8", "de"}, // Austrian German -> de
		{"fr_CA.UTF-8", "fr"}, // Canadian French -> fr
	}

	for _, tt := range tests {
		t.Run(tt.sysLocale, func(t *testing.T) {
			result := matchLocale(tt.sysLocale)
			if result != tt.expected {
				t.Errorf("matchLocale(%q): expected %q, got %q", tt.sysLocale, tt.expected, result)
			}
		})
	}
}

func TestResolveLocale(t *testing.T) {
	tests := []struct {
		name         string
		flagLocale   string
		configLocale string
		expected     string
	}{
		{"flag takes priority", "de", "fr", "de"},
		{"config when no flag", "", "fr", "fr"},
		{"invalid flag falls to config", "invalid", "de", "de"},
		{"invalid config falls to system", "", "invalid", DetectSystemLocale()},
		{"both empty uses system", "", "", DetectSystemLocale()},
		{"valid flag with empty config", "ru", "", "ru"},
		{"zh-tw via flag", "zh-tw", "", "zh-tw"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveLocale(tt.flagLocale, tt.configLocale)
			if result != tt.expected {
				t.Errorf("ResolveLocale(%q, %q): expected %q, got %q",
					tt.flagLocale, tt.configLocale, tt.expected, result)
			}
		})
	}
}

func TestDetectSystemLocale(t *testing.T) {
	// Save original env vars
	origLang := os.Getenv("LANG")
	origLcAll := os.Getenv("LC_ALL")
	origLcMessages := os.Getenv("LC_MESSAGES")
	origLanguage := os.Getenv("LANGUAGE")

	// Restore at end
	defer func() {
		os.Setenv("LANG", origLang)
		os.Setenv("LC_ALL", origLcAll)
		os.Setenv("LC_MESSAGES", origLcMessages)
		os.Setenv("LANGUAGE", origLanguage)
	}()

	// Clear all locale env vars
	os.Unsetenv("LANGUAGE")
	os.Unsetenv("LC_ALL")
	os.Unsetenv("LC_MESSAGES")
	os.Unsetenv("LANG")

	// Test with no env vars set - should return default
	result := DetectSystemLocale()
	if result != DefaultLocale {
		t.Errorf("with no env vars, expected %q, got %q", DefaultLocale, result)
	}

	// Test LANG
	os.Setenv("LANG", "de_DE.UTF-8")
	result = DetectSystemLocale()
	if result != "de" {
		t.Errorf("with LANG=de_DE.UTF-8, expected 'de', got %q", result)
	}

	// Test LC_ALL overrides LANG
	os.Setenv("LC_ALL", "fr_FR.UTF-8")
	result = DetectSystemLocale()
	if result != "fr" {
		t.Errorf("with LC_ALL=fr_FR.UTF-8, expected 'fr', got %q", result)
	}

	// Test LANGUAGE has highest priority
	os.Setenv("LANGUAGE", "ru")
	result = DetectSystemLocale()
	if result != "ru" {
		t.Errorf("with LANGUAGE=ru, expected 'ru', got %q", result)
	}
}

func TestCLITranslationKeys(t *testing.T) {
	i := New()

	// Test that CLI translation keys exist
	cliKeys := []string{
		"cmd.bib.short",
		"cmd.bib.long",
		"cmd.tui.short",
		"cmd.tui.long",
		"cmd.version.short",
		"cmd.setup.short",
		"cmd.config.short",
		"cmd.admin.short",
		"cmd.demo.short",
	}

	for _, key := range cliKeys {
		t.Run(key, func(t *testing.T) {
			if !i.Has(key) {
				t.Errorf("expected key %q to exist", key)
			}
			val := i.T(key)
			if val == key {
				t.Errorf("expected translation for %q, got key back", key)
			}
		})
	}
}
