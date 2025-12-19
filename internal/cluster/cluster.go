// Package cluster provides Raft-based consensus for HA clusters using etcd/raft.
package cluster

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"bib/internal/config"
)

// Common errors
var (
	ErrNotLeader       = errors.New("not the cluster leader")
	ErrClusterNotReady = errors.New("cluster not ready")
	ErrNoQuorum        = errors.New("no quorum available")
	ErrNodeNotFound    = errors.New("node not found")
	ErrAlreadyMember   = errors.New("node is already a cluster member")
	ErrMinimumNodes    = errors.New("minimum cluster size is 3 voting nodes")
)

// MinimumVoters is the minimum number of voting nodes for a cluster
const MinimumVoters = 3

// NodeRole represents the role of a node in the cluster
type NodeRole string

const (
	RoleVoter    NodeRole = "voter"
	RoleNonVoter NodeRole = "non-voter"
)

// NodeState represents the state of a node in the cluster
type NodeState string

const (
	StateFollower  NodeState = "follower"
	StateCandidate NodeState = "candidate"
	StateLeader    NodeState = "leader"
)

// ClusterMember represents a node in the cluster
type ClusterMember struct {
	NodeID      string    `json:"node_id"`
	Address     string    `json:"address"`
	Role        NodeRole  `json:"role"`
	State       NodeState `json:"state"`
	LastContact time.Time `json:"last_contact"`
	IsHealthy   bool      `json:"is_healthy"`
	RaftIndex   uint64    `json:"raft_index"`
	PeerID      string    `json:"peer_id,omitempty"` // libp2p peer ID if known
}

// ClusterStatus represents the current cluster status
type ClusterStatus struct {
	ClusterName   string          `json:"cluster_name"`
	State         NodeState       `json:"state"`
	Leader        string          `json:"leader"`
	Term          uint64          `json:"term"`
	CommitIndex   uint64          `json:"commit_index"`
	AppliedIndex  uint64          `json:"applied_index"`
	Members       []ClusterMember `json:"members"`
	VoterCount    int             `json:"voter_count"`
	NonVoterCount int             `json:"non_voter_count"`
	HasQuorum     bool            `json:"has_quorum"`
}

