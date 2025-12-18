//go:build integration

// Package cluster_test contains integration tests for the Raft cluster.
package cluster_test

import (
	"fmt"
	"testing"
	"time"

	"bib/internal/cluster"
	"bib/internal/config"
	"bib/test/testutil"
	"bib/test/testutil/helpers"
)

// TestCluster_SingleNode_Integration tests a single-node cluster.
func TestCluster_SingleNode_Integration(t *testing.T) {
	ctx := testutil.TestContext(t)
	dataDir := testutil.TempDir(t, "cluster")
	port := helpers.GetFreePort(t)

	cfg := config.ClusterConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		ListenAddr:  fmt.Sprintf("127.0.0.1:%d", port),
		Raft: config.RaftConfig{
			HeartbeatTimeout: 500 * time.Millisecond,
			ElectionTimeout:  2 * time.Second,
		},
	}

	c, err := cluster.New(cfg, dataDir)
	if err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}
	defer c.Close()

	// Start the cluster (bootstrap as single node)
	if err := c.Start(ctx); err != nil {
		t.Fatalf("failed to start cluster: %v", err)
	}

	// Wait for leader election
	helpers.Eventually(t, 10*time.Second, func() bool {
		return c.IsLeader()
	}, "node should become leader")

	t.Logf("Node %s is leader", c.NodeID())
}

// TestCluster_ThreeNode_Integration tests a three-node cluster.
func TestCluster_ThreeNode_Integration(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	// Get three free ports
	ports := helpers.GetFreePorts(t, 3)

	// Create three cluster nodes
	nodes := make([]*cluster.Cluster, 3)
	dataDirs := make([]string, 3)

	for i := 0; i < 3; i++ {
		dataDir := testutil.TempDir(t, fmt.Sprintf("cluster-node%d", i))
		dataDirs[i] = dataDir

		cfg := config.ClusterConfig{
			Enabled:     true,
			ClusterName: "test-cluster",
			NodeID:      fmt.Sprintf("node-%d", i),
			ListenAddr:  fmt.Sprintf("127.0.0.1:%d", ports[i]),
			Raft: config.RaftConfig{
				HeartbeatTimeout: 500 * time.Millisecond,
				ElectionTimeout:  2 * time.Second,
			},
		}

		c, err := cluster.New(cfg, dataDir)
		if err != nil {
			t.Fatalf("failed to create cluster node %d: %v", i, err)
		}
		nodes[i] = c
		defer c.Close()
	}

	// Bootstrap the first node
	if err := nodes[0].Bootstrap(ctx); err != nil {
		t.Fatalf("failed to bootstrap: %v", err)
	}

	// Start all nodes
	for i, node := range nodes {
		if err := node.Start(ctx); err != nil {
			t.Fatalf("failed to start node %d: %v", i, err)
		}
	}

	// Join nodes 1 and 2 to the cluster
	for i := 1; i < 3; i++ {
		peerAddr := fmt.Sprintf("127.0.0.1:%d", ports[0])
		if err := nodes[i].Join(ctx, peerAddr); err != nil {
			t.Fatalf("failed to join node %d: %v", i, err)
		}
	}

	// Wait for cluster to stabilize
	time.Sleep(5 * time.Second)

	// Verify exactly one leader
	leaderCount := 0
	var leaderID string
	for _, node := range nodes {
		if node.IsLeader() {
			leaderCount++
			leaderID = node.NodeID()
		}
	}

	if leaderCount != 1 {
		t.Errorf("expected exactly 1 leader, got %d", leaderCount)
	}
	t.Logf("Leader: %s", leaderID)

	// Verify all nodes are healthy
	for i, node := range nodes {
		state := node.State()
		t.Logf("Node %d (%s): state=%s, isLeader=%v", i, node.NodeID(), state, node.IsLeader())
	}
}

// TestCluster_LeaderFailover_Integration tests leader failover.
func TestCluster_LeaderFailover_Integration(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	// Get three free ports
	ports := helpers.GetFreePorts(t, 3)

	// Create three cluster nodes
	nodes := make([]*cluster.Cluster, 3)

	for i := 0; i < 3; i++ {
		dataDir := testutil.TempDir(t, fmt.Sprintf("failover-node%d", i))

		cfg := config.ClusterConfig{
			Enabled:     true,
			ClusterName: "failover-cluster",
			NodeID:      fmt.Sprintf("node-%d", i),
			ListenAddr:  fmt.Sprintf("127.0.0.1:%d", ports[i]),
			Raft: config.RaftConfig{
				HeartbeatTimeout: 200 * time.Millisecond,
				ElectionTimeout:  1 * time.Second,
			},
		}

		c, err := cluster.New(cfg, dataDir)
		if err != nil {
			t.Fatalf("failed to create cluster node %d: %v", i, err)
		}
		nodes[i] = c
	}

	// Bootstrap and start first node
	if err := nodes[0].Bootstrap(ctx); err != nil {
		t.Fatalf("failed to bootstrap: %v", err)
	}
	if err := nodes[0].Start(ctx); err != nil {
		t.Fatalf("failed to start node 0: %v", err)
	}

	// Wait for first node to become leader
	helpers.Eventually(t, 10*time.Second, func() bool {
		return nodes[0].IsLeader()
	}, "node 0 should become leader")

	// Start and join other nodes
	for i := 1; i < 3; i++ {
		if err := nodes[i].Start(ctx); err != nil {
			t.Fatalf("failed to start node %d: %v", i, err)
		}
		peerAddr := fmt.Sprintf("127.0.0.1:%d", ports[0])
		if err := nodes[i].Join(ctx, peerAddr); err != nil {
			t.Fatalf("failed to join node %d: %v", i, err)
		}
	}

	// Wait for cluster to stabilize
	time.Sleep(3 * time.Second)

	// Find and kill the leader
	var leaderIdx int
	for i, node := range nodes {
		if node.IsLeader() {
			leaderIdx = i
			break
		}
	}
	t.Logf("Killing leader: node %d", leaderIdx)
	nodes[leaderIdx].Close()

	// Wait for new leader election
	helpers.Eventually(t, 15*time.Second, func() bool {
		for i, node := range nodes {
			if i == leaderIdx {
				continue
			}
			if node.IsLeader() {
				return true
			}
		}
		return false
	}, "new leader should be elected")

	// Find new leader
	var newLeaderID string
	for i, node := range nodes {
		if i == leaderIdx {
			continue
		}
		if node.IsLeader() {
			newLeaderID = node.NodeID()
			break
		}
		node.Close()
	}

	t.Logf("New leader: %s", newLeaderID)
}
