package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"bib/internal/config"
)

// RaftNode wraps the etcd/raft implementation
// TODO: This is a placeholder implementation. The actual etcd/raft integration
// will be added once the go.mod is updated with the dependency.
type RaftNode struct {
	cfg       config.ClusterConfig
	nodeID    string
	storage   *Storage
	transport *Transport
	fsm       *FSM

	mu           sync.RWMutex
	state        NodeState
	leader       string
	term         uint64
	commitIndex  uint64
	appliedIndex uint64
	members      map[string]string // nodeID -> address

	// Channels for Raft operations
	proposeCh    chan []byte
	confChangeCh chan confChange
	commitCh     chan *commit

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type confChange struct {
	nodeID  string
	address string
	isAdd   bool
	isVoter bool
}

type commit struct {
	index uint64
	term  uint64
	data  []byte
}

// NewRaftNode creates a new Raft node
func NewRaftNode(cfg config.ClusterConfig, nodeID string, storage *Storage, transport *Transport, fsm *FSM) (*RaftNode, error) {
	ctx, cancel := context.WithCancel(context.Background())

	rn := &RaftNode{
		cfg:          cfg,
		nodeID:       nodeID,
		storage:      storage,
		transport:    transport,
		fsm:          fsm,
		state:        StateFollower,
		members:      make(map[string]string),
		proposeCh:    make(chan []byte, 256),
		confChangeCh: make(chan confChange, 16),
		commitCh:     make(chan *commit, 256),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Load persisted state
	if err := rn.loadState(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load raft state: %w", err)
	}

	// Start background loops
	rn.wg.Add(2)
	go rn.applyLoop()
	go rn.snapshotLoop()

	return rn, nil
}

// loadState loads persisted Raft state
func (rn *RaftNode) loadState() error {
	state, err := rn.storage.GetHardState()
	if err != nil {
		return err
	}

	rn.term = state.Term
	rn.commitIndex = state.Commit

	// Load members
	members, err := rn.storage.GetMembers()
	if err != nil {
		return err
	}

	for _, m := range members {
		rn.members[m.NodeID] = m.Address
	}

	return nil
}

// Shutdown stops the Raft node
func (rn *RaftNode) Shutdown() error {
	rn.cancel()
	rn.wg.Wait()

	// Save state before shutting down
	state := &HardState{
		Term:   rn.term,
		Commit: rn.commitIndex,
	}
	return rn.storage.SetHardState(state)
}

// Bootstrap bootstraps a new cluster with this node as the only member
func (rn *RaftNode) Bootstrap(nodeID, address string) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	// Check if already bootstrapped
	if len(rn.members) > 0 {
		return fmt.Errorf("cluster already bootstrapped")
	}

	// Add self as the first member
	rn.members[nodeID] = address
	rn.state = StateLeader
	rn.leader = nodeID
	rn.term = 1

	// Persist membership
	if err := rn.storage.AddMember(&ClusterMember{
		NodeID:  nodeID,
		Address: address,
		Role:    RoleVoter,
	}); err != nil {
		return err
	}

	// Persist state
	state := &HardState{
		Term:   rn.term,
		Vote:   nodeID,
		Commit: 0,
	}
	return rn.storage.SetHardState(state)
}

// State returns the current node state
func (rn *RaftNode) State() NodeState {
	rn.mu.RLock()
	defer rn.mu.RUnlock()
	return rn.state
}

// Leader returns the current leader
func (rn *RaftNode) Leader() string {
	rn.mu.RLock()
	defer rn.mu.RUnlock()
	return rn.leader
}

// Term returns the current term
func (rn *RaftNode) Term() uint64 {
	rn.mu.RLock()
	defer rn.mu.RUnlock()
	return rn.term
}

// CommitIndex returns the commit index
func (rn *RaftNode) CommitIndex() uint64 {
	rn.mu.RLock()
	defer rn.mu.RUnlock()
	return rn.commitIndex
}

// AppliedIndex returns the applied index
func (rn *RaftNode) AppliedIndex() uint64 {
	rn.mu.RLock()
	defer rn.mu.RUnlock()
	return rn.appliedIndex
}

// Members returns the current cluster members
func (rn *RaftNode) Members() map[string]string {
	rn.mu.RLock()
	defer rn.mu.RUnlock()

	result := make(map[string]string, len(rn.members))
	for k, v := range rn.members {
		result[k] = v
	}
	return result
}

// Apply proposes a command to be applied to the FSM
func (rn *RaftNode) Apply(cmd []byte) error {
	rn.mu.RLock()
	if rn.state != StateLeader {
		rn.mu.RUnlock()
		return ErrNotLeader
	}
	rn.mu.RUnlock()

	select {
	case rn.proposeCh <- cmd:
		return nil
	case <-rn.ctx.Done():
		return rn.ctx.Err()
	default:
		return fmt.Errorf("proposal channel full")
	}
}

// AddVoter adds a voting member
func (rn *RaftNode) AddVoter(nodeID, address string) error {
	return rn.addMember(nodeID, address, true)
}

// AddNonVoter adds a non-voting member
func (rn *RaftNode) AddNonVoter(nodeID, address string) error {
	return rn.addMember(nodeID, address, false)
}

func (rn *RaftNode) addMember(nodeID, address string, isVoter bool) error {
	rn.mu.RLock()
	if rn.state != StateLeader {
		rn.mu.RUnlock()
		return ErrNotLeader
	}
	rn.mu.RUnlock()

	select {
	case rn.confChangeCh <- confChange{nodeID: nodeID, address: address, isAdd: true, isVoter: isVoter}:
		return nil
	case <-rn.ctx.Done():
		return rn.ctx.Err()
	default:
		return fmt.Errorf("configuration change channel full")
	}
}

// RemoveNode removes a node from the cluster
func (rn *RaftNode) RemoveNode(nodeID string) error {
	rn.mu.RLock()
	if rn.state != StateLeader {
		rn.mu.RUnlock()
		return ErrNotLeader
	}
	rn.mu.RUnlock()

	select {
	case rn.confChangeCh <- confChange{nodeID: nodeID, isAdd: false}:
		return nil
	case <-rn.ctx.Done():
		return rn.ctx.Err()
	default:
		return fmt.Errorf("configuration change channel full")
	}
}

// TakeSnapshot triggers a manual snapshot
func (rn *RaftNode) TakeSnapshot() error {
	rn.mu.RLock()
	index := rn.appliedIndex
	term := rn.term
	rn.mu.RUnlock()

	// Get FSM snapshot
	data, err := rn.fsm.Snapshot()
	if err != nil {
		return fmt.Errorf("failed to get FSM snapshot: %w", err)
	}

	// Get configuration
	config, err := json.Marshal(rn.Members())
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Create snapshot
	_, err = rn.storage.CreateSnapshot(index, term, config, data)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Compact log
	if err := rn.storage.DeleteRange(0, index-rn.cfg.Raft.TrailingLogs); err != nil {
		// Log warning but don't fail
	}

	return nil
}

// Restore restores from a snapshot
func (rn *RaftNode) Restore(snapshotID string) error {
	data, err := rn.storage.ReadSnapshot(snapshotID)
	if err != nil {
		return fmt.Errorf("failed to read snapshot: %w", err)
	}

	if err := rn.fsm.Restore(data); err != nil {
		return fmt.Errorf("failed to restore FSM: %w", err)
	}

	return nil
}

// applyLoop applies committed entries to the FSM
func (rn *RaftNode) applyLoop() {
	defer rn.wg.Done()

	for {
		select {
		case <-rn.ctx.Done():
			return
		case c := <-rn.commitCh:
			if err := rn.fsm.Apply(c.data); err != nil {
				// Log error
				continue
			}

			rn.mu.Lock()
			if c.index > rn.appliedIndex {
				rn.appliedIndex = c.index
			}
			rn.mu.Unlock()
		}
	}
}

// snapshotLoop periodically takes snapshots
func (rn *RaftNode) snapshotLoop() {
	defer rn.wg.Done()

	if rn.cfg.Snapshot.Interval <= 0 {
		return // Automatic snapshots disabled
	}

	ticker := time.NewTicker(rn.cfg.Snapshot.Interval)
	defer ticker.Stop()

	var lastSnapshotIndex uint64

	for {
		select {
		case <-rn.ctx.Done():
			return
		case <-ticker.C:
			rn.mu.RLock()
			currentIndex := rn.appliedIndex
			rn.mu.RUnlock()

			// Check if we've applied enough entries since last snapshot
			if currentIndex-lastSnapshotIndex >= rn.cfg.Snapshot.Threshold {
				if err := rn.TakeSnapshot(); err != nil {
					// Log error
					continue
				}
				lastSnapshotIndex = currentIndex
			}
		}
	}
}
