package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"bib/internal/config"
)

// Transport handles network communication between Raft nodes
type Transport struct {
	cfg      config.ClusterConfig
	nodeID   string
	listener net.Listener

	mu    sync.RWMutex
	peers map[string]*peerConn

	ctx    context.Context
	cancel context.CancelFunc
}

// peerConn represents a connection to a peer
type peerConn struct {
	nodeID string
	addr   string
	conn   net.Conn
	mu     sync.Mutex
}

// Message types
const (
	MsgTypeRequestVote uint8 = iota + 1
	MsgTypeRequestVoteResp
	MsgTypeAppendEntries
	MsgTypeAppendEntriesResp
	MsgTypeInstallSnapshot
	MsgTypeInstallSnapshotResp
	MsgTypeTimeoutNow
	MsgTypeJoinRequest
	MsgTypeJoinResponse
)

// RaftMessage represents a message between Raft nodes
type RaftMessage struct {
	Type     uint8  `json:"type"`
	From     string `json:"from"`
	To       string `json:"to"`
	Term     uint64 `json:"term"`
	LogIndex uint64 `json:"log_index"`
	LogTerm  uint64 `json:"log_term"`
	Commit   uint64 `json:"commit"`
	Entries  []byte `json:"entries,omitempty"`
	Success  bool   `json:"success"`
	Reject   bool   `json:"reject"`
	Data     []byte `json:"data,omitempty"`
}

// NewTransport creates a new transport
func NewTransport(cfg config.ClusterConfig, nodeID string) (*Transport, error) {
	addr := cfg.ListenAddr
	if addr == "" {
		addr = "0.0.0.0:4002"
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	t := &Transport{
		cfg:      cfg,
		nodeID:   nodeID,
		listener: listener,
		peers:    make(map[string]*peerConn),
		ctx:      ctx,
		cancel:   cancel,
	}

	go t.acceptLoop()

	return t, nil
}

// Close closes the transport
func (t *Transport) Close() error {
	t.cancel()

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, p := range t.peers {
		p.mu.Lock()
		if p.conn != nil {
			p.conn.Close()
		}
		p.mu.Unlock()
	}

	return t.listener.Close()
}

// LocalAddr returns the local address
func (t *Transport) LocalAddr() string {
	return t.listener.Addr().String()
}

// Connect connects to a peer
func (t *Transport) Connect(nodeID, addr string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.peers[nodeID]; exists {
		return nil // Already connected
	}

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	t.peers[nodeID] = &peerConn{
		nodeID: nodeID,
		addr:   addr,
		conn:   conn,
	}

	go t.handleConn(conn, nodeID)

	return nil
}

// Disconnect disconnects from a peer
func (t *Transport) Disconnect(nodeID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	p, exists := t.peers[nodeID]
	if !exists {
		return nil
	}

	p.mu.Lock()
	if p.conn != nil {
		p.conn.Close()
	}
	p.mu.Unlock()

	delete(t.peers, nodeID)
	return nil
}

// Send sends a message to a peer
func (t *Transport) Send(msg *RaftMessage) error {
	t.mu.RLock()
	p, exists := t.peers[msg.To]
	t.mu.RUnlock()

	if !exists {
		return fmt.Errorf("peer %s not connected", msg.To)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil {
		return fmt.Errorf("peer %s connection is nil", msg.To)
	}

	// Simple wire protocol: length-prefixed protobuf or JSON
	// For now, using a simple format
	data, err := encodeMessage(msg)
	if err != nil {
		return err
	}

	// Write length prefix (4 bytes, big endian)
	length := uint32(len(data))
	header := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	p.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := p.conn.Write(header); err != nil {
		return err
	}
	if _, err := p.conn.Write(data); err != nil {
		return err
	}

	return nil
}

// acceptLoop accepts incoming connections
func (t *Transport) acceptLoop() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.ctx.Done():
				return
			default:
				continue
			}
		}

		go t.handleIncomingConn(conn)
	}
}

// handleIncomingConn handles a new incoming connection
func (t *Transport) handleIncomingConn(conn net.Conn) {
	defer conn.Close()

	// Read handshake to get peer node ID
	nodeID, err := t.readHandshake(conn)
	if err != nil {
		return
	}

	t.mu.Lock()
	t.peers[nodeID] = &peerConn{
		nodeID: nodeID,
		addr:   conn.RemoteAddr().String(),
		conn:   conn,
	}
	t.mu.Unlock()

	t.handleConn(conn, nodeID)
}

// handleConn handles messages from a connection
func (t *Transport) handleConn(conn net.Conn, nodeID string) {
	defer func() {
		t.mu.Lock()
		delete(t.peers, nodeID)
		t.mu.Unlock()
	}()

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		msg, err := t.readMessage(conn)
		if err != nil {
			if err != io.EOF {
				// Log error
			}
			return
		}

		// TODO: Dispatch message to Raft node
		_ = msg
	}
}

// readHandshake reads the initial handshake from a connection
func (t *Transport) readHandshake(conn net.Conn) (string, error) {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Simple handshake: 4-byte length + node ID string
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}

	length := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
	if length > 1024 {
		return "", fmt.Errorf("handshake too large: %d", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return "", err
	}

	return string(data), nil
}

// readMessage reads a message from a connection
func (t *Transport) readMessage(conn net.Conn) (*RaftMessage, error) {
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Read length header
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	length := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
	if length > 64*1024*1024 { // 64MB max message size
		return nil, fmt.Errorf("message too large: %d", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}

	return decodeMessage(data)
}

// encodeMessage encodes a message for transmission
func encodeMessage(msg *RaftMessage) ([]byte, error) {
	// Simple encoding: use JSON for now
	// TODO: Use protobuf for efficiency
	return json.Marshal(msg)
}

// decodeMessage decodes a message from wire format
func decodeMessage(data []byte) (*RaftMessage, error) {
	var msg RaftMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
