package discovery

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"

	"bib/internal/auth"

	services "bib/api/gen/go/bib/v1/services"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuthStatus represents the status of an authentication test
type AuthStatus string

const (
	AuthStatusSuccess         AuthStatus = "success"
	AuthStatusFailed          AuthStatus = "failed"
	AuthStatusNoKey           AuthStatus = "no_key"
	AuthStatusKeyRejected     AuthStatus = "key_rejected"
	AuthStatusNotRegistered   AuthStatus = "not_registered"
	AuthStatusAutoRegistered  AuthStatus = "auto_registered"
	AuthStatusConnectionError AuthStatus = "connection_error"
	AuthStatusUnknown         AuthStatus = "unknown"
)

// AuthTestResult contains the result of testing authentication with a node
type AuthTestResult struct {
	// Address is the node address that was tested
	Address string

	// Status is the authentication status
	Status AuthStatus

	// SessionToken is the session token (if authentication succeeded)
	SessionToken string

	// SessionInfo contains session information (if available)
	SessionInfo *SessionInfo

	// ServerConfig contains server auth configuration
	ServerConfig *ServerAuthConfig

	// Error contains any error message
	Error string

	// Duration is how long the authentication took
	Duration time.Duration

	// TestedAt is when the test was performed
	TestedAt time.Time
}

// SessionInfo contains information about the authenticated session
type SessionInfo struct {
	// UserID is the authenticated user ID
	UserID string

	// Username is the authenticated username
	Username string

	// Role is the user's role (admin, user, readonly)
	Role string

	// ExpiresAt is when the session expires
	ExpiresAt time.Time

	// IsNewUser indicates if this was a newly auto-registered user
	IsNewUser bool
}

// ServerAuthConfig contains server authentication configuration
type ServerAuthConfig struct {
	// AllowAutoRegistration indicates if the server allows auto-registration
	AllowAutoRegistration bool

	// RequireEmail indicates if email is required for registration
	RequireEmail bool

	// SupportedKeyTypes lists supported key types (ed25519, rsa, etc.)
	SupportedKeyTypes []string
}

// AuthTester tests authentication with bibd nodes
type AuthTester struct {
	// IdentityKey is the identity key to use for authentication
	IdentityKey *auth.IdentityKey

	// Timeout for authentication attempts
	Timeout time.Duration

	// Name and Email for auto-registration (if supported)
	Name  string
	Email string
}

// NewAuthTester creates a new authentication tester
func NewAuthTester(identityKey *auth.IdentityKey) *AuthTester {
	return &AuthTester{
		IdentityKey: identityKey,
		Timeout:     10 * time.Second,
	}
}

// WithTimeout sets the authentication timeout
func (t *AuthTester) WithTimeout(timeout time.Duration) *AuthTester {
	t.Timeout = timeout
	return t
}

// WithRegistrationInfo sets the name and email for auto-registration
func (t *AuthTester) WithRegistrationInfo(name, email string) *AuthTester {
	t.Name = name
	t.Email = email
	return t
}

// TestAuth tests authentication with a single node
func (t *AuthTester) TestAuth(ctx context.Context, address string) *AuthTestResult {
	result := &AuthTestResult{
		Address:  address,
		Status:   AuthStatusUnknown,
		TestedAt: time.Now(),
	}

	if t.IdentityKey == nil {
		result.Status = AuthStatusNoKey
		result.Error = "no identity key available"
		return result
	}

	// Create context with timeout
	if t.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Timeout)
		defer cancel()
	}

	start := time.Now()

	// Connect to the node
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		result.Status = AuthStatusConnectionError
		result.Error = fmt.Sprintf("connection failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer conn.Close()

	// Get auth config first
	authClient := services.NewAuthServiceClient(conn)
	result.ServerConfig = t.getServerConfig(ctx, authClient)

	// Perform authentication
	sessionToken, sessionInfo, err := t.authenticate(ctx, authClient)
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = t.classifyAuthError(err)
		result.Error = err.Error()
		return result
	}

	result.Status = AuthStatusSuccess
	result.SessionToken = sessionToken
	result.SessionInfo = sessionInfo

	// Check if this was an auto-registration
	if sessionInfo != nil && sessionInfo.IsNewUser {
		result.Status = AuthStatusAutoRegistered
	}

	return result
}

// TestAuths tests authentication with multiple nodes in parallel
func (t *AuthTester) TestAuths(ctx context.Context, addresses []string) []*AuthTestResult {
	results := make([]*AuthTestResult, len(addresses))
	var wg sync.WaitGroup

	for i, addr := range addresses {
		wg.Add(1)
		go func(idx int, address string) {
			defer wg.Done()
			results[idx] = t.TestAuth(ctx, address)
		}(i, addr)
	}

	wg.Wait()
	return results
}

// getServerConfig retrieves the server's authentication configuration
func (t *AuthTester) getServerConfig(ctx context.Context, client services.AuthServiceClient) *ServerAuthConfig {
	resp, err := client.GetAuthConfig(ctx, &services.GetAuthConfigRequest{})
	if err != nil {
		return nil
	}

	config := &ServerAuthConfig{
		AllowAutoRegistration: resp.GetAllowAutoRegistration(),
		RequireEmail:          resp.GetRequireEmail(),
		SupportedKeyTypes:     resp.GetSupportedKeyTypes(),
	}

	return config
}

