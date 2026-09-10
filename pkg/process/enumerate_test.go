package process

import (
	"os"
	"path/filepath"
	"testing"
)

// The enumeration pass must see the test binary itself with the parentage
// and basename the OS reports, since the subtree selection keys on exactly
// those fields. On platforms whose pass reports thread counts, a live
// process always has at least one.
func TestEnumerateLite_FindsCurrentProcess(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable not available: %v", err)
	}

	lites, err := enumerateLite()
	if err != nil {
		t.Fatalf("enumerateLite: %v", err)
	}
	if len(lites) < 2 {
		t.Fatalf("enumerateLite returned %d processes, expected a populated host", len(lites))
	}

	myPID := int32(os.Getpid())
	var me *liteProc
	for i := range lites {
		if lites[i].PID == myPID {
			me = &lites[i]
			break
		}
	}
	if me == nil {
		t.Fatalf("current PID %d missing from %d enumerated processes", myPID, len(lites))
	}
	if me.PPID != int32(os.Getppid()) {
		t.Errorf("PPID = %d, want %d", me.PPID, os.Getppid())
	}
	if NormalizeTargetName(me.Name) != NormalizeTargetName(filepath.Base(exe)) {
		t.Errorf("Name = %q, want basename of %q", me.Name, exe)
	}
	if enumerationReportsThreads && me.Threads < 1 {
		t.Errorf("Threads = %d, want >= 1 from the enumeration pass", me.Threads)
	}
}

// End-to-end through the provider: the enrichment pass must fill thread
// count and RSS for the current process regardless of which platform pass
// supplied the identity fields.
func TestGopsutilProvider_EnrichesCurrentProcess(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable not available: %v", err)
	}
	target := filepath.Base(exe)

	raws, err := NewGopsutilProvider().Snapshot(target)
	if err != nil {
		t.Fatalf("Snapshot(%q): %v", target, err)
	}
	myPID := int32(os.Getpid())
	for _, r := range raws {
		if r.PID != myPID {
			continue
		}
		if r.Threads < 1 {
			t.Errorf("Threads = %d, want >= 1", r.Threads)
		}
		if r.MemRSS == 0 {
			t.Errorf("MemRSS = 0, want the test binary's working set")
		}
		if NormalizeTargetName(r.Name) != NormalizeTargetName(target) {
			t.Errorf("Name = %q, want %q", r.Name, target)
		}
		return
	}
	t.Fatalf("current PID %d not in snapshot of %q (%d rows)", myPID, target, len(raws))
}
