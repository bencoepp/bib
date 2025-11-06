package transparency

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"sync"
)

// Manager handles append-only log and root signing.
type Manager struct {
	mu        sync.Mutex
	tree      *MerkleTree
	logPath   string
	rootsPath string
	logPrivy  ed25519.PrivateKey
	logPub    ed25519.PublicKey
}

func NewManager(logPath, rootsPath string, logPrivy ed25519.PrivateKey, logPub ed25519.PublicKey) *Manager {
	return &Manager{
		tree:      NewMerkleTree(),
		logPath:   logPath,
		rootsPath: rootsPath,
		logPrivy:  logPrivy,
		logPub:    logPub,
	}
}

// Load reconstructs Merkle tree from existing log file.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // empty log
		}
		return err
	}

	start := 0
	for start < len(data) {
		i := start
		for i < len(data) && data[i] != '\n' {
			i++
		}
		line := data[start:i]
		start = i + 1
		if len(line) == 0 {
			continue
		}
		var reg Registration
		if err := json.Unmarshal(line, &reg); err != nil {
			return err
		}
		canon, err := Canonical(&reg)
		if err != nil {
			return err
		}
		m.tree.AddLeaf(canon)
	}
	return nil
}

// AppendRegistration adds a registration leaf and writes a new signed root.
func (m *Manager) AppendRegistration(reg *Registration) (*SignedLogRoot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	canon, err := Canonical(reg)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(m.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(append(canon, '\n')); err != nil {
		err := f.Close()
		if err != nil {
			return nil, err
		}
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}

	m.tree.AddLeaf(canon)

	root, err := SignRoot(m.tree, m.logPrivy, m.logPub)
	if err != nil {
		return nil, err
	}
	rf, err := os.OpenFile(m.rootsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	rootBytes, _ := json.Marshal(root)
	if _, err := rf.Write(append(rootBytes, '\n')); err != nil {
		err := rf.Close()
		if err != nil {
			return nil, err
		}
		return nil, err
	}
	err = rf.Close()
	if err != nil {
		return nil, err
	}

	return root, nil
}

// InclusionProof returns leaf hash + sibling path.
func (m *Manager) InclusionProof(index uint64) ([]byte, [][]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index >= uint64(len(m.tree.Leaves)) {
		return nil, nil, errors.New("index out of range")
	}
	leaf := m.tree.Leaves[index]
	path, err := m.tree.InclusionProof(index)
	return leaf, path, err
}

func (m *Manager) CurrentRoot() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tree.Root()
}

func (m *Manager) TreeSize() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return uint64(len(m.tree.Leaves))
}
