package icons

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func bundledFS(keys ...string) fstest.MapFS {
	m := fstest.MapFS{}
	for _, k := range keys {
		m["app-"+k+".svg"] = &fstest.MapFile{Data: []byte("<svg/>")}
	}
	// Noise that must be ignored.
	m["readme.md"] = &fstest.MapFile{Data: []byte("x")}
	m["app-.svg"] = &fstest.MapFile{Data: []byte("x")}
	m["logo.svg"] = &fstest.MapFile{Data: []byte("x")}
	return m
}

func writePrivate(t *testing.T, dir string, keys ...string) {
	t.Helper()
	for _, k := range keys {
		p := filepath.Join(dir, "app-"+k+".svg")
		if err := os.WriteFile(p, []byte("<svg id=\""+k+"\"/>"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}

func TestLoadBundledOnly(t *testing.T) {
	idx, err := Load(bundledFS("chrome", "docker-desktop", "default"), "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	keys := idx.Keys()
	if len(keys) != 2 {
		t.Fatalf("Keys = %v, want chrome + docker-desktop (default excluded)", keys)
	}
	if got := idx.DefaultSrc(); got != "/app-icons/app-default.svg" {
		t.Fatalf("DefaultSrc = %q", got)
	}
}

func TestResolveFuzzyAgainstBundled(t *testing.T) {
	idx, err := Load(bundledFS("chrome", "docker-desktop", "edge", "msedgewebview2", "chatgpt", "default"), "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cases := map[string]string{
		"Docker Desktop.exe":      "/app-icons/app-docker-desktop.svg",
		"docker desktop":          "/app-icons/app-docker-desktop.svg",
		"Docker Desktop for Mac":  "/app-icons/app-docker-desktop.svg",
		"Google Chrome":           "/app-icons/app-chrome.svg",
		"chrome":                  "/app-icons/app-chrome.svg",
		"Microsoft Edge":          "/app-icons/app-edge.svg",
		"msedgewebview2":          "/app-icons/app-msedgewebview2.svg",
		"ChatGPT":                 "/app-icons/app-chatgpt.svg",
		"cursor":                  "/app-icons/app-default.svg",
		"":                        "/app-icons/app-default.svg",
		"default":                 "/app-icons/app-default.svg", // reserved key never matches itself
		"sumwall.browser":         "/app-icons/app-default.svg",
		"totally unrelated thing": "/app-icons/app-default.svg",
	}
	for name, want := range cases {
		if got := idx.Resolve(name); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestPrivateOverridesBundledOnTie(t *testing.T) {
	dir := t.TempDir()
	writePrivate(t, dir, "chrome", "sumwall.browser", "default")

	idx, err := Load(bundledFS("chrome", "edge", "default"), dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Same key on both sides → private wins.
	if got := idx.Resolve("chrome.exe"); !strings.HasPrefix(got, "data:image/svg+xml;base64,") {
		t.Fatalf("private chrome should override bundled, got %q", got)
	}
	// Bundled-only key still resolves to the URL.
	if got := idx.Resolve("Microsoft Edge"); got != "/app-icons/app-edge.svg" {
		t.Fatalf("Resolve(edge) = %q", got)
	}
	// Private-only key with fuzzy name.
	if got := idx.Resolve("Sumwall Browser.exe"); !strings.HasPrefix(got, "data:") {
		t.Fatalf("Resolve(Sumwall Browser.exe) = %q, want private data URI", got)
	}
	// Private default replaces the bundled fallback.
	if got := idx.DefaultSrc(); !strings.HasPrefix(got, "data:") {
		t.Fatalf("DefaultSrc should be the private override, got %q", got)
	}
	if got := idx.Resolve("nothing-like-this"); got != idx.DefaultSrc() {
		t.Fatalf("unmatched name should fall back to DefaultSrc")
	}

	// PrivateKeys excludes the reserved default and is sorted.
	pk := idx.PrivateKeys()
	if len(pk) != 2 || pk[0] != "chrome" || pk[1] != "sumwall.browser" {
		t.Fatalf("PrivateKeys = %v", pk)
	}
}

func TestMoreSpecificBundledBeatsLooserPrivate(t *testing.T) {
	dir := t.TempDir()
	writePrivate(t, dir, "docker") // containment only (65) against "Docker Desktop"

	idx, err := Load(bundledFS("docker-desktop"), dir) // separator-only difference (95)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := idx.Resolve("Docker Desktop.exe"); got != "/app-icons/app-docker-desktop.svg" {
		t.Fatalf("stronger bundled match should win over weaker private, got %q", got)
	}
	// ...but a plain "docker" process picks the private icon exactly.
	if got := idx.Resolve("docker"); !strings.HasPrefix(got, "data:") {
		t.Fatalf("Resolve(docker) = %q, want private", got)
	}
}

func TestLoadMissingPrivateDirIsFine(t *testing.T) {
	idx, err := Load(nil, filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("missing private dir must not error: %v", err)
	}
	if len(idx.Keys()) != 0 {
		t.Fatalf("expected no keys, got %v", idx.Keys())
	}
	if got := idx.Resolve("anything"); got != "/app-icons/app-default.svg" {
		t.Fatalf("Resolve on empty index = %q", got)
	}
}
