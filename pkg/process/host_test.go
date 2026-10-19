package process

import "testing"

const wv2 = `"C:\Program Files (x86)\Microsoft\EdgeWebView\Application\155.0.4283.62\msedgewebview2.exe"`

func TestEmbeddedHost(t *testing.T) {
	cases := []struct {
		name         string
		cmdline      string
		wantExe      string
		wantEmbedded bool
	}{
		{"webview2 root", wv2 + ` --embedded-browser-webview=1 --webview-exe-name=SearchHost.exe --webview-exe-version=2608.25008.200.0`, "SearchHost.exe", true},
		{"value at end of line", wv2 + ` --embedded-browser-webview=1 --webview-exe-name=WhatsApp.exe`, "WhatsApp.exe", true},
		{"quoted value with spaces", wv2 + ` --embedded-browser-webview=1 --webview-exe-name="NVIDIA App.exe" --lang=en-US`, "NVIDIA App.exe", true},
		{"quoted argument with spaces", wv2 + ` --embedded-browser-webview=1 "--webview-exe-name=Docker Desktop.exe" --lang=en-US`, "Docker Desktop.exe", true},
		{"embedded without name", wv2 + ` --embedded-browser-webview=1 --lang=en-US`, "", true},
		{"renderer child", wv2 + ` --type=renderer --lang=en-US`, "", false},
		{"plain browser", `chrome.exe --no-sandbox`, "", false},
		{"empty", ``, "", false},
	}
	for _, c := range cases {
		exe, embedded := EmbeddedHost(c.cmdline)
		if exe != c.wantExe || embedded != c.wantEmbedded {
			t.Errorf("%s: EmbeddedHost = (%q, %v), want (%q, %v)", c.name, exe, embedded, c.wantExe, c.wantEmbedded)
		}
	}
}

// Four WebView2 groups, each hosted by a different app, plus one whose host
// is not in the catalog at all: every main process must learn its host from
// the process data alone, and the renderers must stay untouched.
func TestAssembleTree_WebView2GroupsGetTheirHost(t *testing.T) {
	raws := []RawProcess{
		{PID: 10, PPID: 1, Name: "msedgewebview2.exe", ParentName: "SearchHost.exe", Cmdline: wv2 + ` --embedded-browser-webview=1 --webview-exe-name=SearchHost.exe`},
		{PID: 11, PPID: 10, Name: "msedgewebview2.exe", ParentName: "msedgewebview2.exe", Cmdline: wv2 + ` --type=renderer`},
		{PID: 20, PPID: 2, Name: "msedgewebview2.exe", ParentName: "GoogleDriveFS.exe", Cmdline: wv2 + ` --embedded-browser-webview=1 --webview-exe-name=GoogleDriveFS.exe`},
		{PID: 30, PPID: 3, Name: "msedgewebview2.exe", ParentName: "SomeNewApp.exe", Cmdline: wv2 + ` --embedded-browser-webview=1 --webview-exe-name=SomeNewApp.exe`},
	}
	snap := assembleTree(raws, "msedgewebview2", testTime, nil)
	if len(snap.Roots) != 3 {
		t.Fatalf("expected 3 roots, got %d", len(snap.Roots))
	}
	want := map[int32]struct {
		host string
		pid  int32
	}{
		10: {"SearchHost.exe", 1},
		20: {"GoogleDriveFS.exe", 2},
		30: {"SomeNewApp.exe", 3},
	}
	for _, r := range snap.Roots {
		w := want[r.PID]
		if r.Role != RoleMain || r.Host != w.host || r.HostPID != w.pid {
			t.Errorf("root %d = role %q host %q/%d, want %q host %q/%d", r.PID, r.Role, r.Host, r.HostPID, RoleMain, w.host, w.pid)
		}
		for _, c := range r.Children {
			if c.Host != "" {
				t.Errorf("child %d should have no host, got %q", c.PID, c.Host)
			}
		}
	}
}

func TestResolveHost(t *testing.T) {
	isWV := func(name string) bool { return NormalizeTargetName(name) == "msedgewebview2" }
	cases := []struct {
		name     string
		raw      RawProcess
		hint     func(string) bool
		wantHost string
		wantPID  int32
	}{
		{
			name:     "named host that is not the parent keeps the name, no PID",
			raw:      RawProcess{PID: 5, PPID: 4, Name: "msedgewebview2.exe", ParentName: "launcher.exe", Cmdline: `x --webview-exe-name=Real.exe`},
			wantHost: "Real.exe",
		},
		{
			name:     "embedded flag without name falls back to the parent",
			raw:      RawProcess{PID: 5, PPID: 4, Name: "msedgewebview2.exe", ParentName: "Host.exe", Cmdline: `x --embedded-browser-webview=1`},
			wantHost: "Host.exe", wantPID: 4,
		},
		{
			name:     "catalog multiGroup hint without flags uses the parent",
			raw:      RawProcess{PID: 5, PPID: 4, Name: "msedgewebview2.exe", ParentName: "Host.exe", Cmdline: `x`},
			hint:     isWV,
			wantHost: "Host.exe", wantPID: 4,
		},
		{
			name: "no flags and no hint: a regular browser under the shell has no host",
			raw:  RawProcess{PID: 5, PPID: 4, Name: "chrome.exe", ParentName: "explorer.exe", Cmdline: `chrome.exe`},
			hint: isWV,
		},
		{
			name: "parent gone",
			raw:  RawProcess{PID: 5, PPID: 4, Name: "msedgewebview2.exe", Cmdline: `x --embedded-browser-webview=1`},
		},
		{
			name: "parent is the same executable",
			raw:  RawProcess{PID: 5, PPID: 4, Name: "msedgewebview2.exe", ParentName: "MSEdgeWebView2.exe", Cmdline: `x`},
			hint: isWV,
		},
	}
	for _, c := range cases {
		host, pid := resolveHost(c.raw, c.hint)
		if host != c.wantHost || pid != c.wantPID {
			t.Errorf("%s: resolveHost = (%q, %d), want (%q, %d)", c.name, host, pid, c.wantHost, c.wantPID)
		}
	}
}

func TestMonitor_SetMultiGroupReachesTree(t *testing.T) {
	provider := &fakeProvider{raws: []RawProcess{
		{PID: 5, PPID: 4, Name: "runtime.exe", ParentName: "Host.exe", Cmdline: `runtime.exe`},
	}}
	m := NewMonitor(provider)

	snap, err := m.Snapshot("runtime")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Roots[0].Host != "" {
		t.Errorf("without the hint, host = %q, want none", snap.Roots[0].Host)
	}

	m.SetMultiGroup(func(name string) bool { return NormalizeTargetName(name) == "runtime" })
	snap, err = m.Snapshot("runtime")
	if err != nil {
		t.Fatal(err)
	}
	if got := snap.Roots[0]; got.Host != "Host.exe" || got.HostPID != 4 {
		t.Errorf("with the hint, host = %q/%d, want Host.exe/4", got.Host, got.HostPID)
	}
}
