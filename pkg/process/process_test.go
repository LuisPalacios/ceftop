package process

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeProvider is a controllable Provider used to drive tree-assembly tests
// without spawning real processes. snapshots is consumed in order if set,
// letting Monitor tests stage successive ticks.
type fakeProvider struct {
	raws      []RawProcess
	snapshots [][]RawProcess
	err       error
}

func (f *fakeProvider) Snapshot(_ string) ([]RawProcess, error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(f.snapshots) > 0 {
		next := f.snapshots[0]
		f.snapshots = f.snapshots[1:]
		return next, nil
	}
	return f.raws, nil
}

func TestExtractRole(t *testing.T) {
	cases := []struct {
		cmdline string
		want    Role
	}{
		{"", RoleMain},
		{"C:\\Path\\app.exe", RoleMain},
		{"app.exe --no-sandbox --some-flag", RoleMain},
		{"app.exe --type=renderer --enable-features=Foo", RoleRenderer},
		{"app.exe --type=gpu-process", RoleGPUProcess},
		{"app.exe --type=utility --service-sandbox-type=audio", RoleUtility},
		{"app.exe --type=crashpad-handler", RoleCrashpad},
		{"app.exe --no-type=renderer", RoleMain}, // bare --no-type= must not match
	}
	for _, c := range cases {
		got := ExtractRole(c.cmdline)
		if got != c.want {
			t.Errorf("ExtractRole(%q) = %q, want %q", c.cmdline, got, c.want)
		}
	}
}

func TestBuildSnapshot_EmptyTarget_ReturnsEmptySnapshot(t *testing.T) {
	provider := &fakeProvider{raws: []RawProcess{
		{PID: 1, PPID: 0, Name: "anything", Cmdline: ""},
	}}
	snap, err := BuildSnapshot(provider, "")
	if err != nil {
		t.Fatalf("BuildSnapshot(empty target) error: %v", err)
	}
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snap.Roots) != 0 || snap.Total != 0 {
		t.Errorf("expected empty snapshot, got Roots=%d Total=%d", len(snap.Roots), snap.Total)
	}
}

func TestBuildSnapshot_PropagatesProviderError(t *testing.T) {
	want := errors.New("boom")
	_, err := BuildSnapshot(&fakeProvider{err: want}, "anything")
	if !errors.Is(err, want) {
		t.Fatalf("BuildSnapshot error = %v, want %v", err, want)
	}
}

func TestAssembleTree_SingleMain(t *testing.T) {
	raws := []RawProcess{
		{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe"},
	}
	snap := assembleTree(raws, "app", testTime)
	if len(snap.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(snap.Roots))
	}
	if snap.Roots[0].Role != RoleMain {
		t.Errorf("root role = %q, want %q", snap.Roots[0].Role, RoleMain)
	}
}

func TestAssembleTree_ParentWithChildren(t *testing.T) {
	raws := []RawProcess{
		{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe", MemRSS: 100 << 20},
		{PID: 101, PPID: 100, Name: "app.exe", Cmdline: "app.exe --type=renderer", MemRSS: 200 << 20},
		{PID: 102, PPID: 100, Name: "app.exe", Cmdline: "app.exe --type=gpu-process", MemRSS: 50 << 20},
	}
	snap := assembleTree(raws, "app", testTime)
	if len(snap.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(snap.Roots))
	}
	root := snap.Roots[0]
	if root.PID != 100 || root.Role != RoleMain {
		t.Errorf("root mismatch: PID=%d Role=%q", root.PID, root.Role)
	}
	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(root.Children))
	}
	// Children are sorted heaviest-first; renderer (200 MB) before gpu-process (50 MB).
	if root.Children[0].Role != RoleRenderer {
		t.Errorf("first child role = %q, want %q", root.Children[0].Role, RoleRenderer)
	}
	if root.Children[1].Role != RoleGPUProcess {
		t.Errorf("second child role = %q, want %q", root.Children[1].Role, RoleGPUProcess)
	}
}

