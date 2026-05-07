package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Load reads and parses a config file.
//
// A missing file is not an error — it returns (&Config{}, nil) so the GUI
// can route to onboarding when AppName is empty. Onboarding writes the
// first complete file once the user picks a target.
//
// A present file is normalized: any supported key that is absent or
// invalid is filled in with its default and the file is rewritten so the
// on-disk shape always reflects every known field. Rewrites are best-
// effort — a write failure does not turn a successful load into a hard
// error, since the in-memory config is still usable.
//
// A present-but-malformed file returns an error so the caller can surface
// the problem instead of silently overwriting it on the next save.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Normalize() {
		// Best-effort: the user just gained a new supported key, write it
		// back so they see the full shape next time they open the file.
		_ = Save(path, &cfg)
	}
	return &cfg, nil
}
