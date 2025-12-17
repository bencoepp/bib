# Developer Guide

This guide provides a comprehensive overview of the bib codebase for new developers and AI coding agents. It covers project structure, code patterns, and how to effectively work with the codebase.

## Project Overview

Bib is a distributed data management platform consisting of:

- **`bib`** - CLI client (`cmd/bib/`)
- **`bibd`** - Daemon server (`cmd/bibd/`)
- **`internal/`** - Shared internal packages

## Directory Structure

```
bib/
├── api/proto/bib/v1/        # Protobuf definitions
├── cmd/
│   ├── bib/                  # CLI entry point
│   │   ├── main.go
│   │   └── cmd/              # Cobra commands
│   │       ├── root.go       # Root command, global flags
│   │       ├── setup.go      # Interactive setup wizard
│   │       ├── config.go     # Config management commands
│   │       ├── config_tui.go # TUI config editor
│   │       ├── output.go     # Output formatting utilities
│   │       ├── version.go    # Version command
│   │       └── ...
│   └── bibd/                 # Daemon entry point
│       ├── main.go
│       └── daemon.go
├── docs/                     # Documentation
├── internal/
│   ├── cluster/              # Raft clustering
│   ├── config/               # Configuration loading/types
│   ├── domain/               # Domain entities
│   ├── logger/               # Structured logging
│   ├── p2p/                  # libp2p networking
│   ├── storage/              # Database layer
│   │   ├── postgres/         # PostgreSQL implementation
│   │   └── sqlite/           # SQLite implementation
│   └── tui/                  # Terminal UI components
│       ├── component/        # Reusable components
│       ├── layout/           # Layout primitives
│       └── themes/           # Theming system
├── go.mod
└── go.sum
```

## Key Packages

### cmd/bib/cmd

CLI command implementations using Cobra.

**Key files:**
- `root.go` - Root command, global flags (`--config`, `--output`, `--verbose`), initialization
- `setup.go` - Interactive setup wizard using TUI
- `config.go` - Config show/path commands
- `config_tui.go` - Interactive config editor
- `output.go` - Multi-format output (JSON, YAML, table)

**Pattern for new commands:**

```go
package cmd

var myCmd = &cobra.Command{
    Use:   "mycommand [args]",
    Short: "Short description",
    Long:  `Detailed description with examples.`,
    RunE:  runMyCommand,
}

func init() {
    rootCmd.AddCommand(myCmd)
    myCmd.Flags().StringVarP(&myFlag, "flag", "f", "default", "flag description")
}

func runMyCommand(cmd *cobra.Command, args []string) error {
    // Get configuration (loaded in PersistentPreRunE)
    cfg := Config()
    
    // Get logger
    log := Log()
    
    // Get output writer
    out := NewOutputWriter()
    
    // Do work...
    
    // Output results
    return out.Write(result)
}
```

### internal/config

Configuration management with Viper.

**Key types:**
- `BibConfig` - CLI configuration
- `BibdConfig` - Daemon configuration

**Key functions:**
- `LoadBib(path)` / `LoadBibd(path)` - Load configurations
- `GenerateConfigIfNotExists(app, format)` - Auto-generate defaults
- `DefaultBibConfig()` / `DefaultBibdConfig()` - Default values

### internal/tui

Terminal UI system. See [TUI Component System](tui-components.md) for complete documentation.

**Quick reference:**

```go
import (
    "bib/internal/tui"
    "bib/internal/tui/themes"
    "bib/internal/tui/component"
    "bib/internal/tui/layout"
)

// Get current theme
theme := tui.GetTheme()

// Create components
card := tui.NewCard().WithTitle("Title").WithContent("Content")
table := tui.NewTable().WithColumns(...).WithRows(...)

// Use layout
flex := layout.NewFlex().Direction(layout.Row).Gap(2)

// Responsive design
if layout.GetBreakpoint(width) >= layout.BreakpointMD {
    // Show sidebar
}
```

### internal/logger

Structured logging with slog.

```go
import "bib/internal/logger"

// Create logger
log, err := logger.New(cfg.Log)

// Use logger
log.Info("operation completed", "key", value)
log.Error("operation failed", "error", err)
log.Debug("detailed info", "data", data)

// With context
log.With("request_id", id).Info("processing")
```

### internal/domain

Domain entities representing the core data model.

**Key entities:**
- `Topic` - Data category/namespace
- `Dataset` - Versioned data with metadata
- `Job` - Execution request
- `Task` - Reusable instructions
- `User` - Identity with cryptographic keys
- `Query` - Search/filter specification

