# CLI & TUI Restructure Design

This document outlines the new architecture for the bib CLI and TUI system.

---

## Overview

The restructure aims to create:
1. **Domain-based CLI** with organized subcommand packages
2. **Full-screen TUI dashboard** (`bib tui`) like k9s/lazygit
3. **Data-driven wizards** with i18n support
4. **Universal help system** with rich error/warning visualization
5. **E2E testing infrastructure** for comprehensive CLI testing

---

## New Directory Structure

```
cmd/
├── bib/
│   ├── main.go                    # Entry point
│   └── cmd/
│       ├── root.go                # Root command, global flags
│       ├── execute.go             # Execute function, middleware chain
│       ├── context.go             # Command context helpers
│       │
│       ├── admin/                 # Admin command group
│       │   ├── admin.go           # Parent command
│       │   ├── backup.go
│       │   ├── blob.go
│       │   └── breakglass.go
│       │
│       ├── config/                # Config command group
│       │   ├── config.go          # Parent command
│       │   ├── show.go
│       │   ├── get.go
│       │   ├── set.go
│       │   ├── init.go
│       │   ├── validate.go
│       │   └── edit.go            # TUI editor integration
│       │
│       ├── job/                   # Job command group
│       │   ├── job.go
│       │   ├── list.go
│       │   ├── show.go
│       │   ├── create.go
│       │   ├── cancel.go
│       │   └── logs.go
│       │
│       ├── dataset/               # Dataset command group
│       │   ├── dataset.go
│       │   ├── list.go
│       │   ├── show.go
│       │   ├── create.go
│       │   └── delete.go
│       │
│       ├── cluster/               # Cluster command group
│       │   ├── cluster.go
│       │   ├── status.go
│       │   ├── join.go
│       │   └── leave.go
│       │
│       ├── tui/                   # TUI dashboard command
│       │   └── tui.go             # Launches full-screen dashboard
│       │
│       ├── setup/                 # Setup wizard command
│       │   └── setup.go
│       │
│       └── util/                  # Utility commands
│           ├── version.go
│           ├── cleanup.go
│           ├── reset.go
│           └── demo.go
│
├── bibd/
│   └── ...                        # Daemon (unchanged for now)

internal/
├── cli/                           # NEW: CLI infrastructure
│   ├── middleware/                # Command middleware
│   │   ├── middleware.go          # Middleware interface
│   │   ├── auth.go                # Authentication middleware
│   │   ├── logging.go             # Logging middleware
│   │   └── output.go              # Output formatting middleware
│   │
│   ├── output/                    # Output formatting
│   │   ├── format.go              # Format interface
│   │   ├── table.go               # Table formatter
│   │   ├── json.go                # JSON formatter
│   │   ├── yaml.go                # YAML formatter
│   │   └── writer.go              # Output writer
│   │
│   ├── errors/                    # Rich error handling
│   │   ├── errors.go              # Error types
│   │   ├── display.go             # Error display (TUI-aware)
│   │   └── suggestions.go         # Error suggestions
│   │
│   └── help/                      # Universal help system
│       ├── help.go                # Help renderer
│       ├── examples.go            # Example registry
│       └── docs.go                # Doc links
│
├── tui/                           # Restructured TUI
│   ├── app/                       # NEW: Main TUI application
│   │   ├── app.go                 # Main app model
│   │   ├── state.go               # Global state management
│   │   ├── router.go              # Page/view router
│   │   ├── keybindings.go         # Global keybindings
│   │   └── commands.go            # Tea commands/messages
│   │
│   ├── pages/                     # NEW: Full-screen pages
│   │   ├── page.go                # Page interface
│   │   ├── dashboard.go           # Main dashboard
│   │   ├── jobs.go                # Jobs list/detail
│   │   ├── datasets.go            # Dataset browser
│   │   ├── cluster.go             # Cluster status
│   │   ├── logs.go                # Log viewer
│   │   └── settings.go            # Settings page
│   │
│   ├── dialogs/                   # NEW: Modal dialogs
│   │   ├── dialog.go              # Dialog interface
│   │   ├── confirm.go             # Confirmation dialog
│   │   ├── input.go               # Input dialog
│   │   ├── select.go              # Selection dialog
│   │   └── error.go               # Error dialog
│   │
│   ├── wizard/                    # NEW: Data-driven wizards
│   │   ├── wizard.go              # Wizard engine
│   │   ├── step.go                # Step definitions
│   │   ├── loader.go              # YAML step loader
│   │   ├── validation.go          # Validation rules
│   │   └── definitions/           # Wizard YAML files
│   │       ├── setup_bib.yaml
│   │       ├── setup_bibd.yaml
│   │       └── cluster_join.yaml
│   │
│   ├── component/                 # (existing, enhanced)
│   │   ├── base.go
│   │   ├── containers.go
│   │   ├── ...
│   │   └── command_palette.go     # NEW: k9s-style command palette
│   │
│   ├── layout/                    # (existing)
│   │
│   ├── themes/                    # (existing)
│   │
│   ├── i18n/                      # NEW: Internationalization
│   │   ├── i18n.go                # i18n engine
│   │   ├── loader.go              # Translation loader
│   │   ├── plurals.go             # Plural rules
│   │   └── locales/               # Translation files
│   │       ├── en.yaml
│   │       ├── de.yaml
│   │       ├── es.yaml
│   │       └── ...
│   │
│   └── tui.go                     # Entry point (slimmed down)

test/
├── e2e/
│   ├── cli/                       # CLI E2E tests
│   │   ├── suite_test.go          # Test suite setup
│   │   ├── config_test.go         # Config commands
│   │   ├── job_test.go            # Job commands
│   │   ├── admin_test.go          # Admin commands
│   │   └── tui_test.go            # TUI interaction tests
│   │
│   └── fixtures/                  # Test fixtures
│       ├── configs/
│       └── wizards/
│
└── integration/
    └── cli/                       # CLI integration tests
        └── ...
```

