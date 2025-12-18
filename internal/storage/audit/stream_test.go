package audit

import (
	"sync"
	"testing"
	"time"
)

func TestNewStreamer(t *testing.T) {
	cfg := DefaultStreamerConfig()
	streamer := NewStreamer(cfg)

	if streamer == nil {
		t.Fatal("NewStreamer returned nil")
	}

	if streamer.bufferSize != cfg.BufferSize {
		t.Errorf("bufferSize = %d, want %d", streamer.bufferSize, cfg.BufferSize)
	}
}

func TestStreamer_PublishAndSubscribe(t *testing.T) {
	streamer := NewStreamer(StreamerConfig{
		BufferSize:  100,
		ChannelSize: 10,
	})
	defer streamer.Close()

	// Subscribe
	ch, unsubscribe := streamer.Subscribe("test-subscriber", 10)
	defer unsubscribe()

	// Publish
	entry := NewEntry("node-1", "op-123", "bibd_query", "test", ActionSelect)
	streamer.Publish(entry)

	// Receive
	select {
	case received := <-ch:
		if received.OperationID != entry.OperationID {
			t.Errorf("Received wrong entry: got %s, want %s", received.OperationID, entry.OperationID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for entry")
	}
}

func TestStreamer_MultipleSubscribers(t *testing.T) {
	streamer := NewStreamer(DefaultStreamerConfig())
	defer streamer.Close()

	// Create multiple subscribers
	ch1, unsub1 := streamer.Subscribe("sub-1", 10)
	defer unsub1()
	ch2, unsub2 := streamer.Subscribe("sub-2", 10)
	defer unsub2()
	ch3, unsub3 := streamer.Subscribe("sub-3", 10)
	defer unsub3()

	if streamer.SubscriberCount() != 3 {
		t.Errorf("SubscriberCount = %d, want 3", streamer.SubscriberCount())
	}

	// Publish
	entry := NewEntry("node-1", "op-123", "bibd_query", "test", ActionSelect)
	streamer.Publish(entry)

	// All subscribers should receive
	for i, ch := range []<-chan *Entry{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			if received.OperationID != entry.OperationID {
				t.Errorf("Subscriber %d received wrong entry", i+1)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Subscriber %d timeout", i+1)
		}
	}
}

func TestStreamer_Unsubscribe(t *testing.T) {
	streamer := NewStreamer(DefaultStreamerConfig())
	defer streamer.Close()

	ch, unsubscribe := streamer.Subscribe("test", 10)

	if streamer.SubscriberCount() != 1 {
		t.Errorf("SubscriberCount = %d, want 1", streamer.SubscriberCount())
	}

	unsubscribe()

	if streamer.SubscriberCount() != 0 {
		t.Errorf("SubscriberCount after unsubscribe = %d, want 0", streamer.SubscriberCount())
	}

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("Channel should be closed after unsubscribe")
		}
	default:
		// Channel closed, as expected
	}
}

func TestStreamer_Buffer(t *testing.T) {
	bufferSize := 5
	streamer := NewStreamer(StreamerConfig{
		BufferSize:  bufferSize,
		ChannelSize: 10,
	})
	defer streamer.Close()

	// Publish more than buffer size
	for i := 0; i < bufferSize+3; i++ {
		entry := NewEntry("node-1", GenerateOperationID(), "bibd_query", "test", ActionSelect)
		streamer.Publish(entry)
	}

	// Should only have bufferSize entries
	recent := streamer.RecentEntries(100)
	if len(recent) != bufferSize {
		t.Errorf("RecentEntries = %d, want %d", len(recent), bufferSize)
	}
}

func TestStreamer_RecentEntries(t *testing.T) {
	streamer := NewStreamer(StreamerConfig{
		BufferSize:  100,
		ChannelSize: 10,
	})
	defer streamer.Close()

	// Publish some entries
	for i := 0; i < 10; i++ {
		entry := NewEntry("node-1", GenerateOperationID(), "bibd_query", "test", ActionSelect)
		streamer.Publish(entry)
	}

	// Get recent 5
	recent := streamer.RecentEntries(5)
	if len(recent) != 5 {
		t.Errorf("RecentEntries(5) = %d, want 5", len(recent))
	}

	// Get more than available
	recent = streamer.RecentEntries(100)
	if len(recent) != 10 {
		t.Errorf("RecentEntries(100) = %d, want 10", len(recent))
	}
}

func TestStreamer_ConcurrentAccess(t *testing.T) {
	streamer := NewStreamer(StreamerConfig{
		BufferSize:  1000,
		ChannelSize: 100,
	})
	defer streamer.Close()

	var wg sync.WaitGroup

	// Concurrent publishers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				entry := NewEntry("node-1", GenerateOperationID(), "bibd_query", "test", ActionSelect)
				streamer.Publish(entry)
			}
		}()
	}

	// Concurrent subscribers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ch, unsub := streamer.Subscribe(GenerateOperationID(), 100)
			defer unsub()

			count := 0
			timeout := time.After(500 * time.Millisecond)
			for {
				select {
				case <-ch:
					count++
					if count >= 50 {
						return
					}
				case <-timeout:
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestStreamer_Close(t *testing.T) {
	streamer := NewStreamer(DefaultStreamerConfig())

	ch1, _ := streamer.Subscribe("sub-1", 10)
	ch2, _ := streamer.Subscribe("sub-2", 10)

	// Close streamer
	err := streamer.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Channels should be closed
	for _, ch := range []<-chan *Entry{ch1, ch2} {
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("Channel should be closed after streamer close")
			}
		default:
			// Expected
		}
	}

	// Subscriber count should be 0
	if streamer.SubscriberCount() != 0 {
		t.Errorf("SubscriberCount after close = %d, want 0", streamer.SubscriberCount())
	}

	// Publishing after close should not panic
	entry := NewEntry("node-1", "op-123", "bibd_query", "test", ActionSelect)
	streamer.Publish(entry) // Should not panic
}
