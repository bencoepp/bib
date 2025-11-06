package util

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrConfigNotFound = errors.New("config file not found in any known location")

// Options controls how FindConfigPath searches for a configuration file.
type Options struct {
	// AppName is the name of your application (used for folder names like ~/.<appname>, XDG, AppData).
	AppName string
	// FileNames is an ordered list of candidate file names (e.g., []string{"config.yaml", "config.yml"}).
	// If empty or nil, it defaults to []string{"config.yaml", "config.yml"}.
	FileNames []string
	// EnvVar is the environment variable to check first. If it points to a file, use it directly.
	// If it points to a directory, the candidates in FileNames will be probed inside it.
	// If empty, defaults to strings.ToUpper(AppName) + "_CONFIG".
	EnvVar string
	// AlsoCheckCWD controls whether to probe the current working directory.
	// Defaults to true.
	AlsoCheckCWD bool
}

// FindConfigPath searches for a config file and returns the absolute path of the first match.
//
// The default search order (can be influenced by Options) is:
// 1) Environment variable path (file or directory) via EnvVar
// 2) User's home directory: ~/.<appname>/
// 3) macOS: ~/Library/Application Support/<AppName>/
// 4) Windows: %PROGRAMDATA%\<AppName>\
// 5) System directory (Linux/Unix): /etc/<appname>/
// 6) Current working directory (optional)
//
// You can override candidate file names and the environment variable via Options.
func FindConfigPath(opts Options) (string, error) {
	if opts.AppName == "" {
		return "", errors.New("Options.AppName must be set")
	}

	// Defaults
	if len(opts.FileNames) == 0 {
		opts.FileNames = []string{"config.yaml", "config.yml"}
	}
	if opts.EnvVar == "" {
		opts.EnvVar = strings.ToUpper(opts.AppName) + "_CONFIG"
	}
	if !opts.AlsoCheckCWD {
		opts.AlsoCheckCWD = true
	}

	// 1) ENV var
	if pathFromEnv := strings.TrimSpace(os.Getenv(opts.EnvVar)); pathFromEnv != "" {
		if resolvedPath, ok := resolveEnvPath(pathFromEnv, opts.FileNames); ok {
			return resolvedPath, nil
		}
	}

	// 2) User's home directory: ~/.<appname>/
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		homeAppDir := filepath.Join(homeDir, "."+opts.AppName)
		if resolvedPath, ok := probeDir(homeAppDir, opts.FileNames); ok {
			return resolvedPath, nil
		}
	}

	// 3) macOS Application Support
	if runtime.GOOS == "darwin" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			macOSDir := filepath.Join(homeDir, "Library", "Application Support", titleCase(opts.AppName))
			if resolvedPath, ok := probeDir(macOSDir, opts.FileNames); ok {
				return resolvedPath, nil
			}
		}
	}

	// 4) Windows AppData and ProgramData
	if runtime.GOOS == "windows" {
		if programData := strings.TrimSpace(os.Getenv("PROGRAMDATA")); programData != "" {
			winConfigDir := filepath.Join(programData, titleCase(opts.AppName))
			if resolvedPath, ok := probeDir(winConfigDir, opts.FileNames); ok {
				return resolvedPath, nil
			}
		}
	}

	// 5) System directory (Linux/Unix): /etc/<appname>/
	if runtime.GOOS != "windows" {
		systemDir := filepath.Join(string(filepath.Separator), "etc", opts.AppName)
		if resolvedPath, ok := probeDir(systemDir, opts.FileNames); ok {
			return resolvedPath, nil
		}
	}

	// 6) Current working directory (optional)
	if opts.AlsoCheckCWD {
		if cwd, err := os.Getwd(); err == nil {
			if resolvedPath, ok := probeDir(cwd, opts.FileNames); ok {
				return resolvedPath, nil
			}
		}
	}

	return "", ErrConfigNotFound
}

// resolveEnvPath handles the ENV var pointing to either a file or a directory.
func resolveEnvPath(path string, fileNames []string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	if info.Mode().IsRegular() {
		// Path points to a file
		return absPath(path), true
	}
	if info.IsDir() {
		// Path points to a directory; probe file candidates inside
		if resolvedPath, ok := probeDir(path, fileNames); ok {
			return resolvedPath, true
		}
	}
	return "", false
}

// probeDir looks for the first existing file among candidates inside the specified directory.
func probeDir(dir string, fileNames []string) (string, bool) {
	for _, name := range fileNames {
		checkPath := filepath.Join(dir, name)
		if isFile(checkPath) {
			return absPath(checkPath), true
		}
	}
	return "", false
}

// isFile checks if the given path is a regular file.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// absPath resolves the absolute path for the given file/directory.
func absPath(path string) string {
	if resolvedPath, err := filepath.Abs(path); err == nil {
		return resolvedPath
	}
	return path
}

// titleCase converts the input string to Title Case (e.g., my-app -> MyApp).
func titleCase(s string) string {
	if s == "" {
		return s
	}
	// Split using separators and capitalize the first rune of each token
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.' || r == '/'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = toUpperRune(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

// toUpperRune converts a single rune to its uppercase representation.
func toUpperRune(r rune) rune {
	return []rune(strings.ToUpper(string(r)))[0]
}
