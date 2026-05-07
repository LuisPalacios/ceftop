package process

import (
	"errors"
	"testing"
)

func TestDiscoverApps_SingleCEFTree(t *testing.T) {
	raws := []RawProcess{
		{PID: 100, PPID: 1, Name: "chrome.exe", Cmdline: "chrome.exe"},
		{PID: 101, PPID: 100, Name: "chrome.exe", Cmdline: "chrome.exe --type=renderer"},
		{PID: 102, PPID: 100, Name: "chrome.exe", Cmdline: "chrome.exe --type=gpu-process"},
	}
	apps, err := DiscoverApps(&fakeProvider{raws: raws})
	if err != nil {
		t.Fatalf("DiscoverApps error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("want 1 app, got %d (%+v)", len(apps), apps)
	}
	if apps[0].Name != "chrome" {
		t.Errorf("Name = %q, want %q", apps[0].Name, "chrome")
	}
	if apps[0].ChildCount != 2 {
		t.Errorf("ChildCount = %d, want 2", apps[0].ChildCount)
	}
}

func TestDiscoverApps_MultipleCoresidentApps(t *testing.T) {
	raws := []RawProcess{
		// Chrome
		{PID: 100, PPID: 1, Name: "chrome.exe", Cmdline: "chrome.exe"},
		{PID: 101, PPID: 100, Name: "chrome.exe", Cmdline: "chrome.exe --type=renderer"},
		// VS Code
		{PID: 200, PPID: 1, Name: "Code.exe", Cmdline: "Code.exe"},
		{PID: 201, PPID: 200, Name: "Code.exe", Cmdline: "Code.exe --type=renderer"},
		{PID: 202, PPID: 200, Name: "Code.exe", Cmdline: "Code.exe --type=utility"},
		// Personal CEF app
		{PID: 300, PPID: 1, Name: "myapp.exe", Cmdline: "myapp.exe"},
		{PID: 301, PPID: 300, Name: "myapp.exe", Cmdline: "myapp.exe --type=gpu-process"},
		// Non-CEF noise: must not surface.
		{PID: 400, PPID: 1, Name: "explorer.exe", Cmdline: "explorer.exe"},
		{PID: 401, PPID: 400, Name: "notepad.exe", Cmdline: "notepad.exe foo.txt"},
	}
	apps, err := DiscoverApps(&fakeProvider{raws: raws})
	if err != nil {
		t.Fatalf("DiscoverApps error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("want 3 apps, got %d (%+v)", len(apps), apps)
	}
	// Sorted by Name asc: chrome, code, myapp.
	want := []DiscoveredApp{
		{Name: "chrome", ChildCount: 1},
		{Name: "code", ChildCount: 2},
		{Name: "myapp", ChildCount: 1},
	}
	for i, w := range want {
		if apps[i] != w {
			t.Errorf("apps[%d] = %+v, want %+v", i, apps[i], w)
		}
	}
}

func TestDiscoverApps_RenamedHelperWalksToBrowser(t *testing.T) {
	// macOS-style: the "Google Chrome Helper" cmdline still carries --type=,
	// so walking PPID must skip past it to the parent that has no --type=.
	raws := []RawProcess{
		{PID: 100, PPID: 1, Name: "Google Chrome", Cmdline: "Google Chrome"},
		{PID: 110, PPID: 100, Name: "Google Chrome Helper", Cmdline: "Google Chrome Helper --type=renderer"},
		{PID: 111, PPID: 110, Name: "Google Chrome Helper", Cmdline: "Google Chrome Helper --type=utility"},
	}
	apps, err := DiscoverApps(&fakeProvider{raws: raws})
	if err != nil {
		t.Fatalf("DiscoverApps error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("want 1 app, got %d (%+v)", len(apps), apps)
	}
	if apps[0].Name != "google chrome" {
		t.Errorf("Name = %q, want %q", apps[0].Name, "google chrome")
	}
	// Both helper processes resolve to the same Main / Browser, so ChildCount=2.
	if apps[0].ChildCount != 2 {
		t.Errorf("ChildCount = %d, want 2", apps[0].ChildCount)
	}
}

func TestDiscoverApps_OrphanChildIsSkipped(t *testing.T) {
	// The renderer's PPID isn't in the snapshot — there's no browser to
	// attribute it to, so it must drop rather than show up as its own app.
	raws := []RawProcess{
		{PID: 200, PPID: 1, Name: "code.exe", Cmdline: "code.exe"},
		{PID: 201, PPID: 200, Name: "code.exe", Cmdline: "code.exe --type=renderer"},
		{PID: 999, PPID: 99999, Name: "ghost.exe", Cmdline: "ghost.exe --type=renderer"},
	}
	apps, err := DiscoverApps(&fakeProvider{raws: raws})
	if err != nil {
		t.Fatalf("DiscoverApps error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("want 1 app (orphan dropped), got %d (%+v)", len(apps), apps)
	}
	if apps[0].Name != "code" {
		t.Errorf("Name = %q, want %q", apps[0].Name, "code")
	}
}

func TestDiscoverApps_EmptyHostReturnsEmpty(t *testing.T) {
	apps, err := DiscoverApps(&fakeProvider{raws: nil})
	if err != nil {
		t.Fatalf("DiscoverApps error: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("want empty, got %+v", apps)
	}
}

func TestDiscoverApps_NoCEFNoApps(t *testing.T) {
	raws := []RawProcess{
		{PID: 1, PPID: 0, Name: "init", Cmdline: "init"},
		{PID: 2, PPID: 1, Name: "bash", Cmdline: "/bin/bash"},
		{PID: 3, PPID: 2, Name: "vim", Cmdline: "vim file.txt"},
	}
	apps, err := DiscoverApps(&fakeProvider{raws: raws})
	if err != nil {
		t.Fatalf("DiscoverApps error: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("want empty, got %+v", apps)
	}
}

func TestDiscoverApps_PropagatesProviderError(t *testing.T) {
	want := errors.New("boom")
	_, err := DiscoverApps(&fakeProvider{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("DiscoverApps error = %v, want %v", err, want)
	}
}

func TestDiscoverApps_SelfParentIsSkipped(t *testing.T) {
	// Pathological: a CEF child claims itself as its own parent. walkToBrowser
	// must bail rather than spin.
	raws := []RawProcess{
		{PID: 500, PPID: 500, Name: "weird.exe", Cmdline: "weird.exe --type=renderer"},
	}
	apps, err := DiscoverApps(&fakeProvider{raws: raws})
	if err != nil {
		t.Fatalf("DiscoverApps error: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("want empty (self-parent dropped), got %+v", apps)
	}
}
