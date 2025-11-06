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
// 2) Application directory (next to the running executable)
// 3) Current working directory (optional, defaults to ON)
// 4) User's home dot directory: ~/.<appname>/
// 5) XDG config directory: $XDG_CONFIG_HOME/<appname>/ or ~/.config/<appname>/ (Unix-like)
// 6) macOS: ~/Library/Application Support/<AppName>/
// 7) Windows: %APPDATA%\<AppName>\ and then %PROGRAMDATA%\<AppName>\
// 8) System directory (Unix-like): /etc/<appname>/
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
	if p := strings.TrimSpace(os.Getenv(opts.EnvVar)); p != "" {
		if path, ok := resolveEnvPath(p, opts.FileNames); ok {
			return path, nil
		}
	}

	// 2) Application directory (directory of the running executable)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if path, ok := probeDir(exeDir, opts.FileNames); ok {
			return path, nil
		}
	}

	// 3) Current working directory (optional)
	if opts.AlsoCheckCWD {
		if cwd, err := os.Getwd(); err == nil {
			if path, ok := probeDir(cwd, opts.FileNames); ok {
				return path, nil
			}
		}
	}

	// 4) Home dot directory: ~/.<appname>/
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		dotApp := "." + opts.AppName
		if path, ok := probeDir(filepath.Join(homeDir, dotApp), opts.FileNames); ok {
			return path, nil
		}
	}

	// 5) XDG config (Unix-like)
	if runtime.GOOS != "windows" {
		xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
		if xdg != "" {
			if path, ok := probeDir(filepath.Join(xdg, opts.AppName), opts.FileNames); ok {
				return path, nil
			}
		} else if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			if path, ok := probeDir(filepath.Join(homeDir, ".config", opts.AppName), opts.FileNames); ok {
				return path, nil
			}
		}
	}

	// 6) macOS Application Support
	if runtime.GOOS == "darwin" {
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			if path, ok := probeDir(filepath.Join(homeDir, "Library", "Application Support", titleCase(opts.AppName)), opts.FileNames); ok {
				return path, nil
			}
		}
	}

	// 7) Windows AppData and ProgramData
	if runtime.GOOS == "windows" {
		if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
			if path, ok := probeDir(filepath.Join(appData, titleCase(opts.AppName)), opts.FileNames); ok {
				return path, nil
			}
		}
		if programData := strings.TrimSpace(os.Getenv("PROGRAMDATA")); programData != "" {
			if path, ok := probeDir(filepath.Join(programData, titleCase(opts.AppName)), opts.FileNames); ok {
				return path, nil
			}
		}
	}

	// 8) System directory (Unix-like): /etc/<appname>/
	if runtime.GOOS != "windows" {
		if path, ok := probeDir(filepath.Join(string(filepath.Separator), "etc", opts.AppName), opts.FileNames); ok {
			return path, nil
		}
	}

	return "", ErrConfigNotFound
}

// resolveEnvPath handles the ENV var pointing to either a file or a directory.
func resolveEnvPath(p string, fileNames []string) (string, bool) {
	info, err := os.Stat(p)
	if err != nil {
		return "", false
	}
	if info.Mode().IsRegular() {
		// p is a file
		return absPath(p), true
	}
	if info.IsDir() {
		// p is a directory; probe inside it
		if path, ok := probeDir(p, fileNames); ok {
			return path, true
		}
	}
	return "", false
}

// probeDir looks for the first existing file among candidates inside dir.
func probeDir(dir string, fileNames []string) (string, bool) {
	for _, name := range fileNames {
		p := filepath.Join(dir, name)
		if isFile(p) {
			return absPath(p), true
		}
	}
	return "", false
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

func absPath(p string) string {
	if ap, err := filepath.Abs(p); err == nil {
		return ap
	}
	return p
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	// Basic Title-Case: split on separators and capitalize first rune of each token.
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

func toUpperRune(r rune) rune {
	return []rune(strings.ToUpper(string(r)))[0]
}