### internal/storage

Database abstraction with SQLite and PostgreSQL implementations.

**Interface pattern:**
```go
type Store interface {
    // Topics
    CreateTopic(ctx, topic) error
    GetTopic(ctx, id) (*Topic, error)
    ListTopics(ctx, filter) ([]*Topic, error)
    
    // Datasets
    CreateDataset(ctx, dataset) error
    GetDataset(ctx, id) (*Dataset, error)
    // ... etc
}
```

### internal/p2p

libp2p networking layer.

**Key components:**
- `Host` - libp2p host wrapper
- `Identity` - Node identity management
- `DHT` - Kademlia DHT for discovery
- `Bootstrap` - Peer bootstrapping
- `Transfer` - Data transfer protocols
- `PubSub` - Topic-based messaging
- `Mode` - Node mode implementations (proxy/selective/full)

### internal/cluster

Raft consensus for HA deployments.

**Key components:**
- `Cluster` - Cluster manager
- `FSM` - Finite state machine for Raft
- `Transport` - Raft transport layer
- `Storage` - Raft log storage

## Common Patterns

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

// Domain errors (internal/domain/errors.go)
var ErrNotFound = errors.New("not found")
var ErrAlreadyExists = errors.New("already exists")

// Check specific errors
if errors.Is(err, domain.ErrNotFound) {
    // Handle not found
}
```

### Context Propagation

```go
// CLI commands create context with logger
ctx := logger.WithLogger(context.Background(), log)

// Add request ID for tracing
ctx = logger.WithCommandContext(ctx, cmdContext)

// Pass through all layers
result, err := service.DoSomething(ctx, params)
```

### Configuration Access

```go
// In CLI commands
cfg := Config()  // Returns *config.BibConfig

// Access nested config
serverAddr := cfg.Server
logLevel := cfg.Log.Level
```

### Output Formatting

```go
// Create output writer (respects --output flag)
out := NewOutputWriter()

// For structured data
out.Write(dataStruct)

// For tables
out.Write(TableData{
    Headers: []string{"ID", "Name", "Status"},
    Rows: [][]string{
        {"1", "Alice", "Active"},
        {"2", "Bob", "Inactive"},
    },
})

// Success messages (hidden in quiet mode)
out.WriteSuccess("Operation completed")

// Error messages (always shown)
out.WriteError("Error: " + err.Error())
```

### TUI Integration

```go
// For simple forms
theme := tui.HuhTheme()
form := huh.NewForm(
    huh.NewGroup(
        huh.NewInput().Title("Name").Value(&name),
    ),
).WithTheme(theme)
form.Run()

// For complex wizards
wizard := tui.NewWizard("Title", "Description", steps, onComplete)
p := tea.NewProgram(wizard)
p.Run()

// For status output
status := tui.NewStatusIndicator()
fmt.Println(status.Success("Done!"))
fmt.Println(status.Error("Failed!"))
```

## Working with Components

### Stateless Components

```go
// Create and render immediately
card := tui.NewCard().
    WithTitle("Title").
    WithContent("Content")
    
output := card.View(80)  // width in columns
fmt.Println(output)
```

### Stateful Components (tea.Model)

```go
// Create component
table := tui.NewTable().WithColumns(...).WithRows(...)

// In a parent model's Update:
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    tableModel, cmd := m.table.Update(msg)
    m.table = tableModel.(*component.Table)
    return m, cmd
}

// In View:
func (m *Model) View() string {
    return m.table.View()
}
```

### Layout

```go
// Flex layout
flex := layout.NewFlex().
    Direction(layout.Row).
    Justify(layout.JustifySpaceBetween).
    Gap(2).
    Width(80).
    Item("Left").
    ItemWithGrow("Center", 1).
    Item("Right")

output := flex.Render()

// Grid layout
grid := layout.NewGrid(3).
    Width(80).
    Items("A", "B", "C", "D", "E", "F")

output := grid.Render()

// Responsive
bp := layout.GetBreakpoint(width)
if bp >= layout.BreakpointLG {
    // Wide layout
}
```

## Testing

### Unit Tests

```go
func TestMyFunction(t *testing.T) {
    // Setup
    input := ...
    
    // Execute
    result, err := MyFunction(input)
    
    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("expected %v, got %v", expected, result)
    }
}
```

### TUI Component Tests

```go
func TestCard(t *testing.T) {
    card := component.NewCard().
        WithTitle("Test").
        WithContent("Content")
    
    output := card.View(40)
    
    if !strings.Contains(output, "Test") {
        t.Error("should contain title")
    }
}

