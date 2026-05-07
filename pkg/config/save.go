package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Save writes the configuration to the given file path as indented JSON,
// creating the parent directory if it doesn't yet exist. The 4-space indent
// and trailing newline match the gitbox sibling project's on-disk format.
func Save(path string, cfg *Config) error {
	if err := EnsureDir(path); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
