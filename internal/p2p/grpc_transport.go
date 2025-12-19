// Package p2p provides peer-to-peer networking functionality for bib.
package p2p

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// ErrStreamClosed indicates the stream has been closed.
var ErrStreamClosed = errors.New("stream closed")

// streamConn wraps a libp2p stream to implement net.Conn.
// This allows gRPC to use libp2p streams as the underlying transport.
type streamConn struct {
	stream   network.Stream
	mu       sync.Mutex
	closed   bool
	deadline time.Time

	// For net.Addr implementation
	localPeer  peer.ID
	remotePeer peer.ID
}

// newStreamConn creates a new streamConn wrapping a libp2p stream.
func newStreamConn(s network.Stream) *streamConn {
	return &streamConn{
		stream:     s,
		localPeer:  s.Conn().LocalPeer(),
		remotePeer: s.Conn().RemotePeer(),
	}
}

// Read implements net.Conn.
func (c *streamConn) Read(b []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, ErrStreamClosed
	}
	c.mu.Unlock()

	n, err := c.stream.Read(b)
	if err != nil {
		// Map libp2p errors to standard errors
		if errors.Is(err, network.ErrReset) {
			return n, io.EOF
		}
	}
	return n, err
}

// Write implements net.Conn.
func (c *streamConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, ErrStreamClosed
	}
	c.mu.Unlock()

	return c.stream.Write(b)
}

// Close implements net.Conn.
func (c *streamConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	return c.stream.Close()
}

// LocalAddr implements net.Conn.
func (c *streamConn) LocalAddr() net.Addr {
	return &peerAddr{id: c.localPeer}
}

// RemoteAddr implements net.Conn.
func (c *streamConn) RemoteAddr() net.Addr {
	return &peerAddr{id: c.remotePeer}
}

// SetDeadline implements net.Conn.
func (c *streamConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.deadline = t
	c.mu.Unlock()
	return c.stream.SetDeadline(t)
}

// SetReadDeadline implements net.Conn.
func (c *streamConn) SetReadDeadline(t time.Time) error {
	return c.stream.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn.
func (c *streamConn) SetWriteDeadline(t time.Time) error {
	return c.stream.SetWriteDeadline(t)
}

// PeerID returns the remote peer's ID.
func (c *streamConn) PeerID() peer.ID {
	return c.remotePeer
}

// peerAddr implements net.Addr for libp2p peer IDs.
type peerAddr struct {
	id peer.ID
}

// Network implements net.Addr.
func (a *peerAddr) Network() string {
	return "libp2p"
}

// String implements net.Addr.
func (a *peerAddr) String() string {
	return a.id.String()
}

// p2pListener implements net.Listener for libp2p streams.
// It accepts incoming gRPC connections over the libp2p gRPC protocol.
type p2pListener struct {
	host       host.Host
	ctx        context.Context
	cancel     context.CancelFunc
	acceptCh   chan network.Stream
	closedOnce sync.Once
	closed     bool
	mu         sync.Mutex
}

// newP2PListener creates a new listener for gRPC-over-P2P connections.
func newP2PListener(ctx context.Context, h host.Host) *p2pListener {
	ctx, cancel := context.WithCancel(ctx)
	l := &p2pListener{
		host:     h,
		ctx:      ctx,
		cancel:   cancel,
		acceptCh: make(chan network.Stream, 16),
	}

	// Register stream handler for gRPC protocol
	h.SetStreamHandler(ProtocolGRPC, l.handleStream)

	return l
}

// handleStream is called when a new gRPC stream is opened.
func (l *p2pListener) handleStream(s network.Stream) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		s.Reset()
		return
	}
	l.mu.Unlock()

	select {
	case l.acceptCh <- s:
	case <-l.ctx.Done():
		s.Reset()
	}
}

// Accept implements net.Listener.
func (l *p2pListener) Accept() (net.Conn, error) {
	select {
	case s := <-l.acceptCh:
		return newStreamConn(s), nil
	case <-l.ctx.Done():
		return nil, l.ctx.Err()
	}
}

// Close implements net.Listener.
func (l *p2pListener) Close() error {
	l.closedOnce.Do(func() {
		l.mu.Lock()
		l.closed = true
		l.mu.Unlock()

		l.cancel()
		l.host.RemoveStreamHandler(ProtocolGRPC)
		close(l.acceptCh)
	})
	return nil
}

// Addr implements net.Listener.
func (l *p2pListener) Addr() net.Addr {
	return &peerAddr{id: l.host.ID()}
}

// p2pDialer provides dialing functionality for gRPC over libp2p.
type p2pDialer struct {
	host    host.Host
	timeout time.Duration
}

// newP2PDialer creates a new dialer for gRPC-over-P2P connections.
func newP2PDialer(h host.Host, timeout time.Duration) *p2pDialer {
	return &p2pDialer{
		host:    h,
		timeout: timeout,
	}
}

// DialContext connects to a peer and returns a net.Conn for gRPC.
func (d *p2pDialer) DialContext(ctx context.Context, peerID peer.ID) (net.Conn, error) {
	if d.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.timeout)
		defer cancel()
	}

	s, err := d.host.NewStream(ctx, peerID, ProtocolGRPC)
	if err != nil {
		return nil, err
	}

	return newStreamConn(s), nil
}

// DialContextAddr connects to a peer specified as a string address.
// The address should be a peer ID string.
func (d *p2pDialer) DialContextAddr(ctx context.Context, addr string) (net.Conn, error) {
	peerID, err := peer.Decode(addr)
	if err != nil {
		return nil, err
	}
	return d.DialContext(ctx, peerID)
}
