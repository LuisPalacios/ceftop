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