func TestTableNavigation(t *testing.T) {
    table := component.NewTable().
        WithRows(
            component.TableRow{Cells: []string{"1"}},
            component.TableRow{Cells: []string{"2"}},
        )
    table.Focus()
    
    // Simulate key press
    table.Update(tea.KeyMsg{Type: tea.KeyDown})
    
    if table.SelectedIndex() != 1 {
        t.Error("should select second row")
    }
}
```

### Integration Tests

```go
func TestConfigLoading(t *testing.T) {
    // Create temp config file
    tmpDir := t.TempDir()
    cfgPath := filepath.Join(tmpDir, "config.yaml")
    os.WriteFile(cfgPath, []byte(`
log:
  level: debug
`), 0644)
    
    // Load config
    cfg, err := config.LoadBib(cfgPath)
    if err != nil {
        t.Fatal(err)
    }
    
    if cfg.Log.Level != "debug" {
        t.Error("wrong log level")
    }
}
```

## Adding New Features

### New CLI Command

1. Create file in `cmd/bib/cmd/` (e.g., `mycommand.go`)
2. Define command struct with `Use`, `Short`, `Long`, `RunE`
3. Add to root in `init()`: `rootCmd.AddCommand(myCmd)`
4. Implement `runMyCommand(cmd, args)` function
5. Use `NewOutputWriter()` for output
6. Add tests in `cmd/bib/cmd/mycommand_test.go`
7. Update `docs/cli-reference.md`

### New TUI Component

1. Create file in `internal/tui/component/` (e.g., `mycomponent.go`)
2. Embed `BaseComponent` for theme support
3. Embed `FocusState` if focusable
4. Implement required interface (`Renderer` or `tea.Model`)
5. Add constructor: `NewMyComponent()`
6. Use builder pattern: `WithX()` methods return `*MyComponent`
7. Export from `internal/tui/tui.go` if commonly used
8. Add tests in `internal/tui/component/mycomponent_test.go`
9. Document in `docs/tui-components.md`

### New Domain Entity

1. Create file in `internal/domain/` (e.g., `myentity.go`)
2. Define struct with JSON/YAML tags
3. Add validation methods
4. Add to storage interface in `internal/storage/`
5. Implement in `internal/storage/sqlite/` and `internal/storage/postgres/`
6. Add proto definition if exposed via API
7. Document in `docs/domain-entities.md`

## Code Style

### General

- Follow standard Go conventions
- Use `gofmt`/`goimports`
- Keep functions focused and small
- Document public APIs with godoc comments

### Error Messages

```go
// Good: lowercase, no punctuation, context
return fmt.Errorf("failed to load config from %s: %w", path, err)

// Bad
return fmt.Errorf("Failed to load config: %w", err)  // capitalized
return fmt.Errorf("failed to load config: %w.", err) // punctuation
```

### Logging

```go
// Good: structured, lowercase keys
log.Info("processing request", "user_id", userID, "action", action)

// Bad: unstructured message
log.Info(fmt.Sprintf("Processing request for user %s", userID))
```

### TUI Components

```go
// Good: builder pattern, chainable
card := NewCard().
    WithTitle("Title").
    WithContent(content).
    WithTheme(theme)

// Good: consistent interface
func (c *Card) WithTitle(title string) *Card {
    c.title = title
    return c
}
```

## Debugging Tips

### Logging

```bash
# Enable debug logging
bib --verbose mycommand

# Or set in config
log:
  level: debug
```

### TUI Debugging

```go
// Log to file (stdout interferes with TUI)
f, _ := os.Create("/tmp/tui-debug.log")
log := log.New(f, "", log.LstdFlags)

// In Update:
log.Printf("msg: %T %+v", msg, msg)
```

### P2P Debugging

```bash
# Environment variable for libp2p debug
GOLOG_LOG_LEVEL=debug bibd

# Specific subsystem
GOLOG_LOG_LEVEL=pubsub=debug,dht=warn bibd
```

## See Also

- [Architecture](architecture.md) - System design overview
- [TUI Component System](tui-components.md) - Complete TUI documentation
- [CLI Reference](cli-reference.md) - Command documentation
- [Configuration](configuration.md) - Config options
- [Domain Entities](domain-entities.md) - Data model

