package main

import (
	"path/filepath"
	"testing"
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
