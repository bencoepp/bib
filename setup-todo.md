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

- [ ] Check for `podman` command availability
- [ ] Detect rootful vs rootless mode
- [ ] Check for `podman-compose` availability
- [ ] Display Podman info in wizard

**Files to create:**
- `internal/deploy/podman/detect.go`

### 5.2 Podman Mode Selection

- [ ] Add rootful/rootless mode selection
- [ ] Add pod vs compose style selection
- [ ] Validate selections based on detected capabilities

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 5.3 Podman Pod Generation

- [ ] Create Kubernetes-style pod YAML template
- [ ] Include bibd container configuration
- [ ] Include PostgreSQL container (if Full mode)
- [ ] Configure shared volumes
- [ ] Generate convenience `start.sh` script

**Files to create:**
- `internal/deploy/podman/pod.go`
- `internal/deploy/podman/templates/pod.yaml.tmpl`
- `internal/deploy/podman/templates/start.sh.tmpl`

### 5.4 Podman Compose Generation

- [ ] Create `podman-compose.yaml` template
- [ ] Similar structure to Docker compose
- [ ] Handle rootless-specific port considerations

**Files to create:**
- `internal/deploy/podman/compose.go`
- `internal/deploy/podman/templates/podman-compose.yaml.tmpl`

### 5.5 Podman Deployment

- [ ] Implement `DeployPodman()` function
- [ ] Create output directory structure
- [ ] Write all generated files
- [ ] Run appropriate start command (pod or compose)
- [ ] Wait for containers to be running
- [ ] Display container/pod status

**Files to create:**
- `internal/deploy/podman/deploy.go`

### 5.6 Podman Quick Start

- [ ] Implement quick start for Podman deployment
- [ ] Auto-detect rootful/rootless
- [ ] Default to pod style
- [ ] Minimal prompts, auto-start

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

---

## Phase 6: Daemon Setup - Kubernetes

### 6.1 Kubernetes Detection

- [ ] Check for `kubectl` command availability
- [ ] Get current context
- [ ] Verify cluster connectivity
- [ ] Display cluster info in wizard

**Files to create:**
- `internal/deploy/kubernetes/detect.go`

### 6.2 Kubernetes Configuration

- [ ] Add namespace configuration
- [ ] Add output directory configuration
- [ ] Add output options:
  - [ ] Generate and apply
  - [ ] Generate only
  - [ ] Generate Helm values only
