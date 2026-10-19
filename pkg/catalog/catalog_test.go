package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeOverride(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_EmbeddedParses(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") error: %v", err)
	}
	if len(c.Keys()) == 0 {
		t.Fatal("embedded catalog is empty")
	}
	for _, k := range c.Keys() {
		e, _ := c.Lookup(k)
		if e.FriendlyName == "" {
			t.Errorf("embedded entry %q has no friendlyName", k)
		}
	}
	if !c.IsMultiGroup("msedgewebview2") {
		t.Error("msedgewebview2 should be multiGroup in the embedded catalog")
	}
}

func TestLoad_MissingOverrideIsNotAnError(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatalf("missing override should not error, got %v", err)
	}
}

func TestLoad_OverrideMergesFieldByField(t *testing.T) {
	p := writeOverride(t, `{
		"schemaVersion": 1,
		"apps": {
			"MSEdgeWebView2.exe": { "friendlyName": "WebView2" },
			"slack": { "multiGroup": true },
			"myapp": { "friendlyName": "My App", "icon": "electron" }
		}
	}`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	wv, ok := c.Lookup("msedgewebview2")
	if !ok {
		t.Fatal("msedgewebview2 missing after merge")
	}
	if wv.FriendlyName != "WebView2" {
		t.Errorf("friendlyName = %q, want override %q", wv.FriendlyName, "WebView2")
	}
	if !wv.MultiGroup || wv.Vendor != "Microsoft" {
		t.Errorf("fields absent from the override must keep embedded values, got %+v", wv)
	}

	slack, _ := c.Lookup("Slack.exe")
	if !slack.MultiGroup || slack.FriendlyName != "Slack" {
		t.Errorf("slack = %+v, want multiGroup on and embedded friendlyName kept", slack)
	}

	mine, ok := c.Lookup(`C:\Apps\MyApp.exe`)
	if !ok || mine.FriendlyName != "My App" || mine.Icon != "electron" {
		t.Errorf("new override entry = %+v, %v", mine, ok)
	}
}

func TestLoad_OverrideCanSwitchMultiGroupOff(t *testing.T) {
	p := writeOverride(t, `{"apps": {"msedgewebview2": {"multiGroup": false}}}`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if c.IsMultiGroup("msedgewebview2") {
		t.Error("explicit multiGroup:false in the override must win")
	}
}

func TestLoad_InvalidOverrideKeepsEmbedded(t *testing.T) {
	p := writeOverride(t, `{"apps": {"slack": {"friendlyName": "Broken"`)
	c, err := Load(p)
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("error %q should name the file", err)
	}
	if c == nil {
		t.Fatal("catalog must still be usable after a bad override")
	}
	if e, _ := c.Lookup("slack"); e.FriendlyName != "Slack" {
		t.Errorf("slack friendlyName = %q, want embedded %q", e.FriendlyName, "Slack")
	}
}

func TestFriendlyName_CatalogBeatsHeuristic(t *testing.T) {
	c, _ := Load("")
	if got := c.FriendlyName("Code.exe", 0); got != "VS Code" {
		t.Errorf("FriendlyName(Code.exe) = %q, want %q", got, "VS Code")
	}
	if got := c.FriendlyName("totally-unknown.exe", 0); got != "Totally Unknown" {
		t.Errorf("FriendlyName(totally-unknown.exe) = %q, want heuristic %q", got, "Totally Unknown")
	}
}

func TestFriendlyName_NilCatalog(t *testing.T) {
	var c *Catalog
	if got := c.FriendlyName("foo.bar", 0); got != "Foo Bar" {
		t.Errorf("nil catalog FriendlyName = %q, want heuristic %q", got, "Foo Bar")
	}
}

func TestHeuristic(t *testing.T) {
	cases := []struct{ in, want string }{
		// Same examples as the former frontend friendlyName.ts.
		{"program.browser", "Program"},
		{"my.super-program", "Program"},
		{"browser.cool", "Browser Cool"},
		{"foo.bar", "Foo Bar"},
		// Case is preserved past the first letter; .exe and paths go.
		{"SearchHost.exe", "SearchHost"},
		{`C:\Program Files\Foo\whatsapp.exe`, "Whatsapp"},
		{"app", "App"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Heuristic(c.in); got != c.want {
			t.Errorf("Heuristic(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUsableDescription(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Google Drive", true},
		{"  ", false},
		{"Electron", false},
		{"Chromium", false},
		{"SearchHost.exe", false},
		{"Microsoft® Windows® Operating System", false},
		{strings.Repeat("x", 41), false},
	}
	for _, c := range cases {
		if got := usableDescription(c.in); got != c.want {
			t.Errorf("usableDescription(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
