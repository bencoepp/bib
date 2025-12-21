// Package ssh provides the Wish SSH server for bibd TUI access.
package ssh

import (
	"context"
	"fmt"
	"net"

	"bib/internal/auth"
	"bib/internal/config"
	"bib/internal/domain"
	"bib/internal/logger"
	"bib/internal/storage"
	"bib/internal/tui/i18n"
	"bib/internal/tui/shared"
	"bib/internal/tui/themes"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
)

// Server is the SSH server for bibd TUI access.
type Server struct {
	cfg      *config.SSHConfig
	log      *logger.Logger
	store    storage.Store
	authSvc  *auth.Service
	server   *ssh.Server
	listener net.Listener
}

// ServerOption configures the SSH server.
type ServerOption func(*Server)

// WithLogger sets the logger.
func WithLogger(log *logger.Logger) ServerOption {
	return func(s *Server) {
		s.log = log
	}
}

// WithStore sets the storage.
func WithStore(store storage.Store) ServerOption {
	return func(s *Server) {
		s.store = store
	}
}

// WithAuthService sets the authentication service.
func WithAuthService(authSvc *auth.Service) ServerOption {
	return func(s *Server) {
		s.authSvc = authSvc
	}
}

// NewServer creates a new SSH server.
func NewServer(cfg *config.SSHConfig, opts ...ServerOption) (*Server, error) {
	s := &Server{
		cfg: cfg,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Start starts the SSH server.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	// Create Wish server with middleware
	srv, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath(s.cfg.HostKeyPath),
		wish.WithPublicKeyAuth(s.publicKeyHandler),
		wish.WithMiddleware(
			s.tuiMiddleware(),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH server: %w", err)
	}

	s.server = srv

	// Start server in goroutine
	go func() {
		if s.log != nil {
			s.log.Info("starting SSH server", "addr", addr)
		}
		if err := srv.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			if s.log != nil {
				s.log.Error("SSH server error", "error", err)
			}
		}
	}()

	return nil
}

// Stop stops the SSH server.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	if s.log != nil {
		s.log.Info("stopping SSH server")
	}

	return s.server.Shutdown(ctx)
}

// publicKeyHandler handles SSH public key authentication.
func (s *Server) publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	// Parse the SSH public key
	pubKeyBytes, keyType, err := auth.ParseSSHPublicKey(key)
	if err != nil {
		if s.log != nil {
			s.log.Debug("failed to parse SSH public key", "error", err)
		}
		return false
	}

	// If we have an auth service, authenticate the user
	if s.authSvc != nil && s.store != nil {
		result, err := s.authSvc.Authenticate(context.Background(), auth.AuthenticateRequest{
			PublicKey: pubKeyBytes,
			KeyType:   keyType,
		})
		if err != nil {
			if s.log != nil {
				s.log.Debug("authentication failed", "error", err)
			}
			return false
		}

		// Store user info in context for later use
		ctx.SetValue("user", result.User)
		ctx.SetValue("session", result.Session)
	}

	return true
}

// tuiMiddleware creates the Bubble Tea middleware for the TUI.
func (s *Server) tuiMiddleware() wish.Middleware {
	teaHandler := func(sshSession ssh.Session) (tea.Model, []tea.ProgramOption) {
		// Get terminal size
		_, _, active := sshSession.Pty()
		if !active {
			return nil, nil
		}

		// Get user info from context
		var username string
		var userID string
		if user, ok := sshSession.Context().Value("user").(*domain.User); ok && user != nil {
			username = user.Name
			if username == "" {
				username = user.Email
			}
			userID = user.ID.String()
		}

		// Create shared TUI
		tui := shared.New(
			shared.WithMode(shared.ModeSSH),
			shared.WithTheme(themes.Global().Active()),
			shared.WithI18n(i18n.Global()),
			shared.WithSSHContext(username, userID),
		)

		return tui, []tea.ProgramOption{
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		}
	}

	return bubbletea.Middleware(teaHandler)
}

// unused - placeholder to avoid import errors
var _ = list.Model{}
var _ = lipgloss.Style{}
