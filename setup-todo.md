# Setup Flow Implementation TODO

This document tracks the implementation tasks for the bib/bibd setup flow as defined in [Setup Flow](docs/getting-started/setup-flow.md).

> **Usage**: This file is designed for step-by-step implementation with Copilot. Work through each section in order, checking off tasks as they are completed.

---

## Table of Contents

1. [Phase 1: Core Infrastructure](#phase-1-core-infrastructure)
2. [Phase 2: CLI Setup (bib setup)](#phase-2-cli-setup-bib-setup)
3. [Phase 3: Daemon Setup - Local](#phase-3-daemon-setup---local)
4. [Phase 4: Daemon Setup - Docker](#phase-4-daemon-setup---docker)
5. [Phase 5: Daemon Setup - Podman](#phase-5-daemon-setup---podman)
6. [Phase 6: Daemon Setup - Kubernetes](#phase-6-daemon-setup---kubernetes)
7. [Phase 7: Node Discovery](#phase-7-node-discovery)
8. [Phase 8: Connection & Trust](#phase-8-connection--trust)
9. [Phase 9: Post-Setup Actions](#phase-9-post-setup-actions)
10. [Phase 10: Error Recovery & Reconfiguration](#phase-10-error-recovery--reconfiguration)
11. [Phase 11: Testing](#phase-11-testing)
12. [Phase 12: Documentation & Polish](#phase-12-documentation--polish)

---

## Phase 1: Core Infrastructure

### 1.1 Setup Command Flags

- [x] Add `--quick` / `-q` flag to setup command
- [x] Add `--target` / `-t` flag with validation for: `local`, `docker`, `podman`, `kubernetes`
- [x] Add `--reconfigure` flag with section validation
- [x] Add `--fresh` flag to reset configuration
- [x] Update help text for all new flags
- [x] Add flag validation (e.g., `--target` only valid with `--daemon`)

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Added `DeploymentTarget` type with validation
- Added `ValidReconfigureSections()` for context-aware section validation
- Added `validateSetupFlags()` for comprehensive flag validation
- Added `handleFreshSetup()` with user confirmation
- Added placeholder functions for `setupBibQuick()` and `setupBibdQuick()`
- Added placeholder for `runReconfigure()`

### 1.2 Setup Data Model

- [x] Add `DeploymentTarget` field to `SetupData` struct
- [x] Add `SelectedNodes` field for multi-node selection (CLI setup)
- [x] Add `BibDevConfirmed` field for public network confirmation
- [x] Add deployment-target-specific fields:
  - [x] `DockerComposeDir` string
  - [x] `PodmanMode` string (rootful/rootless)
  - [x] `PodmanStyle` string (pod/compose)
  - [x] `KubernetesNamespace` string
  - [x] `KubernetesContext` string
  - [x] `KubernetesOutputDir` string
  - [x] `KubernetesApplyManifests` bool
  - [x] `KubernetesExternalAccess` string (none/loadbalancer/nodeport/ingress)
- [x] Add PostgreSQL deployment fields:
  - [x] `PostgresDeployment` string (managed/local/remote/statefulset/cloudnativepg/external)

**Files modified:**
- `internal/tui/setup.go`

**Implementation notes:**
- Added `NodeSelection` struct for multi-node selection
- Added constants for all deployment options (DeployTarget*, PodmanMode*, PodmanStyle*, K8sAccess*, PostgresDeploy*)
- Added PostgreSQL connection fields (host, port, database, user, password, sslmode, storage class/size)
- Added bootstrap peer fields (UsePublicBootstrap, CustomBootstrapPeers)
- Added `KubernetesIngressHost` for ingress hostname configuration
- Updated `DefaultSetupData()` with sensible defaults for all new fields
- Added helper methods:
  - `AddSelectedNode()`, `RemoveSelectedNode()`, `GetDefaultNode()`, `SetDefaultNode()`, `HasNode()`
  - `IsContainerDeployment()`, `IsKubernetesDeployment()`, `IsLocalDeployment()`
  - `RequiresPostgres()`, `GetPostgresConnectionString()`
  - `AddCustomBootstrapPeer()`, `RemoveCustomBootstrapPeer()`

### 1.3 Partial Configuration Save/Resume

- [x] Create `SetupProgress` struct to track wizard state
- [x] Implement `SavePartialConfig()` function
- [x] Implement `LoadPartialConfig()` function
- [x] Implement `DetectPartialConfig()` function
- [x] Add resume prompt to setup wizard entry
- [x] Store partial config at `~/.config/bib/config.yaml.partial` or `~/.config/bibd/config.yaml.partial`

**Files created:**
- `internal/config/partial.go`
- `internal/config/partial_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- `SetupProgress` struct tracks: version, app name, timestamps, current step, completed steps, raw JSON data
- Progress is saved as JSON for schema flexibility
- Atomic file writes using temp file + rename
- Version field for forward compatibility
- Helper methods: `MarkStepCompleted()`, `IsStepCompleted()`, `SetCurrentStep()`, `SetData()`, `GetData()`
- Utility methods: `ProgressPercentage()`, `TimeSinceStart()`, `TimeSinceLastUpdate()`, `Summary()`
- `checkAndOfferResume()` prompts user with Resume/Start Over/Cancel options
- Progress saved on Ctrl+C/q with user feedback
- Progress tracking integrated into `SetupWizardModel.Update()`
- Partial config deleted on successful wizard completion
- Comprehensive unit tests for all functionality

---

## Phase 2: CLI Setup (bib setup)

### 2.1 Identity Key Generation

- [x] Implement `GenerateIdentityKey()` function
- [x] Generate Ed25519 keypair
- [x] Save private key to `~/.config/bib/identity.pem`
- [x] Display public key and fingerprint to user
- [x] Add key generation step to wizard

**Files created:**
- `internal/auth/identity.go`
- `internal/auth/identity_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`
- `internal/tui/setup.go`

**Implementation notes:**
- Created `IdentityKey` struct with methods:
  - `GenerateIdentityKey()` - generates new Ed25519 keypair
  - `LoadIdentityKey(path)` - loads existing key from PEM file
  - `Save(path)` - saves key in OpenSSH format with 0600 permissions
  - `Fingerprint()` - returns SHA256 fingerprint
  - `FingerprintLegacy()` - returns MD5 fingerprint
  - `AuthorizedKey()` - returns public key in OpenSSH format
  - `Sign(message)` - signs a message
  - `Signer()` - returns ssh.Signer for authentication
  - `Info()` - returns displayable key information
- Added `LoadOrGenerateIdentityKey()` convenience function
- Added `DefaultIdentityKeyPath()` helper
- Added `IdentityKeyExists()` check
- Added `IdentityKeyPath` field to `SetupData`
- Added "identity-key" wizard step that:
  - Generates or loads existing key
  - Displays key location, fingerprint, and truncated public key
  - Shows warning to keep key secure
- Updated `ToBibConfig()` and `ToBibdConfig()` to include key path
- Comprehensive unit tests (13 tests) covering all functionality

### 2.2 Node Discovery

- [x] Implement `DiscoverNodes()` function that combines all discovery methods
- [x] Implement localhost port scanning (4000, 8080)
- [x] Implement Unix socket detection
- [x] Integrate mDNS discovery for `_bib._tcp.local`
- [x] Integrate P2P/DHT nearby peer discovery
- [x] Return discovered nodes with latency measurements
- [x] Create discovery progress UI component

**Files created:**
- `internal/discovery/discovery.go`
- `internal/discovery/localhost.go`
- `internal/discovery/mdns.go`
- `internal/discovery/p2p.go`
- `internal/discovery/progress.go`
- `internal/discovery/discovery_test.go`

**Implementation notes:**
- Created `discovery` package with multiple discovery methods:
  - **Localhost**: Scans ports 4000, 8080 and checks Unix sockets
  - **mDNS**: Uses hashicorp/mdns to discover `_bib._tcp.local` services
  - **P2P**: Placeholder for DHT-based discovery (requires P2P infrastructure)
- `DiscoveredNode` struct with Address, Method, Latency, NodeInfo, DiscoveredAt
- `DiscoveryMethod` type: local, mdns, p2p, manual, public
- `DiscoveryOptions` for configuring timeouts and enabled methods
- `Discoverer` struct with methods:
  - `Discover()` - runs all enabled methods in parallel
  - `DiscoverWithProgress()` - with progress callbacks
  - `DiscoverLocalhost()`, `DiscoverMDNS()` - individual methods
  - `CheckAddress()` - manual address verification
  - `MeasureLatency()` - TCP latency measurement
- Progress and formatting utilities:
  - `FormatDiscoveryResult()` - formatted output for display
  - `DiscoverySummary()` - brief summary string
- mDNS TXT record parsing for node info (name, version, mode, peer_id)
- `RegisterMDNSService()` for bibd to register itself
- Comprehensive unit tests (16 tests)

### 2.3 Node Selection UI

- [x] Create multi-select node list TUI component
- [x] Display discovered nodes with:
  - [x] Address
  - [x] Discovery method (local, mDNS, P2P)
  - [x] Latency
  - [x] Node info (if available)
- [x] Add bib.dev as separate "Public Network" option
- [x] Implement "Add Custom..." option for manual address entry
- [x] Implement "Select All Local" convenience button
- [x] Store selected nodes in config

**Files created:**
- `internal/tui/component/node_selector.go`
- `internal/tui/component/node_selector_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `NodeSelector` TUI component with:
  - Multi-select list with checkboxes
  - Cursor navigation (j/k or arrows)
  - Space to toggle selection
  - 'a' to select all local nodes
  - 'n' to deselect all
  - 'd' to set current as default
  - Custom address entry mode
- `NodeSelectorItem` struct with Node, Selected, IsDefault, Alias, Status, Error
- Discovery method icons (🏠 local, 📡 mDNS, 🌐 P2P, ☁️ public, ✎ manual)
- bib.dev shown as separate option with public network warning
- "Add custom address..." option for manual entry
- Auto-adds port 4000 if not specified
- Callbacks for selection changes
- Helper methods: SelectedItems(), SelectedNodes(), HasSelection(), SelectFirst()
- Added "node-discovery" and "node-selection" wizard steps
- `runNodeDiscovery()` method runs discovery in background
- Discovery results displayed with latency and method
- Selected nodes stored in SetupData.SelectedNodes
- Connection step shows summary and allows editing default address
- Comprehensive unit tests (20 tests)

### 2.4 bib.dev Confirmation

- [x] Create confirmation dialog for bib.dev connection
- [x] Explain implications (public network, visibility, data publishing)
- [x] Require explicit "Yes, Connect to Public Network" confirmation
- [x] Store confirmation status in setup data

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Added `bibDevConfirmed` field to `SetupWizardModel`
- Added "bib-dev-confirm" wizard step between node-selection and connection
- Step is dynamically shown only when bib.dev is selected in node selector
- Confirmation dialog explains:
  - Public identity visibility
  - Data publishing visibility
  - IP logging
  - Terms of Service
  - Use cases (collaboration, public datasets)
- Explicit "Yes, Connect to Public Network" vs "No, Go Back" buttons
- If user selects "No", goes back to node-selection and deselects bib.dev
- If user confirms, `m.bibDevConfirmed` and `m.data.BibDevConfirmed` are set
- Added `handleStepCompletion()` method for step-specific completion logic:
  - Handles bib-dev-confirm to check confirmation and go back on rejection
  - Handles node-selection to reset confirmation when bib.dev is selected

### 2.5 Multi-Node Configuration

- [x] Update config structure to support multiple nodes
- [x] Add `connection.favorite_nodes` array to bib config:
  ```yaml
  connection:
    default_node: "localhost:4000"
    favorite_nodes:
      - address: "localhost:4000"
        alias: "local"
        default: true
      - address: "bib.dev:4000"
        alias: "public"
        discovery_method: "public"
  ```
- [x] Implement default node selection
- [x] Remove legacy `server` field (no backward compatibility needed)

**Files modified:**
- `internal/config/types.go`
- `internal/config/loader.go`
- `internal/tui/setup.go`
- `internal/config/config_test.go`
- `cmd/bib/cmd/client.go`
- `cmd/bib/cmd/connect/connect.go`
- `cmd/bib/cmd/config/config_tui.go`

**Implementation notes:**
- Removed legacy `Server` field from `BibConfig`
- Updated `FavoriteNode` struct with new fields:
  - `Default bool` - marks this as the default node
  - `DiscoveryMethod string` - how the node was discovered (local, mdns, p2p, manual, public)
- Updated `ConnectionConfig` with:
  - `BibDevConfirmed bool` - user explicitly confirmed bib.dev connection
- Added helper methods to `BibConfig`:
  - `GetDefaultServerAddress()` - returns default address from DefaultNode or FavoriteNodes
  - `GetFavoriteNodes()` - returns configured FavoriteNodes
  - `HasBibDevNode()` - checks if bib.dev is in the config
  - `IsBibDevConfirmed()` - checks if user confirmed bib.dev
- Updated `LoadBib()` to ensure DefaultNode is set from FavoriteNodes if missing
- Updated `SaveBib()` to save new fields
- Updated `ToBibConfig()` to convert `SetupData.SelectedNodes` to `FavoriteNodes`
- Updated all code using old `cfg.Server` to use `GetDefaultServerAddress()` or `Connection.DefaultNode`
- Comprehensive unit tests covering all functionality

### 2.6 Connection Testing

- [x] Implement `TestConnection()` for each selected node
- [x] Display connection status, latency, node info
- [x] Handle connection failures gracefully
- [x] Offer retry/remove options for failed nodes

**Files created:**
- `internal/discovery/connection.go`
- `internal/discovery/connection_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `ConnectionTester` struct with methods:
  - `TestConnection()` - tests a single node
  - `TestConnections()` - tests multiple nodes in parallel
  - `TestNodes()` - tests discovered nodes
- `ConnectionTestResult` struct with:
  - Address, Status, Latency, NodeInfo, TLSInfo, Error, TestedAt
- `ConnectionStatus` constants:
  - connected, disconnected, timeout, refused, unreachable, auth_failed, tls_error, unknown
- `TLSInfo` struct for certificate information:
  - Enabled, Fingerprint, Subject, Issuer, NotAfter, Trusted
- Features:
  - TCP connectivity check before gRPC
  - TLS auto-detection via probe
  - gRPC health service integration (GetNodeInfo, Check)
  - Error classification (timeout, refused, TLS, auth, etc.)
  - Parallel testing for multiple nodes
- Helper functions:
  - `FormatConnectionResult()` - formats single result with icons
  - `FormatConnectionResults()` - formats multiple results with summary
  - `fingerprintCert()` - SHA256 certificate fingerprint
  - `classifyError()` - classifies errors into status codes
- Added "connection-test" wizard step:
  - Runs after "connection" step
  - Tests all selected nodes in parallel (30s total timeout)
  - Shows results with icons (✓ connected, ✗ failed, ⏱ timeout, etc.)
  - Displays latency, version, mode for connected nodes
  - Shows warning count for failed nodes
  - Retry option available
- Added `runConnectionTests()` method to SetupWizardModel
- Comprehensive unit tests (15 tests)

### 2.7 Authentication Testing

- [x] Implement `TestAuthentication()` for each node
- [x] Use generated identity key
- [x] Handle auto-registration if enabled on server
- [x] Display session info on success
- [x] Handle authentication failures

**Files created:**
- `internal/discovery/auth.go`
- `internal/discovery/auth_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `AuthTester` struct with methods:
  - `TestAuth()` - tests authentication with a single node
  - `TestAuths()` - tests multiple nodes in parallel
  - `WithTimeout()` - sets authentication timeout
  - `WithRegistrationInfo()` - sets name/email for auto-registration
- `AuthTestResult` struct with:
  - Address, Status, SessionToken, SessionInfo, ServerConfig, Error, Duration, TestedAt
- `AuthStatus` constants:
  - success, failed, no_key, key_rejected, not_registered, auto_registered, connection_error, unknown
- `SessionInfo` struct:
  - UserID, Username, Role, ExpiresAt, IsNewUser
- `ServerAuthConfig` struct:
  - AllowAutoRegistration, RequireEmail, SupportedKeyTypes
- Features:
  - SSH challenge-response authentication flow
  - Server auth config retrieval (GetAuthConfig)
  - Session token and user info extraction
  - Error classification (not registered, key rejected, connection error, etc.)
  - Auto-registration detection (IsNewUser flag)
- Helper functions:
  - `FormatAuthResult()` - formats single result with icons (✓, ✓+, ✗, ?, 🔑, ⚡)
  - `FormatAuthResults()` - formats multiple results with summary
  - `AuthSummary()` - brief summary string
  - `classifyAuthError()` - classifies errors into status codes
- Added "auth-test" wizard step:
  - Runs after "connection-test" step
  - Only tests against nodes that passed connection test
  - Uses identity key generated in earlier step
  - Shows results with icons and session info
  - Shows auto-registration count if applicable
  - Shows warning for failed authentications
  - Retry option available
- Added `runAuthTests()` method to SetupWizardModel
- Comprehensive unit tests (13 tests)

### 2.8 Network Health Check

- [x] Query peer count from each connected node
- [x] Display bootstrap connection status
- [x] Show DHT status
- [x] Create network health summary UI

**Files created:**
- `internal/discovery/network_health.go`
- `internal/discovery/network_health_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `NetworkHealthChecker` struct with methods:
  - `CheckHealth()` - checks health of a single node
  - `CheckHealthMultiple()` - checks multiple nodes in parallel
  - `GetSummary()` - creates aggregated summary from results
  - `WithTimeout()` - sets health check timeout
- `NetworkHealthResult` struct with:
  - Address, Status, NodeInfo, Network, Error, Duration, TestedAt
- `NetworkHealthStatus` constants:
  - good, degraded, poor, offline
- `NetworkStats` struct with:
  - ConnectedPeers, KnownPeers, BootstrapConnected, DHTRoutingTableSize
  - ActiveStreams, BytesSent, BytesReceived
- `NetworkHealthSummary` struct with:
  - TotalNodes, HealthyNodes, DegradedNodes, OfflineNodes
  - TotalConnectedPeers, AverageConnectedPeers, BootstrapConnected
  - OverallStatus
- Features:
  - gRPC GetNodeInfo with IncludeNetwork flag
  - Peer count extraction from NetworkInfo
  - Bootstrap connection status detection
  - DHT routing table size retrieval
  - Health status determination based on:
    - Good: peers > 0 and bootstrap connected
    - Degraded: peers > 0 or bootstrap connected (not both)
    - Poor: no peers, no bootstrap
    - Offline: connection failed
  - Aggregated summary with overall status
- Helper functions:
  - `FormatNetworkHealthResult()` - formats single result with icons (✓, ⚠, ✗, ⊘)
  - `FormatNetworkHealthResults()` - formats multiple results with summary
  - `FormatNetworkHealthSummary()` - formats summary with status details
  - `NetworkHealthStatusIcon()` - returns icon for status
  - `NetworkHealthBrief()` - brief one-line summary
- Added "network-health" wizard step:
  - Runs after "auth-test" step
  - Only checks nodes that passed connection test
  - Shows peer count, bootstrap status, DHT info per node
  - Shows aggregated summary
  - Provides recommendations based on status
  - Retry option available
- Added `runNetworkHealthCheck()` method to SetupWizardModel
- Comprehensive unit tests (17 tests)

### 2.9 Quick Start Mode (CLI)

- [x] Implement `setupBibQuick()` function
- [x] Prompt only for name and email
- [x] Auto-generate identity key
- [x] Auto-discover and select local nodes
- [x] Prompt for bib.dev confirmation if no local nodes
- [x] Test connections
- [x] Save minimal config

**Files created:**
- `cmd/bib/cmd/setup/setup_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Implemented complete `setupBibQuick()` function with 9 steps:
  1. **Name/Email Prompt**: Simple huh form asking for essential identity info
  2. **Identity Key Generation**: Auto-generates Ed25519 key using `auth.GenerateIdentityKey()`
  3. **Node Discovery**: Runs 5-second discovery for local/mDNS nodes
  4. **bib.dev Prompt**: If no local nodes found, prompts for public network connection
  5. **Node Configuration**: Auto-selects discovered local nodes, adds bib.dev if confirmed
  6. **Connection Testing**: Tests all configured nodes with 5s timeout per node
  7. **Default Preferences**: Sets table output, color enabled, info log level
  8. **Config Save**: Generates and saves config to `~/.config/bib/config.yaml`
  9. **Summary Display**: Shows identity, key fingerprint, configured nodes, next steps
- Features:
  - Minimal user interaction (only name/email required)
  - Auto-discovery of local bibd instances
  - Clear progress output with icons
  - Connection test results displayed
  - Helpful next steps shown based on configuration
  - Graceful cancellation with Ctrl+C
- User-friendly output with:
  - Step icons (🚀, 👤, 🔑, 🔍, 🌐, 🔌, 💾, ✅)
  - Status indicators (✓ for success, ✗ for failure)
  - Summary box with configuration details
  - Context-aware next steps
- Added unit tests for:
  - DeploymentTarget string/IsValid
  - ValidReconfigureSections for bib/bibd
  - Setup flag variable access

---

## Phase 3: Daemon Setup - Local

### 3.1 Deployment Target Selection

- [x] Add deployment target selection as first wizard step
- [x] Create deployment target selector UI component
- [x] Detect available targets:
  - [x] Local: always available
  - [x] Docker: check for `docker` command
  - [x] Podman: check for `podman` command
  - [x] Kubernetes: check for `kubectl` command and valid context
- [x] Show detection status for each target

**Files created:**
- `internal/deploy/detect.go`
- `internal/deploy/detect_test.go`
- `internal/tui/component/target_selector.go`
- `internal/tui/component/target_selector_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `deploy` package with `TargetDetector`:
  - `DetectAll()` - detects all targets in parallel
  - `DetectLocal()` - always available, shows OS/arch and init system
  - `DetectDocker()` - checks docker command and daemon, gets version and compose status
  - `DetectPodman()` - checks podman command, detects rootful/rootless mode
  - `DetectKubernetes()` - checks kubectl, current context, cluster connectivity, CloudNativePG
- `TargetInfo` struct with:
  - Type, Available, Version, Status, Error, Details map
- `TargetType` constants: local, docker, podman, kubernetes
- Helper functions:
  - `FormatTargetInfo()` - formats with icons (🖥️, 🐳, 🦭, ☸️)
  - `TargetDisplayName()` - human-readable names
  - `TargetDescription()` - descriptions for each target
- Created `TargetSelector` TUI component:
  - Bubble Tea model with spinner for detection
  - Detection runs in parallel on init
  - Shows status icons (✓ available, ✗ unavailable)
  - Keyboard navigation (up/down, j/k)
  - Shows target details for selected item
  - Helper methods: SelectedTarget(), SelectedTargetType(), IsSelectedAvailable(), etc.
- Added "deployment-target" wizard step:
  - Runs after "identity-key" step (daemon only)
  - Runs target detection on step entry
  - Shows all detected targets with status
  - Select dropdown to choose target
  - Stores selection in `data.DeploymentTarget`
- Added `runTargetDetection()` method to SetupWizardModel
- Added `targetSelector` and `targetDetected` fields to model
- Comprehensive unit tests:
  - 13 tests for detect.go
  - 15 tests for target_selector.go

### 3.2 Local PostgreSQL Options

- [x] Update storage step to show local-specific PostgreSQL options:
  - [x] SQLite (Proxy/Selective only)
  - [x] Managed Container (Docker/Podman)
  - [x] Local Installation
  - [x] Remote Server
- [x] Implement managed container PostgreSQL setup
- [x] Implement local PostgreSQL connection test
- [x] Implement remote PostgreSQL connection test

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`
- `cmd/bib/cmd/setup/setup_test.go`

**Implementation notes:**
- Updated storage step to show deployment-specific PostgreSQL options:
  - **Local deployment**: SQLite, Managed Container (Docker/Podman), Local Installation, Remote Server
  - **Docker/Podman**: SQLite, Managed Container (recommended)
  - **Kubernetes**: SQLite, StatefulSet, CloudNativePG Operator, External Server
- Added "postgres-config" wizard step:
  - Shows after storage step if PostgreSQL is selected
  - Different form fields based on deployment mode:
    - **Container mode**: Database name, user, password (auto-generate option)
    - **Local mode**: Host, port, database, user, password, SSL mode
    - **Remote mode**: Same as local with remote-focused description and SSL default to require
  - Port validation (1-65535)
  - SSL mode options: disable, require, verify-ca, verify-full
- Added "postgres-test" wizard step:
  - Shows after postgres-config step if PostgreSQL is selected
  - For container deployments: Shows skip message (tested after deployment)
  - For local/remote: Runs TCP connection test
  - Shows success with connection time, or failure with troubleshooting tips
  - Retry option available
- Added `PostgresTestResult` struct:
  - Success, ServerVersion, Database, User, Duration, Error
- Added `getPostgresDeploymentMode()` helper method:
  - Returns mode based on storage backend selection: container, local, remote, cnpg, statefulset
- Added `runPostgresTest()` method:
  - Converts port string to int
  - Sets defaults for empty fields
  - Builds connection string and tests TCP connection
- Added `testPostgresConnection()` function:
  - Parses host/port from connection string
  - Tests TCP connectivity with 5s timeout
  - Returns result with success/error status
- Added `validatePort()` validation function
- Added `postgresPortStr` field to model for form binding
- Added unit tests for:
  - validatePort (10 cases)
  - truncateString (6 cases)
  - PostgresTestResult fields
  - testPostgresConnection with invalid host

### 3.3 Bootstrap Peers with bib.dev Confirmation

- [x] Update bootstrap peer step to require bib.dev confirmation
- [x] Create confirmation dialog explaining public network implications
- [x] Allow "No, Private Only" option

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`
- `cmd/bib/cmd/setup/setup_test.go`

**Implementation notes:**
- Added three new wizard steps for daemon P2P setup:
  1. **bootstrap-peers**: Initial selection of public vs private network
  2. **bootstrap-confirm**: Detailed confirmation for public network
  3. **custom-bootstrap**: Add custom bootstrap peers in multiaddr format
- Added `customBootstrapInput` field to SetupWizardModel for form binding
- **bootstrap-peers step**:
  - Shows description of bootstrap peer options
  - Confirm: "Use bib.dev public bootstrap?"
  - Options: "Yes, use public network" / "No, private only"
  - Sets `UsePublicBootstrap` in data
- **bootstrap-confirm step**:
  - Only shows if `UsePublicBootstrap` is true
  - Detailed warning about public network implications:
    - Node discoverable worldwide
    - Public identity visible
    - Published data accessible
  - Confirm: "Connect to bib.dev public network?"
  - Options: "Yes, Connect to Public Network" / "No, Private Network Only"
  - Sets `BibDevConfirmed` in data
  - If declined, automatically sets `UsePublicBootstrap = false`
- **custom-bootstrap step**:
  - Shows current custom bootstrap peers (truncated if long)
  - Shows status (public confirmed, public not confirmed, private only)
  - Input field for adding custom peer in multiaddr format
  - Validates multiaddr starts with `/`
  - Calls `AddCustomBootstrapPeer()` on data
- Updated `handleStepCompletion()` with:
  - `bootstrap-confirm` case: disables public bootstrap if declined
  - `custom-bootstrap` case: adds valid peer to list, clears input
- Added unit test for multiaddr validation (6 cases)
- Steps skip correctly based on P2P enabled state

### 3.4 Service Installation

- [x] Implement systemd service file generation (Linux)
- [x] Implement launchd plist generation (macOS)
- [x] Implement Windows Service installation
- [x] Add user vs system service option (Linux)
- [x] Enable and start service after installation

**Files created:**
- `internal/deploy/local/service.go`
- `internal/deploy/local/service_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `local` package under `internal/deploy/` with full service installation support:
  - `ServiceType` constants: systemd, launchd, windows
  - `ServiceConfig` struct with all configuration options:
    - Name, DisplayName, Description
    - ExecutablePath, ConfigPath, WorkingDirectory
    - User, Group, UserService flag
    - Environment map, RestartPolicy, RestartDelaySec
  - `DefaultServiceConfig()` - sensible defaults for bibd
  - `NewServiceInstaller()` - creates installer with config
  - `DetectServiceType()` - auto-detects based on OS:
    - Linux → systemd
    - macOS → launchd  
    - Windows → windows service
- **Systemd generation** (`generateSystemd()`):
  - [Unit] section with network dependencies
  - [Service] section with ExecStart, User, Group, Restart policy
  - Security hardening (NoNewPrivileges, ProtectSystem, etc.)
  - [Install] section with multi-user.target or default.target
  - Environment variable support
- **Launchd generation** (`generateLaunchd()`):
  - Full plist XML format
  - ProgramArguments with executable and config
  - RunAtLoad, KeepAlive settings
  - ThrottleInterval for restart delay
  - EnvironmentVariables support
  - StandardOutPath/StandardErrorPath for logging
- **Windows generation** (`generateWindowsPowerShell()`):
  - NSSM (recommended) installation commands
  - Native `New-Service` PowerShell commands
  - Start-Service and Get-Service commands
- `GetServiceFilePath()` - returns correct path:
  - systemd: `/etc/systemd/system/` or `~/.config/systemd/user/`
  - launchd: `/Library/LaunchDaemons/` or `~/Library/LaunchAgents/`
- `InstallInstructions()` - human-readable install steps
- Added "service-install" wizard step:
  - Only shows for local daemon deployments
  - Detects service type automatically
  - Shows preview of generated service file
  - Options: Install service (yes/no), User service (yes/no)
  - Updates ServiceInstaller.Config.UserService based on selection
- Added fields to SetupWizardModel:
  - `serviceInstaller *local.ServiceInstaller`
  - `installService bool`
  - `userService bool`
- Comprehensive unit tests (12 tests covering all platforms)

### 3.5 Local Quick Start

- [x] Implement quick start for local deployment
- [x] Minimal prompts (name, email)
- [x] SQLite + Proxy mode defaults
- [x] Auto-install service and start

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Implemented `setupBibdQuick()` router function:
  - Routes to `setupBibdQuickLocal()` for local target
  - Placeholder messages for Docker/Podman (Phase 4.5) and Kubernetes (Phase 5.6)
- Implemented `setupBibdQuickLocal()` with 7 steps:
  1. **Name/Email Prompt**: Simple huh form for identity
  2. **Identity Key Generation**: Auto-generates Ed25519 key for bibd
  3. **Public Network Prompt**: Simple yes/no for bib.dev connection
  4. **Server Defaults**: Sets host=0.0.0.0, port=4000, no TLS
  5. **Config Save**: Marshals BibdConfig to YAML and saves
  6. **Service Installation**: Optional systemd/launchd/Windows service
  7. **Summary Display**: Shows configuration and next steps
- Quick setup defaults:
  - SQLite storage backend
  - Proxy P2P mode
  - No TLS (for simplicity)
  - Info log level with pretty format
- Service installation flow:
  - Detects service type automatically
  - Asks user vs system service (Linux/macOS)
  - Generates and saves service file
  - Shows installation instructions
- User-friendly output with icons and clear progress
- Context-aware next steps based on configuration
- Uses yaml.v3 for config marshaling
- Added yaml import to setup.go

**Usage:**
```bash
# Quick local daemon setup
bib setup --daemon --quick
bib setup --daemon -q

# Quick setup for specific target
bib setup --daemon --quick --target=local
```

---

## Phase 4: Daemon Setup - Docker

### 4.1 Docker Detection

- [x] Check for `docker` command availability
- [x] Verify Docker daemon is running
- [x] Check Docker version compatibility
- [x] Display Docker info in wizard

**Files created:**
- `internal/deploy/docker/detect.go`
- `internal/deploy/docker/detect_test.go`

**Implementation notes:**
- Created `docker` package under `internal/deploy/` with detection support:
  - `Detector` struct with configurable timeout
  - `DockerInfo` struct containing:
    - Available, DaemonRunning flags
    - Version, APIVersion, ServerOS
    - ComposeAvailable, ComposeVersion, ComposeCommand
    - Error message
  - `NewDetector()` with 10s default timeout
  - `Detect()` method that checks:
    - docker command existence
    - Docker daemon status via `docker version`
    - Docker API version
    - Server OS
    - docker-compose (standalone) availability
    - docker compose (plugin) availability - preferred
- Helper functions:
  - `FormatDockerInfo()` - formats info with icons (🐳, ✓, ✗, ⚠)
  - `IsUsable()` - returns true if Docker can be used for deployment
  - `GetComposeCommand()` - returns command parts for compose
- Comprehensive unit tests (10 tests)

### 4.2 Docker Compose Generation

- [x] Create `docker-compose.yaml` template
- [x] Include bibd service configuration
- [x] Include PostgreSQL service (if Full mode)
- [x] Configure volumes for data persistence
- [x] Configure network for inter-container communication
- [x] Generate `.env` file for environment variables
- [x] Generate `config/config.yaml`
- [x] Generate `config/identity.pem`

**Files created:**
- `internal/deploy/docker/compose.go`
- `internal/deploy/docker/compose_test.go`

**Implementation notes:**
- Created `ComposeConfig` struct with all configuration options:
  - ProjectName, BibdImage, BibdTag
  - P2PEnabled, P2PMode
  - StorageBackend (sqlite/postgres)
  - PostgreSQL settings: Image, Tag, Database, User, Password
  - Network ports: APIPort, P2PPort, MetricsPort
  - TLSEnabled, Bootstrap settings
  - Name, Email for identity
  - OutputDir, ExtraEnv
- `DefaultComposeConfig()` with sensible defaults:
  - bibd image: ghcr.io/bib-project/bibd:latest
  - postgres image: postgres:16-alpine
  - Ports: 4000 (API), 4001 (P2P), 9090 (metrics)
- `ComposeGenerator` with methods:
  - `Generate()` - generates all files, returns `GeneratedFiles`
  - `generateCompose()` - docker-compose.yaml with:
    - bibd service with ports, volumes, healthcheck
    - postgres service (if needed) with healthcheck
    - Named volumes for persistence
    - Bridge network for P2P
    - Environment variable substitution
  - `generateEnvFile()` - .env with all config
  - `generateConfigYaml()` - bibd config.yaml
- `GeneratedFiles` struct with:
  - `Files` map of filename to content
  - `WriteToDir()` method to write all files
- Helper functions:
  - `GeneratePassword()` - generates random password
  - `GetComposeUpCommand()`, `GetComposeDownCommand()`, `GetComposeLogsCommand()`
  - `FormatStartInstructions()` - human-readable deployment instructions
- Features:
  - Auto-generates postgres password if not set
  - SQLite mode: simpler compose without postgres
  - PostgreSQL mode: full compose with depends_on and healthchecks
  - Supports custom environment variables
  - Supports custom bootstrap peers
- Comprehensive unit tests (13 tests)

### 4.3 Docker PostgreSQL Setup

- [x] Configure PostgreSQL as separate service in compose
- [x] Set up internal networking between bibd and postgres
- [x] Configure persistent volume for postgres data
- [x] Generate secure database credentials

**Files modified:**
- `internal/deploy/docker/compose.go`

**Implementation notes:**
- Enhanced `ComposeConfig` with additional PostgreSQL options:
  - `PostgresSSLMode` - SSL mode (disable, require, verify-ca, verify-full)
  - `PostgresMaxConns` - Maximum connections (default: 100)
  - `PostgresSharedBufs` - shared_buffers setting (default: 128MB)
  - `PostgresWorkMem` - work_mem setting (default: 4MB)
  - `PostgresExposePort` - Whether to expose postgres port externally
  - `PostgresExternalPort` - External port for postgres (default: 5432)
- Enhanced docker-compose.yaml template for PostgreSQL:
  - Optional external port exposure
  - POSTGRES_INITDB_ARGS for encoding settings
  - Custom postgres.conf mount when memory settings configured
  - Always uses bibd-network for internal communication
  - Proper healthcheck with pg_isready
- Added `generatePostgresConf()` method:
  - Connection settings (listen_addresses, max_connections)
  - Memory settings (shared_buffers, work_mem, maintenance_work_mem, effective_cache_size)
  - WAL settings (wal_level, max_wal_size, min_wal_size)
  - Logging settings
  - Locale settings (UTC timezone, C locale)
- Added `generateInitSQL()` method:
  - Enables uuid-ossp and pgcrypto extensions
  - Creates bib schema
  - Grants permissions to bibd user
  - Sets default search path
- Updated `Generate()` to include postgres.conf and init.sql for PostgreSQL mode

### 4.4 Docker Deployment

- [x] Implement `DeployDocker()` function
- [x] Create output directory structure
- [x] Write all generated files
- [x] Run `docker compose up -d`
- [x] Wait for containers to be healthy
- [x] Display container status

**Files created:**
- `internal/deploy/docker/deploy.go`
- `internal/deploy/docker/deploy_test.go`

**Implementation notes:**
- Created `DeployConfig` struct with options:
  - ComposeConfig, OutputDir
  - AutoStart, PullImages, WaitForHealthy
  - HealthTimeout (default: 120s)
  - Verbose mode
- Created `Deployer` struct with methods:
  - `Deploy()` - full deployment workflow:
    1. Detect Docker availability
    2. Generate all files
    3. Create output directory
    4. Write files
    5. Generate identity key placeholder
    6. Pull images (optional)
    7. Start containers (optional)
    8. Wait for healthy (optional)
    9. Show status
  - `Stop()` - stops containers
  - `Logs()` - gets container logs
  - `Status()` - gets deployment status
- Created `DeployResult` struct with:
  - Success, OutputDir, FilesGenerated
  - ContainersStarted, ContainersHealthy
  - Error message, Logs array
- Created `DeploymentStatus` and `ContainerStatus` structs
- Added `FormatStatus()` for display with icons
- Helper methods:
  - `pullImages()` - runs docker compose pull
  - `startContainers()` - runs docker compose up -d
  - `waitForHealthy()` - polls until containers healthy or timeout
  - `checkHealth()` - checks container health status
  - `getContainerStatus()` - gets docker compose ps output
  - `generateIdentityKey()` - creates placeholder identity key
- Comprehensive unit tests (12 tests)

### 4.5 Docker Quick Start

- [x] Implement quick start for Docker deployment
- [x] Minimal prompts (name, email)
- [x] SQLite + Proxy mode defaults
- [x] Auto-generate and start containers

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Added `docker` package import
- Updated `setupBibdQuick()` to route to:
  - `setupBibdQuickDocker()` for Docker target
  - `setupBibdQuickPodman()` for Podman target
- Implemented `setupBibdQuickDocker()`:
  1. Detect Docker availability
  2. Prompt for name/email
  3. Prompt for public network (bib.dev)
  4. Prompt for output directory (default: ~/bibd-docker)
  5. Prompt for auto-start
  6. Deploy using Docker deployer
  7. Show summary with commands
- Implemented `setupBibdQuickPodman()`:
  1. Check for Podman (or Docker compatibility)
  2. Prompt for name/email
  3. Prompt for public network
  4. Prompt for output directory (default: ~/bibd-podman)
  5. Generate files (no auto-start for Podman)
  6. Show summary with Podman-specific commands
- Features:
  - Minimal prompts (4-5 questions)
  - SQLite + Proxy mode defaults
  - Auto-detection of compose command
  - Clear status indicators
  - Helpful command examples

**Usage:**
```bash
# Docker quick setup
bib setup --daemon --quick --target=docker

# Podman quick setup
bib setup --daemon --quick --target=podman
```

---

## Phase 5: Daemon Setup - Podman

### 5.1 Podman Detection

- [x] Check for `podman` command availability
- [x] Detect rootful vs rootless mode
- [x] Check for `podman-compose` availability
- [x] Display Podman info in wizard

**Files created:**
- `internal/deploy/podman/detect.go`
- `internal/deploy/podman/detect_test.go`

**Implementation notes:**
- Created `podman` package under `internal/deploy/` with detection support:
  - `Detector` struct with configurable timeout
  - `PodmanInfo` struct containing:
    - Available, Version, APIVersion
    - Rootless flag and RootlessUID
    - SocketPath for API access
    - MachineRunning, MachineName (for macOS/Windows)
    - ComposeAvailable, ComposeVersion, ComposeCommand
    - PodmanComposeAvailable (plugin detection)
    - KubePlayAvailable (podman kube play support)
    - Error message
  - `NewDetector()` with 10s default timeout
  - `Detect()` method that checks:
    - podman command existence
    - Podman version and API version
    - Rootless mode detection via euid and XDG_RUNTIME_DIR
    - Socket path detection
    - Machine status (macOS/Windows)
    - podman-compose standalone availability
    - podman compose plugin availability
    - podman kube play availability
- Helper functions:
  - `FormatPodmanInfo()` - formats info with icons (🦭, ✓, ✗)
  - `IsUsable()` - returns true if Podman can be used
  - `GetComposeCommand()` - returns command parts for compose
  - `PreferredDeployMethod()` - returns "kube", "compose", or "none"
- Comprehensive unit tests (12 tests)

### 5.2 Podman Mode Selection

- [x] Add rootful/rootless mode selection
- [x] Add pod vs compose style selection
- [x] Validate selections based on detected capabilities

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Updated `setupBibdQuickPodman()` with mode selection:
  - Auto-detects rootless/rootful mode from PodmanInfo
  - If both compose and kube play available, prompts user to choose
  - Otherwise uses preferred deploy method automatically
- Deploy style options:
  - "pod" - Kubernetes-style YAML via podman kube play
  - "compose" - Docker Compose compatible via podman-compose/podman compose

### 5.3 Podman Pod Generation

- [x] Create Kubernetes-style pod YAML template
- [x] Include bibd container configuration
- [x] Include PostgreSQL container (if Full mode)
- [x] Configure shared volumes
- [x] Generate convenience `start.sh` script

**Files created:**
- `internal/deploy/podman/pod.go`
- `internal/deploy/podman/pod_test.go`

**Implementation notes:**
- Created `PodConfig` struct with all configuration options:
  - PodName, BibdImage, BibdTag
  - P2PEnabled, P2PMode
  - StorageBackend (sqlite/postgres)
  - PostgreSQL settings
  - Rootless flag and PortOffset
  - DeployStyle ("pod" or "compose")
  - Bootstrap settings, Identity, OutputDir, ExtraEnv
- `DefaultPodConfig()` with sensible defaults:
  - bibd image: ghcr.io/bib-project/bibd:latest
  - postgres image: docker.io/library/postgres:16-alpine
  - Ports: 4000 (API), 4001 (P2P), 9090 (metrics)
  - Rootless: true by default
  - DeployStyle: "pod" by default
- `PodGenerator` with methods:
  - `Generate()` - generates all files
  - `generatePodYaml()` - Kubernetes-style pod YAML for podman kube play
  - `generateCompose()` - podman-compose.yaml with rootless options
  - `generateEnvFile()` - .env with all config
  - `generateConfigYaml()` - bibd config.yaml
  - `generateStartScript()` - start.sh convenience script
  - `generateStopScript()` - stop.sh convenience script
  - `generateStatusScript()` - status.sh convenience script
- Features:
  - Auto-generates postgres password if not set
  - Rootless port offset handling (ports < 1024)
  - SELinux labels (:Z) in compose volumes
  - userns_mode: keep-id for rootless
  - Comprehensive pod YAML with resource limits
- Comprehensive unit tests (17 tests)

### 5.4 Podman Compose Generation

- [x] Create `podman-compose.yaml` template
- [x] Similar structure to Docker compose
- [x] Handle rootless-specific port considerations

**Implementation notes (in pod.go):**
- `generateCompose()` method creates podman-compose.yaml with:
  - Same structure as Docker compose
  - SELinux volume labels (:Z) for Podman compatibility
  - userns_mode: keep-id for rootless containers
  - Port offset support for rootless mode
  - Depends_on for PostgreSQL if needed

### 5.5 Podman Deployment

- [x] Implement `DeployPodman()` function
- [x] Create output directory structure
- [x] Write all generated files
- [x] Run appropriate start command (pod or compose)
- [x] Wait for containers to be running
- [x] Display container/pod status

**Files created:**
- `internal/deploy/podman/deploy.go`
- `internal/deploy/podman/deploy_test.go`

**Implementation notes:**
- Created `DeployConfig` struct with options:
  - PodConfig, OutputDir
  - AutoStart, PullImages, WaitForRunning
  - WaitTimeout (default: 120s)
  - Verbose mode
- Created `Deployer` struct with methods:
  - `Deploy()` - full deployment workflow:
    1. Detect Podman availability and capabilities
    2. Auto-detect rootless mode
    3. Select deploy style based on capabilities
    4. Generate all files
    5. Create output directory
    6. Write files with proper permissions (scripts executable)
    7. Generate identity key placeholder
    8. Pull images (optional)
    9. Start containers using pod or compose method
    10. Wait for running (optional)
    11. Show status
  - `Stop()` - stops containers
  - `Logs()` - gets container logs
  - `Status()` - gets deployment status
- Created `DeployResult` struct with:
  - Success, OutputDir, FilesGenerated
  - ContainersStarted, ContainersRunning
  - DeployStyle, Error message, Logs array
- Created `DeploymentStatus` and `ContainerStatus` structs
- Added `FormatStatus()` for display with icons
- Helper methods:
  - `pullImages()` - runs podman pull
  - `startPod()` - runs podman kube play
  - `startCompose()` - runs podman-compose/podman compose up
  - `waitForRunning()` - polls until containers running
  - `checkRunning()` - checks container status
  - `getContainerStatus()` - gets podman ps output
- Comprehensive unit tests (10 tests)

### 5.6 Podman Quick Start

- [x] Implement quick start for Podman deployment
- [x] Auto-detect rootful/rootless
- [x] Default to pod style
- [x] Minimal prompts, auto-start

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Added `podman` package import
- Completely rewrote `setupBibdQuickPodman()` to use native Podman deployer:
  1. Detect Podman with full capability info
  2. Show rootless/rootful mode and available features
  3. Prompt for name/email
  4. Prompt for public network (bib.dev)
  5. If both compose and kube available, prompt for style
  6. Prompt for output directory (default: ~/bibd-podman)
  7. Prompt for auto-start
  8. Deploy using Podman deployer
  9. Show comprehensive summary with commands
- Features:
  - Full rootless/rootful detection
  - Deploy style selection when both available
  - Generates convenience scripts (start.sh, stop.sh, status.sh)
  - Auto-start containers if requested
  - Clear status indicators and command examples

**Usage:**
```bash
# Podman quick setup
bib setup --daemon --quick --target=podman
bib setup -d -q -t podman
```

**Example Output:**
```
🦭 Quick Setup - bibd (Podman)

🔍 Detecting Podman...
   ✓ Podman 4.7.0
   ✓ Rootless mode (UID 1000)
   ✓ Compose: podman compose
   ✓ Kube Play available

[Deployment Style]
  • Pod (Kubernetes-style YAML)
  • Compose (Docker Compose compatible)

[Name, Email, Network, Output Dir, Auto-start prompts...]

✅ Podman Quick Setup Complete!
──────────────────────────────────────────────────
  Identity:    John Doe <john@example.com>
  Output:      /home/user/bibd-podman
  Storage:     SQLite (proxy mode)
  Deploy:      pod
  Mode:        Rootless
  Network:     Public (bib.dev)
  Status:      🟢 Running

Commands:
  cd /home/user/bibd-podman
  • Start:  ./start.sh
  • Stop:   ./stop.sh
  • Status: ./status.sh

Or manually:
  podman kube play pod.yaml
  podman kube down pod.yaml

  • Connect CLI: bib setup
```

---

## Phase 6: Daemon Setup - Kubernetes

### 6.1 Kubernetes Detection

- [x] Check for `kubectl` command availability
- [x] Get current context
- [x] Verify cluster connectivity
- [x] Display cluster info in wizard

**Files created:**
- `internal/deploy/kubernetes/detect.go`
- `internal/deploy/kubernetes/detect_test.go`

**Implementation notes:**
- Created `kubernetes` package under `internal/deploy/` with detection support:
  - `Detector` struct with configurable timeout, kubeconfig, and context
  - `KubeInfo` struct containing:
    - Available, ClusterReachable flags
    - CurrentContext, ClusterName, ServerURL, ServerVersion
    - ClientVersion, Namespace
    - CloudNativePGAvailable, CloudNativePGVersion
    - IngressControllerAvailable, IngressClassName
    - StorageClasses, DefaultStorageClass
    - Error message
  - `NewDetector()` with 15s default timeout
  - `WithTimeout()`, `WithKubeconfig()`, `WithContext()` builder methods
  - `Detect()` method that checks:
    - kubectl command existence
    - kubectl version (client and server)
    - Current context
    - Cluster connectivity
    - CloudNativePG operator (CRD detection)
    - Ingress controller (ingress classes)
    - Storage classes with default
- Helper functions:
  - `FormatKubeInfo()` - formats info with icons (☸️, ✓, ✗, ⚠)
  - `IsUsable()` - returns true if Kubernetes can be used
  - `GetContexts()` - returns all available contexts
  - `GetNamespaces()` - returns all namespaces
- Comprehensive unit tests (10 tests)

### 6.2 Kubernetes Configuration

- [x] Add namespace configuration
- [x] Add output directory configuration
- [x] Add output options:
  - [x] Generate and apply
  - [x] Generate only
  - [x] Generate Helm values only
- [x] Add external access configuration:
  - [x] None (internal only)
  - [x] LoadBalancer
  - [x] NodePort
  - [x] Ingress (with hostname)

**Implementation notes (in manifests.go):**
- `ManifestConfig` struct with comprehensive options:
  - Namespace, BibdImage, BibdTag, Replicas
  - P2PEnabled, P2PMode, StorageBackend
  - PostgresMode: statefulset, cloudnativepg, external
  - PostgreSQL settings (Image, Tag, Database, User, Password, Host, Port)
  - StorageClass, PVCSize
  - ServiceType: ClusterIP, LoadBalancer, NodePort
  - NodePort for NodePort services
  - IngressHost, IngressClass, IngressTLS, TLSSecretName
  - Bootstrap settings, Identity, OutputDir, OutputMode
  - Labels, Annotations

### 6.3 Kubernetes PostgreSQL Options

- [x] Add StatefulSet PostgreSQL option
- [x] Add CloudNativePG option (with operator detection)
- [x] Add external PostgreSQL option
- [x] Configure storage class and PVC size for StatefulSet

**Implementation notes:**
- PostgresMode field supports: "statefulset", "cloudnativepg", "external"
- StatefulSet mode: Full StatefulSet with PVC template
- CloudNativePG mode: Creates CNPG Cluster CR with credentials secret
- External mode: Uses PostgresHost and PostgresPort for connection string
- Storage class and PVC size configurable for both StatefulSet and CloudNativePG

### 6.4 Kubernetes Manifest Generation

- [x] Create `namespace.yaml` template
- [x] Create `configmap.yaml` template
- [x] Create `secret.yaml` template
- [x] Create `bibd-deployment.yaml` template (or statefulset)
- [x] Create `bibd-service.yaml` template
- [x] Create `bibd-ingress.yaml` template (optional)
- [x] Create PostgreSQL StatefulSet templates (if selected):
  - [x] `postgres-statefulset.yaml`
  - [x] `postgres-service.yaml`
  - [x] `postgres-pvc.yaml`
  - [x] `postgres-secret.yaml`
- [x] Create `kustomization.yaml` template
- [x] Create `values.yaml` for future Helm chart

**Files created:**
- `internal/deploy/kubernetes/manifests.go`
- `internal/deploy/kubernetes/manifests_test.go`

**Implementation notes:**
- `ManifestGenerator` with `Generate()` method that creates all manifests
- Generated files:
  - `namespace.yaml` - Kubernetes namespace
  - `configmap.yaml` - bibd configuration as ConfigMap
  - `secrets.yaml` - PostgreSQL credentials, DATABASE_URL
  - `bibd-deployment.yaml` - Deployment with probes, resources, volumes
  - `bibd-service.yaml` - Service (ClusterIP/LoadBalancer/NodePort)
  - `bibd-ingress.yaml` - Ingress if hostname configured
  - `postgres-statefulset.yaml` - PostgreSQL StatefulSet with PVC
  - `postgres-service.yaml` - PostgreSQL headless service
  - `cloudnativepg-cluster.yaml` - CloudNativePG Cluster CR
  - `kustomization.yaml` - Kustomize configuration
  - `apply.sh` - Convenience apply script
  - `delete.sh` - Convenience delete script
- Features:
  - ServiceAccount for bibd
  - PVC for data persistence
  - Liveness and readiness probes
  - Resource requests and limits
  - Proper labeling for kustomize
  - Base64 encoding for secrets
- Comprehensive unit tests (20 tests)

### 6.5 CloudNativePG Support

- [x] Detect CloudNativePG operator installation
- [x] Create CloudNativePG Cluster CR template
- [x] Configure backup settings (optional)

**Implementation notes:**
- Detection via CRD: `clusters.postgresql.cnpg.io`
- Version detection from operator deployment image
- Cluster CR includes:
  - 1 instance (configurable)
  - PostgreSQL parameters (max_connections, shared_buffers)
  - Bootstrap initdb configuration
  - Storage size and class
  - Credentials secret reference

### 6.6 Kubernetes Deployment

- [x] Implement `DeployKubernetes()` function
- [x] Create output directory structure
- [x] Write all generated manifests
- [x] Optionally apply with `kubectl apply -k`
- [x] Wait for pods to be ready (if applied)
- [x] Display deployment status
- [x] Show external access info (LoadBalancer IP, etc.)

**Files created:**
- `internal/deploy/kubernetes/deploy.go`
- `internal/deploy/kubernetes/deploy_test.go`

**Implementation notes:**
- Created `DeployConfig` struct with options:
  - ManifestConfig, OutputDir
  - AutoApply, WaitForReady
  - WaitTimeout (default: 300s)
  - Verbose mode
- Created `Deployer` struct with methods:
  - `Deploy()` - full deployment workflow:
    1. Detect Kubernetes availability and cluster connectivity
    2. Check for CloudNativePG if needed
    3. Generate all manifests
    4. Create output directory
    5. Write files with proper permissions
    6. Apply manifests with kubectl apply -k (optional)
    7. Wait for rollout status (optional)
    8. Get external IP for LoadBalancer
    9. Show pod status
  - `Delete()` - deletes deployment with kubectl delete -k
  - `Logs()` - gets deployment logs
  - `Status()` - gets deployment status
- Created `DeployResult` struct with:
  - Success, OutputDir, FilesGenerated
  - ManifestsApplied, PodsReady
  - ExternalIP, IngressURL
  - Error message, Logs array
- Created `DeploymentStatus` and `PodStatus` structs
- Added `FormatStatus()` for display with icons
- Comprehensive unit tests (10 tests)

### 6.7 Kubernetes Quick Start

- [x] Implement quick start for Kubernetes deployment
- [x] Use current context
- [x] Default namespace `bibd`
- [x] StatefulSet PostgreSQL
- [x] LoadBalancer external access
- [x] Auto-apply manifests

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Added `kubernetes` package import
- Updated `setupBibdQuick()` to route to `setupBibdQuickKubernetes()`
- Implemented `setupBibdQuickKubernetes()` with:
  1. Detect Kubernetes with full cluster info
  2. Show context, cluster, CloudNativePG, Ingress, Storage info
  3. Prompt for name/email
  4. Prompt for namespace (default: bibd)
  5. Prompt for public network (bib.dev)
  6. Prompt for service type (LoadBalancer/NodePort/ClusterIP)
  7. Prompt for ingress hostname if available
  8. Prompt for output directory (default: ~/bibd-kubernetes)
  9. Prompt for auto-apply
  10. Deploy using Kubernetes deployer
  11. Show comprehensive summary
- Features:
  - Auto-detects CloudNativePG and uses if available
  - Auto-detects default storage class
  - Auto-detects ingress class
  - Uses PostgreSQL for Kubernetes deployments
  - Shows external IP and ingress URL if available
  - Generates convenience scripts

**Usage:**
```bash
# Kubernetes quick setup
bib setup --daemon --quick --target=kubernetes
bib setup -d -q -t kubernetes
```

**Example Output:**
```
☸️  Quick Setup - bibd (Kubernetes)

🔍 Detecting Kubernetes...
   ✓ Context: production
   ✓ Cluster: prod-cluster (v1.28.0)
   ✓ CloudNativePG: 1.20.0
   ✓ Ingress: nginx
   ✓ Storage: standard, fast

[Name, Email, Namespace, Network, Service Type, Ingress, Output Dir, Auto-apply prompts...]

☸️  Applying manifests...
⏳ Waiting for pods to be ready...
🌐 Getting external IP...

──────────────────────────────────────────────────
✅ Kubernetes Quick Setup Complete!
──────────────────────────────────────────────────

  Identity:    John Doe <john@example.com>
  Context:     production
  Namespace:   bibd
  Storage:     PostgreSQL (cloudnativepg)
  Service:     LoadBalancer
  Ingress:     bibd.example.com
  Network:     Public (bib.dev)
  Output:      /home/user/bibd-kubernetes
  Status:      🟢 Running
  External IP: 34.56.78.90
  Ingress URL: https://bibd.example.com

Commands:
  cd /home/user/bibd-kubernetes
  • Apply:  ./apply.sh  (or: kubectl apply -k .)
  • Delete: ./delete.sh (or: kubectl delete -k .)
  • Status: kubectl -n bibd get pods
  • Logs:   kubectl -n bibd logs -f deployment/bibd

  • Connect CLI: bib setup
```

**Generated Kubernetes Manifests:**
```
bibd-kubernetes/
├── namespace.yaml
├── configmap.yaml
├── secrets.yaml
├── bibd-deployment.yaml
├── bibd-service.yaml
├── bibd-ingress.yaml           # if hostname configured
├── postgres-statefulset.yaml   # if statefulset mode
├── postgres-service.yaml       # if statefulset mode
├── cloudnativepg-cluster.yaml  # if cloudnativepg mode
├── kustomization.yaml
├── apply.sh
└── delete.sh
```

---

## Phase 7: Node Discovery

### 7.1 mDNS Service Registration

- [x] Register bibd as `_bib._tcp.local` service on startup
- [x] Include node info in TXT records
- [x] Handle service deregistration on shutdown

**Files created:**
- `internal/discovery/mdns_server.go`
- `internal/discovery/mdns_server_test.go`

**Implementation notes:**
- Created `MDNSServer` struct for managing mDNS service registration:
  - `Start()` - starts advertising the service
  - `Stop()` - stops advertising and cleans up
  - `IsRunning()` - returns running state
  - `UpdateInfo()` - updates advertised info (requires restart)
  - `GetAdvertisedAddress()` - returns the advertised address
  - `GetTXTRecords()` - returns the TXT records
  - `WaitForShutdown()` - blocks until server stops
- Created `MDNSServerConfig` with fields:
  - InstanceName - unique instance name
  - Port - bibd API port
  - Host - hostname to advertise (must be FQDN ending in .)
  - NodeName - display name for TXT record
  - Version - bibd version for TXT record
  - PeerID - P2P peer ID for TXT record
  - Mode - P2P mode for TXT record
  - IPs - IP addresses to advertise (auto-detected if empty)
- TXT records include: name, version, peer_id, mode
- Created `MDNSServerManager` for lifecycle management:
  - `StartServer()` - starts or restarts server
  - `StopServer()` - stops server
  - `IsRunning()` - returns running state
  - `GetServer()` - returns underlying server
  - `RunWithContext()` - runs until context cancelled
- Comprehensive unit tests (10 tests)

### 7.2 mDNS Client Discovery

- [x] Implement mDNS browsing for `_bib._tcp.local`
- [x] Parse TXT records for node info
- [x] Return discovered nodes with addresses

**Implementation notes (existing in mdns.go, enhanced):**
- `discoverMDNS()` method on Discoverer
- `mdnsEntryToNode()` converts mDNS entries to DiscoveredNode
- `parseMDNSTxtRecords()` parses TXT records into NodeInfo:
  - name - node display name
  - version - bibd version
  - peer_id - P2P peer ID
  - mode - P2P mode
- `BrowseMDNS()` convenience function for one-time discovery
- `RegisterMDNSService()` registers a service and returns cleanup function
- `getLocalIPs()` gets all non-loopback local IPs

### 7.3 P2P Nearby Discovery

- [x] Implement DHT-based peer discovery
- [x] Filter for nearby/local peers
- [x] Return peer info with addresses

**Files modified:**
- `internal/discovery/p2p.go`

**Files created:**
- `internal/discovery/p2p_test.go`

**Implementation notes:**
- Enhanced `discoverP2P()` method:
  - Connects to bootstrap peers in parallel
  - Uses `probeP2PPeer()` to verify connectivity
  - Returns discovered nodes with latency measurements
- Added `P2PDiscoveryConfig` struct with:
  - BootstrapPeers - initial peers to connect to
  - Timeout - discovery timeout
  - MaxPeers - maximum peers to return
  - MeasureLatency - enable latency measurement
  - LatencyTimeout - latency measurement timeout
- `DefaultP2PDiscoveryConfig()` with sensible defaults:
  - Bootstrap peer: bootstrap.bib.dev:4001
  - Timeout: 10s, MaxPeers: 50
- Added helper functions:
  - `DiscoverP2P()` - convenience function
  - `DiscoverFromBootstrapPeers()` - discover from specific peers
  - `ParseMultiaddr()` - parses multiaddr strings
  - `MultiaddrsToAddresses()` - converts multiaddrs to host:port
  - `IsPublicBootstrapPeer()` - checks if peer is public
- Added `P2PNodeInfo` struct for P2P peer metadata
- Comprehensive unit tests (10 tests)

### 7.4 Discovery Aggregation

- [x] Combine results from all discovery methods
- [x] Deduplicate nodes
- [x] Sort by latency/preference
- [x] Add latency measurements

**Files modified:**
- `internal/discovery/discovery.go`

**Implementation notes:**
- Enhanced `DiscoveryResult` struct with:
  - `MethodCounts` - tracks nodes per discovery method
  - `HasNodes()` - returns true if nodes found
  - `HasErrors()` - returns true if errors occurred
  - `GetLocalNodes()` - filters local nodes
  - `GetMDNSNodes()` - filters mDNS nodes
  - `GetP2PNodes()` - filters P2P nodes
  - `GetPublicNodes()` - filters public nodes
  - `GetFastestNode()` - returns lowest latency node
  - `Summary()` - returns formatted summary string
- Enhanced `Discover()` method:
  - Populates MethodCounts during aggregation
  - Deduplicates by address, keeping lowest latency
  - Sorts by latency (measured first, then by value)
- Added aggregation helper functions:
  - `MergeResults()` - merges multiple DiscoveryResults
  - `DeduplicateNodes()` - removes duplicate nodes
  - `FilterNodesByMethod()` - filters by discovery method
  - `FilterNodesByLatency()` - filters by max latency
  - `GroupNodesByMethod()` - groups nodes by method
  - `FormatNodeList()` - formats nodes for display

**Example Discovery Flow:**
```go
// Create discoverer with all methods enabled
d := discovery.New(discovery.DiscoveryOptions{
    EnableMDNS:     true,
    EnableP2P:      true,
    MeasureLatency: true,
    Timeout:        10 * time.Second,
})

// Run discovery
result := d.Discover(ctx)

// Check results
fmt.Println(result.Summary())
// "Found 5 node(s) in 3.2s (local: 1, mdns: 2, p2p: 2)"

// Get fastest node
if fastest := result.GetFastestNode(); fastest != nil {
    fmt.Printf("Fastest: %s (%v)\n", fastest.Address, fastest.Latency)
}

// Filter by method
localNodes := result.GetLocalNodes()
mdnsNodes := result.GetMDNSNodes()

// Format for display
fmt.Println(discovery.FormatNodeList(result.Nodes))
```

**mDNS Service Registration Flow:**
```go
// Start mDNS server
config := discovery.MDNSServerConfig{
    InstanceName: "my-bibd",
    Port:         4000,
    Host:         "myhost.local.",
    NodeName:     "My Node",
    Version:      "1.0.0",
    PeerID:       "12D3KooW...",
    Mode:         "proxy",
}

server := discovery.NewMDNSServer(config)
if err := server.Start(); err != nil {
    log.Fatal(err)
}
defer server.Stop()

// Or use manager with context
manager := discovery.NewMDNSServerManager()
err := manager.RunWithContext(ctx, config)
```

---

## Phase 8: Connection & Trust

### 8.1 TOFU Implementation

- [x] Implement certificate fingerprint extraction
- [x] Implement TOFU prompt UI
- [x] Display node ID, address, fingerprint
- [x] Require explicit confirmation for new nodes
- [x] Save trusted node to trust store

**Files created:**
- `internal/certs/tofu_verifier.go`
- `internal/certs/tofu_verifier_test.go`

**Implementation notes:**
- Created `TOFUVerifier` struct for handling TOFU verification:
  - `NewTOFUVerifier(store)` - creates verifier with trust store
  - `WithAutoTrust(bool)` - enables automatic trusting
  - `Verify(nodeID, address, certPEM)` - verifies and returns VerifyResult
  - `VerifyWithCallback()` - uses custom callback for trust decisions
  - `DisplayMismatchWarning()` - shows MITM warning
  - `DisplayNewTrust()` - shows trust confirmation
- Created `VerifyResult` struct with:
  - Trusted, NewTrust flags
  - Node reference
  - FingerprintMismatch flag
  - Error message
- Created `CertInfo` struct for certificate display:
  - Fingerprint, Subject, Issuer
  - NotBefore, NotAfter, IsCA
  - PEM content
- Helper functions:
  - `ParseCertInfo()` - extracts info from PEM
  - `FingerprintFromCert()` - calculates fingerprint from cert
  - `FormatFingerprint()` - formats with colons (AB:CD:EF...)
- Interactive prompt shows:
  - Node ID and address
  - Certificate subject, issuer, validity
  - SHA256 fingerprint with colon formatting
  - Warning about TOFU security
  - y/N confirmation prompt
- MITM warning displays:
  - Expected vs received fingerprints
  - Possible causes
  - Instructions for recovery
- Comprehensive unit tests (15 tests):
  - Auto-trust mode
  - Already trusted nodes
  - Fingerprint mismatch detection
  - Interactive accept/reject
  - Prompt output verification
  - Callback-based verification

### 8.2 Trust-First-Use Flag

- [x] Implement `--trust-first-use` flag for `bib connect`
- [x] Auto-trust and save certificate when flag is set
- [x] Add warning in help text about security implications

**Files modified:**
- `cmd/bib/cmd/connect/connect.go`
- `internal/grpc/client/options.go`

**Implementation notes:**
- Added flags to `bib connect`:
  - `--trust-first-use` - automatically trust new certificates
  - `--insecure-skip-verify` - skip TLS verification entirely (hidden, dangerous)
- Updated help text with TOFU documentation:
  - Explains trust-on-first-use behavior
  - Notes security implications of --trust-first-use
- Added to client Options:
  - `TOFUCallback TOFUCallbackFunc` - callback for certificate verification
  - `WithTOFUCallback()` builder method
  - `WithInsecureSkipVerify()` builder method
- Connect command now:
  - Creates trust store in ~/.config/bib/trusted_nodes/
  - Creates TOFUVerifier with auto-trust based on flag
  - Sets TOFU callback on client options
  - Displays new trust confirmation or mismatch warning

**Usage:**
```bash
# Normal connection with TOFU prompt
bib connect node1.example.com:4000

# Auto-trust new certificates (less secure)
bib connect --trust-first-use node1.example.com:4000

# Skip verification entirely (dangerous, hidden flag)
bib connect --insecure-skip-verify node1.example.com:4000
```

### 8.3 Trust Commands

- [x] Implement `bib trust list` command
- [x] Implement `bib trust add` command with `--fingerprint` flag
- [x] Implement `bib trust remove` command
- [x] Implement `bib trust pin` command

**Files modified:**
- `cmd/bib/cmd/trust/trust.go`
- `cmd/bib/cmd/trust/commands.go`

**Implementation notes:**
All trust commands were already implemented. Added new commands:
- `bib trust show <node-id>` - shows detailed node info:
  - Node ID, alias, address
  - Fingerprint (formatted)
  - Trust method, verified status, pinned time
  - First seen, last seen timestamps
  - Certificate details (subject, issuer, validity)
- `bib trust verify <node-id>` - verifies fingerprint:
  - Shows fingerprint if no --fingerprint flag
  - Compares normalized fingerprints
  - Marks node as verified on success
  - Shows error on mismatch

**Complete trust command tree:**
```
bib trust
├── add       # Add trusted node manually
│   --node-id      # Node ID (required)
│   --fingerprint  # Certificate fingerprint
│   --cert         # Path to certificate file
│   --alias        # Friendly alias
│   --address      # Node address
├── list|ls   # List all trusted nodes
├── remove|rm # Remove trusted node
├── pin       # Pin certificate (stronger trust)
├── show      # Show detailed node info
└── verify    # Verify fingerprint
    --fingerprint  # Expected fingerprint
```

**Example Output - bib trust list:**
```
NODE ID          ALIAS         FINGERPRINT        METHOD    VERIFIED  LAST SEEN
-------          -----         -----------        ------    --------  ---------
12D3Koo...xyz    production    ABC123DEF456...    tofu      No        2024-01-15
12D3Koo...abc    staging       789XYZ012345...    manual    Yes       2024-01-16
12D3Koo...def    local         FEDCBA987654...    pinned    Pinned    2024-01-17
```

**Example Output - bib trust show:**
```
═══════════════════════════════════════════════════════════════
  Trusted Node Details
═══════════════════════════════════════════════════════════════

  Node ID:      12D3KooWTestNode1
  Alias:        production
  Address:      node1.example.com:4000

  Fingerprint:
    AB:CD:EF:12:34:56:78:90:AB:CD:EF:12:34:56:78:90...

  Trust Method: tofu
  Verified:     false
  
  First Seen:   2024-01-15T10:30:00Z
  Last Seen:    2024-01-17T15:45:00Z

───────────────────────────────────────────────────────────────
  Certificate Details
───────────────────────────────────────────────────────────────

  Subject:      CN=node1.example.com,O=Example Org
  Issuer:       CN=Example CA
  Valid From:   2024-01-01T00:00:00Z
  Valid Until:  2025-01-01T00:00:00Z
  Is CA:        false
```

---

## Phase 9: Post-Setup Actions

### 9.1 Local Post-Setup

- [x] Verify service is running
- [x] Run health check
- [x] Display status summary
- [x] Show management commands

**Files created:**
- `internal/postsetup/local.go`
- `internal/postsetup/postsetup_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `LocalVerifier` struct:
  - `NewLocalVerifier(address)` - creates verifier with bibd address
  - `Verify(ctx)` - verifies connectivity and returns LocalStatus
- Created `LocalStatus` struct with:
  - Running, PID, Address, Version, Uptime
  - Healthy, HealthStatus, Mode
  - Error message
- Created `CheckServiceStatus()` function:
  - Checks systemd on Linux
  - Checks launchd on macOS
  - Checks Windows Service on Windows
- Created `ServiceStatus` struct:
  - Installed, Running, Enabled, ServiceName
- Helper functions:
  - `FormatLocalStatus()` - formats status for display
  - `FormatServiceStatus()` - formats service status
  - `GetLocalManagementCommands()` - platform-specific commands
- Integration in setup.go:
  - `runLocalPostSetup()` function called after local quick setup

### 9.2 Docker Post-Setup

- [x] Wait for containers to be healthy
- [x] Display container status
- [x] Show docker compose management commands
- [x] Test bibd connectivity

**Files created:**
- `internal/postsetup/docker.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `DockerVerifier` struct:
  - `NewDockerVerifier(projectName, composeFile)` - creates verifier
  - `Verify(ctx)` - verifies deployment and returns DockerStatus
  - `WaitForHealthy(ctx, timeout)` - waits for all containers healthy
- Created `DockerStatus` struct:
  - Containers list, AllRunning, AllHealthy
  - BibdReachable, BibdAddress
- Created `ContainerStatus` struct:
  - Name, ID, Image, Status, State, Health, Ports
  - Running, Healthy flags
- Uses `docker compose ps --format json` for status
- Falls back to legacy format for older Docker
- Helper functions:
  - `FormatDockerStatus()` - formats status for display
  - `GetDockerManagementCommands()` - docker compose commands
- Integration in setup.go:
  - `runDockerPostSetup()` called after Docker quick setup

### 9.3 Podman Post-Setup

- [x] Wait for containers/pod to be running
- [x] Display status
- [x] Show podman management commands
- [x] Test bibd connectivity

**Files created:**
- `internal/postsetup/podman.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `PodmanVerifier` struct:
  - `NewPodmanVerifier(deployStyle, podName, composeFile)` - creates verifier
  - `Verify(ctx)` - verifies deployment and returns PodmanStatus
  - `WaitForRunning(ctx, timeout)` - waits for containers running
- Supports both "pod" and "compose" deploy styles
- Created `PodmanStatus` struct:
  - DeployStyle, PodName, Containers
  - AllRunning, BibdReachable, BibdAddress
- Uses `podman pod ps` and `podman ps` for status
- Helper functions:
  - `FormatPodmanStatus()` - formats status for display
  - `GetPodmanManagementCommands()` - pod or compose commands
- Integration in setup.go:
  - `runPodmanPostSetup()` called after Podman quick setup

### 9.4 Kubernetes Post-Setup

- [x] Wait for pods to be ready (if applied)
- [x] Display pod status
- [x] Show external access info
- [x] Show kubectl management commands
- [x] Offer port-forward for testing

**Files created:**
- `internal/postsetup/kubernetes.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `KubernetesVerifier` struct:
  - `NewKubernetesVerifier(namespace)` - creates verifier
  - `Verify(ctx)` - verifies deployment and returns KubernetesStatus
  - `WaitForReady(ctx, timeout)` - uses kubectl rollout status
  - `PortForward(ctx, localPort)` - starts port-forward
- Created `KubernetesStatus` struct:
  - Namespace, Pods list, AllReady
  - ExternalIP, NodePort, IngressHost
  - BibdReachable, BibdAddress
- Created `PodStatus` struct:
  - Name, Phase, Ready, Restarts, Age
- Uses `kubectl get pods -o json` for pod status
- Gets service and ingress info for external access
- Helper functions:
  - `FormatKubernetesStatus()` - formats status for display
  - `GetKubernetesManagementCommands()` - kubectl commands
- Integration in setup.go:
  - `runKubernetesPostSetup()` called after Kubernetes quick setup

### 9.5 CLI Post-Setup Verification

- [x] Test connection to all selected nodes
- [x] Verify authentication
- [x] Display network health summary
- [x] Show next steps and helpful commands

**Files created:**
- `internal/postsetup/cli.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `CLIVerifier` struct:
  - `NewCLIVerifier(nodes)` - creates verifier with node list
  - `Verify(ctx)` - verifies all nodes and returns CLIStatus
- Created `CLIStatus` struct:
  - Nodes list, AllConnected, AllAuthenticated
  - NetworkHealth (good/degraded/poor/offline)
- Created `NodeStatus` struct:
  - Address, Alias, Connected, Authenticated
  - Latency, Version, NodeID
- Created `NodeConfig` for node configuration
- Created `NetworkHealth` type with constants
- Helper functions:
  - `FormatCLIStatus()` - formats status with icons
  - `GetCLINextSteps()` - bib commands to try
  - `GetCLIHelpfulCommands()` - additional commands
- Integration in setup.go:
  - `runCLIPostSetup()` called after CLI quick setup

**Example Post-Setup Output - Local:**
```
🔍 Verifying installation...

🟢 bibd is running
   Address: localhost:4000
   Version: 1.0.0
   Health:  ✓ ok

📦 Service: bibd
   Status:  🟢 Running
   Enabled: ✓ Yes (starts at boot)

Management Commands:
  sudo systemctl status bibd
  sudo systemctl start bibd
  sudo systemctl stop bibd
  sudo journalctl -u bibd -f
```

**Example Post-Setup Output - Docker:**
```
🔍 Verifying Docker deployment...

🟢 All containers running and healthy

Containers:
  ✓ bibd: Up 5 minutes (healthy)
  ✓ postgres: Up 5 minutes (healthy)

🌐 bibd reachable at localhost:4000

Management Commands:
  docker compose -f docker-compose.yaml ps
  docker compose -f docker-compose.yaml logs -f
  docker compose -f docker-compose.yaml up -d
  docker compose -f docker-compose.yaml down
```

**Example Post-Setup Output - Kubernetes:**
```
🔍 Verifying Kubernetes deployment...

🟢 All pods ready
   Namespace: bibd

Pods:
  ✓ bibd-abc123xyz: Running (restarts: 0, age: 5m)
  ✓ postgres-0: Running (restarts: 0, age: 5m)

🌐 External IP: 34.56.78.90
🔗 Ingress: bibd.example.com
✓ bibd reachable at 34.56.78.90:4000

Management Commands:
  kubectl -n bibd get pods
  kubectl -n bibd get svc
  kubectl -n bibd logs -f deployment/bibd
  kubectl -n bibd port-forward svc/bibd 4000:4000
```

**Example Post-Setup Output - CLI:**
```
🔍 Verifying connections...

🟢 Network Health: Good

Nodes:
  ✓ local (localhost:4000): connected (5ms)
  ✓ remote (node1.example.com:4000): connected (45ms)

Helpful Commands:
  bib status                # Check connection status
  bib topic list            # List available topics
  bib dataset list          # List datasets
  bib query                 # Query data
  bib config show           # Show current configuration
```

---

## Phase 10: Error Recovery & Reconfiguration

### 10.1 Partial Config Detection

- [x] Check for `*.partial` config files on setup start
- [x] Parse partial config to determine last completed step
- [x] Offer resume/restart/cancel options

**Files created:**
- `internal/setup/partial/partial.go`
- `internal/setup/partial/partial_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `partial.PartialConfig` struct:
  - Version, SetupType, DeployTarget
  - CurrentStep, CompletedSteps
  - Data map for storing setup values
  - StartedAt, UpdatedAt timestamps
  - Error message
- Created `partial.Manager` for file operations:
  - `NewManager(configDir)` - creates manager
  - `HasPartial(setupType)` - checks if partial exists
  - `Load(setupType)` - loads partial config
  - `Save(config)` - saves partial config
  - `Delete(setupType)` - removes partial config
  - `List()` - lists all partial configs
- Created `SetupStep` constants:
  - StepIdentity, StepNetwork, StepStorage
  - StepDatabase, StepDeployment, StepService
  - StepNodes, StepConfig, StepComplete
- Helper functions:
  - `AllSteps()` - returns ordered steps
  - `StepDescription(step)` - human-readable descriptions
  - `FormatPartialSummary(config)` - formats for display
- Integration in setup.go:
  - `checkAndOfferResume()` - checks for partial config and prompts
  - `resumeSetupFromPartial()` - loads saved data
  - `savePartialConfig()` - saves current progress
  - `deletePartialConfigForSetup()` - cleanup on success

### 10.2 Graceful Interruption

- [x] Handle Ctrl+C gracefully
- [x] Prompt to save progress before exit
- [x] Save partial config with step metadata

**Files created:**
- `internal/setup/recovery/recovery.go`
- `internal/setup/recovery/recovery_test.go`

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- Created `recovery.InterruptHandler` for signal handling:
  - Catches SIGINT and SIGTERM
  - Calls OnInterrupt callback
  - Context-based cancellation
- Created `recovery.ErrorHandler` for error management:
  - Tracks retry counts per step
  - Configurable max retries
  - Custom OnError callback
- Added signal handling in `runSetup()`:
  - Creates context with cancel
  - Listens for os.Interrupt, syscall.SIGTERM
  - Displays "Setup interrupted. Progress will be saved."
- SetupWizardModel saves progress on exit:
  - `saveProgressOnExit()` saves current data
  - Only saves if progress has been made
  - Shows confirmation message

### 10.3 Error Handling

- [x] Catch errors during setup steps
- [x] Offer retry/reconfigure/skip options
- [x] For mode-incompatible errors (e.g., SQLite + Full), offer to switch modes

**Files created:**
- `internal/setup/recovery/recovery.go`

**Implementation notes:**
- Created `recovery.SetupError` struct:
  - Step, Message, Cause
  - Recoverable flag
  - Suggestions list
- Created `recovery.ErrorAction` type:
  - ErrorActionRetry
  - ErrorActionReconfigure
  - ErrorActionSkip
  - ErrorActionAbort
- Created common error factories:
  - `CommonErrors.NetworkUnavailable()`
  - `CommonErrors.DatabaseConnection(err)`
  - `CommonErrors.DockerNotRunning()`
  - `CommonErrors.KubectlNotFound()`
  - `CommonErrors.PermissionDenied(path)`
  - `CommonErrors.ConfigExists(path)`
  - `CommonErrors.InvalidConfiguration(field, reason)`
  - `CommonErrors.SQLiteWithFullMode()`
  - `CommonErrors.ServiceInstallFailed(err)`
  - `CommonErrors.ConnectionFailed(address, err)`
- Helper function `handleSetupError()` in setup.go:
  - Formats error with suggestions
  - Prompts for action (retry/skip/configure/abort)

### 10.4 Reconfigure Command

- [x] Implement `bib setup --reconfigure <section>`
- [x] Load existing config
- [x] Run only specified section wizard
- [x] Merge changes back to config

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- `--reconfigure` flag already implemented
- `ValidReconfigureSections()` returns valid sections:
  - CLI: identity, connection, output, nodes
  - Daemon: identity, p2p-mode, storage, network, service
- `runReconfigure()` function placeholder
- TODO: Full implementation of section-specific wizards

### 10.5 Fresh Start

- [x] Implement `bib setup --fresh`
- [x] Delete existing config (with confirmation)
- [x] Delete partial config
- [x] Run full setup wizard

**Files modified:**
- `cmd/bib/cmd/setup/setup.go`

**Implementation notes:**
- `--fresh` flag deletes existing configuration:
  - Prompts for confirmation
  - Deletes config file
  - Deletes partial config files
  - Proceeds with fresh setup
- `handleFreshSetup()` function handles the flow

### 10.6 Config Reset

- [x] Implement `bib config reset <section>`
- [x] Implement `bib config reset --all`
- [x] Reset to defaults without running wizard

**Files created:**
- `cmd/bib/cmd/config/reset.go`

**Implementation notes:**
- Created `bib config reset` command with sections:
  - `all` - reset entire configuration
  - `connection` - reset connection settings
  - `output` - reset output format settings
  - `nodes` - clear favorite nodes
  - `identity` - delete identity key
- Flags:
  - `--force` - skip confirmation prompt
  - `--all` - reset all sections
  - `--keep-nodes` - preserve nodes when resetting all
- Each section reset function:
  - `resetAll()` - deletes config and partial, creates default
  - `resetConnection()` - clears connection settings
  - `resetOutput()` - resets format and color
  - `resetNodes()` - clears favorite nodes list
  - `resetIdentity()` - deletes identity key file

**Example Usage:**
```bash
# Reset all configuration
bib config reset --all

# Reset without confirmation
bib config reset connection --force

# Reset all but keep nodes
bib config reset --all --keep-nodes

# Reset just output settings
bib config reset output
```

**Example Partial Config Resume Flow:**
```
┌─────────────────────────────────────────────────────────────┐
│              Partial Configuration Detected                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Type: daemon                                                │
│  Target: docker                                              │
│  Last Step: Network (public/private)                        │
│                                                              │
│  Started: 2024-01-15 10:30                                  │
│  Progress: 25% complete (2/8 steps)                         │
│                                                              │
└─────────────────────────────────────────────────────────────┘

What would you like to do?
  [R] Resume from where you left off
  [S] Start over (delete partial config)
  [C] Cancel

Choice [R/s/c]: 
```

**Test Coverage:**
- 20+ tests for partial config management
- 15+ tests for error recovery
- Tests for:
  - Partial config save/load/delete
  - Step completion tracking
  - Error handling with retries
  - Common error scenarios
  - Interrupt handling

---

## Phase 11: Testing

### 11.1 Unit Tests

- [ ] Test identity key generation
- [ ] Test node discovery (mocked)
- [ ] Test config generation for each deployment target
- [ ] Test partial config save/load
- [ ] Test TOFU trust store operations

**Files to create:**
- `internal/auth/identity_test.go`
- `internal/discovery/discovery_test.go`
- `internal/deploy/docker/compose_test.go`
- `internal/deploy/podman/pod_test.go`
- `internal/deploy/kubernetes/manifests_test.go`
- `internal/config/partial_test.go`

### 11.2 Integration Tests

- [ ] Test full CLI setup flow (mocked connections)
- [ ] Test local daemon setup flow
- [ ] Test Docker deployment (requires Docker)
- [ ] Test Podman deployment (requires Podman)
- [ ] Test Kubernetes manifest generation

**Files to create:**
- `test/integration/setup/cli_test.go`
- `test/integration/setup/daemon_local_test.go`
- `test/integration/setup/daemon_docker_test.go`
- `test/integration/setup/daemon_podman_test.go`
- `test/integration/setup/daemon_k8s_test.go`

### 11.3 E2E Tests

- [ ] Test complete CLI setup with real bibd
- [ ] Test Docker deployment end-to-end
- [ ] Test quick start modes

**Files to create:**
- `test/e2e/setup_test.go`

---

## Phase 12: Documentation & Polish

### 12.1 Help Text

- [ ] Update all command help text
- [ ] Add examples for new flags
- [ ] Ensure consistency across commands

### 12.2 Error Messages

- [ ] Review and improve error messages
- [ ] Add actionable suggestions to errors
- [ ] Ensure i18n keys exist for all messages

### 12.3 Wizard Polish

- [ ] Review wizard step flow
- [ ] Ensure consistent styling
- [ ] Add progress indicators
- [ ] Add keyboard shortcuts help

### 12.4 Documentation Updates

- [ ] Update README.md with setup examples
- [ ] Verify all doc links work
- [ ] Add troubleshooting section for setup issues

### 12.5 Release Notes

- [ ] Document new setup features
- [ ] Document breaking changes (if any)
- [ ] Create migration guide (if needed)

---

## Implementation Order Recommendation

1. **Start with Phase 1** (Core Infrastructure) - Foundation for everything
2. **Phase 2.1-2.2** (Identity + Discovery) - Core CLI functionality
3. **Phase 3** (Local Daemon) - Get basic daemon setup working
4. **Phase 2.3-2.9** (Complete CLI setup) - Finish CLI with node selection
5. **Phase 4** (Docker) - Container deployment
6. **Phase 5** (Podman) - Similar to Docker, good progression
7. **Phase 6** (Kubernetes) - Most complex, do last
8. **Phase 7-8** (Discovery + Trust) - Can be done in parallel
9. **Phase 9** (Post-Setup) - Polish deployment completion
10. **Phase 10** (Error Recovery) - Improve robustness
11. **Phase 11** (Testing) - Throughout, but formalize at end
12. **Phase 12** (Polish) - Final pass

---

## Notes

- Each phase should be completable independently
- Test after each phase before moving on
- Update this TODO as implementation progresses
- Mark items with `[x]` when complete
- Add sub-tasks as needed during implementation

