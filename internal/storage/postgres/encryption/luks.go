package encryption

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// LUKSEncryption implements VolumeEncryption using LUKS/dm-crypt.
// This is only supported on Linux.
type LUKSEncryption struct {
	config      LUKSConfig
	dataDir     string
	deviceName  string
	volumePath  string
	mountPoint  string
	initialized bool
}

// NewLUKSEncryption creates a new LUKS encryption handler.
func NewLUKSEncryption(config LUKSConfig) *LUKSEncryption {
	if config.Cipher == "" {
		config.Cipher = "aes-xts-plain64"
	}
	if config.KeySize == 0 {
		config.KeySize = 512
	}
	if config.HashAlgorithm == "" {
		config.HashAlgorithm = "sha512"
	}

	return &LUKSEncryption{
		config: config,
	}
}

// Method returns the encryption method.
func (l *LUKSEncryption) Method() Method {
	return MethodLUKS
}

// IsSupported checks if LUKS is available on this system.
func (l *LUKSEncryption) IsSupported() bool {
	// LUKS is only supported on Linux
	if runtime.GOOS != "linux" {
		return false
	}

	// Check if cryptsetup is available
	_, err := exec.LookPath("cryptsetup")
	if err != nil {
		return false
	}

	// Check if we can access device mapper
	if _, err := os.Stat("/dev/mapper"); err != nil {
		return false
	}

	return true
}

// Initialize sets up the encrypted volume.
func (l *LUKSEncryption) Initialize(ctx context.Context, dataDir string, key []byte) error {
	if !l.IsSupported() {
		return ErrNotSupported
	}

	if len(key) < 32 {
		return ErrInvalidKey
	}

	l.dataDir = dataDir
	l.volumePath = filepath.Join(dataDir, "encrypted.img")
	l.deviceName = fmt.Sprintf("bibd-encrypted-%d", os.Getpid())
	l.mountPoint = filepath.Join(dataDir, "data")

	// Check if already initialized
	if _, err := os.Stat(l.volumePath); err == nil {
		l.initialized = true
		return nil
	}

	// Create the volume file
	if err := l.createVolumeFile(ctx); err != nil {
		return fmt.Errorf("failed to create volume file: %w", err)
	}

	// Format with LUKS
	if err := l.formatLUKS(ctx, key); err != nil {
		os.Remove(l.volumePath)
		return fmt.Errorf("failed to format LUKS: %w", err)
	}

	l.initialized = true
	return nil
}

