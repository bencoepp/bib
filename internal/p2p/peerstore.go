package p2p

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	_ "modernc.org/sqlite"
)

// PeerScore tracks connection quality metrics for a peer.
type PeerScore struct {
	// ConnectionSuccesses is the number of successful connections.
	ConnectionSuccesses int
	// ConnectionFailures is the number of failed connection attempts.
	ConnectionFailures int
	// AverageLatencyMs is the average connection latency in milliseconds.
	AverageLatencyMs float64
	// LastSeen is the last time we successfully connected to this peer.
	LastSeen time.Time
	// IsBootstrap indicates if this is a bootstrap peer.
	IsBootstrap bool
}

// Score returns a computed score for peer ranking.
// Higher is better. Bootstrap peers get a large bonus.
func (ps *PeerScore) Score() float64 {
	if ps.IsBootstrap {
		return 1000000 // Bootstrap peers always have highest priority
	}

	// Base score from success rate
	total := ps.ConnectionSuccesses + ps.ConnectionFailures
	if total == 0 {
		return 0
	}
	successRate := float64(ps.ConnectionSuccesses) / float64(total)

	// Latency penalty (lower latency is better)
	latencyPenalty := 0.0
	if ps.AverageLatencyMs > 0 {
		latencyPenalty = 100.0 / ps.AverageLatencyMs // Higher latency = lower score
	}

	// Recency bonus (more recent = better)
	recencyBonus := 0.0
	if !ps.LastSeen.IsZero() {
		hoursSinceLastSeen := time.Since(ps.LastSeen).Hours()
		if hoursSinceLastSeen < 1 {
			recencyBonus = 100
		} else if hoursSinceLastSeen < 24 {
			recencyBonus = 50
		} else if hoursSinceLastSeen < 168 { // 1 week
			recencyBonus = 10
		}
	}

	return (successRate * 100) + latencyPenalty + recencyBonus
}

// PeerStore provides persistent storage for peer information.
type PeerStore struct {
	db  *sql.DB
	cfg config.PeerStoreConfig
	mu  sync.RWMutex
}