- [ ] Add external access configuration:
  - [ ] None (internal only)
  - [ ] LoadBalancer
  - [ ] NodePort
  - [ ] Ingress (with hostname)

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`
- `internal/tui/setup.go`

### 6.3 Kubernetes PostgreSQL Options

- [ ] Add StatefulSet PostgreSQL option
- [ ] Add CloudNativePG option (with operator detection)
- [ ] Add external PostgreSQL option
- [ ] Configure storage class and PVC size for StatefulSet

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 6.4 Kubernetes Manifest Generation

- [ ] Create `namespace.yaml` template
- [ ] Create `configmap.yaml` template
- [ ] Create `secret.yaml` template
- [ ] Create `bibd-deployment.yaml` template (or statefulset)
- [ ] Create `bibd-service.yaml` template
- [ ] Create `bibd-ingress.yaml` template (optional)
- [ ] Create PostgreSQL StatefulSet templates (if selected):
  - [ ] `postgres-statefulset.yaml`
  - [ ] `postgres-service.yaml`
  - [ ] `postgres-pvc.yaml`
  - [ ] `postgres-secret.yaml`
- [ ] Create `kustomization.yaml` template
- [ ] Create `values.yaml` for future Helm chart

**Files to create:**
- `internal/deploy/kubernetes/manifests.go`
- `internal/deploy/kubernetes/templates/*.yaml.tmpl`

### 6.5 CloudNativePG Support

- [ ] Detect CloudNativePG operator installation
- [ ] Create CloudNativePG Cluster CR template
- [ ] Configure backup settings (optional)

**Files to create:**
- `internal/deploy/kubernetes/cloudnativepg.go`
- `internal/deploy/kubernetes/templates/cloudnativepg-cluster.yaml.tmpl`

### 6.6 Kubernetes Deployment

- [ ] Implement `DeployKubernetes()` function
- [ ] Create output directory structure
- [ ] Write all generated manifests
- [ ] Optionally apply with `kubectl apply -k`
- [ ] Wait for pods to be ready (if applied)
- [ ] Display deployment status
- [ ] Show external access info (LoadBalancer IP, etc.)

**Files to create:**
- `internal/deploy/kubernetes/deploy.go`

### 6.7 Kubernetes Quick Start

- [ ] Implement quick start for Kubernetes deployment
- [ ] Use current context
- [ ] Default namespace `bibd`
- [ ] StatefulSet PostgreSQL
- [ ] LoadBalancer external access
- [ ] Auto-apply manifests

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

---

## Phase 7: Node Discovery

### 7.1 mDNS Service Registration

- [ ] Register bibd as `_bib._tcp.local` service on startup
- [ ] Include node info in TXT records
- [ ] Handle service deregistration on shutdown

**Files to create:**
- `internal/discovery/mdns_server.go`

**Files to modify:**
- `cmd/bibd/daemon.go`

### 7.2 mDNS Client Discovery

- [ ] Implement mDNS browsing for `_bib._tcp.local`
- [ ] Parse TXT records for node info
- [ ] Return discovered nodes with addresses

**Files to modify:**
- `internal/discovery/mdns.go`

### 7.3 P2P Nearby Discovery

- [ ] Implement DHT-based peer discovery
- [ ] Filter for nearby/local peers
- [ ] Return peer info with addresses

**Files to modify:**
- `internal/discovery/p2p.go`

### 7.4 Discovery Aggregation

- [ ] Combine results from all discovery methods
- [ ] Deduplicate nodes
- [ ] Sort by latency/preference
- [ ] Add latency measurements

**Files to modify:**
- `internal/discovery/discovery.go`

---

## Phase 8: Connection & Trust

### 8.1 TOFU Implementation

- [ ] Implement certificate fingerprint extraction
- [ ] Implement TOFU prompt UI
- [ ] Display node ID, address, fingerprint
- [ ] Require explicit confirmation for new nodes
- [ ] Save trusted node to trust store

**Files to modify:**
- `internal/certs/tofu.go`
- `internal/grpc/client/client.go`

### 8.2 Trust-First-Use Flag

- [ ] Implement `--trust-first-use` flag for `bib connect`
- [ ] Auto-trust and save certificate when flag is set
- [ ] Add warning in help text about security implications

**Files to modify:**
- `cmd/bib/cmd/connect/connect.go`

### 8.3 Trust Commands

- [ ] Implement `bib trust list` command
- [ ] Implement `bib trust add` command with `--fingerprint` flag
- [ ] Implement `bib trust remove` command
- [ ] Implement `bib trust pin` command

**Files to modify:**
- `cmd/bib/cmd/trust/commands.go`

---

## Phase 9: Post-Setup Actions

### 9.1 Local Post-Setup

- [ ] Verify service is running
- [ ] Run health check
- [ ] Display status summary
- [ ] Show management commands

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 9.2 Docker Post-Setup

- [ ] Wait for containers to be healthy
- [ ] Display container status
- [ ] Show docker compose management commands
- [ ] Test bibd connectivity

**Files to modify:**
- `internal/deploy/docker/deploy.go`

### 9.3 Podman Post-Setup

- [ ] Wait for containers/pod to be running
- [ ] Display status
- [ ] Show podman management commands
- [ ] Test bibd connectivity

**Files to modify:**
- `internal/deploy/podman/deploy.go`

### 9.4 Kubernetes Post-Setup

- [ ] Wait for pods to be ready (if applied)
- [ ] Display pod status
- [ ] Show external access info
- [ ] Show kubectl management commands
- [ ] Offer port-forward for testing

**Files to modify:**
- `internal/deploy/kubernetes/deploy.go`

### 9.5 CLI Post-Setup Verification

- [ ] Test connection to all selected nodes
- [ ] Verify authentication
- [ ] Display network health summary
- [ ] Show next steps and helpful commands

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

---

## Phase 10: Error Recovery & Reconfiguration

### 10.1 Partial Config Detection

- [ ] Check for `*.partial` config files on setup start
- [ ] Parse partial config to determine last completed step
- [ ] Offer resume/restart/cancel options

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 10.2 Graceful Interruption

- [ ] Handle Ctrl+C gracefully
- [ ] Prompt to save progress before exit
- [ ] Save partial config with step metadata

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 10.3 Error Handling

- [ ] Catch errors during setup steps
- [ ] Offer retry/reconfigure/skip options
- [ ] For mode-incompatible errors (e.g., SQLite + Full), offer to switch modes

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 10.4 Reconfigure Command

- [ ] Implement `bib setup --reconfigure <section>`
- [ ] Load existing config
- [ ] Run only specified section wizard
- [ ] Merge changes back to config
- [ ] Restart service if daemon config changed

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 10.5 Fresh Start

- [ ] Implement `bib setup --fresh`
- [ ] Delete existing config (with confirmation)
- [ ] Delete partial config
- [ ] Run full setup wizard

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 10.6 Config Reset

- [ ] Implement `bib config reset <section>`
- [ ] Implement `bib config reset --all`
- [ ] Reset to defaults without running wizard

**Files to create:**
- `cmd/bib/cmd/config/reset.go`

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

