//go:build integration

package grpc_test

import (
	"testing"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	"bib/test/testutil"

	"google.golang.org/grpc/codes"
)

// =============================================================================
// UserService Integration Tests
// =============================================================================

// TestUserService_GetUser tests retrieving a user by ID.
func TestUserService_GetUser(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	// Authenticate to create a user
	authCtx, user, _ := ts.AuthenticateUser(ctx, "GetUserTest")

	t.Run("GetSelf", func(t *testing.T) {
		resp, err := userClient.GetUser(authCtx, &services.GetUserRequest{
			UserId: string(user.ID),
		})
		assertNoError(t, err)

		if resp.User.Id != string(user.ID) {
			t.Errorf("expected user ID %s, got %s", user.ID, resp.User.Id)
		}
		if resp.User.Name != "GetUserTest" {
			t.Errorf("expected name 'GetUserTest', got '%s'", resp.User.Name)
		}
	})

	t.Run("GetNonExistent", func(t *testing.T) {
		_, err := userClient.GetUser(authCtx, &services.GetUserRequest{
			UserId: "non-existent-user-id",
		})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("MissingUserID", func(t *testing.T) {
		_, err := userClient.GetUser(authCtx, &services.GetUserRequest{
			UserId: "",
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

// TestUserService_GetUserByPublicKey tests retrieving a user by public key.
func TestUserService_GetUserByPublicKey(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	// First authenticate with a known key
	pubKey, privKey := generateTestKeyPair(t)
	authClient := services.NewAuthServiceClient(conn)

	// Create challenge and verify
	challengeResp, _ := authClient.Challenge(ctx, &services.ChallengeRequest{
		PublicKey: pubKey,
	})
	sig := signChallengeBytes(t, privKey, challengeResp.Challenge)
	verifyResp, err := authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
		ChallengeId: challengeResp.ChallengeId,
		Signature:   sig,
		Name:        "PubKeyUser",
	})
	assertNoError(t, err)

	// Get authenticated context
	authCtx, _, _ := ts.AuthenticateUser(ctx, "AdminUser")

	t.Run("GetByValidKey", func(t *testing.T) {
		resp, err := userClient.GetUserByPublicKey(authCtx, &services.GetUserByPublicKeyRequest{
			PublicKey: pubKey,
		})
		assertNoError(t, err)

		if resp.User.Id != verifyResp.User.Id {
			t.Errorf("expected user ID %s, got %s", verifyResp.User.Id, resp.User.Id)
		}
	})

	t.Run("GetByInvalidKey", func(t *testing.T) {
		unknownKey, _ := generateTestKeyPair(t)
		_, err := userClient.GetUserByPublicKey(authCtx, &services.GetUserByPublicKeyRequest{
			PublicKey: unknownKey,
		})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("MissingPublicKey", func(t *testing.T) {
		_, err := userClient.GetUserByPublicKey(authCtx, &services.GetUserByPublicKeyRequest{
			PublicKey: nil,
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

// TestUserService_ListUsers tests listing users with pagination.
func TestUserService_ListUsers(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	// Create several users
	for i := 0; i < 10; i++ {
		ts.AuthenticateUser(ctx, "ListUser"+string(rune('A'+i)))
	}

	authCtx, _, _ := ts.AuthenticateUser(ctx, "ListUsersAdmin")

	t.Run("ListAll", func(t *testing.T) {
		resp, err := userClient.ListUsers(authCtx, &services.ListUsersRequest{})
		assertNoError(t, err)

		if len(resp.Users) == 0 {
			t.Error("expected at least one user")
		}
		if resp.PageInfo == nil {
			t.Error("expected page info")
		}
	})

	t.Run("ListWithPagination", func(t *testing.T) {
		// First page
		resp1, err := userClient.ListUsers(authCtx, &services.ListUsersRequest{
			Page: &bibv1.PageRequest{
				Limit:  5,
				Offset: 0,
			},
		})
		assertNoError(t, err)

		if len(resp1.Users) > 5 {
			t.Errorf("expected at most 5 users, got %d", len(resp1.Users))
		}

		// Second page
		resp2, err := userClient.ListUsers(authCtx, &services.ListUsersRequest{
			Page: &bibv1.PageRequest{
				Limit:  5,
				Offset: 5,
			},
		})
		assertNoError(t, err)

		// Verify no overlap
		if len(resp2.Users) > 0 && len(resp1.Users) > 0 {
			for _, u1 := range resp1.Users {
				for _, u2 := range resp2.Users {
					if u1.Id == u2.Id {
						t.Errorf("duplicate user %s in paginated results", u1.Id)
					}
				}
			}
		}
	})

	t.Run("ListByStatus", func(t *testing.T) {
		resp, err := userClient.ListUsers(authCtx, &services.ListUsersRequest{
			Status: services.UserStatus_USER_STATUS_ACTIVE,
		})
		assertNoError(t, err)

		for _, u := range resp.Users {
			if u.Status != services.UserStatus_USER_STATUS_ACTIVE {
				t.Errorf("expected active status, got %v", u.Status)
			}
		}
	})
}

// TestUserService_SearchUsers tests user search functionality.
func TestUserService_SearchUsers(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	// Create users with distinct names
	ts.AuthenticateUser(ctx, "SearchableAlice")
	ts.AuthenticateUser(ctx, "SearchableBob")
	ts.AuthenticateUser(ctx, "DifferentCharlie")

	authCtx, _, _ := ts.AuthenticateUser(ctx, "SearchAdmin")

	t.Run("SearchByName", func(t *testing.T) {
		resp, err := userClient.SearchUsers(authCtx, &services.SearchUsersRequest{
			Query: "Searchable",
		})
		assertNoError(t, err)

		// Should find Alice and Bob, but not Charlie
		foundAlice, foundBob, foundCharlie := false, false, false
		for _, u := range resp.Users {
			if u.Name == "SearchableAlice" {
				foundAlice = true
			}
			if u.Name == "SearchableBob" {
				foundBob = true
			}
			if u.Name == "DifferentCharlie" {
				foundCharlie = true
			}
		}

		if !foundAlice || !foundBob {
			t.Error("expected to find Alice and Bob")
		}
		if foundCharlie {
			t.Error("should not find Charlie with 'Searchable' query")
		}
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		_, err := userClient.SearchUsers(authCtx, &services.SearchUsersRequest{
			Query: "",
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("TooShortQuery", func(t *testing.T) {
		_, err := userClient.SearchUsers(authCtx, &services.SearchUsersRequest{
			Query: "ab", // Typically min 3 chars
		})
		// May return error or empty results depending on implementation
		if err != nil {
			assertGRPCCode(t, err, codes.InvalidArgument)
		}
	})
}

// TestUserService_UpdateUser tests user profile updates.
func TestUserService_UpdateUser(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	authCtx, user, _ := ts.AuthenticateUser(ctx, "UpdateUser")

	t.Run("UpdateOwnProfile", func(t *testing.T) {
		newName := "Updated Name"
		resp, err := userClient.UpdateUser(authCtx, &services.UpdateUserRequest{
			UserId: string(user.ID),
			Name:   &newName,
		})
		assertNoError(t, err)

		if resp.User.Name != newName {
			t.Errorf("expected name '%s', got '%s'", newName, resp.User.Name)
		}
	})

	t.Run("UpdateMetadata", func(t *testing.T) {
		resp, err := userClient.UpdateUser(authCtx, &services.UpdateUserRequest{
			UserId: string(user.ID),
			Metadata: map[string]string{
				"department": "Engineering",
				"location":   "Remote",
			},
		})
		assertNoError(t, err)

		if resp.User.Metadata["department"] != "Engineering" {
			t.Error("expected metadata to be updated")
		}
	})

	t.Run("UpdateNonExistent", func(t *testing.T) {
		name := "Test"
		_, err := userClient.UpdateUser(authCtx, &services.UpdateUserRequest{
			UserId: "non-existent-id",
			Name:   &name,
		})
		// Should fail - can't update non-existent user
		if err == nil {
			t.Error("expected error for non-existent user")
		}
	})
}

// TestUserService_GetOwnUser tests getting current user info via GetUser.
func TestUserService_GetOwnUser(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	authCtx, user, _ := ts.AuthenticateUser(ctx, "GetOwnUser")

	resp, err := userClient.GetUser(authCtx, &services.GetUserRequest{
		UserId: string(user.ID),
	})
	assertNoError(t, err)

	if resp.User.Id != string(user.ID) {
		t.Errorf("expected user ID %s, got %s", user.ID, resp.User.Id)
	}
	if resp.User.Name != "GetOwnUser" {
		t.Errorf("expected name 'GetOwnUser', got '%s'", resp.User.Name)
	}
}

// TestUserService_UpdateOwnUser tests updating current user profile via UpdateUser.
func TestUserService_UpdateOwnUser(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	authCtx, user, _ := ts.AuthenticateUser(ctx, "UpdateOwnUser")

	t.Run("UpdateName", func(t *testing.T) {
		newName := "My New Name"
		resp, err := userClient.UpdateUser(authCtx, &services.UpdateUserRequest{
			UserId: string(user.ID),
			Name:   &newName,
		})
		assertNoError(t, err)

		if resp.User.Name != newName {
			t.Errorf("expected name '%s', got '%s'", newName, resp.User.Name)
		}
	})

	t.Run("UpdateEmail", func(t *testing.T) {
		newEmail := "newemail@example.com"
		resp, err := userClient.UpdateUser(authCtx, &services.UpdateUserRequest{
			UserId: string(user.ID),
			Email:  &newEmail,
		})
		assertNoError(t, err)

		if resp.User.Email != newEmail {
			t.Errorf("expected email '%s', got '%s'", newEmail, resp.User.Email)
		}
	})
}

// TestUserService_UserPreferences tests user preferences CRUD.
func TestUserService_UserPreferences(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	authCtx, user, _ := ts.AuthenticateUser(ctx, "PrefsUser")

	t.Run("UpdatePreferences", func(t *testing.T) {
		resp, err := userClient.UpdateUserPreferences(authCtx, &services.UpdateUserPreferencesRequest{
			UserId: string(user.ID),
			Preferences: &services.UserPreferences{
				Theme:  "dark",
				Locale: "en",
			},
		})
		assertNoError(t, err)

		if resp.Preferences.Theme != "dark" {
			t.Error("expected theme preference to be set")
		}
	})

	t.Run("GetPreferences", func(t *testing.T) {
		resp, err := userClient.GetUserPreferences(authCtx, &services.GetUserPreferencesRequest{
			UserId: string(user.ID),
		})
		assertNoError(t, err)

		if resp.Preferences.Theme != "dark" {
			t.Error("expected theme preference")
		}
		if resp.Preferences.Locale != "en" {
			t.Error("expected locale preference")
		}
	})
}

// TestUserService_AdminOperations tests admin-only user operations.
func TestUserService_AdminOperations(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	// Create admin user
	adminCtx, _, _ := ts.CreateAdminUser(ctx, "AdminUser")

	// Create regular user
	_, targetUser, _ := ts.AuthenticateUser(ctx, "TargetUser")

	t.Run("AdminCanUpdateOtherUser", func(t *testing.T) {
		newName := "Updated by Admin"
		resp, err := userClient.UpdateUser(adminCtx, &services.UpdateUserRequest{
			UserId: string(targetUser.ID),
			Name:   &newName,
		})
		assertNoError(t, err)

		if resp.User.Name != newName {
			t.Errorf("expected name '%s', got '%s'", newName, resp.User.Name)
		}
	})

	t.Run("NonAdminCannotUpdateOtherUser", func(t *testing.T) {
		userCtx, _, _ := ts.AuthenticateUser(ctx, "RegularUser")
		newName := "Hacked Name"
		_, err := userClient.UpdateUser(userCtx, &services.UpdateUserRequest{
			UserId: string(targetUser.ID),
			Name:   &newName,
		})
		// Should fail - regular users can't update other users
		if err == nil {
			t.Error("expected error for non-admin updating other user")
		}
	})
}
