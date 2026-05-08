package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrivateIconsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadPrivateIcons(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestLoadPrivateIconsMissingDir(t *testing.T) {
	got, err := LoadPrivateIcons(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestLoadPrivateIconsPicksAppPrefixSvgOnly(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	mustWrite("app-foo.svg", "<svg/>")
	mustWrite("app-msedgewebview2.svg", "<svg id=\"mew2\"/>")
	mustWrite("foo.svg", "ignored")        // missing app- prefix
	mustWrite("app-bar.png", "ignored")    // wrong extension
	mustWrite("app-.svg", "ignored")       // empty key
	mustWrite("ceftop.json", "{}")         // unrelated config file

	got, err := LoadPrivateIcons(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 icons, got %d: %v", len(got), keysOf(got))
	}
	if !strings.HasPrefix(got["foo"], "data:image/svg+xml;base64,") {
		t.Fatalf("expected data URI for foo, got %q", got["foo"])
	}
	if got["msedgewebview2"] == "" {
		t.Fatalf("expected msedgewebview2 to be present")
	}
	if _, ok := got[""]; ok {
		t.Fatalf("empty-key icon should be skipped")
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestPrivateIconNamesSortedAndFiltered(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("<svg/>"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	mustWrite("app-sumwall.browser.svg")
	mustWrite("app-chrome.svg")
	mustWrite("app-default.svg") // reserved fallback — must be excluded
	mustWrite("app-.svg")        // empty key — must be excluded
	mustWrite("readme.txt")      // unrelated — must be excluded
	mustWrite("ceftop.json")     // unrelated — must be excluded

	got, err := PrivateIconNames(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"chrome", "sumwall.browser"}
	if len(got) != len(want) {
		t.Fatalf("len = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPrivateIconNamesMissingDir(t *testing.T) {
	got, err := PrivateIconNames(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestPrivateIconNamesEmptyConfigDir(t *testing.T) {
	got, err := PrivateIconNames("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}
