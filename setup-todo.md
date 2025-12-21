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

- [ ] Implement `GenerateIdentityKey()` function
- [ ] Generate Ed25519 keypair
- [ ] Save private key to `~/.config/bib/identity.pem`
- [ ] Display public key and fingerprint to user
- [ ] Add key generation step to wizard

**Files to create:**
- `internal/auth/identity.go`

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`
- `internal/tui/setup.go`

### 2.2 Node Discovery

- [ ] Implement `DiscoverNodes()` function that combines all discovery methods
- [ ] Implement localhost port scanning (4000, 8080)
- [ ] Implement Unix socket detection
- [ ] Integrate mDNS discovery for `_bib._tcp.local`
- [ ] Integrate P2P/DHT nearby peer discovery
- [ ] Return discovered nodes with latency measurements
- [ ] Create discovery progress UI component

**Files to create:**
- `internal/discovery/discovery.go`
- `internal/discovery/localhost.go`
- `internal/discovery/mdns.go`
- `internal/discovery/p2p.go`

### 2.3 Node Selection UI

- [ ] Create multi-select node list TUI component
- [ ] Display discovered nodes with:
  - [ ] Address
  - [ ] Discovery method (local, mDNS, P2P)
  - [ ] Latency
  - [ ] Node info (if available)
- [ ] Add bib.dev as separate "Public Network" option
- [ ] Implement "Add Custom..." option for manual address entry
- [ ] Implement "Select All Local" convenience button
- [ ] Store selected nodes in config

**Files to create:**
- `internal/tui/component/node_selector.go`

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 2.4 bib.dev Confirmation

- [ ] Create confirmation dialog for bib.dev connection
- [ ] Explain implications (public network, visibility, data publishing)
- [ ] Require explicit "Yes, Connect to Public Network" confirmation
- [ ] Store confirmation status in setup data

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 2.5 Multi-Node Configuration

- [ ] Update config structure to support multiple nodes
- [ ] Add `nodes` array to bib config:
  ```yaml
  nodes:
    - address: "localhost:4000"
      alias: "local"
      default: true
    - address: "bib.dev:4000"
      alias: "public"
  ```
- [ ] Maintain backward compatibility with `server` field
- [ ] Implement default node selection

**Files to modify:**
- `internal/config/types.go`
- `internal/config/loader.go`

### 2.6 Connection Testing

- [ ] Implement `TestConnection()` for each selected node
- [ ] Display connection status, latency, node info
- [ ] Handle connection failures gracefully
- [ ] Offer retry/remove options for failed nodes

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 2.7 Authentication Testing

- [ ] Implement `TestAuthentication()` for each node
- [ ] Use generated identity key
- [ ] Handle auto-registration if enabled on server
- [ ] Display session info on success
- [ ] Handle authentication failures

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 2.8 Network Health Check

- [ ] Query peer count from each connected node
- [ ] Display bootstrap connection status
- [ ] Show DHT status
- [ ] Create network health summary UI

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 2.9 Quick Start Mode (CLI)

- [ ] Implement `setupBibQuick()` function
- [ ] Prompt only for name and email
- [ ] Auto-generate identity key
- [ ] Auto-discover and select local nodes
- [ ] Prompt for bib.dev confirmation if no local nodes
- [ ] Test connections
- [ ] Save minimal config

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

---

## Phase 3: Daemon Setup - Local

### 3.1 Deployment Target Selection

- [ ] Add deployment target selection as first wizard step
- [ ] Create deployment target selector UI component
- [ ] Detect available targets:
  - [ ] Local: always available
  - [ ] Docker: check for `docker` command
  - [ ] Podman: check for `podman` command
  - [ ] Kubernetes: check for `kubectl` command and valid context
- [ ] Show detection status for each target

**Files to create:**
- `internal/tui/component/target_selector.go`
- `internal/deploy/detect.go`

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 3.2 Local PostgreSQL Options

- [ ] Update storage step to show local-specific PostgreSQL options:
  - [ ] SQLite (Proxy/Selective only)
  - [ ] Managed Container (Docker/Podman)
  - [ ] Local Installation
  - [ ] Remote Server
- [ ] Implement managed container PostgreSQL setup
- [ ] Implement local PostgreSQL connection test
- [ ] Implement remote PostgreSQL connection test

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`
- `internal/tui/setup.go`

### 3.3 Bootstrap Peers with bib.dev Confirmation

- [ ] Update bootstrap peer step to require bib.dev confirmation
- [ ] Create confirmation dialog explaining public network implications
- [ ] Allow "No, Private Only" option

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

### 3.4 Service Installation

- [ ] Implement systemd service file generation (Linux)
- [ ] Implement launchd plist generation (macOS)
- [ ] Implement Windows Service installation
- [ ] Add user vs system service option (Linux)
- [ ] Enable and start service after installation

**Files to create:**
- `internal/deploy/local/systemd.go`
- `internal/deploy/local/launchd.go`
- `internal/deploy/local/windows.go`
- `internal/deploy/local/service.go`

### 3.5 Local Quick Start

- [ ] Implement quick start for local deployment
- [ ] Minimal prompts (name, email)
- [ ] SQLite + Proxy mode defaults
- [ ] Auto-install service and start

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

---

## Phase 4: Daemon Setup - Docker

### 4.1 Docker Detection

- [ ] Check for `docker` command availability
- [ ] Verify Docker daemon is running
- [ ] Check Docker version compatibility
- [ ] Display Docker info in wizard

**Files to create:**
- `internal/deploy/docker/detect.go`

### 4.2 Docker Compose Generation

- [ ] Create `docker-compose.yaml` template
- [ ] Include bibd service configuration
- [ ] Include PostgreSQL service (if Full mode)
- [ ] Configure volumes for data persistence
- [ ] Configure network for inter-container communication
- [ ] Generate `.env` file for environment variables
- [ ] Generate `config/config.yaml`
- [ ] Generate `config/identity.pem`

**Files to create:**
- `internal/deploy/docker/compose.go`
- `internal/deploy/docker/templates/docker-compose.yaml.tmpl`
- `internal/deploy/docker/templates/env.tmpl`

### 4.3 Docker PostgreSQL Setup

- [ ] Configure PostgreSQL as separate service in compose
- [ ] Set up internal networking between bibd and postgres
- [ ] Configure persistent volume for postgres data
- [ ] Generate secure database credentials

**Files to modify:**
- `internal/deploy/docker/compose.go`

### 4.4 Docker Deployment

- [ ] Implement `DeployDocker()` function
- [ ] Create output directory structure
- [ ] Write all generated files
- [ ] Run `docker compose up -d`
- [ ] Wait for containers to be healthy
- [ ] Display container status

**Files to create:**
- `internal/deploy/docker/deploy.go`

### 4.5 Docker Quick Start

- [ ] Implement quick start for Docker deployment
- [ ] Minimal prompts (name, email)
- [ ] SQLite + Proxy mode defaults
- [ ] Auto-generate and start containers

**Files to modify:**
- `cmd/bib/cmd/setup/setup.go`

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

