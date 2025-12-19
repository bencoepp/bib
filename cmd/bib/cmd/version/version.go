package version

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

// Version information (set via ldflags at build time)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// VersionInfo holds all version-related information
type VersionInfo struct {
	Version     string `json:"version" yaml:"version"`
	Commit      string `json:"commit" yaml:"commit"`
	BuildDate   string `json:"build_date" yaml:"build_date"`
	GoVersion   string `json:"go_version" yaml:"go_version"`
	Platform    string `json:"platform" yaml:"platform"`
	NodeID      string `json:"node_id,omitempty" yaml:"node_id,omitempty"`
	StorageType string `json:"storage_type,omitempty" yaml:"storage_type,omitempty"`
	BibdVersion string `json:"bibd_version,omitempty" yaml:"bibd_version,omitempty"`
	BibdStatus  string `json:"bibd_status,omitempty" yaml:"bibd_status,omitempty"`
}

// Cmd represents the version command
var Cmd = &cobra.Command{
	Use:         "version",
	Short:       "version.short",
	Long:        "version.long",
	Annotations: map[string]string{"i18n": "true"},
	RunE:        runVersion,
}

// NewCommand returns the version command
func NewCommand() *cobra.Command {
	return Cmd
}

func runVersion(cmd *cobra.Command, args []string) error {
	info := VersionInfo{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	// Try to load config to get storage type and node ID
	cfg, _ := config.LoadBib("")

	// Check for local bibd instance
	bibdInfo := checkLocalBibd()
	info.NodeID = bibdInfo.nodeID
	info.StorageType = bibdInfo.storageType
	info.BibdVersion = bibdInfo.version
	info.BibdStatus = bibdInfo.status

	// Suppress unused variable warning
	_ = cfg

	// Output based on format - use simple text output for version
	fmt.Printf("bib version %s\n", info.Version)
	fmt.Printf("  commit:     %s\n", info.Commit)
	fmt.Printf("  built:      %s\n", info.BuildDate)
	fmt.Printf("  go version: %s\n", info.GoVersion)
	fmt.Printf("  platform:   %s\n", info.Platform)

	if info.BibdStatus != "" {
		fmt.Println()
		fmt.Printf("bibd daemon:\n")
		fmt.Printf("  status:     %s\n", info.BibdStatus)
		if info.BibdVersion != "" {
			fmt.Printf("  version:    %s\n", info.BibdVersion)
		}
		if info.NodeID != "" {
			fmt.Printf("  node ID:    %s\n", info.NodeID)
		}
		if info.StorageType != "" {
			fmt.Printf("  storage:    %s\n", info.StorageType)
		}
	}
	return nil
}

// bibdInfo holds information about a local bibd instance
type bibdInfo struct {
	status      string
	version     string
	nodeID      string
	storageType string
}

// checkLocalBibd checks for a local bibd instance and returns info about it
func checkLocalBibd() bibdInfo {
	info := bibdInfo{}

	// Check for PID file
	home, err := os.UserHomeDir()
	if err != nil {
		return info
	}

	dataDir := filepath.Join(home, ".local", "share", "bibd")
	pidFile := filepath.Join(dataDir, "bibd.pid")

	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		info.status = "not running"
		return info
	}

	// PID file exists, check if process is running
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		info.status = "unknown"
		return info
	}

	pid := strings.TrimSpace(string(pidData))
	if pid == "" {
		info.status = "not running"
		return info
	}

	// Try to connect to the daemon socket to verify it's running
	socketPath := filepath.Join(dataDir, "bibd.sock")
	conn, err := net.DialTimeout("unix", socketPath, 1*time.Second)
	if err != nil {
		// Also try TCP on default port
		conn, err = net.DialTimeout("tcp", "localhost:8080", 1*time.Second)
		if err != nil {
			info.status = "not responding"
			return info
		}
	}
	conn.Close()

	info.status = "running"

	// Try to load bibd config to get storage type and node ID
	bibdCfg, err := config.LoadBibd("")
	if err == nil && bibdCfg != nil {
		info.storageType = bibdCfg.Database.Backend
		info.nodeID = bibdCfg.Cluster.NodeID
	}

	// TODO: Connect to bibd gRPC API to get version
	// For now, we can't easily get the daemon version without gRPC

	return info
}
