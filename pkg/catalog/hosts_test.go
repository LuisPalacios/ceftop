package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostName_EmbeddedList(t *testing.T) {
	c, _ := Load("")
	cases := []struct{ host, want string }{
		{"SearchHost.exe", "Windows Search"},
		{"GoogleDriveFS.exe", "Google Drive"},
		{"WhatsApp.Root.exe", "WhatsApp"}, // via the "whatsapp*" pattern
		{"WhatsApp.exe", "WhatsApp"},
		// Not in the list: still named, from the heuristic here (pid 0).
		{"some-new.app.exe", "Some New"},
	}
	for _, tc := range cases {
		if got := c.HostName("msedgewebview2.exe", tc.host, 0); got != tc.want {
			t.Errorf("HostName(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
	// The list is scoped to its runtime.
	if got := c.HostName("chrome", "SearchHost.exe", 0); got != "SearchHost" {
		t.Errorf("HostName under another runtime = %q, want heuristic %q", got, "SearchHost")
	}
}

func TestHostName_ExactBeatsPatternAndLongerPatternWins(t *testing.T) {
	p := writeOverride(t, `{"apps": {"msedgewebview2": {"hosts": {
		"whatsapp.root": "WA",
		"foo*": "Foo",
		"foo.bar*": "Foo Bar",
		"[": "broken pattern is ignored"
	}}}}`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ host, want string }{
		{"WhatsApp.Root.exe", "WA"},
		{"WhatsApp.Beta.exe", "WhatsApp"},
		{"foo.bar.baz.exe", "Foo Bar"},
		{"foo.exe", "Foo"},
	}
	for _, tc := range cases {
		if got := c.HostName("msedgewebview2", tc.host, 0); got != tc.want {
			t.Errorf("HostName(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}

func TestHostName_OverrideEmptyValueRemovesEmbeddedName(t *testing.T) {
	p := writeOverride(t, `{"apps": {"msedgewebview2": {"hosts": {"searchhost": ""}}}}`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.HostName("msedgewebview2", "SearchHost.exe", 0); got != "SearchHost" {
		t.Errorf("HostName = %q, want discovery fallback %q", got, "SearchHost")
	}
	// The embedded catalog must be untouched by the override.
	base, _ := Load("")
	if got := base.HostName("msedgewebview2", "SearchHost.exe", 0); got != "Windows Search" {
		t.Errorf("embedded HostName = %q after an override load, want %q", got, "Windows Search")
	}
}

func readJSON(t *testing.T, p string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("written file is not valid JSON: %v\n%s", err, data)
	}
	return doc
}

func TestSetHostName_CreatesFileAndLoadsBack(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", FileName)
	if err := SetHostName(p, "msedgewebview2.exe", "WhatsApp.Root.exe", "  WA  "); err != nil {
		t.Fatalf("SetHostName: %v", err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.HostName("msedgewebview2", "WhatsApp.Root.exe", 0); got != "WA" {
		t.Errorf("HostName after SetHostName = %q, want %q", got, "WA")
	}
	if doc := readJSON(t, p); doc["schemaVersion"] != float64(1) {
		t.Errorf("schemaVersion = %v, want 1", doc["schemaVersion"])
	}
}

func TestSetHostName_PreservesUserContentAndSpelling(t *testing.T) {
	p := writeOverride(t, `{
		"schemaVersion": 1,
		"futureField": {"keep": true},
		"apps": {
			"slack": {"friendlyName": "Slack!", "unknown": 42},
			"MSEdgeWebView2.exe": {"friendlyName": "WV2", "hosts": {"googledrivefs": "Drive"}}
		}
	}`)
	if err := SetHostName(p, "msedgewebview2", "WhatsApp.Root", "WhatsApp"); err != nil {
		t.Fatalf("SetHostName: %v", err)
	}
	doc := readJSON(t, p)
	if _, ok := doc["futureField"]; !ok {
		t.Error("unknown top-level field was dropped")
	}
	apps := doc["apps"].(map[string]any)
	if _, ok := apps["msedgewebview2"]; ok {
		t.Error("a second key was added instead of reusing the user's spelling")
	}
	wv := apps["MSEdgeWebView2.exe"].(map[string]any)
	hosts := wv["hosts"].(map[string]any)
	if hosts["googledrivefs"] != "Drive" || hosts["whatsapp.root"] != "WhatsApp" || wv["friendlyName"] != "WV2" {
		t.Errorf("entry = %v", wv)
	}
	if apps["slack"].(map[string]any)["unknown"] != float64(42) {
		t.Error("unknown entry field was dropped")
	}
}

func TestSetHostName_EmptyLabelRemovesAndPrunes(t *testing.T) {
	p := writeOverride(t, `{"apps": {"msedgewebview2": {"hosts": {"whatsapp.root": "WA"}}}}`)
	if err := SetHostName(p, "msedgewebview2", "WhatsApp.Root.exe", ""); err != nil {
		t.Fatalf("SetHostName: %v", err)
	}
	apps := readJSON(t, p)["apps"].(map[string]any)
	if len(apps) != 0 {
		t.Errorf("apps = %v, want the emptied entry pruned", apps)
	}
}

func TestSetHostName_RefusesToOverwriteInvalidJSON(t *testing.T) {
	const broken = `{"apps": {`
	p := writeOverride(t, broken)
	err := SetHostName(p, "msedgewebview2", "x", "X")
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("err = %v, want a not-valid-JSON error", err)
	}
	if data, _ := os.ReadFile(p); string(data) != broken {
		t.Errorf("file was modified: %q", data)
	}
}

func TestCreateUserFile_SeedsHostsOnlyWhenMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	seen := map[string]map[string]string{
		"msedgewebview2": {"WhatsApp.Root.exe": "WhatsApp.Root", "SearchHost.exe": "Windows Search"},
		"chrome":         {},
	}
	created, err := CreateUserFile(p, seen)
	if err != nil || !created {
		t.Fatalf("CreateUserFile = %v, %v; want created", created, err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.HostName("msedgewebview2", "WhatsApp.Root.exe", 0); got != "WhatsApp.Root" {
		t.Errorf("seeded label = %q, want %q", got, "WhatsApp.Root")
	}
	if _, ok := readJSON(t, p)["apps"].(map[string]any)["chrome"]; ok {
		t.Error("runtime without hosts should not be seeded")
	}

	created, err = CreateUserFile(p, nil)
	if err != nil || created {
		t.Errorf("second CreateUserFile = %v, %v; want not created, no error", created, err)
	}
}
