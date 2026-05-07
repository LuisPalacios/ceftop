package config

import (
	"os"
	"path/filepath"
)

const (
	// AppDir is the directory name under the user's config root.
	AppDir = "ceftop"
	// AppFile is the configuration file name.
	AppFile = "ceftop.json"
)

// DefaultPath returns the default config file path.
// On every platform: <ConfigRoot>/ceftop/ceftop.json — matches the
// cross-platform layout used by the gitbox sibling project so a single user
// folder hosts every Luis-Palacios tool.
func DefaultPath() string {
	return filepath.Join(ConfigRoot(), AppDir, AppFile)
}

// ConfigRoot returns the base config directory.
// Honors XDG_CONFIG_HOME when set; otherwise resolves to ~/.config.
// Falling back to ./.config (rather than aborting) keeps test runs and
// container environments without a HOME usable.
func ConfigRoot() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config")
	}
	return filepath.Join(home, ".config")
}

// EnsureDir creates the parent directory of the given file path if it does
// not exist. The 0o755 mode matches the prevailing convention for user
// config directories on Unix.
func EnsureDir(filePath string) error {
	dir := filepath.Dir(filePath)
	return os.MkdirAll(dir, 0o755)
}