// NewPeerStore creates a new SQLite-backed peer store.
func NewPeerStore(cfg config.PeerStoreConfig, configDir string) (*PeerStore, error) {
	path := cfg.Path
	if path == "" {
		path = filepath.Join(configDir, "peers.db")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open peer store: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	ps := &PeerStore{db: db, cfg: cfg}

	if err := ps.init(); err != nil {
		db.Close()
		return nil, err
	}

	return ps, nil
}

// init creates the necessary tables.
func (ps *PeerStore) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS peers (
		id TEXT PRIMARY KEY,
		addrs TEXT,
		connection_successes INTEGER DEFAULT 0,
		connection_failures INTEGER DEFAULT 0,
		average_latency_ms REAL DEFAULT 0,
		last_seen TIMESTAMP,
		is_bootstrap INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_peers_last_seen ON peers(last_seen);
	CREATE INDEX IF NOT EXISTS idx_peers_is_bootstrap ON peers(is_bootstrap);
	`

	_, err := ps.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create peer store schema: %w", err)
	}

	return nil
}

// AddPeer adds or updates a peer in the store.
func (ps *PeerStore) AddPeer(info peer.AddrInfo, isBootstrap bool) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	addrs := marshalAddrs(info.Addrs)

	_, err := ps.db.Exec(`
		INSERT INTO peers (id, addrs, is_bootstrap, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			addrs = excluded.addrs,
			is_bootstrap = CASE WHEN excluded.is_bootstrap = 1 THEN 1 ELSE is_bootstrap END,
			updated_at = CURRENT_TIMESTAMP
	`, info.ID.String(), addrs, boolToInt(isBootstrap))

	return err
}

// RecordConnection records a connection attempt result.
func (ps *PeerStore) RecordConnection(id peer.ID, success bool, latencyMs float64) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if success {
		_, err := ps.db.Exec(`
			UPDATE peers SET
				connection_successes = connection_successes + 1,
				average_latency_ms = (average_latency_ms * connection_successes + ?) / (connection_successes + 1),
				last_seen = CURRENT_TIMESTAMP,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, latencyMs, id.String())
		return err
	}

	_, err := ps.db.Exec(`
		UPDATE peers SET
			connection_failures = connection_failures + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id.String())
	return err
}

// GetPeer retrieves a peer's information.
func (ps *PeerStore) GetPeer(id peer.ID) (*peer.AddrInfo, *PeerScore, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var addrsStr string
	var score PeerScore
	var lastSeen sql.NullTime
	var isBootstrap int

	err := ps.db.QueryRow(`
		SELECT addrs, connection_successes, connection_failures, average_latency_ms, last_seen, is_bootstrap
		FROM peers WHERE id = ?
	`, id.String()).Scan(&addrsStr, &score.ConnectionSuccesses, &score.ConnectionFailures,
		&score.AverageLatencyMs, &lastSeen, &isBootstrap)

	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	addrs := unmarshalAddrs(addrsStr)
	score.LastSeen = lastSeen.Time
	score.IsBootstrap = isBootstrap == 1

	return &peer.AddrInfo{ID: id, Addrs: addrs}, &score, nil
}

// GetBestPeers returns peers sorted by score (best first).
func (ps *PeerStore) GetBestPeers(limit int) ([]peer.AddrInfo, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Compute score in SQL for efficiency
	// Priority: 1) Bootstrap, 2) Success rate, 3) Recency
	rows, err := ps.db.Query(`
		SELECT id, addrs FROM peers
		ORDER BY 
			is_bootstrap DESC,
			CASE WHEN connection_successes + connection_failures > 0 
				THEN CAST(connection_successes AS REAL) / (connection_successes + connection_failures)
				ELSE 0 
			END DESC,
			last_seen DESC NULLS LAST
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []peer.AddrInfo
	for rows.Next() {
		var idStr, addrsStr string
		if err := rows.Scan(&idStr, &addrsStr); err != nil {
			return nil, err
		}

		id, err := peer.Decode(idStr)
		if err != nil {
			continue // Skip invalid peer IDs
		}

		peers = append(peers, peer.AddrInfo{
			ID:    id,
			Addrs: unmarshalAddrs(addrsStr),
		})
	}

	return peers, rows.Err()
}

// GetBootstrapPeers returns all bootstrap peers.
func (ps *PeerStore) GetBootstrapPeers() ([]peer.AddrInfo, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	rows, err := ps.db.Query(`SELECT id, addrs FROM peers WHERE is_bootstrap = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []peer.AddrInfo
	for rows.Next() {
		var idStr, addrsStr string
		if err := rows.Scan(&idStr, &addrsStr); err != nil {
			return nil, err
		}

		id, err := peer.Decode(idStr)
		if err != nil {
			continue
		}

		peers = append(peers, peer.AddrInfo{
			ID:    id,
			Addrs: unmarshalAddrs(addrsStr),
		})
	}

	return peers, rows.Err()
}

// PruneOldPeers removes peers not seen in the given duration.
// Bootstrap peers are never pruned.
func (ps *PeerStore) PruneOldPeers(maxAge time.Duration) (int64, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	result, err := ps.db.Exec(`
		DELETE FROM peers 
		WHERE is_bootstrap = 0 
		AND (last_seen IS NULL OR last_seen < ?)
	`, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// Count returns the total number of peers in the store.
func (ps *PeerStore) Count() (int, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var count int
	err := ps.db.QueryRow(`SELECT COUNT(*) FROM peers`).Scan(&count)
	return count, err
}

// Close closes the peer store.
func (ps *PeerStore) Close() error {
	return ps.db.Close()
}

// marshalAddrs converts multiaddrs to a comma-separated string.
func marshalAddrs(addrs []multiaddr.Multiaddr) string {
	strs := make([]string, len(addrs))
	for i, addr := range addrs {
		strs[i] = addr.String()
	}
	return joinStrings(strs, ",")
}

// unmarshalAddrs parses a comma-separated string into multiaddrs.
func unmarshalAddrs(s string) []multiaddr.Multiaddr {
	if s == "" {
		return nil
	}

	parts := splitString(s, ",")
	addrs := make([]multiaddr.Multiaddr, 0, len(parts))
	for _, p := range parts {
		if addr, err := multiaddr.NewMultiaddr(p); err == nil {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

// boolToInt converts a bool to an int for SQLite.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// joinStrings joins strings with a separator (avoiding strings import).
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// splitString splits a string by separator (avoiding strings import).
func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}

	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}