func TestAssembleTree_OrphanBecomesRoot(t *testing.T) {
	// PPID=999 is not in the snapshot — the orphan must surface as a second root.
	raws := []RawProcess{
		{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe", MemRSS: 50 << 20},
		{PID: 101, PPID: 999, Name: "app.exe", Cmdline: "app.exe --type=utility", MemRSS: 10 << 20},
	}
	snap := assembleTree(raws, "app", testTime)
	if len(snap.Roots) != 2 {
		t.Fatalf("expected 2 roots (parent + orphan), got %d", len(snap.Roots))
	}
}

func TestAssembleTree_RootsSortedByMem(t *testing.T) {
	raws := []RawProcess{
		{PID: 1, PPID: 0, Name: "app.exe", Cmdline: "app.exe", MemRSS: 10 << 20},
		{PID: 2, PPID: 0, Name: "app.exe", Cmdline: "app.exe", MemRSS: 50 << 20},
		{PID: 3, PPID: 0, Name: "app.exe", Cmdline: "app.exe", MemRSS: 30 << 20},
	}
	snap := assembleTree(raws, "app", testTime)
	if len(snap.Roots) != 3 {
		t.Fatalf("expected 3 roots, got %d", len(snap.Roots))
	}
	if snap.Roots[0].PID != 2 || snap.Roots[1].PID != 3 || snap.Roots[2].PID != 1 {
		t.Errorf("roots not sorted by MemMB desc: got %d %d %d",
			snap.Roots[0].PID, snap.Roots[1].PID, snap.Roots[2].PID)
	}
}

func TestNormalizeTargetName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"app", "app"},
		{"app.exe", "app"},
		{"App.EXE", "app"},
		{"C:\\Path\\app.exe", "app"},
		{"/usr/bin/chrome", "chrome"},
		{"   ", "   "}, // NormalizeTargetName does not trim — that's the caller's job
	}
	for _, c := range cases {
		got := NormalizeTargetName(c.in)
		if got != c.want {
			t.Errorf("NormalizeTargetName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBytesToMB(t *testing.T) {
	cases := []struct {
		bytes uint64
		want  float64
	}{
		{0, 0},
		{1024 * 1024, 1.0},
		{2 * 1024 * 1024, 2.0},
		{1024*1024 + 512*1024, 1.5},
	}
	for _, c := range cases {
		got := bytesToMB(c.bytes)
		if got != c.want {
			t.Errorf("bytesToMB(%d) = %v, want %v", c.bytes, got, c.want)
		}
	}
}

// TestGopsutilProvider_FindsCurrentProcess is the integration smoke that
// proves the load-bearing risk in the roadmap: gopsutil's Cmdline() returns
// the running test binary's command line on this platform. If this fails,
// Phase 2 cannot exit until a platform-specific cmdline reader is added.
func TestGopsutilProvider_FindsCurrentProcess(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable not available: %v", err)
	}
	target := filepath.Base(exe)

	provider := NewGopsutilProvider()
	raws, err := provider.Snapshot(target)
	if err != nil {
		t.Fatalf("Snapshot(%q) error: %v", target, err)
	}

	myPID := int32(os.Getpid())
	var found *RawProcess
	for i := range raws {
		if raws[i].PID == myPID {
			found = &raws[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("did not find current PID %d in snapshot of %q (got %d processes)",
			myPID, target, len(raws))
	}
	if found.Cmdline == "" {
		t.Errorf("current process Cmdline is empty — gopsutil cannot read cmdline on this platform")
	}
	if !strings.Contains(strings.ToLower(found.Cmdline), strings.ToLower(target)) {
		t.Errorf("current process Cmdline %q does not contain target %q", found.Cmdline, target)
	}
}

func TestAssembleTree_InfersCrashpadRoleFromBinaryName(t *testing.T) {
	// Upstream Crashpad's standalone handler runs without --type= and with
	// a binary name that doesn't match the target. ExtractRole alone would
	// label it RoleMain (a duplicate "Main / Browser" row); the basename
	// fallback in assembleTree must catch it and emit RoleCrashpad.
	raws := []RawProcess{
		{PID: 100, PPID: 1, Name: "sumwall.browser", Cmdline: "sumwall.browser"},
		{PID: 102, PPID: 100, Name: "crashpad_handler.exe", Cmdline: "crashpad_handler.exe --initial-client-data=0,1,2"},
	}
	snap := assembleTree(raws, "sumwall.browser", testTime)
	if len(snap.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(snap.Roots))
	}
	root := snap.Roots[0]
	if root.Role != RoleMain {
		t.Errorf("root role = %q, want %q", root.Role, RoleMain)
	}
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(root.Children))
	}
	if root.Children[0].Role != RoleCrashpad {
		t.Errorf("crashpad child role = %q, want %q", root.Children[0].Role, RoleCrashpad)
	}
}

