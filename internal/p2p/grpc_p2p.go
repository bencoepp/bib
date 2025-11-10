package p2p

import (
	"context"
	"errors"
	"net"

	"github.com/charmbracelet/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/gostream"
	"google.golang.org/grpc"
)

// DefaultGRPCProtocolID is the libp2p protocol used to serve gRPC.
const DefaultGRPCProtocolID protocol.ID = "/bib/grpc/1"

// GRPCOverP2P holds the gRPC server and listener bound to a libp2p protocol.
type GRPCOverP2P struct {
	ln  net.Listener
	srv *grpc.Server
}

// Stop gracefully stops the gRPC server and closes the listener.
func (g *GRPCOverP2P) Stop() {
	if g.srv != nil {
		g.srv.GracefulStop()
	}
	if g.ln != nil {
		_ = g.ln.Close()
	}
}

// StartGRPCOverP2P creates a net.Listener over libp2p streams for protocolID,
// constructs a gRPC server, calls register to register your services,
// and starts serving in background until ctx is canceled.
func StartGRPCOverP2P(ctx context.Context, h host.Host, protocolID protocol.ID, register func(*grpc.Server)) (*GRPCOverP2P, error) {
	ln, err := gostream.Listen(h, protocolID)
	if err != nil {
		return nil, err
	}

	srv := grpc.NewServer()
	if register != nil {
		register(srv)
	}

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
		_ = ln.Close()
	}()

	go func() {
		if err := srv.Serve(ln); err != nil &&
			!errors.Is(err, net.ErrClosed) {
			log.Printf("p2p: gRPC over libp2p serve exited: %v", err)
		}
	}()

	return &GRPCOverP2P{ln: ln, srv: srv}, nil
}
