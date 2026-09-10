package process

import (
	"sync"
	"time"

	psprocess "github.com/shirou/gopsutil/v4/process"
)

// liteCacheTTL bounds how long one host-wide enumeration pass is reused by
// a second Snapshot call. The snapshot loop and the discovery loop fire at
// the same instant on FrontendReady and whenever their tickers coincide;
// the window is wide enough to cover that and narrow enough that a process
// spawned between two genuinely separate ticks is never missed.
const liteCacheTTL = 100 * time.Millisecond

// GopsutilProvider is the production Provider. Host-wide enumeration goes
// through the platform-specific enumerateLite (one Toolhelp walk on
// Windows, gopsutil's /proc and sysctl readers elsewhere); the per-process
// telemetry for the target's subtree is read with
// github.com/shirou/gopsutil/v4/process:
//
//   - Linux:   /proc/<pid>/{stat,cmdline,statm}
//   - macOS:   sysctl + KERN_PROCARGS2, proc_pidinfo
//   - Windows: OpenProcess + PEB read for cmdline, GetProcessMemoryInfo,
//     GetProcessTimes
//
// All three preserve the --type=<role> argument intact, which is the
// load-bearing assumption for the role parser. If a future gopsutil release
// breaks that on a platform, swap this implementation for a platform-specific
// reader behind the same Provider interface.
type GopsutilProvider struct {
	// enumerate and now are swappable so tests can count passes and drive
	// the cache clock without touching the OS.
	enumerate func() ([]liteProc, error)
	now       func() time.Time

	mu       sync.Mutex
	cachedAt time.Time
	cached   []liteProc
}

// NewGopsutilProvider returns a Provider ready to enumerate processes. The
// only state it carries is the short-lived enumeration cache; see
// liteCacheTTL.
func NewGopsutilProvider() *GopsutilProvider {
	return &GopsutilProvider{enumerate: enumerateLite, now: time.Now}
}

// lites returns the host-wide enumeration pass, reusing the previous one
// when it is younger than liteCacheTTL. The slice is shared read-only
// between callers; nothing downstream mutates it.
func (p *GopsutilProvider) lites() ([]liteProc, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.now()
	if !p.cachedAt.IsZero() && now.Sub(p.cachedAt) < liteCacheTTL {
		return p.cached, nil
	}
	lites, err := p.enumerate()
	if err != nil {
		return nil, err
	}
	p.cached = lites
	p.cachedAt = now
	return lites, nil
}

// Snapshot enumerates the host's processes and returns the target's subtree:
// every process whose normalized basename equals the target plus every
// transitive OS-level child by PPID. See [selectSubtreePIDs] for the exact
// inclusion rule and the rationale for going by ancestry rather than by
// basename match — the short version is that helpers like an externally-named
// crashpad_handler.exe are real children of the target's main process and
// must appear under it. An empty target returns every process; that's the
// path DiscoverApps relies on.
//
// Two phases keep the per-tick cost bounded on hosts with hundreds of
// processes: the platform enumeration pass (see lites) collects PID / PPID /
// basename for everyone so the subtree can be computed, then the per-PID
// lookups (cmdline, memory, thread count, CPU times) are issued only for the
// kept PIDs. Process handles are built directly from the PID rather than via
// psprocess.NewProcess, which would add an existence probe and a creation-
// time read per process for no benefit here.
//
// Per-process errors (process gone, permission denied) are silently skipped
// — Snapshot is called on a tick and a short-lived enumeration error must
// not corrupt the tree.
func (p *GopsutilProvider) Snapshot(target string) ([]RawProcess, error) {
	lites, err := p.lites()
	if err != nil {
		return nil, err
	}

	refs := make([]procRef, len(lites))
	for i := range lites {
		refs[i] = lites[i].procRef
	}
	keep := selectSubtreePIDs(refs, target)

	out := make([]RawProcess, 0, len(keep))
	for _, l := range lites {
		if _, ok := keep[l.PID]; !ok {
			continue
		}
		pr := &psprocess.Process{Pid: l.PID}

		cmdline, _ := pr.Cmdline()

		threads := l.Threads
		if !enumerationReportsThreads {
			threads, _ = pr.NumThreads()
		}

		var rss uint64
		if mem, err := pr.MemoryInfo(); err == nil && mem != nil {
			rss = mem.RSS
		}

		// Times() may return nil + permission error for system processes the
		// caller can't introspect; treat that as zero CPU rather than skipping
		// the row entirely (we still want to display PID / role / mem).
		var cpuSec float64
		if t, err := pr.Times(); err == nil && t != nil {
			cpuSec = t.User + t.System
		}

		out = append(out, RawProcess{
			PID:        l.PID,
			PPID:       l.PPID,
			Name:       l.Name,
			Cmdline:    cmdline,
			Threads:    threads,
			MemRSS:     rss,
			CPUSeconds: cpuSec,
		})
	}
	return out, nil
}