// JoinToken contains information needed to join a cluster
type JoinToken struct {
	ClusterName string    `json:"cluster_name"`
	LeaderAddr  string    `json:"leader_addr"`
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Cluster manages the Raft consensus for HA operations
type Cluster struct {
	cfg       config.ClusterConfig
	configDir string

	mu      sync.RWMutex
	nodeID  string
	state   NodeState
	leader  string
	term    uint64
	members map[string]*ClusterMember

	storage   *Storage
	transport *Transport
	raft      *RaftNode

	// FSM for state machine
	fsm *FSM

	// Event callbacks
	onLeaderChange func(leaderID string)
	onMemberChange func(members []ClusterMember)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new cluster manager
func New(cfg config.ClusterConfig, configDir string) (*Cluster, error) {
	clusterLog := getLogger("manager")

	if !cfg.Enabled {
		clusterLog.Debug("cluster disabled in configuration")
		return nil, nil
	}

	clusterLog.Debug("creating cluster manager",
		"cluster_name", cfg.ClusterName,
		"is_voter", cfg.IsVoter,
		"bootstrap", cfg.Bootstrap,
	)

	nodeID := cfg.NodeID
	if nodeID == "" {
		var err error
		nodeID, err = generateNodeID()
		if err != nil {
			clusterLog.Error("failed to generate node ID", "error", err)
			return nil, fmt.Errorf("failed to generate node ID: %w", err)
		}
		clusterLog.Debug("generated node ID", "node_id", nodeID)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Cluster{
		cfg:       cfg,
		configDir: configDir,
		nodeID:    nodeID,
		state:     StateFollower,
		members:   make(map[string]*ClusterMember),
		ctx:       ctx,
		cancel:    cancel,
	}

	clusterLog.Info("cluster manager created", "node_id", nodeID)
	return c, nil
}

// Start initializes and starts the cluster
func (c *Cluster) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	clusterLog := getLogger("manager")
	clusterLog.Info("starting cluster", "node_id", c.nodeID)

	// Initialize storage
	clusterLog.Debug("initializing cluster storage")
	storage, err := NewStorage(c.cfg, c.configDir)
	if err != nil {
		clusterLog.Error("failed to initialize storage", "error", err)
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	c.storage = storage

	// Initialize FSM
	clusterLog.Debug("initializing FSM")
	c.fsm = NewFSM(c.storage)

	// Initialize transport
	clusterLog.Debug("initializing transport", "listen_addr", c.cfg.ListenAddr)
	transport, err := NewTransport(c.cfg, c.nodeID)
	if err != nil {
		_ = storage.Close()
		clusterLog.Error("failed to initialize transport", "error", err)
		return fmt.Errorf("failed to initialize transport: %w", err)
	}
	c.transport = transport

	// Initialize Raft node
	clusterLog.Debug("initializing Raft node")
	raftNode, err := NewRaftNode(c.cfg, c.nodeID, c.storage, c.transport, c.fsm)
	if err != nil {
		_ = transport.Close()
		_ = storage.Close()
		clusterLog.Error("failed to initialize raft node", "error", err)
		return fmt.Errorf("failed to initialize raft node: %w", err)
	}
	c.raft = raftNode

	// Bootstrap if this is the first node
	if c.cfg.Bootstrap {
		clusterLog.Info("bootstrapping new cluster")
		if err := c.bootstrap(); err != nil {
			clusterLog.Error("failed to bootstrap cluster", "error", err)
			return fmt.Errorf("failed to bootstrap cluster: %w", err)
		}
	}

	// Join existing cluster if join token or addresses provided
	if c.cfg.JoinToken != "" || len(c.cfg.JoinAddrs) > 0 {
		clusterLog.Info("joining existing cluster")
		if err := c.join(); err != nil {
			clusterLog.Error("failed to join cluster", "error", err)
			return fmt.Errorf("failed to join cluster: %w", err)
		}
	}

	// Start background processes
	c.wg.Add(1)
	go c.monitorLoop()

	clusterLog.Info("cluster started successfully",
		"node_id", c.nodeID,
		"state", c.state,
	)
	return nil
}

// Stop stops the cluster
func (c *Cluster) Stop() error {
	clusterLog := getLogger("manager")
	clusterLog.Info("stopping cluster", "node_id", c.nodeID)

	c.cancel()
	c.wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error

	if c.raft != nil {
		clusterLog.Debug("shutting down Raft")
		if err := c.raft.Shutdown(); err != nil {
			clusterLog.Error("failed to shutdown raft", "error", err)
			errs = append(errs, fmt.Errorf("failed to shutdown raft: %w", err))
		}
	}

	if c.transport != nil {
		clusterLog.Debug("closing transport")
		if err := c.transport.Close(); err != nil {
			clusterLog.Error("failed to close transport", "error", err)
			errs = append(errs, fmt.Errorf("failed to close transport: %w", err))
		}
	}

	if c.storage != nil {
		clusterLog.Debug("closing storage")
		if err := c.storage.Close(); err != nil {
			clusterLog.Error("failed to close storage", "error", err)
			errs = append(errs, fmt.Errorf("failed to close storage: %w", err))
		}
	}

	if len(errs) > 0 {
		clusterLog.Warn("cluster stopped with errors", "error_count", len(errs))
		return errors.Join(errs...)
	}

	clusterLog.Info("cluster stopped successfully")
	return nil
}

// IsLeader returns true if this node is the cluster leader
func (c *Cluster) IsLeader() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state == StateLeader
}

// Leader returns the current leader's node ID
func (c *Cluster) Leader() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.leader
}

// NodeID returns this node's ID
func (c *Cluster) NodeID() string {
	return c.nodeID
}

// State returns the current node state
func (c *Cluster) State() NodeState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Status returns the current cluster status
func (c *Cluster) Status() ClusterStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	members := make([]ClusterMember, 0, len(c.members))
	voterCount := 0
	nonVoterCount := 0

	for _, m := range c.members {
		members = append(members, *m)
		if m.Role == RoleVoter {
			voterCount++
		} else {
			nonVoterCount++
		}
	}

	// Calculate quorum: majority of voters
	hasQuorum := voterCount > 0 && c.countHealthyVoters() > voterCount/2

	return ClusterStatus{
		ClusterName:   c.cfg.ClusterName,
		State:         c.state,
		Leader:        c.leader,
		Term:          c.term,
		CommitIndex:   c.raft.CommitIndex(),
		AppliedIndex:  c.raft.AppliedIndex(),
		Members:       members,
		VoterCount:    voterCount,
		NonVoterCount: nonVoterCount,
		HasQuorum:     hasQuorum,
	}
}

