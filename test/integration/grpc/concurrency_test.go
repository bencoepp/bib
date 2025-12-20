//go:build integration

package grpc_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/test/testutil"
)

// =============================================================================
// Concurrency & Stress Tests
// =============================================================================

// TestConcurrency_ManyConnections tests handling many concurrent connections.
func TestConcurrency_ManyConnections(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)

	const numConnections = 20

	var wg sync.WaitGroup
	errors := make(chan error, numConnections)

	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			conn := ts.Dial()
			healthClient := services.NewHealthServiceClient(conn)

			// Each connection makes several requests
			for j := 0; j < 10; j++ {
				_, err := healthClient.Ping(ctx, &services.PingRequest{
					Payload: []byte(fmt.Sprintf("conn-%d-req-%d", id, j)),
				})
				if err != nil {
					errors <- fmt.Errorf("conn %d req %d: %w", id, j, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	var errCount int
	for err := range errors {
		t.Logf("Error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("%d requests failed", errCount)
	}
}

// TestConcurrency_ParallelAuthentication tests many concurrent authentications.
func TestConcurrency_ParallelAuthentication(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	const numAuth = 30

	var wg sync.WaitGroup
	var successCount, failCount int32

	for i := 0; i < numAuth; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			pubKey, privKey := generateTestKeyPair(t)

			// Challenge
			challengeResp, err := authClient.Challenge(reqCtx, &services.ChallengeRequest{
				PublicKey: pubKey,
			})
			if err != nil {
				atomic.AddInt32(&failCount, 1)
				return
			}

			// Sign
			sig := signChallengeBytes(t, privKey, challengeResp.Challenge)

			// Verify
			_, err = authClient.VerifyChallenge(reqCtx, &services.VerifyChallengeRequest{
				ChallengeId: challengeResp.ChallengeId,
				Signature:   sig,
				Name:        fmt.Sprintf("ParallelUser%d", id),
			})
			if err != nil {
				atomic.AddInt32(&failCount, 1)
				return
			}

			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	t.Logf("Success: %d, Failed: %d", successCount, failCount)

	if failCount > 0 {
		t.Errorf("%d authentications failed", failCount)
	}
}

// TestConcurrency_ParallelTopicOperations tests concurrent topic CRUD.
func TestConcurrency_ParallelTopicOperations(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "ConcTopicAdmin")

	const numTopics = 20
	topicIDs := make([]string, numTopics)

	// Create topics concurrently
	t.Run("ParallelCreate", func(t *testing.T) {
		var wg sync.WaitGroup
		var mu sync.Mutex

		for i := 0; i < numTopics; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Create per-request context with its own timeout
				reqCtx, cancel := context.WithTimeout(adminCtx, 30*time.Second)
				defer cancel()

				resp, err := topicClient.CreateTopic(reqCtx, &services.CreateTopicRequest{
					Name:        fmt.Sprintf("concurrent-topic-%d", id),
					Description: fmt.Sprintf("Topic %d", id),
				})
				if err != nil {
					t.Logf("Create topic %d error: %v", id, err)
					return
				}

				mu.Lock()
				topicIDs[id] = resp.Topic.Id
				mu.Unlock()
			}(i)
		}

		wg.Wait()
	})

	// Read topics concurrently
	t.Run("ParallelRead", func(t *testing.T) {
		var wg sync.WaitGroup
		var successCount int32

		for i := 0; i < numTopics; i++ {
			if topicIDs[i] == "" {
				continue
			}
			wg.Add(1)
			go func(id string) {
				defer wg.Done()

				reqCtx, cancel := context.WithTimeout(adminCtx, 30*time.Second)
				defer cancel()

				_, err := topicClient.GetTopic(reqCtx, &services.GetTopicRequest{
					Id: id,
				})
				if err == nil {
					atomic.AddInt32(&successCount, 1)
				}
			}(topicIDs[i])
		}

		wg.Wait()
		t.Logf("Successfully read %d topics", successCount)
	})

	// Update topics concurrently
	t.Run("ParallelUpdate", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := 0; i < numTopics; i++ {
			if topicIDs[i] == "" {
				continue
			}
			wg.Add(1)
			go func(id string, idx int) {
				defer wg.Done()

				reqCtx, cancel := context.WithTimeout(adminCtx, 30*time.Second)
				defer cancel()

				desc := fmt.Sprintf("Updated by goroutine %d", idx)
				_, _ = topicClient.UpdateTopic(reqCtx, &services.UpdateTopicRequest{
					Id:          id,
					Description: &desc,
				})
			}(topicIDs[i], i)
		}

		wg.Wait()
	})

	// Delete topics concurrently
	t.Run("ParallelDelete", func(t *testing.T) {
		var wg sync.WaitGroup
		var deleteCount int32

		for i := 0; i < numTopics; i++ {
			if topicIDs[i] == "" {
				continue
			}
			wg.Add(1)
			go func(id string) {
				defer wg.Done()

				reqCtx, cancel := context.WithTimeout(adminCtx, 30*time.Second)
				defer cancel()

				_, err := topicClient.DeleteTopic(reqCtx, &services.DeleteTopicRequest{
					Id: id,
				})
				if err == nil {
					atomic.AddInt32(&deleteCount, 1)
				}
			}(topicIDs[i])
		}

		wg.Wait()
		t.Logf("Deleted %d topics", deleteCount)
	})
}

