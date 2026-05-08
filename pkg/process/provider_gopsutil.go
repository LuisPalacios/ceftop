package process

import (
	"fmt"

	psprocess "github.com/shirou/gopsutil/v4/process"
)

// GopsutilProvider is the production Provider, backed by
// github.com/shirou/gopsutil/v4/process. It delegates per-platform process
// enumeration to gopsutil's already-portable layer:
//
//   - Linux:   /proc/<pid>/{stat,cmdline,status}
//   - macOS:   sysctl + KERN_PROCARGS2
//   - Windows: NtQueryInformationProcess + PEB read for cmdline
//
// All three preserve the --type=<role> argument intact, which is the
// load-bearing assumption for the role parser. If a future gopsutil release
// breaks that on a platform, swap this implementation for a platform-specific
// reader behind the same Provider interface.
type GopsutilProvider struct{}

// NewGopsutilProvider returns a Provider ready to enumerate processes. The
// type carries no state; the constructor exists for API symmetry with future
// providers (e.g. a fake provider for tests, a sampling provider for perf
// tuning).
func NewGopsutilProvider() *GopsutilProvider {
	return &GopsutilProvider{}
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
// processes: a cheap pass collects PID / PPID / basename for everyone so
// the subtree can be computed, then the expensive lookups (cmdline, memory,
// thread count, CPU times) are issued only for the kept PIDs. On Windows
// each Cmdline call is an NtQueryInformationProcess + PEB read; doing that
// for every process every tick was the dominant cost before this split.
//
// Per-process errors (process gone, permission denied) are silently skipped
// — Snapshot is called on a tick and a short-lived enumeration error must
// not corrupt the tree.
func (p *GopsutilProvider) Snapshot(target string) ([]RawProcess, error) {
	procs, err := psprocess.Processes()
	if err != nil {
		return nil, fmt.Errorf("enumerating processes: %w", err)
	}

	type lite struct {
		proc *psprocess.Process
		ref  procRef
	}
	lites := make([]lite, 0, len(procs))
	refs := make([]procRef, 0, len(procs))
	for _, pr := range procs {
		name, err := pr.Name()
		if err != nil {
			continue
		}
		ppid, _ := pr.Ppid()
		ref := procRef{PID: pr.Pid, PPID: ppid, Name: name}
		lites = append(lites, lite{proc: pr, ref: ref})
		refs = append(refs, ref)
	}

	keep := selectSubtreePIDs(refs, target)

	out := make([]RawProcess, 0, len(keep))
	for _, l := range lites {
		if _, ok := keep[l.ref.PID]; !ok {
			continue
		}

		cmdline, _ := l.proc.Cmdline()
		threads, _ := l.proc.NumThreads()

		var rss uint64
		if mem, err := l.proc.MemoryInfo(); err == nil && mem != nil {
			rss = mem.RSS
		}

		// Times() may return nil + permission error for system processes the
		// caller can't introspect; treat that as zero CPU rather than skipping
		// the row entirely (we still want to display PID / role / mem).
		var cpuSec float64
		if t, err := l.proc.Times(); err == nil && t != nil {
			cpuSec = t.User + t.System
		}

		out = append(out, RawProcess{
			PID:        l.ref.PID,
			PPID:       l.ref.PPID,
			Name:       l.ref.Name,
			Cmdline:    cmdline,
			Threads:    threads,
			MemRSS:     rss,
			CPUSeconds: cpuSec,
		})
	}
	return out, nil
}
