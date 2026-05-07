// Package config handles loading and saving CefTop configuration.
package config

import "time"

// Tick interval bounds. The default lives at the calm end of the slider
// range so first-launch users do not get hammered with refreshes; min and
// max define the legal field range and the fallback for any out-of-range
// value (in JSON or via SetTickInterval).
const (
	DefaultTickIntervalSeconds = 5
	MinTickIntervalSeconds     = 1
	MaxTickIntervalSeconds     = 999
)

// Config holds CefTop user configuration. Every supported key is always
// written to disk: a config that loaded with missing fields gets defaults
// filled in by Normalize and is rewritten on the next save.
type Config struct {
	// AppName is the target executable name to monitor (e.g. "chrome",
	// "code", "msedge"). Empty means onboarding has not run yet.
	AppName string `json:"AppName"`

	// TickIntervalSeconds is the snapshot refresh cadence in seconds.
	// Always present on disk. Loading an older config without this key
	// fills in DefaultTickIntervalSeconds and rewrites the file.
	TickIntervalSeconds int `json:"TickIntervalSeconds"`
}

// Normalize fills in defaults for any supported key that is missing or
// invalid, and clamps anything outside the legal range back to the default.
// Returns true if the in-memory config was changed; the caller can use that
// signal to rewrite the on-disk file so every supported field is always
// materialized.
func (c *Config) Normalize() bool {
	if c == nil {
		return false
	}
	changed := false
	if c.TickIntervalSeconds < MinTickIntervalSeconds || c.TickIntervalSeconds > MaxTickIntervalSeconds {
		c.TickIntervalSeconds = DefaultTickIntervalSeconds
		changed = true
	}
	return changed
}

// TickInterval returns the snapshot tick duration. Goes through this helper
// so the default lives in one place; Normalize guarantees the field is set
// at rest, but callers can still hold a stale zero or out-of-range value in
// flight (e.g. mid-onboarding, before Save).
func (c *Config) TickInterval() time.Duration {
	if c == nil || c.TickIntervalSeconds < MinTickIntervalSeconds || c.TickIntervalSeconds > MaxTickIntervalSeconds {
		return DefaultTickIntervalSeconds * time.Second
	}
	return time.Duration(c.TickIntervalSeconds) * time.Second
}
