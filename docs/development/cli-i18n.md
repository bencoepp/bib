# CLI Internationalization (i18n) Guide

This document describes how to add internationalization (i18n) support to CLI commands in bib.

## Overview

The bib CLI supports multiple languages for command descriptions, help text, and user-facing messages. Currently supported locales are:

| Code | Language |
|------|----------|
| `en` | English (default) |
| `de` | German (Deutsch) |
| `fr` | French (Français) |
| `ru` | Russian (Русский) |
| `zh-tw` | Traditional Chinese (繁體中文) |

## How It Works

### Locale Resolution

The locale is determined in the following priority order:

1. **`--locale` / `-L` flag** (highest priority)
2. **Config file** (`locale:` setting in `~/.config/bib/config.yaml`)
3. **System locale** (auto-detected from `LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, `LANG` environment variables)
4. **Default** (`en`)

### Architecture

The i18n system consists of two packages:

- **`internal/tui/i18n`** - Core i18n engine with locale files, translation lookup, and pluralization
- **`internal/cli/i18n`** - CLI-specific helpers for translating Cobra commands

Translation files are embedded into the binary using Go's `embed` package, located in:
```
internal/tui/i18n/locales/
├── en.yaml
├── de.yaml
├── fr.yaml
├── ru.yaml
└── zh-tw.yaml
```

## Adding Translations to a New Command

### Step 1: Mark Command for Translation

When creating a Cobra command, use translation keys instead of hardcoded strings, and add the i18n annotation:

```go
import (
    clii18n "bib/internal/cli/i18n"
)

var myCmd = &cobra.Command{
    Use:         "mycommand",
    Short:       "mycommand.short",    // Translation key (without "cmd." prefix)
    Long:        "mycommand.long",     // Translation key
    Example:     "mycommand.example",  // Translation key (optional)
    Annotations: clii18n.MarkForTranslation(),
    RunE:        runMyCommand,
}
```

**Key points:**
- `Short`, `Long`, and `Example` contain translation keys, not actual text
- Keys are relative to `cmd.` prefix (the system adds it automatically)
- Use `clii18n.MarkForTranslation()` to add the required annotation

### Step 2: Add Translation Keys to Locale Files

Add the translations to each locale file under the `cmd:` section:

**`internal/tui/i18n/locales/en.yaml`:**
```yaml
cmd:
  mycommand:
    short: "Brief description of my command"
    long: |
      Detailed description of my command.
      
      This can span multiple lines and include
      formatting information.
    example: |
      # Example usage
      bib mycommand --flag value
      
      # Another example
      bib mycommand --other-flag
```

**`internal/tui/i18n/locales/de.yaml`:**
```yaml
cmd:
  mycommand:
    short: "Kurze Beschreibung meines Befehls"
    long: |
      Ausführliche Beschreibung meines Befehls.
      
      Dies kann über mehrere Zeilen gehen.
    example: |
      # Beispielverwendung
      bib mycommand --flag wert
```

Repeat for all supported locales (fr.yaml, ru.yaml, zh-tw.yaml).

### Step 3: Translating Flags (Optional)

Flag descriptions can also be translated. Add them under the command's `flags:` section:

**In locale files:**
```yaml
cmd:
  mycommand:
    short: "..."
    long: "..."
    flags:
      verbose: "Enable verbose output"
      output: "Output format (json, yaml, table)"
```

The flag translation keys are automatically looked up based on the command path:
- `cmd.bib.flags.locale` for root command flags
- `cmd.bib.mycommand.flags.verbose` for subcommand flags

## Translating Runtime Messages

For runtime messages (errors, status updates, etc.), use the translation functions directly:

```go
import (
    clii18n "bib/internal/cli/i18n"
)

func runMyCommand(cmd *cobra.Command, args []string) error {
    // Simple translation
    fmt.Println(clii18n.T("status.success"))
    
    // Translation with interpolation
    fmt.Println(clii18n.T("errors.not_found", map[string]any{
        "resource": "Job",
        "id": args[0],
    }))
    
    // Pluralization
    fmt.Println(clii18n.TPlural("jobs.count", len(jobs), "count", len(jobs)))
    
    return nil
}
```

## Translation Key Naming Conventions

Follow these conventions for consistency:

| Type | Pattern | Example |
|------|---------|---------|
| Command short | `cmd.<name>.short` | `cmd.tui.short` |
| Command long | `cmd.<name>.long` | `cmd.tui.long` |
| Command example | `cmd.<name>.example` | `cmd.tui.example` |
| Subcommand | `cmd.<parent>.<child>.short` | `cmd.admin.backup.short` |
| Flag | `cmd.<path>.flags.<flag>` | `cmd.bib.flags.locale` |
| Common UI | `common.<key>` | `common.ok`, `common.cancel` |
| Status | `status.<key>` | `status.success`, `status.error` |
| Errors | `errors.<key>` | `errors.not_found` |
| Validation | `validation.<key>` | `validation.required` |

## Testing Translations

### Manual Testing

```bash
# Test with system locale
LANG=de_DE.UTF-8 bib --help

# Test with explicit locale flag
bib --locale fr tui --help

# Test with config file
echo 'locale: "ru"' >> ~/.config/bib/config.yaml
bib --help
```

### Automated Testing

Add tests in `internal/tui/i18n/i18n_test.go` to verify new keys exist:

```go
func TestMyCommandTranslationKeys(t *testing.T) {
    i := New()
    
    keys := []string{
        "cmd.mycommand.short",
        "cmd.mycommand.long",
    }
    
    for _, key := range keys {
        if !i.Has(key) {
            t.Errorf("missing translation key: %s", key)
        }
    }
}
```

## Pluralization

### English/German/French

Simple one/other rules:
```yaml
jobs:
  count:
    one: "{{ .count }} job"
    other: "{{ .count }} jobs"
```

### Russian

Russian has three plural forms (one, few, many):
```yaml
jobs:
  count:
    one: "{{ .count }} задача"      # 1, 21, 31...
    few: "{{ .count }} задачи"      # 2-4, 22-24...
    many: "{{ .count }} задач"      # 0, 5-20, 25-30...
```

### Chinese

Chinese has no plural forms:
```yaml
jobs:
  count:
    other: "{{ .count }} 個工作"
```

## Template Variables

Use Go template syntax for variable interpolation:

```yaml
errors:
  connection_failed: "Failed to connect to bibd at {{ .address }}"
  timeout: "Operation timed out after {{ .duration }}"
```

Usage:
```go
clii18n.T("errors.connection_failed", map[string]any{
    "address": "localhost:8080",
})
```

## Best Practices

1. **Keep translations in sync** - When adding a key to one locale, add it to all locales
2. **Use placeholders wisely** - Keep placeholders language-neutral when possible
3. **Test all locales** - Verify your command works in all supported languages
4. **Avoid concatenation** - Don't build sentences by concatenating translated parts
5. **Consider text length** - German and Russian translations are often longer than English

## Adding a New Locale

1. Create a new file `internal/tui/i18n/locales/<locale>.yaml`
2. Add the locale to `SupportedLocales` in `internal/tui/i18n/i18n.go`
3. Add the locale to `LocaleDisplayNames` map
4. Add pluralization rules if needed in `getPluralKey()`
5. Update the `--locale` flag description in all existing locale files
6. Copy all keys from `en.yaml` and translate them

## File Locations

| File | Purpose |
|------|---------|
| `internal/tui/i18n/i18n.go` | Core i18n engine |
| `internal/tui/i18n/locales/*.yaml` | Translation files |
| `internal/cli/i18n/i18n.go` | CLI command translation helpers |
| `cmd/bib/cmd/root.go` | i18n initialization (in `init()` and `initI18n()`) |

