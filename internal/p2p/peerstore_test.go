package p2p

import (
	"os"
	"testing"
	"time"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func TestPeerStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-peerstore-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.PeerStoreConfig{}
	ps, err := NewPeerStore(cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create peer store: %v", err)
	}
	defer ps.Close()

	// Generate a test peer ID
	testAddr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/4001")
	testPeerID, _ := peer.Decode("12D3KooWDXHortdoEeRiLx7FAtQ2ebEvS25R4ZKmVzpiz2sa2JWG")
	testInfo := peer.AddrInfo{
		ID:    testPeerID,
		Addrs: []multiaddr.Multiaddr{testAddr},
	}

	// Test adding a peer
	err = ps.AddPeer(testInfo, false)
	if err != nil {
		t.Fatalf("failed to add peer: %v", err)
	}

	// Test count
	count, err := ps.Count()
	if err != nil {
		t.Fatalf("failed to count peers: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 peer, got %d", count)
	}

	// Test getting the peer
	info, score, err := ps.GetPeer(testPeerID)
	if err != nil {
		t.Fatalf("failed to get peer: %v", err)
	}
	if info == nil {
		t.Fatal("peer not found")
	}
	if info.ID != testPeerID {
		t.Fatalf("wrong peer ID: %s", info.ID)
	}
	if score.IsBootstrap {
		t.Fatal("peer should not be bootstrap")
	}
}

func TestPeerStoreBootstrapPriority(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-peerstore-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.PeerStoreConfig{}
	ps, err := NewPeerStore(cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create peer store: %v", err)
	}
	defer ps.Close()

	// Add a regular peer
	regularID, _ := peer.Decode("12D3KooWDXHortdoEeRiLx7FAtQ2ebEvS25R4ZKmVzpiz2sa2JWG")
	regularInfo := peer.AddrInfo{ID: regularID}
	ps.AddPeer(regularInfo, false)

	// Add a bootstrap peer
	bootstrapID, _ := peer.Decode("12D3KooWEjmxuA1NS51nPY72AP7TLyGHCKP3Y2DD1yofM1zyRsKu")
	bootstrapInfo := peer.AddrInfo{ID: bootstrapID}
	ps.AddPeer(bootstrapInfo, true)

	// Get best peers - bootstrap should be first
	best, err := ps.GetBestPeers(10)
	if err != nil {
		t.Fatalf("failed to get best peers: %v", err)
	}
	if len(best) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(best))
	}
	if best[0].ID != bootstrapID {
		t.Fatal("bootstrap peer should be first")
	}
}

func TestPeerScoring(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-peerstore-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.PeerStoreConfig{}
	ps, err := NewPeerStore(cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create peer store: %v", err)
	}
	defer ps.Close()

	testID, _ := peer.Decode("12D3KooWDXHortdoEeRiLx7FAtQ2ebEvS25R4ZKmVzpiz2sa2JWG")
	testInfo := peer.AddrInfo{ID: testID}
	ps.AddPeer(testInfo, false)

	// Record some connections
	ps.RecordConnection(testID, true, 50)
	ps.RecordConnection(testID, true, 60)
	ps.RecordConnection(testID, false, 0)

	_, score, err := ps.GetPeer(testID)
	if err != nil {
		t.Fatalf("failed to get peer: %v", err)
	}

	if score.ConnectionSuccesses != 2 {
		t.Fatalf("expected 2 successes, got %d", score.ConnectionSuccesses)
	}
	if score.ConnectionFailures != 1 {
		t.Fatalf("expected 1 failure, got %d", score.ConnectionFailures)
	}
	if score.LastSeen.IsZero() {
		t.Fatal("last seen should be set")
	}
}

func TestPeerStorePruning(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-peerstore-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.PeerStoreConfig{}
	ps, err := NewPeerStore(cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create peer store: %v", err)
	}
	defer ps.Close()

	// Add a regular peer (never seen)
	regularID, _ := peer.Decode("12D3KooWDXHortdoEeRiLx7FAtQ2ebEvS25R4ZKmVzpiz2sa2JWG")
	ps.AddPeer(peer.AddrInfo{ID: regularID}, false)

	// Add a bootstrap peer (never seen)
	bootstrapID, _ := peer.Decode("12D3KooWEjmxuA1NS51nPY72AP7TLyGHCKP3Y2DD1yofM1zyRsKu")
	ps.AddPeer(peer.AddrInfo{ID: bootstrapID}, true)

	// Prune peers not seen in 1 nanosecond (should prune regular, keep bootstrap)
	pruned, err := ps.PruneOldPeers(time.Nanosecond)
	if err != nil {
		t.Fatalf("failed to prune peers: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned, got %d", pruned)
	}

	// Verify bootstrap peer still exists
	bootstrapPeers, err := ps.GetBootstrapPeers()
	if err != nil {
		t.Fatalf("failed to get bootstrap peers: %v", err)
	}
	if len(bootstrapPeers) != 1 {
		t.Fatalf("expected 1 bootstrap peer, got %d", len(bootstrapPeers))
	}

	// Verify regular peer was pruned
	count, _ := ps.Count()
	if count != 1 {
		t.Fatalf("expected 1 peer remaining, got %d", count)
	}
}