func TestAssembleTree_DoesNotOverrideTargetMainRole(t *testing.T) {
	// The target's own main process must stay RoleMain even though its
	// basename matches the target (the inference guard kicks in only for
	// non-target binaries).
	raws := []RawProcess{
		{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe"},
	}
	snap := assembleTree(raws, "app", testTime)
	if snap.Roots[0].Role != RoleMain {
		t.Errorf("root role = %q, want %q", snap.Roots[0].Role, RoleMain)
	}
}

func TestInferRoleFromName(t *testing.T) {
	cases := []struct {
		name string
		want Role
	}{
		{"crashpad_handler.exe", RoleCrashpad},
		{"chrome_crashpad_handler.exe", RoleCrashpad},
		{"CRASHPAD_HANDLER", RoleCrashpad},
		{"renderer.exe", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := InferRoleFromName(c.name)
		if got != c.want {
			t.Errorf("InferRoleFromName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestAssembleTree_PropagatesCPUTime(t *testing.T) {
	raws := []RawProcess{
		{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe", CPUSeconds: 1.234},
		{PID: 101, PPID: 100, Name: "app.exe", Cmdline: "app.exe --type=renderer", CPUSeconds: 0.5},
	}
	snap := assembleTree(raws, "app", testTime)
	if snap.Roots[0].CPUTimeMs != 1234 {
		t.Errorf("root CPUTimeMs = %d, want 1234", snap.Roots[0].CPUTimeMs)
	}
	if snap.Roots[0].Children[0].CPUTimeMs != 500 {
		t.Errorf("child CPUTimeMs = %d, want 500", snap.Roots[0].Children[0].CPUTimeMs)
	}
	// Stateless BuildSnapshot leaves CPUPercent at zero.
	if snap.Roots[0].CPUPercent != 0 {
		t.Errorf("BuildSnapshot CPUPercent = %v, want 0", snap.Roots[0].CPUPercent)
	}
}

func TestMonitor_FirstTickCPUPercentIsZero(t *testing.T) {
	provider := &fakeProvider{raws: []RawProcess{
		{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe", CPUSeconds: 5.0},
	}}
	m := NewMonitor(provider)
	snap, err := m.Snapshot("app")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Roots[0].CPUPercent != 0 {
		t.Errorf("first tick CPUPercent = %v, want 0", snap.Roots[0].CPUPercent)
	}
}

func TestMonitor_SecondTickComputesCPUPercent(t *testing.T) {
	// Two ticks, ~1s apart in wall clock. Between them the process burned
	// 1.0s of CPU. With NumCPU>=1 the upper bound is 100/NumCPU and the
	// expected value is 100/NumCPU exactly when dCPU==dWall.
	provider := &fakeProvider{snapshots: [][]RawProcess{
		{{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe", CPUSeconds: 0.0}},
		{{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe", CPUSeconds: 1.0}},
	}}
	m := NewMonitor(provider)

	if _, err := m.Snapshot("app"); err != nil {
		t.Fatalf("first Snapshot: %v", err)
	}
	// Force a measurable wall delta by rewinding the stored sample.
	m.mu.Lock()
	for pid, s := range m.prev {
		s.at = s.at.Add(-1 * time.Second)
		m.prev[pid] = s
	}
	m.mu.Unlock()

	snap, err := m.Snapshot("app")
	if err != nil {
		t.Fatalf("second Snapshot: %v", err)
	}
	pct := snap.Roots[0].CPUPercent
	if pct <= 0 {
		t.Fatalf("expected positive CPUPercent, got %v", pct)
	}
	if pct > 100 {
		t.Errorf("CPUPercent clamped above 100: %v", pct)
	}
}

func TestMonitor_EmptyTargetResetsState(t *testing.T) {
	provider := &fakeProvider{snapshots: [][]RawProcess{
		{{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe", CPUSeconds: 0.0}},
		{{PID: 100, PPID: 1, Name: "app.exe", Cmdline: "app.exe", CPUSeconds: 5.0}},
	}}
	m := NewMonitor(provider)
	if _, err := m.Snapshot("app"); err != nil {
		t.Fatalf("first Snapshot: %v", err)
	}
	if _, err := m.Snapshot(""); err != nil {
		t.Fatalf("empty Snapshot: %v", err)
	}
	// State must be cleared, so the next non-empty tick reports 0.
	snap, err := m.Snapshot("app")
	if err != nil {
		t.Fatalf("third Snapshot: %v", err)
	}
	if snap.Roots[0].CPUPercent != 0 {
		t.Errorf("CPUPercent after reset = %v, want 0", snap.Roots[0].CPUPercent)
	}
}

var testTime = time.Date(2026, time.May, 6, 12, 0, 0, 0, time.UTC)
