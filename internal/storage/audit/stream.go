package audit

import (
	"context"
	"sync"
	"time"
)

// Streamer handles real-time streaming of audit events.
type Streamer struct {
	mu          sync.RWMutex
	subscribers map[string]chan *Entry
	buffer      []*Entry
	bufferSize  int
	closed      bool
}

// StreamerConfig holds configuration for the audit streamer.
type StreamerConfig struct {
	// BufferSize is the number of recent entries to keep in memory.
	BufferSize int

	// ChannelSize is the buffer size for subscriber channels.
	ChannelSize int
}

// DefaultStreamerConfig returns default streamer configuration.
func DefaultStreamerConfig() StreamerConfig {
	return StreamerConfig{
		BufferSize:  1000,
		ChannelSize: 100,
	}
}

// NewStreamer creates a new audit event streamer.
func NewStreamer(cfg StreamerConfig) *Streamer {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}

	return &Streamer{
		subscribers: make(map[string]chan *Entry),
		buffer:      make([]*Entry, 0, cfg.BufferSize),
		bufferSize:  cfg.BufferSize,
	}
}

// Publish sends an audit entry to all subscribers.
func (s *Streamer) Publish(entry *Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	// Add to buffer (ring buffer behavior)
	if len(s.buffer) >= s.bufferSize {
		// Remove oldest entry
		s.buffer = s.buffer[1:]
	}
	s.buffer = append(s.buffer, entry)

	// Send to all subscribers (non-blocking)
	for id, ch := range s.subscribers {
		select {
		case ch <- entry:
		default:
			// Channel full, skip (subscriber too slow)
			// Could log this or track metrics
			_ = id
		}
	}
}

// Subscribe creates a new subscription to audit events.
// Returns a channel that receives entries and a function to unsubscribe.
func (s *Streamer) Subscribe(id string, channelSize int) (<-chan *Entry, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if channelSize <= 0 {
		channelSize = 100
	}

	ch := make(chan *Entry, channelSize)
	s.subscribers[id] = ch

	unsubscribe := func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if existingCh, ok := s.subscribers[id]; ok {
			close(existingCh)
			delete(s.subscribers, id)
		}
	}

	return ch, unsubscribe
}

// SubscriberCount returns the number of active subscribers.
func (s *Streamer) SubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers)
}

// RecentEntries returns the most recent entries from the buffer.
func (s *Streamer) RecentEntries(limit int) []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.buffer) {
		limit = len(s.buffer)
	}

	// Return most recent entries
	start := len(s.buffer) - limit
	result := make([]*Entry, limit)
	copy(result, s.buffer[start:])
	return result
}

// Close closes the streamer and all subscriber channels.
func (s *Streamer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	for id, ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, id)
	}

	return nil
}

// StreamProcessor processes audit entries from a stream.
type StreamProcessor struct {
	streamer    *Streamer
	handlers    []EntryHandler
	errorCh     chan error
	doneCh      chan struct{}
	wg          sync.WaitGroup
	subscribeID string
}

// EntryHandler processes an audit entry.
type EntryHandler func(ctx context.Context, entry *Entry) error

// NewStreamProcessor creates a processor that handles entries from a stream.
func NewStreamProcessor(streamer *Streamer, subscribeID string, handlers ...EntryHandler) *StreamProcessor {
	return &StreamProcessor{
		streamer:    streamer,
		handlers:    handlers,
		errorCh:     make(chan error, 100),
		doneCh:      make(chan struct{}),
		subscribeID: subscribeID,
	}
}

// Start begins processing entries.
func (p *StreamProcessor) Start(ctx context.Context) {
	entryCh, unsubscribe := p.streamer.Subscribe(p.subscribeID, 100)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return
			case <-p.doneCh:
				return
			case entry, ok := <-entryCh:
				if !ok {
					return
				}
				p.processEntry(ctx, entry)
			}
		}
	}()
}

// processEntry runs all handlers on an entry.
func (p *StreamProcessor) processEntry(ctx context.Context, entry *Entry) {
	for _, handler := range p.handlers {
		if err := handler(ctx, entry); err != nil {
			select {
			case p.errorCh <- err:
			default:
				// Error channel full, skip
			}
		}
	}
}

// Errors returns a channel of processing errors.
func (p *StreamProcessor) Errors() <-chan error {
	return p.errorCh
}

// Stop stops the processor.
func (p *StreamProcessor) Stop() {
	close(p.doneCh)
	p.wg.Wait()
	close(p.errorCh)
}

// BatchedStreamer buffers entries and processes them in batches.
type BatchedStreamer struct {
	streamer      *Streamer
	batchSize     int
	flushInterval time.Duration
	handler       BatchHandler
	batch         []*Entry
	mu            sync.Mutex
	timer         *time.Timer
	doneCh        chan struct{}
	wg            sync.WaitGroup
	subscribeID   string
}

// BatchHandler processes a batch of audit entries.
type BatchHandler func(ctx context.Context, entries []*Entry) error

// NewBatchedStreamer creates a streamer that processes entries in batches.
func NewBatchedStreamer(streamer *Streamer, subscribeID string, batchSize int, flushInterval time.Duration, handler BatchHandler) *BatchedStreamer {
	if batchSize <= 0 {
		batchSize = 100
	}
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}

	return &BatchedStreamer{
		streamer:      streamer,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		handler:       handler,
		batch:         make([]*Entry, 0, batchSize),
		doneCh:        make(chan struct{}),
		subscribeID:   subscribeID,
	}
}

// Start begins batch processing.
func (b *BatchedStreamer) Start(ctx context.Context) {
	entryCh, unsubscribe := b.streamer.Subscribe(b.subscribeID, 100)

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer unsubscribe()

		b.timer = time.NewTimer(b.flushInterval)
		defer b.timer.Stop()

		for {
			select {
			case <-ctx.Done():
				b.flush(ctx)
				return
			case <-b.doneCh:
				b.flush(ctx)
				return
			case <-b.timer.C:
				b.flush(ctx)
				b.timer.Reset(b.flushInterval)
			case entry, ok := <-entryCh:
				if !ok {
					b.flush(ctx)
					return
				}
				b.addToBatch(ctx, entry)
			}
		}
	}()
}

// addToBatch adds an entry to the current batch.
func (b *BatchedStreamer) addToBatch(ctx context.Context, entry *Entry) {
	b.mu.Lock()
	b.batch = append(b.batch, entry)
	shouldFlush := len(b.batch) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		b.flush(ctx)
	}
}

// flush processes the current batch.
func (b *BatchedStreamer) flush(ctx context.Context) {
	b.mu.Lock()
	if len(b.batch) == 0 {
		b.mu.Unlock()
		return
	}
	batch := b.batch
	b.batch = make([]*Entry, 0, b.batchSize)
	b.mu.Unlock()

	// Process batch (ignore errors for now, could add error handling)
	_ = b.handler(ctx, batch)
}

// Stop stops the batch processor.
func (b *BatchedStreamer) Stop() {
	close(b.doneCh)
	b.wg.Wait()
}