---

## Key Components

### 1. Command Middleware Chain

```go
// internal/cli/middleware/middleware.go
type Middleware func(next RunFunc) RunFunc
type RunFunc func(cmd *cobra.Command, args []string) error

// Chain applies middleware in order
func Chain(middlewares ...Middleware) Middleware
```

Middleware order:
1. **Logging** - Request ID, timing
2. **Config** - Load configuration
3. **Auth** - Validate credentials (if needed)
4. **Output** - Set up output formatter

### 2. Universal Help System

```go
// internal/cli/help/help.go
type Help struct {
    Command     string
    Short       string
    Long        string
    Examples    []Example
    SeeAlso     []string
    DocURL      string
    Flags       []FlagHelp
}

type Example struct {
    Description string
    Command     string
    Output      string // Optional expected output
}
```

### 3. Rich Error Display

```go
// internal/cli/errors/display.go
type RichError struct {
    Code        string      // Error code (e.g., "CONFIG_NOT_FOUND")
    Message     string      // User-friendly message
    Details     string      // Technical details
    Suggestions []string    // Suggested actions
    DocURL      string      // Link to documentation
}

func Display(err error) // Renders error with TUI styling
```

### 4. TUI Application Architecture

```go
// internal/tui/app/app.go
type App struct {
    state    *State        // Global state
    router   *Router       // Page router
    theme    *themes.Theme
    i18n     *i18n.I18n
    
    // Current view
    page     pages.Page
    dialog   dialogs.Dialog // Optional overlay
    
    width, height int
}

// State holds application-wide state
type State struct {
    Config      *config.BibConfig
    Connection  *client.Connection // To bibd
    User        *domain.User
    
    // Cached data
    Jobs        []domain.Job
    Datasets    []domain.Dataset
    ClusterInfo *cluster.Info
}
```

### 5. Data-Driven Wizards

