package process

import "testing"

func TestSelectSubtreePIDs_EmptyTargetReturnsAll(t *testing.T) {
	refs := []procRef{
		{PID: 1, PPID: 0, Name: "init"},
		{PID: 2, PPID: 1, Name: "chrome.exe"},
	}
	keep := selectSubtreePIDs(refs, "")
	if len(keep) != 2 {
		t.Fatalf("len(keep) = %d, want 2", len(keep))
	}
}

func TestSelectSubtreePIDs_WhitespaceTargetReturnsAll(t *testing.T) {
	refs := []procRef{{PID: 1, PPID: 0, Name: "init"}}
	keep := selectSubtreePIDs(refs, "   ")
	if len(keep) != 1 {
		t.Errorf("whitespace target should behave like empty: got %d, want 1", len(keep))
	}
}

// The regression: an externally-named crashpad_handler.exe is a real OS-level
// child of the target's main process. Its binary name does not match the
// target, and its cmdline carries no --type= flag. It must still be included.
func TestSelectSubtreePIDs_HelperWithDifferentBinaryNameIncluded(t *testing.T) {
	refs := []procRef{
		{PID: 100, PPID: 1, Name: "sumwall.browser"},
		{PID: 101, PPID: 100, Name: "sumwall.browser"},      // renderer
		{PID: 102, PPID: 100, Name: "crashpad_handler.exe"}, // upstream Crashpad helper
		{PID: 999, PPID: 1, Name: "explorer.exe"},           // unrelated noise
	}
	keep := selectSubtreePIDs(refs, "sumwall.browser")
	for _, want := range []int32{100, 101, 102} {
		if _, ok := keep[want]; !ok {
			t.Errorf("PID %d missing from subtree", want)
		}
	}
	if _, leaked := keep[999]; leaked {
		t.Error("unrelated PID 999 leaked into subtree")
	}
}

func TestSelectSubtreePIDs_TransitiveDescendants(t *testing.T) {
	// Browser → utility → utility's own helper. The grandchild's name
	// differs from the target and its cmdline carries no --type= flag,
	// yet it must still appear under the target.
	refs := []procRef{
		{PID: 100, PPID: 1, Name: "app.exe"},
		{PID: 101, PPID: 100, Name: "app.exe"},
		{PID: 102, PPID: 101, Name: "child-helper.exe"},
	}
	keep := selectSubtreePIDs(refs, "app")
	for _, want := range []int32{100, 101, 102} {
		if _, ok := keep[want]; !ok {
			t.Errorf("PID %d missing from subtree", want)
		}
	}
}

func TestSelectSubtreePIDs_MultipleMatchingRoots(t *testing.T) {
	// Two independent chrome instances; each must keep its own subtree.
	refs := []procRef{
		{PID: 100, PPID: 1, Name: "chrome.exe"},
		{PID: 101, PPID: 100, Name: "chrome.exe"},
		{PID: 200, PPID: 1, Name: "chrome.exe"},
		{PID: 201, PPID: 200, Name: "chrome_crashpad_handler.exe"},
	}
	keep := selectSubtreePIDs(refs, "chrome")
	for _, want := range []int32{100, 101, 200, 201} {
		if _, ok := keep[want]; !ok {
			t.Errorf("PID %d missing from subtree", want)
		}
	}
}

func TestSelectSubtreePIDs_NoMatchReturnsEmpty(t *testing.T) {
	refs := []procRef{
		{PID: 1, PPID: 0, Name: "init"},
		{PID: 2, PPID: 1, Name: "bash"},
	}
	keep := selectSubtreePIDs(refs, "chrome")
	if len(keep) != 0 {
		t.Errorf("len(keep) = %d, want 0 (no matching process on host)", len(keep))
	}
}

func TestSelectSubtreePIDs_OrphanMainSurvives(t *testing.T) {
	// The main browser's PPID (e.g. services.exe = 555) is not in the
	// snapshot. It must still be selected as a seed and pull in its
	// descendants — this preserves assembleTree's "orphan becomes root"
	// behavior at the provider level.
	refs := []procRef{
		{PID: 100, PPID: 555, Name: "app.exe"},
		{PID: 101, PPID: 100, Name: "app.exe"},
	}
	keep := selectSubtreePIDs(refs, "app")
	for _, want := range []int32{100, 101} {
		if _, ok := keep[want]; !ok {
			t.Errorf("PID %d missing", want)
		}
	}
}

func TestSelectSubtreePIDs_SelfParentDoesNotInfloop(t *testing.T) {
	refs := []procRef{
		{PID: 7, PPID: 7, Name: "weird.exe"},
	}
	keep := selectSubtreePIDs(refs, "weird")
	if _, ok := keep[7]; !ok {
		t.Error("seed PID 7 missing")
	}
	if len(keep) != 1 {
		t.Errorf("len(keep) = %d, want 1", len(keep))
	}
}

func TestSelectSubtreePIDs_PPIDCycleTerminates(t *testing.T) {
	// Mutual parentage between two PIDs; the visited-set guard must stop
	// BFS rather than spin.
	refs := []procRef{
		{PID: 1, PPID: 2, Name: "app.exe"},
		{PID: 2, PPID: 1, Name: "helper.exe"},
	}
	keep := selectSubtreePIDs(refs, "app")
	if _, ok := keep[1]; !ok {
		t.Error("seed PID 1 missing")
	}
	if _, ok := keep[2]; !ok {
		t.Error("PID 2 missing — child of seed via PPID=1")
	}
}

func TestSelectSubtreePIDs_NormalizesBothSides(t *testing.T) {
	// Target passed with .EXE casing; process Name without extension.
	// Both sides go through NormalizeTargetName, so the seed must still match.
	refs := []procRef{
		{PID: 100, PPID: 1, Name: "MyApp"},
		{PID: 101, PPID: 100, Name: "MyApp"},
	}
	keep := selectSubtreePIDs(refs, "MYAPP.EXE")
	for _, want := range []int32{100, 101} {
		if _, ok := keep[want]; !ok {
			t.Errorf("PID %d missing — case/extension normalization regressed", want)
		}
	}
}