// AddVoter adds a voting member to the cluster (leader only)
func (c *Cluster) AddVoter(nodeID, address string) error {
	if !c.IsLeader() {
		return ErrNotLeader
	}

	c.mu.Lock()
	if _, exists := c.members[nodeID]; exists {
		c.mu.Unlock()
		return ErrAlreadyMember
	}
	c.mu.Unlock()

	return c.raft.AddVoter(nodeID, address)
}

// AddNonVoter adds a non-voting member to the cluster (leader only)
func (c *Cluster) AddNonVoter(nodeID, address string) error {
	if !c.IsLeader() {
		return ErrNotLeader
	}

	c.mu.Lock()
	if _, exists := c.members[nodeID]; exists {
		c.mu.Unlock()
		return ErrAlreadyMember
	}
	c.mu.Unlock()

	return c.raft.AddNonVoter(nodeID, address)
}

// RemoveNode removes a node from the cluster (leader only)
func (c *Cluster) RemoveNode(nodeID string) error {
	if !c.IsLeader() {
		return ErrNotLeader
	}

	c.mu.Lock()
	if _, exists := c.members[nodeID]; !exists {
		c.mu.Unlock()
		return ErrNodeNotFound
	}

	// Check minimum cluster size
	voterCount := c.countVoters()
	member := c.members[nodeID]
	if member.Role == RoleVoter && voterCount <= MinimumVoters {
		c.mu.Unlock()
		return ErrMinimumNodes
	}
	c.mu.Unlock()

	return c.raft.RemoveNode(nodeID)
}

