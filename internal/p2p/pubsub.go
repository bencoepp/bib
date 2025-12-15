package p2p

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"bib/internal/config"
	"bib/internal/domain"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PubSub topic names
const (
	TopicGlobal = "/bib/global"
	TopicNodes  = "/bib/nodes"
	// Topic pattern: /bib/topics/<topic-id>
)

// PubSubMessageType identifies the type of pubsub message.
type PubSubMessageType string

const (
	PubSubNodeJoin      PubSubMessageType = "node_join"
	PubSubNodeLeave     PubSubMessageType = "node_leave"
	PubSubNodeStatus    PubSubMessageType = "node_status"
	PubSubNewTopic      PubSubMessageType = "new_topic"
	PubSubNewDataset    PubSubMessageType = "new_dataset"
	PubSubTopicUpdate   PubSubMessageType = "topic_update"
	PubSubDeleteDataset PubSubMessageType = "delete_dataset"
)

// PubSubMessage is the wrapper for all pubsub messages.
type PubSubMessage struct {
	Type         PubSubMessageType `json:"type"`
	SenderPeerID string            `json:"sender_peer_id"`
	Timestamp    time.Time         `json:"timestamp"`
	Signature    []byte            `json:"signature"`
	Payload      json.RawMessage   `json:"payload"`
}

// NodeStatusPayload is the payload for node status messages.
type NodeStatusPayload struct {
	PeerID             string    `json:"peer_id"`
	NodeMode           string    `json:"node_mode"`
	ConnectedPeers     int       `json:"connected_peers"`
	DatasetCount       int       `json:"dataset_count"`
	StorageUsedBytes   int64     `json:"storage_used_bytes"`
	StorageTotalBytes  int64     `json:"storage_total_bytes"`
	CPUUsagePercent    float32   `json:"cpu_usage_percent"`
	MemoryUsagePercent float32   `json:"memory_usage_percent"`
	ActiveJobs         int       `json:"active_jobs"`
	UptimeSince        time.Time `json:"uptime_since"`
}

// TopicUpdatePayload is the payload for topic update messages.
type TopicUpdatePayload struct {
	TopicID     string               `json:"topic_id"`
	UpdateType  string               `json:"update_type"` // "new_dataset", "updated_dataset", "deleted_dataset"
	Entry       *domain.CatalogEntry `json:"entry,omitempty"`
	DeletedHash string               `json:"deleted_hash,omitempty"`
}

// PubSubConfig holds pubsub configuration.
type PubSubConfig struct {
	// MessageSignature enables message signing.
	MessageSignature bool
	// StrictSignature requires valid signatures (rejects unsigned).
	StrictSignature bool
	// MaxMessageSize is the maximum message size in bytes.
	MaxMessageSize int
	// MessageTTL is the maximum age of messages to accept.
	MessageTTL time.Duration
}

// DefaultPubSubConfig returns default pubsub configuration.
func DefaultPubSubConfig() PubSubConfig {
	return PubSubConfig{
		MessageSignature: true,
		StrictSignature:  true,
		MaxMessageSize:   1024 * 1024, // 1MB
		MessageTTL:       5 * time.Minute,
	}
}

// PubSub manages GossipSub pubsub for real-time updates.
type PubSub struct {
	host   host.Host
	ps     *pubsub.PubSub
	cfg    PubSubConfig
	p2pCfg config.P2PConfig

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Subscriptions
	mu       sync.RWMutex
	topics   map[string]*pubsub.Topic
	subs     map[string]*pubsub.Subscription
	handlers map[string][]PubSubHandler

	// Identity for signing
	identity *Identity
}

// PubSubHandler handles received pubsub messages.
type PubSubHandler func(ctx context.Context, msg *PubSubMessage) error

// NewPubSub creates a new PubSub manager.
func NewPubSub(ctx context.Context, h host.Host, identity *Identity, p2pCfg config.P2PConfig) (*PubSub, error) {
	cfg := DefaultPubSubConfig()

	// Create GossipSub with options
	opts := []pubsub.Option{
		pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
		pubsub.WithMaxMessageSize(cfg.MaxMessageSize),
	}

	ps, err := pubsub.NewGossipSub(ctx, h, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossipsub: %w", err)
	}

	psCtx, cancel := context.WithCancel(ctx)

	p := &PubSub{
		host:     h,
		ps:       ps,
		cfg:      cfg,
		p2pCfg:   p2pCfg,
		ctx:      psCtx,
		cancel:   cancel,
		topics:   make(map[string]*pubsub.Topic),
		subs:     make(map[string]*pubsub.Subscription),
		handlers: make(map[string][]PubSubHandler),
		identity: identity,
	}

	return p, nil
}

