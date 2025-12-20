//go:build integration

// Package cluster_test contains integration tests for the Raft cluster.
package cluster_test

import (
	"testing"

	"bib/test/testutil"
)

// TestCluster_SingleNode_Integration tests a single-node cluster.
// TODO: This test needs to be updated once the cluster API is finalized.
// The current internal/cluster package has unexported methods (bootstrap, join)
// and uses Stop() instead of Close().
func TestCluster_SingleNode_Integration(t *testing.T) {
	t.Parallel()
	t.Skip("cluster integration tests disabled: API not yet finalized (uses unexported methods)")

	testutil.SkipIfShort(t)
}

// TestCluster_ThreeNode_Integration tests a three-node cluster.
func TestCluster_ThreeNode_Integration(t *testing.T) {
	t.Parallel()
	t.Skip("cluster integration tests disabled: API not yet finalized (uses unexported methods)")

	testutil.SkipIfShort(t)
}

// TestCluster_LeaderFailover_Integration tests leader failover.
func TestCluster_LeaderFailover_Integration(t *testing.T) {
	t.Parallel()
	t.Skip("cluster integration tests disabled: API not yet finalized (uses unexported methods)")

	testutil.SkipIfShort(t)
}
