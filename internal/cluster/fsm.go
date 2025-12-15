package cluster

import (
	"encoding/json"
	"fmt"
	"sync"
)

// CommandType represents the type of command applied to the FSM
type CommandType uint8

const (
	// CmdCatalogUpdate updates the catalog
	CmdCatalogUpdate CommandType = iota + 1
	// CmdCatalogDelete removes a catalog entry
	CmdCatalogDelete
	// CmdJobCreate creates a new job
	CmdJobCreate
	// CmdJobUpdate updates a job
	CmdJobUpdate
	// CmdJobAssign assigns a job to a node
	CmdJobAssign
	// CmdConfigSet sets a configuration value
	CmdConfigSet
	// CmdConfigDelete deletes a configuration value
	CmdConfigDelete
	// CmdJoinTokenCreate creates a join token
	CmdJoinTokenCreate
)

// Command represents a command to be applied to the FSM
type Command struct {
	Type CommandType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

// FSM implements the finite state machine for Raft
// It replicates:
// - Cluster membership
// - Global catalog (authoritative source)
// - Job scheduling/assignments
// - Global configuration
type FSM struct {
	storage *Storage
	mu      sync.RWMutex

	// In-memory caches for fast access
	catalog map[string]*ReplicatedCatalogEntry // key: topicID/datasetID
	jobs    map[string]*ReplicatedJob
	config  map[string][]byte
}

// NewFSM creates a new FSM
func NewFSM(storage *Storage) *FSM {
	return &FSM{
		storage: storage,
		catalog: make(map[string]*ReplicatedCatalogEntry),
		jobs:    make(map[string]*ReplicatedJob),
		config:  make(map[string][]byte),
	}
}

// Apply applies a command to the FSM
func (f *FSM) Apply(data []byte) error {
	var cmd Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	switch cmd.Type {
	case CmdCatalogUpdate:
		return f.applyCatalogUpdate(cmd.Data)
	case CmdCatalogDelete:
		return f.applyCatalogDelete(cmd.Data)
	case CmdJobCreate, CmdJobUpdate:
		return f.applyJobUpdate(cmd.Data)
	case CmdJobAssign:
		return f.applyJobAssign(cmd.Data)
	case CmdConfigSet:
		return f.applyConfigSet(cmd.Data)
	case CmdConfigDelete:
		return f.applyConfigDelete(cmd.Data)
	case CmdJoinTokenCreate:
		return f.applyJoinTokenCreate(cmd.Data)
	default:
		return fmt.Errorf("unknown command type: %d", cmd.Type)
	}
}

// applyCatalogUpdate handles catalog update commands
func (f *FSM) applyCatalogUpdate(data []byte) error {
	var entry ReplicatedCatalogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}

	// Update storage
	if err := f.storage.StoreCatalogEntry(&entry); err != nil {
		return err
	}

	// Update cache
	key := fmt.Sprintf("%s/%s", entry.TopicID, entry.DatasetID)
	f.catalog[key] = &entry

	return nil
}

// applyCatalogDelete handles catalog delete commands
func (f *FSM) applyCatalogDelete(data []byte) error {
	var req struct {
		TopicID   string `json:"topic_id"`
		DatasetID string `json:"dataset_id"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return err
	}

	// TODO: Add delete method to storage
	// For now, just update cache
	key := fmt.Sprintf("%s/%s", req.TopicID, req.DatasetID)
	delete(f.catalog, key)

	return nil
}

// applyJobUpdate handles job create/update commands
func (f *FSM) applyJobUpdate(data []byte) error {
	var job ReplicatedJob
	if err := json.Unmarshal(data, &job); err != nil {
		return err
	}

	// Update storage
	if err := f.storage.StoreJob(&job); err != nil {
		return err
	}

	// Update cache
	f.jobs[job.JobID] = &job

	return nil
}

// applyJobAssign handles job assignment commands
func (f *FSM) applyJobAssign(data []byte) error {
	var req struct {
		JobID        string `json:"job_id"`
		AssignedNode string `json:"assigned_node"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return err
	}

	// Update cache
	if job, exists := f.jobs[req.JobID]; exists {
		job.AssignedNode = req.AssignedNode
		job.Status = "assigned"

		// Update storage
		return f.storage.StoreJob(job)
	}

	return nil
}

// applyConfigSet handles config set commands
func (f *FSM) applyConfigSet(data []byte) error {
	var req struct {
		Key   string `json:"key"`
		Value []byte `json:"value"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return err
	}

	// Update storage
	if err := f.storage.StoreConfig(req.Key, req.Value); err != nil {
		return err
	}

	// Update cache
	f.config[req.Key] = req.Value

	return nil
}

// applyConfigDelete handles config delete commands
func (f *FSM) applyConfigDelete(data []byte) error {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return err
	}

	// TODO: Add delete method to storage
	// For now, just update cache
	delete(f.config, req.Key)

	return nil
}

// applyJoinTokenCreate handles join token creation
func (f *FSM) applyJoinTokenCreate(data []byte) error {
	var token JoinToken
	if err := json.Unmarshal(data, &token); err != nil {
		return err
	}

	return f.storage.StoreJoinToken(&token)
}

// Snapshot creates a snapshot of the FSM state
func (f *FSM) Snapshot() ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	snapshot := struct {
		Catalog map[string]*ReplicatedCatalogEntry `json:"catalog"`
		Jobs    map[string]*ReplicatedJob          `json:"jobs"`
		Config  map[string][]byte                  `json:"config"`
	}{
		Catalog: f.catalog,
		Jobs:    f.jobs,
		Config:  f.config,
	}

	return json.Marshal(snapshot)
}

// Restore restores the FSM from a snapshot
func (f *FSM) Restore(data []byte) error {
	var snapshot struct {
		Catalog map[string]*ReplicatedCatalogEntry `json:"catalog"`
		Jobs    map[string]*ReplicatedJob          `json:"jobs"`
		Config  map[string][]byte                  `json:"config"`
	}

	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Restore in-memory state
	f.catalog = snapshot.Catalog
	f.jobs = snapshot.Jobs
	f.config = snapshot.Config

	// Restore storage
	for _, entry := range f.catalog {
		if err := f.storage.StoreCatalogEntry(entry); err != nil {
			return err
		}
	}

	for _, job := range f.jobs {
		if err := f.storage.StoreJob(job); err != nil {
			return err
		}
	}

	for key, value := range f.config {
		if err := f.storage.StoreConfig(key, value); err != nil {
			return err
		}
	}

	return nil
}

// StoreJoinToken stores a join token (called directly, not through Apply)
func (f *FSM) StoreJoinToken(token *JoinToken) error {
	// Create command for replication
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	cmd := Command{
		Type: CmdJoinTokenCreate,
		Data: data,
	}

	cmdData, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	// This will be applied through Raft
	return f.Apply(cmdData)
}

// GetCatalog returns the current catalog
func (f *FSM) GetCatalog() map[string]*ReplicatedCatalogEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]*ReplicatedCatalogEntry, len(f.catalog))
	for k, v := range f.catalog {
		result[k] = v
	}
	return result
}

// GetJob returns a job by ID
func (f *FSM) GetJob(jobID string) *ReplicatedJob {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.jobs[jobID]
}

// GetConfig returns a config value
func (f *FSM) GetConfig(key string) []byte {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.config[key]
}

// CreateCommand creates a command for the given type and data
func CreateCommand(cmdType CommandType, data interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	cmd := Command{
		Type: cmdType,
		Data: jsonData,
	}

	return json.Marshal(cmd)
}
