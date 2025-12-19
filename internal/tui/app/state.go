package app

import (
	"bib/internal/config"
	"bib/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

// State holds the application-wide state.
type State struct {
	// Configuration
	Config *config.BibConfig

	// Connection state
	Connected    bool
	ServerAddr   string
	ConnectionID string

	// User info
	User *domain.User

	// Cached data
	Jobs     []domain.Job
	Datasets []domain.Dataset

	// Cluster info
	ClusterEnabled bool
	ClusterName    string
	ClusterNodes   []ClusterNode

	// Loading states
	Loading    bool
	LoadingMsg string
}

// ClusterNode represents a node in the cluster.
type ClusterNode struct {
	ID       string
	Address  string
	IsLeader bool
	IsVoter  bool
	Healthy  bool
}

// NewState creates a new application state.
func NewState() *State {
	return &State{
		Jobs:     make([]domain.Job, 0),
		Datasets: make([]domain.Dataset, 0),
	}
}

// Connect initiates connection to bibd.
func (s *State) Connect() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement actual connection logic
		// For now, return a connected state
		return StateMsg{
			Type:      StateMsgConnected,
			Connected: true,
		}
	}
}

// LoadInitialData loads initial data from bibd.
func (s *State) LoadInitialData() tea.Cmd {
	return tea.Batch(
		s.loadJobs(),
		s.loadDatasets(),
		s.loadClusterInfo(),
	)
}

func (s *State) loadJobs() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement actual job loading
		return StateMsg{
			Type: StateMsgJobsLoaded,
			Jobs: []domain.Job{},
		}
	}
}

func (s *State) loadDatasets() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement actual dataset loading
		return StateMsg{
			Type:     StateMsgDatasetsLoaded,
			Datasets: []domain.Dataset{},
		}
	}
}

func (s *State) loadClusterInfo() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement actual cluster info loading
		return StateMsg{
			Type: StateMsgClusterLoaded,
		}
	}
}

// HandleMsg processes state-related messages.
func (s *State) HandleMsg(msg StateMsg) {
	switch msg.Type {
	case StateMsgConnected:
		s.Connected = msg.Connected
	case StateMsgDisconnected:
		s.Connected = false
	case StateMsgJobsLoaded:
		s.Jobs = msg.Jobs
	case StateMsgDatasetsLoaded:
		s.Datasets = msg.Datasets
	case StateMsgClusterLoaded:
		s.ClusterEnabled = msg.ClusterEnabled
		s.ClusterName = msg.ClusterName
		s.ClusterNodes = msg.ClusterNodes
	case StateMsgLoading:
		s.Loading = true
		s.LoadingMsg = msg.LoadingMsg
	case StateMsgLoadingDone:
		s.Loading = false
		s.LoadingMsg = ""
	}
}

// StateMsgType identifies the type of state message.
type StateMsgType int

const (
	StateMsgConnected StateMsgType = iota
	StateMsgDisconnected
	StateMsgJobsLoaded
	StateMsgDatasetsLoaded
	StateMsgClusterLoaded
	StateMsgLoading
	StateMsgLoadingDone
	StateMsgError
)

// StateMsg carries state updates.
type StateMsg struct {
	Type StateMsgType

	// Connection
	Connected bool

	// Jobs
	Jobs []domain.Job

	// Datasets
	Datasets []domain.Dataset

	// Cluster
	ClusterEnabled bool
	ClusterName    string
	ClusterNodes   []ClusterNode

	// Loading
	LoadingMsg string

	// Error
	Error error
}

// NavigateMsg requests navigation to a page.
type NavigateMsg struct {
	PageID string
}

// ShowDialogMsg requests showing a dialog.
type ShowDialogMsg struct {
	Dialog Dialog
}

// CloseDialogMsg requests closing the current dialog.
type CloseDialogMsg struct{}

// Navigate returns a command to navigate to a page.
func Navigate(pageID string) tea.Cmd {
	return func() tea.Msg {
		return NavigateMsg{PageID: pageID}
	}
}

// ShowDialog returns a command to show a dialog.
func ShowDialog(dialog Dialog) tea.Cmd {
	return func() tea.Msg {
		return ShowDialogMsg{Dialog: dialog}
	}
}

// CloseDialog returns a command to close the current dialog.
func CloseDialog() tea.Cmd {
	return func() tea.Msg {
		return CloseDialogMsg{}
	}
}
