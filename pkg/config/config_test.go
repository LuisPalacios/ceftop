package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad_MissingFile_ReturnsEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(missing) returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load(missing) returned nil config")
	}
	if cfg.AppName != "" {
		t.Errorf("Load(missing).AppName = %q, want empty", cfg.AppName)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ceftop.json")

	original := &Config{AppName: "chrome"}
	if err := Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.AppName != original.AppName {
		t.Errorf("AppName round-trip: got %q, want %q", loaded.AppName, original.AppName)
	}
}

func TestSave_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "ceftop.json")

	if err := Save(path, &Config{AppName: "chrome"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s, stat: %v", path, err)
	}
}

func TestLoad_RejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ceftop.json")

	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load on malformed JSON returned nil error")
	}
	if !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("error %q should mention parsing", err.Error())
	}
}

func TestDefaultPath_HasCeftopSegments(t *testing.T) {
	got := DefaultPath()
	wantSuffix := filepath.Join(AppDir, AppFile)
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("DefaultPath() = %q, want suffix %q", got, wantSuffix)
	}
	if got == "" {
		t.Error("DefaultPath() returned empty string")
	}
}

func TestConfigRoot_HonorsXDG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	got := ConfigRoot()
	if got != tmp {
		t.Errorf("ConfigRoot() = %q, want %q (XDG_CONFIG_HOME)", got, tmp)
	}
}

func TestSave_WritesIndentedJSONWithTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ceftop.json")

	if err := Save(path, &Config{AppName: "code"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("expected trailing newline")
	}
	if !strings.Contains(string(data), "    \"AppName\"") {
		t.Errorf("expected 4-space indented JSON, got:\n%s", string(data))
	}
}

func TestTickInterval_DefaultWhenOutOfRange(t *testing.T) {
	cases := []struct {
		name string
		cfg  *Config
	}{
		{"nil config", nil},
		{"zero value", &Config{}},
		{"explicit zero", &Config{TickIntervalSeconds: 0}},
		{"negative", &Config{TickIntervalSeconds: -5}},
		{"above max", &Config{TickIntervalSeconds: MaxTickIntervalSeconds + 1}},
		{"way above max", &Config{TickIntervalSeconds: 99999}},
	}
	want := time.Duration(DefaultTickIntervalSeconds) * time.Second
	for _, c := range cases {
		if got := c.cfg.TickInterval(); got != want {
			t.Errorf("%s: TickInterval() = %v, want %v", c.name, got, want)
		}
	}
}

func TestTickInterval_HonorsExplicitValue(t *testing.T) {
	for _, n := range []int{1, 7, 30, 100, MaxTickIntervalSeconds} {
		cfg := &Config{TickIntervalSeconds: n}
		want := time.Duration(n) * time.Second
		if got := cfg.TickInterval(); got != want {
			t.Errorf("TickInterval(%d) = %v, want %v", n, got, want)
		}
	}
}

func TestNormalize_ClampsOutOfRangeToDefault(t *testing.T) {
	cases := []struct {
		name string
		in   int
	}{
		{"zero", 0},
		{"negative", -1},
		{"max + 1", MaxTickIntervalSeconds + 1},
		{"large", 999999},
	}
	for _, c := range cases {
		cfg := &Config{TickIntervalSeconds: c.in}
		if changed := cfg.Normalize(); !changed {
			t.Errorf("%s: Normalize should report change", c.name)
		}
		if cfg.TickIntervalSeconds != DefaultTickIntervalSeconds {
			t.Errorf("%s: TickIntervalSeconds = %d, want %d (default)",
				c.name, cfg.TickIntervalSeconds, DefaultTickIntervalSeconds)
		}
	}
}

func TestDefaultTickIntervalSeconds_IsFive(t *testing.T) {
	// Pinned constant: first-launch users land at the calm end of the range.
	if DefaultTickIntervalSeconds != 5 {
		t.Errorf("DefaultTickIntervalSeconds = %d, want 5", DefaultTickIntervalSeconds)
	}
}

func TestSave_AlwaysIncludesTickInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ceftop.json")

	// Save with the zero value — Save itself does not normalize, but the
	// JSON tag has no `omitempty` so the field must still appear on disk.
	if err := Save(path, &Config{AppName: "chrome"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "TickIntervalSeconds") {
		t.Errorf("expected TickIntervalSeconds in JSON, got:\n%s", string(data))
	}
}

func TestLoad_FillsMissingKeysAndRewrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ceftop.json")

	// Hand-written legacy file missing TickIntervalSeconds.
	legacy := []byte("{\n    \"AppName\": \"chrome\"\n}\n")
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TickIntervalSeconds != DefaultTickIntervalSeconds {
		t.Errorf("TickIntervalSeconds = %d, want default %d",
			cfg.TickIntervalSeconds, DefaultTickIntervalSeconds)
	}

	// Rewrite must have happened: the file now contains the key.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "TickIntervalSeconds") {
		t.Errorf("Load did not rewrite missing keys; on-disk file:\n%s", string(data))
	}
}

func TestNormalize_NilSafe(t *testing.T) {
	var c *Config
	if changed := c.Normalize(); changed {
		t.Error("Normalize on nil should report no change")
	}
}

func TestNormalize_ReturnsFalseWhenAllKeysInRange(t *testing.T) {
	for _, n := range []int{1, 5, 30, 100, MaxTickIntervalSeconds} {
		cfg := &Config{AppName: "chrome", TickIntervalSeconds: n}
		if changed := cfg.Normalize(); changed {
			t.Errorf("TickIntervalSeconds=%d: Normalize should report no change", n)
		}
	}
}

func TestSave_WritesExplicitTickInterval(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ceftop.json")

	if err := Save(path, &Config{AppName: "chrome", TickIntervalSeconds: 5}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.TickIntervalSeconds != 5 {
		t.Errorf("TickIntervalSeconds round-trip: got %d, want 5", loaded.TickIntervalSeconds)
	}
	if got := loaded.TickInterval(); got != 5*time.Second {
		t.Errorf("loaded.TickInterval() = %v, want 5s", got)
	}
}