// Start subscribes to default topics and begins processing.
func (p *PubSub) Start() error {
	// Subscribe to global topic
	if err := p.Subscribe(TopicGlobal); err != nil {
		return err
	}

	// Subscribe to nodes topic
	if err := p.Subscribe(TopicNodes); err != nil {
		return err
	}

	// Start status broadcaster
	p.wg.Add(1)
	go p.statusBroadcaster()

	return nil
}

// Stop stops the pubsub manager.
func (p *PubSub) Stop() error {
	p.cancel()
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Close all subscriptions
	for _, sub := range p.subs {
		sub.Cancel()
	}

	// Close all topics
	for _, topic := range p.topics {
		topic.Close()
	}

	return nil
}

// Subscribe subscribes to a topic.
func (p *PubSub) Subscribe(topicName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if already subscribed
	if _, ok := p.subs[topicName]; ok {
		return nil
	}

	// Join the topic
	topic, err := p.ps.Join(topicName)
	if err != nil {
		return fmt.Errorf("failed to join topic %s: %w", topicName, err)
	}
	p.topics[topicName] = topic

	// Subscribe to the topic
	sub, err := topic.Subscribe()
	if err != nil {
		topic.Close()
		delete(p.topics, topicName)
		return fmt.Errorf("failed to subscribe to topic %s: %w", topicName, err)
	}
	p.subs[topicName] = sub

	// Start message handler
	p.wg.Add(1)
	go p.handleMessages(topicName, sub)

	return nil
}

// Unsubscribe unsubscribes from a topic.
func (p *PubSub) Unsubscribe(topicName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if sub, ok := p.subs[topicName]; ok {
		sub.Cancel()
		delete(p.subs, topicName)
	}

	if topic, ok := p.topics[topicName]; ok {
		topic.Close()
		delete(p.topics, topicName)
	}

	return nil
}

// SubscribeToDataTopic subscribes to a data topic.
func (p *PubSub) SubscribeToDataTopic(topicID string) error {
	return p.Subscribe(fmt.Sprintf("/bib/topics/%s", topicID))
}

// UnsubscribeFromDataTopic unsubscribes from a data topic.
func (p *PubSub) UnsubscribeFromDataTopic(topicID string) error {
	return p.Unsubscribe(fmt.Sprintf("/bib/topics/%s", topicID))
}

// OnMessage registers a handler for messages on a topic.
func (p *PubSub) OnMessage(topicName string, handler PubSubHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[topicName] = append(p.handlers[topicName], handler)
}