// createVolumeFile creates a sparse file for the encrypted volume.
func (l *LUKSEncryption) createVolumeFile(ctx context.Context) error {
	// Parse volume size
	size := l.parseVolumeSize()

	// Create sparse file using truncate
	cmd := exec.CommandContext(ctx, "truncate", "-s", fmt.Sprintf("%d", size), l.volumePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("truncate failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// parseVolumeSize parses the volume size string (e.g., "50GB") to bytes.
func (l *LUKSEncryption) parseVolumeSize() int64 {
	size := l.config.VolumeSize
	if size == "" {
		return 50 * 1024 * 1024 * 1024 // 50 GB default
	}

	var multiplier int64 = 1
	if len(size) >= 2 {
		suffix := size[len(size)-2:]
		switch suffix {
		case "GB", "Gi":
			multiplier = 1024 * 1024 * 1024
			size = size[:len(size)-2]
		case "MB", "Mi":
			multiplier = 1024 * 1024
			size = size[:len(size)-2]
		case "KB", "Ki":
			multiplier = 1024
			size = size[:len(size)-2]
		}
	}

	var value int64
	fmt.Sscanf(size, "%d", &value)
	if value == 0 {
		value = 50
	}

	return value * multiplier
}

// formatLUKS formats the volume with LUKS encryption.
func (l *LUKSEncryption) formatLUKS(ctx context.Context, key []byte) error {
	// Create a temporary key file (more secure than passing via stdin in some cases)
	keyFile, err := os.CreateTemp("", "luks-key-*")
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer func() {
		keyFile.Close()
		os.Remove(keyFile.Name())
	}()

	if _, err := keyFile.Write(key); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}
	keyFile.Close()

	// Format with cryptsetup
	cmd := exec.CommandContext(ctx, "cryptsetup", "luksFormat",
		"--type", "luks2",
		"--cipher", l.config.Cipher,
		"--key-size", fmt.Sprintf("%d", l.config.KeySize),
		"--hash", l.config.HashAlgorithm,
		"--key-file", keyFile.Name(),
		"--batch-mode",
		l.volumePath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cryptsetup luksFormat failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// Mount opens and mounts the encrypted volume.
func (l *LUKSEncryption) Mount(ctx context.Context, key []byte) error {
	if !l.IsSupported() {
		return ErrNotSupported
	}

	if !l.initialized {
		return ErrNotInitialized
	}

	// Check if already mounted
	if l.isMounted() {
		return nil
	}

	// Create key file
	keyFile, err := os.CreateTemp("", "luks-key-*")
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer func() {
		keyFile.Close()
		os.Remove(keyFile.Name())
	}()

	if _, err := keyFile.Write(key); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}
	keyFile.Close()

	// Open LUKS volume
	cmd := exec.CommandContext(ctx, "cryptsetup", "luksOpen",
		"--key-file", keyFile.Name(),
		l.volumePath,
		l.deviceName,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cryptsetup luksOpen failed: %w\nOutput: %s", err, output)
	}

	// Create mount point if needed
	if err := os.MkdirAll(l.mountPoint, 0700); err != nil {
		l.closeLUKS(ctx)
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	// Check if filesystem exists, create if not
	if err := l.ensureFilesystem(ctx); err != nil {
		l.closeLUKS(ctx)
		return fmt.Errorf("failed to ensure filesystem: %w", err)
	}

	// Mount
	cmd = exec.CommandContext(ctx, "mount",
		fmt.Sprintf("/dev/mapper/%s", l.deviceName),
		l.mountPoint,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		l.closeLUKS(ctx)
		return fmt.Errorf("mount failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// ensureFilesystem creates an ext4 filesystem if one doesn't exist.
func (l *LUKSEncryption) ensureFilesystem(ctx context.Context) error {
	devicePath := fmt.Sprintf("/dev/mapper/%s", l.deviceName)

	// Check if filesystem exists
	cmd := exec.CommandContext(ctx, "blkid", devicePath)
	if cmd.Run() == nil {
		return nil // Filesystem already exists
	}

	// Create ext4 filesystem
	cmd = exec.CommandContext(ctx, "mkfs.ext4", "-q", devicePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mkfs.ext4 failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// Unmount unmounts and closes the encrypted volume.
func (l *LUKSEncryption) Unmount(ctx context.Context) error {
	if !l.IsSupported() {
		return nil
	}

	// Unmount if mounted
	if l.isMounted() {
		cmd := exec.CommandContext(ctx, "umount", l.mountPoint)
		cmd.Run() // Ignore errors
	}

	// Close LUKS
	return l.closeLUKS(ctx)
}

// closeLUKS closes the LUKS volume.
func (l *LUKSEncryption) closeLUKS(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "cryptsetup", "luksClose", l.deviceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if device doesn't exist (already closed)
		if _, statErr := os.Stat(fmt.Sprintf("/dev/mapper/%s", l.deviceName)); os.IsNotExist(statErr) {
			return nil
		}
		return fmt.Errorf("cryptsetup luksClose failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// isMounted checks if the volume is currently mounted.
func (l *LUKSEncryption) isMounted() bool {
	cmd := exec.Command("mountpoint", "-q", l.mountPoint)
	return cmd.Run() == nil
}

// Status returns the encryption status.
func (l *LUKSEncryption) Status(ctx context.Context) (*EncryptionStatus, error) {
	status := &EncryptionStatus{
		Method:      MethodLUKS,
		Initialized: l.initialized,
		Active:      l.isMounted(),
	}

	if l.initialized {
		// Get volume info
		if info, err := os.Stat(l.volumePath); err == nil {
			status.VolumeSize = info.Size()
			status.KeyCreatedAt = info.ModTime()
		}

		// Get used space if mounted
		if status.Active {
			if used, err := l.getUsedSpace(ctx); err == nil {
				status.UsedSpace = used
			}
		}
	}

	if !l.IsSupported() {
		status.Warnings = append(status.Warnings, "LUKS not supported on this platform")
	}

	return status, nil
}

// getUsedSpace returns the used space on the mounted filesystem.
func (l *LUKSEncryption) getUsedSpace(ctx context.Context) (int64, error) {
	cmd := exec.CommandContext(ctx, "df", "-B1", l.mountPoint)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var used int64
	// Parse df output (second line, third column)
	_, err = fmt.Sscanf(string(output), "%*s %*d %d", &used)
	return used, err
}

// TDEEncryption implements VolumeEncryption using PostgreSQL TDE.
// This is a placeholder for PostgreSQL Transparent Data Encryption support.
type TDEEncryption struct {
	config TDEConfig
}

// NewTDEEncryption creates a new TDE encryption handler.
func NewTDEEncryption(config TDEConfig) *TDEEncryption {
	return &TDEEncryption{config: config}
}

// Method returns the encryption method.
func (t *TDEEncryption) Method() Method {
	return MethodTDE
}

// IsSupported checks if TDE is available.
func (t *TDEEncryption) IsSupported() bool {
	// TDE requires specific PostgreSQL build with encryption support
	// This is currently not widely available in standard PostgreSQL distributions
	return false
}

// Initialize sets up TDE encryption.
func (t *TDEEncryption) Initialize(ctx context.Context, dataDir string, key []byte) error {
	if !t.IsSupported() {
		return ErrNotSupported
	}
	// TDE initialization would configure PostgreSQL encryption settings
	return nil
}

// Mount is a no-op for TDE (encryption is handled by PostgreSQL).
func (t *TDEEncryption) Mount(ctx context.Context, key []byte) error {
	return nil
}

// Unmount is a no-op for TDE.
func (t *TDEEncryption) Unmount(ctx context.Context) error {
	return nil
}

// Status returns the TDE status.
func (t *TDEEncryption) Status(ctx context.Context) (*EncryptionStatus, error) {
	return &EncryptionStatus{
		Method:      MethodTDE,
		Initialized: false,
		Active:      false,
		Warnings:    []string{"TDE not currently supported"},
	}, nil
}

// NullEncryption is a no-op encryption for when encryption is disabled.
type NullEncryption struct{}

// NewNullEncryption creates a null encryptor.
func NewNullEncryption() *NullEncryption {
	return &NullEncryption{}
}

func (n *NullEncryption) Method() Method    { return MethodNone }
func (n *NullEncryption) IsSupported() bool { return true }
func (n *NullEncryption) Initialize(ctx context.Context, dataDir string, key []byte) error {
	return nil
}
func (n *NullEncryption) Mount(ctx context.Context, key []byte) error { return nil }
func (n *NullEncryption) Unmount(ctx context.Context) error           { return nil }
func (n *NullEncryption) Status(ctx context.Context) (*EncryptionStatus, error) {
	return &EncryptionStatus{
		Method:         MethodNone,
		Initialized:    true,
		Active:         true,
		LastVerifiedAt: time.Now(),
	}, nil
}