```yaml
# internal/tui/wizard/definitions/setup_bib.yaml
wizard:
  id: setup_bib
  title:
    key: wizard.setup_bib.title  # i18n key
    default: "Bib Setup Wizard"
  
  steps:
    - id: welcome
      type: info
      title:
        key: wizard.setup_bib.welcome.title
        default: "Welcome"
      content:
        key: wizard.setup_bib.welcome.content
        default: "Let's configure bib..."
      help:
        key: wizard.setup_bib.welcome.help
        
    - id: identity
      type: form
      title:
        key: wizard.setup_bib.identity.title
        default: "Identity"
      fields:
        - id: name
          type: text
          label:
            key: wizard.setup_bib.identity.name.label
            default: "Name"
          placeholder: "John Doe"
          required: true
          validation:
            - rule: minLength(2)
              message:
                key: validation.min_length
                args: { min: 2 }
                
        - id: email
          type: text
          label:
            key: wizard.setup_bib.identity.email.label
            default: "Email"
          placeholder: "john@example.com"
          validation:
            - rule: email
              message:
                key: validation.email
      
    - id: output
      type: form
      skip_if: "{{ .IsDaemon }}"
      # ...
```

### 6. i18n System

```yaml
# internal/tui/i18n/locales/en.yaml
wizard:
  setup_bib:
    title: "Bib Setup Wizard"
    welcome:
      title: "Welcome"
      content: "Let's configure bib for your environment."
      help: "This wizard will guide you through the initial setup."
    identity:
      title: "Identity"
      name:
        label: "Name"
        help: "Your display name for commits and attribution."
      email:
        label: "Email"

errors:
  config_not_found: "Configuration file not found"
  connection_failed: "Failed to connect to bibd at {{ .Address }}"
  
validation:
  required: "This field is required"
  min_length: "Must be at least {{ .min }} characters"
  email: "Must be a valid email address"

common:
  next: "Next"
  back: "Back"
  cancel: "Cancel"
  confirm: "Confirm"
  save: "Save"
```

---

## TUI Dashboard Features

The `bib tui` command launches a full-screen dashboard with:

### Navigation
- **Tab/Shift+Tab** - Switch between panels
- **`/`** - Open command palette (k9s-style)
- **`?`** - Show help
- **`q`** - Quit (with confirmation if operations pending)
- **`:`** - Command mode (vim-style)

### Pages
1. **Dashboard** - Overview with stats, recent activity
2. **Jobs** - Job list with filters, detail view, logs
3. **Datasets** - Dataset browser with tree view
4. **Cluster** - Node status, replication lag
5. **Logs** - Real-time log viewer with filters
6. **Settings** - Configuration with live reload

### Status Bar
- Connection status to bibd
- Current user
- Active jobs count
- Cluster status
- Keyboard shortcuts

---

## Migration Plan

### Phase 1: Infrastructure (Current)
- [ ] Create `internal/cli/` package structure
- [ ] Implement middleware chain
- [ ] Create rich error system
- [ ] Set up i18n foundation

### Phase 2: CLI Restructure
- [ ] Reorganize commands into domain packages
- [ ] Apply middleware to all commands
- [ ] Implement universal help system

### Phase 3: TUI Foundation
- [ ] Create `app/`, `pages/`, `dialogs/` structure
- [ ] Implement router and state management
- [ ] Port existing wizard to data-driven system

### Phase 4: TUI Dashboard
- [ ] Implement dashboard page
- [ ] Add command palette
- [ ] Create remaining pages

### Phase 5: Testing
- [ ] E2E test infrastructure
- [ ] CLI command tests
- [ ] TUI interaction tests

---

## Questions Resolved

| Question | Answer |
|----------|--------|
| Domain-based CLI | Yes, with subcommand packages |
| TUI entry point | `bib tui` for full dashboard |
| Dashboard style | k9s/lazygit interactive |
| Wizard data source | YAML definitions with i18n |
| Error display | Rich TUI-styled with suggestions |
| Testing focus | Integration and E2E |
| Component scope | Internal to bib only |