// Publish publishes a message to a topic.
func (p *PubSub) Publish(ctx context.Context, topicName string, msgType PubSubMessageType, payload interface{}) error {
	p.mu.RLock()
	topic, ok := p.topics[topicName]
	p.mu.RUnlock()

	if !ok {
		// Try to join the topic
		var err error
		topic, err = p.ps.Join(topicName)
		if err != nil {
			return fmt.Errorf("failed to join topic: %w", err)
		}
		p.mu.Lock()
		p.topics[topicName] = topic
		p.mu.Unlock()
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := PubSubMessage{
		Type:         msgType,
		SenderPeerID: p.host.ID().String(),
		Timestamp:    time.Now(),
		Payload:      payloadBytes,
	}

	// Sign the message
	if p.cfg.MessageSignature && p.identity != nil {
		sig, err := p.signMessage(&msg)
		if err != nil {
			return err
		}
		msg.Signature = sig
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return topic.Publish(ctx, msgBytes)
}

// signMessage signs a message.
func (p *PubSub) signMessage(msg *PubSubMessage) ([]byte, error) {
	// Create hash of message content
	content := fmt.Sprintf("%s:%s:%d", msg.Type, msg.SenderPeerID, msg.Timestamp.UnixNano())
	content += string(msg.Payload)
	hash := sha256.Sum256([]byte(content))

	// Sign with identity key
	return p.identity.PrivKey.Sign(hash[:])
}

// verifyMessage verifies a message signature.
func (p *PubSub) verifyMessage(msg *PubSubMessage) (bool, error) {
	if len(msg.Signature) == 0 {
		return !p.cfg.StrictSignature, nil
	}

	// Get sender's public key
	senderID, err := peer.Decode(msg.SenderPeerID)
	if err != nil {
		return false, err
	}

	pubKey, err := senderID.ExtractPublicKey()
	if err != nil {
		return false, err
	}

	// Create hash of message content
	content := fmt.Sprintf("%s:%s:%d", msg.Type, msg.SenderPeerID, msg.Timestamp.UnixNano())
	content += string(msg.Payload)
	hash := sha256.Sum256([]byte(content))

	return pubKey.Verify(hash[:], msg.Signature)
}

// handleMessages processes messages from a subscription.
func (p *PubSub) handleMessages(topicName string, sub *pubsub.Subscription) {
	defer p.wg.Done()

	for {
		msg, err := sub.Next(p.ctx)
		if err != nil {
			if p.ctx.Err() != nil {
				return // Context cancelled
			}
			continue
		}

		// Skip our own messages
		if msg.ReceivedFrom == p.host.ID() {
			continue
		}

		// Parse the message
		var psMsg PubSubMessage
		if err := json.Unmarshal(msg.Data, &psMsg); err != nil {
			continue
		}

		// Validate timestamp
		if p.cfg.MessageTTL > 0 {
			age := time.Since(psMsg.Timestamp)
			if age > p.cfg.MessageTTL || age < -p.cfg.MessageTTL {
				continue // Message too old or from the future
			}
		}

		// Verify signature
		if p.cfg.MessageSignature {
			valid, err := p.verifyMessage(&psMsg)
			if err != nil || !valid {
				continue
			}
		}

		// Call handlers
		p.mu.RLock()
		handlers := p.handlers[topicName]
		p.mu.RUnlock()

		for _, handler := range handlers {
			go handler(p.ctx, &psMsg)
		}
	}
}

// statusBroadcaster periodically broadcasts node status.
func (p *PubSub) statusBroadcaster() {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			status := NodeStatusPayload{
				PeerID:         p.host.ID().String(),
				NodeMode:       p.p2pCfg.Mode,
				ConnectedPeers: len(p.host.Network().Peers()),
				UptimeSince:    startTime,
				// TODO: Fill in storage and resource metrics
			}

			if err := p.Publish(p.ctx, TopicNodes, PubSubNodeStatus, status); err != nil {
				// Log error
			}
		}
	}
}

// AnnounceNodeJoin announces this node joining the network.
func (p *PubSub) AnnounceNodeJoin() error {
	payload := map[string]interface{}{
		"peer_id":   p.host.ID().String(),
		"addresses": p.host.Addrs(),
		"node_mode": p.p2pCfg.Mode,
	}
	return p.Publish(p.ctx, TopicGlobal, PubSubNodeJoin, payload)
}

// AnnounceNodeLeave announces this node leaving the network.
func (p *PubSub) AnnounceNodeLeave(reason string) error {
	payload := map[string]interface{}{
		"peer_id": p.host.ID().String(),
		"reason":  reason,
	}
	return p.Publish(p.ctx, TopicGlobal, PubSubNodeLeave, payload)
}

// AnnounceNewDataset announces a new dataset.
func (p *PubSub) AnnounceNewDataset(entry *domain.CatalogEntry) error {
	return p.Publish(p.ctx, TopicGlobal, PubSubNewDataset, entry)
}

// PublishTopicUpdate publishes an update to a data topic.
func (p *PubSub) PublishTopicUpdate(topicID string, updateType string, entry *domain.CatalogEntry, deletedHash string) error {
	payload := TopicUpdatePayload{
		TopicID:     topicID,
		UpdateType:  updateType,
		Entry:       entry,
		DeletedHash: deletedHash,
	}
	return p.Publish(p.ctx, fmt.Sprintf("/bib/topics/%s", topicID), PubSubTopicUpdate, payload)
}

// ListSubscribedTopics returns the list of subscribed topics.
func (p *PubSub) ListSubscribedTopics() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	topics := make([]string, 0, len(p.subs))
	for topic := range p.subs {
		topics = append(topics, topic)
	}
	return topics
}