// TestConcurrency_MixedOperations tests mixed concurrent operations.
func TestConcurrency_MixedOperations(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()

	healthClient := services.NewHealthServiceClient(conn)
	authClient := services.NewAuthServiceClient(conn)
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "MixedOpAdmin")

	// Create some topics first
	var topicIDs []string
	for i := 0; i < 5; i++ {
		resp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: fmt.Sprintf("mixed-op-topic-%d", i),
		})
		if resp != nil {
			topicIDs = append(topicIDs, resp.Topic.Id)
		}
	}

	const numOperations = 100
	var wg sync.WaitGroup
	var pingCount, authCount, topicCount int32

	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			switch id % 3 {
			case 0:
				// Health ping
				reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				_, err := healthClient.Ping(reqCtx, &services.PingRequest{
					Payload: []byte(fmt.Sprintf("ping-%d", id)),
				})
				if err == nil {
					atomic.AddInt32(&pingCount, 1)
				}

			case 1:
				// Auth challenge
				reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				pubKey, _ := generateTestKeyPair(t)
				_, err := authClient.Challenge(reqCtx, &services.ChallengeRequest{
					PublicKey: pubKey,
				})
				if err == nil {
					atomic.AddInt32(&authCount, 1)
				}

			case 2:
				// Topic list
				if len(topicIDs) > 0 {
					reqCtx, cancel := context.WithTimeout(adminCtx, 30*time.Second)
					defer cancel()
					_, err := topicClient.GetTopic(reqCtx, &services.GetTopicRequest{
						Id: topicIDs[id%len(topicIDs)],
					})
					if err == nil {
						atomic.AddInt32(&topicCount, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Completed - Ping: %d, Auth: %d, Topic: %d", pingCount, authCount, topicCount)

	total := pingCount + authCount + topicCount
	if total < int32(numOperations*80/100) {
		t.Errorf("Too many failures: only %d/%d succeeded", total, numOperations)
	}
}

// TestConcurrency_StreamingStress tests concurrent streaming operations.
func TestConcurrency_StreamingStress(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	healthClient := services.NewHealthServiceClient(conn)

	const numStreams = 10
	const streamDuration = 2 * time.Second

	var wg sync.WaitGroup
	var totalMessages int32

	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			streamCtx, cancel := context.WithTimeout(ctx, streamDuration)
			defer cancel()

			stream, err := healthClient.Watch(streamCtx, &services.HealthCheckRequest{})
			if err != nil {
				t.Logf("Stream %d failed to start: %v", id, err)
				return
			}

			for {
				_, err := stream.Recv()
				if err != nil {
					break
				}
				atomic.AddInt32(&totalMessages, 1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Received %d total messages from %d streams", totalMessages, numStreams)
}

// TestConcurrency_SessionManagement tests concurrent session operations.
func TestConcurrency_SessionManagement(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	// Create multiple sessions for the same user
	pubKey, privKey := generateTestKeyPair(t)
	var sessionTokens []string
	var mu sync.Mutex

	const numSessions = 10

	// Create sessions concurrently
	var wg sync.WaitGroup
	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Challenge
			challengeResp, err := authClient.Challenge(reqCtx, &services.ChallengeRequest{
				PublicKey: pubKey,
			})
			if err != nil {
				return
			}

			// Sign
			sig := signChallengeBytes(t, privKey, challengeResp.Challenge)

			// Verify
			verifyResp, err := authClient.VerifyChallenge(reqCtx, &services.VerifyChallengeRequest{
				ChallengeId: challengeResp.ChallengeId,
				Signature:   sig,
				Name:        "SessionUser",
			})
			if err != nil {
				return
			}

			mu.Lock()
			sessionTokens = append(sessionTokens, verifyResp.SessionToken)
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	t.Logf("Created %d sessions", len(sessionTokens))

	// Validate all sessions concurrently
	var validCount int32
	for _, token := range sessionTokens {
		wg.Add(1)
		go func(tok string) {
			defer wg.Done()

			reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			resp, err := authClient.ValidateSession(reqCtx, &services.ValidateSessionRequest{
				SessionToken: tok,
			})
			if err == nil && resp.Valid {
				atomic.AddInt32(&validCount, 1)
			}
		}(token)
	}

	wg.Wait()

	t.Logf("Validated %d/%d sessions", validCount, len(sessionTokens))
}

// TestConcurrency_RapidCreateDelete tests rapid create/delete cycles.
func TestConcurrency_RapidCreateDelete(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "RapidAdmin")

	const iterations = 20
	var successCount int32

	var wg sync.WaitGroup
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			reqCtx, cancel := context.WithTimeout(adminCtx, 30*time.Second)
			defer cancel()

			// Create
			createResp, err := topicClient.CreateTopic(reqCtx, &services.CreateTopicRequest{
				Name: fmt.Sprintf("rapid-topic-%d-%d", id, time.Now().UnixNano()),
			})
			if err != nil {
				return
			}

			// Immediately delete
			_, err = topicClient.DeleteTopic(reqCtx, &services.DeleteTopicRequest{
				Id: createResp.Topic.Id,
			})
			if err != nil {
				return
			}

			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	t.Logf("Completed %d/%d rapid create/delete cycles", successCount, iterations)
}

// TestConcurrency_DatabaseContention tests database lock contention.
func TestConcurrency_DatabaseContention(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "ContentionAdmin")

	// Create a shared topic
	topicResp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
		Name: "contention-topic",
	})
	assertNoError(t, err)
	topicID := topicResp.Topic.Id

	const numWriters = 10
	const writesPerWriter = 5

	var wg sync.WaitGroup
	var successCount int32
	startTime := time.Now()

	// Multiple writers creating datasets in the same topic
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < writesPerWriter; j++ {
				reqCtx, cancel := context.WithTimeout(adminCtx, 30*time.Second)
				_, err := datasetClient.CreateDataset(reqCtx, &services.CreateDatasetRequest{
					TopicId: topicID,
					Name:    fmt.Sprintf("contention-dataset-%d-%d", id, j),
				})
				cancel()
				if err == nil {
					atomic.AddInt32(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	expected := int32(numWriters * writesPerWriter)
	t.Logf("Completed %d/%d operations in %v", successCount, expected, elapsed)

	if successCount < expected*80/100 {
		t.Errorf("Too many failures under contention")
	}
}