// GenerateJoinToken generates a join token for new nodes (leader only)
func (c *Cluster) GenerateJoinToken(ttl time.Duration) (*JoinToken, error) {
	if !c.IsLeader() {
		return nil, ErrNotLeader
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	token := &JoinToken{
		ClusterName: c.cfg.ClusterName,
		LeaderAddr:  c.cfg.AdvertiseAddr,
		Token:       hex.EncodeToString(tokenBytes),
		ExpiresAt:   time.Now().Add(ttl),
	}

	// Store token in FSM for validation
	if err := c.fsm.StoreJoinToken(token); err != nil {
		return nil, fmt.Errorf("failed to store join token: %w", err)
	}

	return token, nil
}

// TakeSnapshot triggers a manual snapshot
func (c *Cluster) TakeSnapshot() error {
	return c.raft.TakeSnapshot()
}

// Snapshot triggers a manual Raft snapshot (alias for TakeSnapshot).
func (c *Cluster) Snapshot() error {
	return c.TakeSnapshot()
}

// TransferLeadership transfers leadership to another node (leader only).
func (c *Cluster) TransferLeadership(targetNodeID string) error {
	if !c.IsLeader() {
		return ErrNotLeader
	}

	c.mu.RLock()
	target, exists := c.members[targetNodeID]
	c.mu.RUnlock()

	if !exists {
		return ErrNodeNotFound
	}

	if target.Role != RoleVoter {
		return fmt.Errorf("target node is not a voter")
	}

	return c.raft.TransferLeadership(targetNodeID)
}

// Restore restores from a snapshot
func (c *Cluster) Restore(snapshotID string) error {
	return c.raft.Restore(snapshotID)
}

// ListSnapshots returns available snapshots
func (c *Cluster) ListSnapshots() ([]SnapshotMeta, error) {
	return c.storage.ListSnapshots()
}

// OnLeaderChange sets a callback for leader changes
func (c *Cluster) OnLeaderChange(fn func(leaderID string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onLeaderChange = fn
}

// OnMemberChange sets a callback for membership changes
func (c *Cluster) OnMemberChange(fn func(members []ClusterMember)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onMemberChange = fn
}

// Apply applies a command to the cluster (leader only)
// This is used to replicate metadata changes across the cluster
func (c *Cluster) Apply(cmd []byte) error {
	if !c.IsLeader() {
		return ErrNotLeader
	}

	return c.raft.Apply(cmd)
}

// bootstrap initializes a new cluster
func (c *Cluster) bootstrap() error {
	return c.raft.Bootstrap(c.nodeID, c.cfg.ListenAddr)
}

// join joins an existing cluster
func (c *Cluster) join() error {
	// Try join token first
	if c.cfg.JoinToken != "" {
		return c.joinWithToken()
	}

	// Try direct addresses
	if len(c.cfg.JoinAddrs) > 0 {
		return c.joinWithAddrs()
	}

	return errors.New("no join method specified")
}

func (c *Cluster) joinWithToken() error {
	// TODO: Implement token-based join
	// 1. Parse the join token
	// 2. Connect to the leader address in the token
	// 3. Request to join the cluster with the token
	return nil
}

func (c *Cluster) joinWithAddrs() error {
	// TODO: Implement address-based join
	// 1. Try each address in order
	// 2. Request cluster membership
	return nil
}

// monitorLoop monitors cluster state changes
func (c *Cluster) monitorLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.updateState()
		}
	}
}

// updateState updates the cluster state from Raft
func (c *Cluster) updateState() {
	c.mu.Lock()
	defer c.mu.Unlock()

	clusterLog := getLogger("manager")

	if c.raft == nil {
		return
	}

	oldState := c.state
	oldLeader := c.leader

	// Update state from raft
	c.state = c.raft.State()
	c.leader = c.raft.Leader()
	c.term = c.raft.Term()

	// Update members
	members := c.raft.Members()
	for id, addr := range members {
		if m, exists := c.members[id]; exists {
			m.Address = addr
			m.LastContact = time.Now()
		} else {
			c.members[id] = &ClusterMember{
				NodeID:      id,
				Address:     addr,
				Role:        RoleVoter, // TODO: Get actual role
				State:       StateFollower,
				LastContact: time.Now(),
				IsHealthy:   true,
			}
		}
	}

	// Trigger callbacks if state changed
	if c.state != oldState || c.leader != oldLeader {
		if c.state != oldState {
			clusterLog.Info("cluster state changed",
				"old_state", oldState,
				"new_state", c.state,
				"term", c.term,
			)
		}
		if c.leader != oldLeader {
			clusterLog.Info("cluster leader changed",
				"old_leader", oldLeader,
				"new_leader", c.leader,
			)
		}
		if c.onLeaderChange != nil && c.leader != oldLeader {
			go c.onLeaderChange(c.leader)
		}
	}
}

func (c *Cluster) countVoters() int {
	count := 0
	for _, m := range c.members {
		if m.Role == RoleVoter {
			count++
		}
	}
	return count
}

func (c *Cluster) countHealthyVoters() int {
	count := 0
	for _, m := range c.members {
		if m.Role == RoleVoter && m.IsHealthy {
			count++
		}
	}
	return count
}

// generateNodeID generates a random node ID
func generateNodeID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