// authenticate performs the challenge-response authentication
func (t *AuthTester) authenticate(ctx context.Context, client services.AuthServiceClient) (string, *SessionInfo, error) {
	// Get SSH signer from identity key
	signer, err := t.IdentityKey.Signer()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create signer: %w", err)
	}

	pubKey := signer.PublicKey()

	// Request challenge
	challengeResp, err := client.Challenge(ctx, &services.ChallengeRequest{
		PublicKey: ssh.MarshalAuthorizedKey(pubKey),
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to get challenge: %w", err)
	}

	// Sign the challenge
	signature, err := signer.Sign(rand.Reader, challengeResp.GetChallenge())
	if err != nil {
		return "", nil, fmt.Errorf("failed to sign challenge: %w", err)
	}

	// Verify challenge
	verifyResp, err := client.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
		ChallengeId: challengeResp.GetChallengeId(),
		Signature:   signature.Blob,
	})
	if err != nil {
		return "", nil, fmt.Errorf("challenge verification failed: %w", err)
	}

	// Extract session info
	sessionInfo := &SessionInfo{}
	if user := verifyResp.GetUser(); user != nil {
		sessionInfo.UserID = user.GetId()
		sessionInfo.Username = user.GetName()
		sessionInfo.Role = user.GetRole()
		sessionInfo.IsNewUser = verifyResp.GetIsNewUser()
	}
	if verifyResp.GetExpiresAt() != nil {
		sessionInfo.ExpiresAt = verifyResp.GetExpiresAt().AsTime()
	}

	return verifyResp.GetSessionToken(), sessionInfo, nil
}

// classifyAuthError classifies an authentication error into a status
func (t *AuthTester) classifyAuthError(err error) AuthStatus {
	if err == nil {
		return AuthStatusSuccess
	}

	errStr := err.Error()

	// Not registered
	if contains(errStr, "not registered") || contains(errStr, "user not found") || contains(errStr, "unknown user") {
		return AuthStatusNotRegistered
	}

	// Key rejected
	if contains(errStr, "key rejected") || contains(errStr, "invalid signature") || contains(errStr, "signature verification failed") {
		return AuthStatusKeyRejected
	}

	// Connection errors
	if contains(errStr, "connection") || contains(errStr, "unavailable") || contains(errStr, "deadline exceeded") {
		return AuthStatusConnectionError
	}

	return AuthStatusFailed
}

// FormatAuthResult formats an authentication result for display
func FormatAuthResult(result *AuthTestResult) string {
	var sb strings.Builder

	// Status icon
	var statusIcon string
	switch result.Status {
	case AuthStatusSuccess:
		statusIcon = "✓"
	case AuthStatusAutoRegistered:
		statusIcon = "✓+"
	case AuthStatusFailed, AuthStatusKeyRejected:
		statusIcon = "✗"
	case AuthStatusNotRegistered:
		statusIcon = "?"
	case AuthStatusNoKey:
		statusIcon = "🔑"
	case AuthStatusConnectionError:
		statusIcon = "⚡"
	default:
		statusIcon = "?"
	}

	sb.WriteString(fmt.Sprintf("%s %s", statusIcon, result.Address))

	switch result.Status {
	case AuthStatusSuccess:
		sb.WriteString(fmt.Sprintf(" - authenticated (%s)", result.Duration.Round(time.Millisecond)))
		if result.SessionInfo != nil {
			if result.SessionInfo.Username != "" {
				sb.WriteString(fmt.Sprintf(" as %s", result.SessionInfo.Username))
			}
			if result.SessionInfo.Role != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", result.SessionInfo.Role))
			}
		}
	case AuthStatusAutoRegistered:
		sb.WriteString(fmt.Sprintf(" - auto-registered and authenticated (%s)", result.Duration.Round(time.Millisecond)))
		if result.SessionInfo != nil && result.SessionInfo.Username != "" {
			sb.WriteString(fmt.Sprintf(" as %s", result.SessionInfo.Username))
		}
	case AuthStatusNotRegistered:
		sb.WriteString(" - not registered (auto-registration disabled)")
	case AuthStatusKeyRejected:
		sb.WriteString(" - key rejected")
	case AuthStatusNoKey:
		sb.WriteString(" - no identity key")
	case AuthStatusConnectionError:
		sb.WriteString(fmt.Sprintf(" - connection error: %s", result.Error))
	default:
		sb.WriteString(fmt.Sprintf(" - %s", result.Status))
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf(": %s", result.Error))
		}
	}

	return sb.String()
}

// FormatAuthResults formats multiple authentication results for display
func FormatAuthResults(results []*AuthTestResult) string {
	var sb strings.Builder

	success := 0
	autoReg := 0
	failed := 0

	for _, r := range results {
		switch r.Status {
		case AuthStatusSuccess:
			success++
		case AuthStatusAutoRegistered:
			autoReg++
		default:
			failed++
		}
	}

	sb.WriteString(fmt.Sprintf("Authentication Test Results: %d success", success))
	if autoReg > 0 {
		sb.WriteString(fmt.Sprintf(" (%d auto-registered)", autoReg))
	}
	if failed > 0 {
		sb.WriteString(fmt.Sprintf(", %d failed", failed))
	}
	sb.WriteString("\n\n")

	for _, r := range results {
		sb.WriteString(FormatAuthResult(r))
		sb.WriteString("\n")
	}

	return sb.String()
}

// AuthSummary returns a brief summary of authentication results
func AuthSummary(results []*AuthTestResult) string {
	success := 0
	failed := 0
	for _, r := range results {
		if r.Status == AuthStatusSuccess || r.Status == AuthStatusAutoRegistered {
			success++
		} else {
			failed++
		}
	}
	return fmt.Sprintf("%d authenticated, %d failed", success, failed)
}
