package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/LuisPalacios/ceftop/pkg/catalog"
	"github.com/LuisPalacios/ceftop/pkg/icons"
	"github.com/LuisPalacios/ceftop/pkg/process"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	a := NewApp(nil)
	a.cfgPath = filepath.Join(t.TempDir(), "ceftop.json")
	a.loadConfig()
	return a
}

// A target change must queue exactly one out-of-band tick for snapshotLoop,
// and repeated changes must never block the binding call on a full channel.
func TestSetTargetApp_QueuesImmediateRefresh(t *testing.T) {
	a := newTestApp(t)

	if err := a.SetTargetApp("chrome"); err != nil {
		t.Fatalf("SetTargetApp: %v", err)
	}
	select {
	case <-a.refresh:
	default:
		t.Fatal("no refresh queued after SetTargetApp")
	}

	// Two back-to-back changes with nobody draining: the second must not
	// block, and only one request may be pending afterwards.
	if err := a.SetTargetApp("code"); err != nil {
		t.Fatalf("SetTargetApp: %v", err)
	}
	if err := a.SetTargetApp("slack"); err != nil {
		t.Fatalf("SetTargetApp: %v", err)
	}
	if got := len(a.refresh); got != 1 {
		t.Errorf("pending refresh requests = %d, want 1", got)
	}
	if got := a.currentTarget(); got != "slack" {
		t.Errorf("currentTarget = %q, want %q", got, "slack")
	}
}

// The frontend's ready signal must wake both loops exactly once each, and
// must stay non-blocking when nobody is draining the channels.
func TestFrontendReady_QueuesBothRefreshes(t *testing.T) {
	a := newTestApp(t)
	a.FrontendReady()
	a.FrontendReady()
	if got := len(a.refresh); got != 1 {
		t.Errorf("pending snapshot refreshes = %d, want 1", got)
	}
	if got := len(a.discoveryRefresh); got != 1 {
		t.Errorf("pending discovery refreshes = %d, want 1", got)
	}
}

// Host labels come from the catalog, including the user's apps.json next to
// the config, and are re-read after the file changes.
func TestLabelHosts_UsesCatalogAndUserOverride(t *testing.T) {
	a := newTestApp(t)
	snap := func() []*process.ProcessNode {
		return []*process.ProcessNode{
			{PID: 10, Name: "msedgewebview2.exe", Host: "SearchHost.exe", Children: []*process.ProcessNode{{PID: 11, Name: "msedgewebview2.exe"}}},
			{PID: 20, Name: "msedgewebview2.exe", Host: "some-new.app.exe"},
			{PID: 30, Name: "msedgewebview2.exe"},
		}
	}

	roots := snap()
	labelHosts(roots, a.reloadCatalog())
	if got := roots[0].HostLabel; got != "Windows Search" {
		t.Errorf("SearchHost label = %q, want %q", got, "Windows Search")
	}
	if got := roots[1].HostLabel; got != "Some New" {
		t.Errorf("unknown host label = %q, want heuristic %q", got, "Some New")
	}
	if roots[0].Children[0].HostLabel != "" || roots[2].HostLabel != "" {
		t.Error("nodes without a host must stay unlabelled")
	}

	override := filepath.Join(filepath.Dir(a.cfgPath), catalog.FileName)
	if err := os.WriteFile(override, []byte(`{"apps":{"some-new.app":{"friendlyName":"Brand New"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	roots = snap()
	labelHosts(roots, a.reloadCatalog())
	if got := roots[1].HostLabel; got != "Brand New" {
		t.Errorf("label after override = %q, want %q", got, "Brand New")
	}
}

// RenameHost writes the name to the user's apps.json, the next labelling
// pass uses it, a refresh is queued, and an empty name restores the default.
func TestRenameHost_RoundTrip(t *testing.T) {
	a := newTestApp(t)
	node := func() []*process.ProcessNode {
		return []*process.ProcessNode{{PID: 1, Name: "msedgewebview2.exe", Host: "WhatsApp.Root.exe"}}
	}

	if err := a.RenameHost("msedgewebview2.exe", "WhatsApp.Root.exe", "WA"); err != nil {
		t.Fatalf("RenameHost: %v", err)
	}
	if len(a.refresh) != 1 || len(a.discoveryRefresh) != 1 {
		t.Error("RenameHost should queue a snapshot and a discovery refresh")
	}
	roots := node()
	labelHosts(roots, a.currentCatalog())
	if roots[0].HostLabel != "WA" {
		t.Errorf("label after rename = %q, want %q", roots[0].HostLabel, "WA")
	}

	if err := a.RenameHost("msedgewebview2.exe", "WhatsApp.Root.exe", ""); err != nil {
		t.Fatalf("RenameHost reset: %v", err)
	}
	roots = node()
	labelHosts(roots, a.currentCatalog())
	if roots[0].HostLabel != "WhatsApp" {
		t.Errorf("label after reset = %q, want embedded %q", roots[0].HostLabel, "WhatsApp")
	}
}

// DescribeApp returns the catalog name, and a catalog icon key wins over
// fuzzy matching.
func TestDescribeApp_CatalogNameAndIcon(t *testing.T) {
	a := newTestApp(t)
	a.iconFS = fstest.MapFS{
		"app-edge.svg":           {Data: []byte("<svg/>")},
		"app-msedgewebview2.svg": {Data: []byte("<svg/>")},
	}
	override := filepath.Join(filepath.Dir(a.cfgPath), catalog.FileName)
	if err := os.WriteFile(override, []byte(`{"apps":{"msedgewebview2":{"icon":"edge"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a.reloadCatalog()

	got := a.DescribeApp("msedgewebview2")
	if got.FriendlyName != "Edge WebView2" {
		t.Errorf("FriendlyName = %q, want %q", got.FriendlyName, "Edge WebView2")
	}
	if want := icons.BundledURLPrefix + "app-edge.svg"; got.IconSrc != want {
		t.Errorf("IconSrc = %q, want catalog icon %q", got.IconSrc, want)
	}
}

// A rejected name must not queue a refresh: nothing changed.
func TestSetTargetApp_EmptyNameDoesNotRefresh(t *testing.T) {
	a := newTestApp(t)
	if err := a.SetTargetApp("   "); err == nil {
		t.Fatal("expected error for blank target")
	}
	if got := len(a.refresh); got != 0 {
		t.Errorf("pending refresh requests = %d, want 0", got)
	}
}
