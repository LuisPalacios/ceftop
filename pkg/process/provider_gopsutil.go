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

// Snapshot enumerates every process on the host and filters down to those
// whose normalized basename equals the normalized target. Per-process errors
// (process gone, permission denied) are silently skipped — Snapshot is called
// on a tick and a short-lived enumeration error must not corrupt the tree.
func (p *GopsutilProvider) Snapshot(target string) ([]RawProcess, error) {
	procs, err := psprocess.Processes()
	if err != nil {
		return nil, fmt.Errorf("enumerating processes: %w", err)
	}

	wantAll := target == ""
	wantName := NormalizeTargetName(target)

	out := make([]RawProcess, 0, 16)
	for _, proc := range procs {
		name, err := proc.Name()
		if err != nil {
			continue
		}
		if !wantAll && NormalizeTargetName(name) != wantName {
			continue
		}

		ppid, _ := proc.Ppid()
		cmdline, _ := proc.Cmdline()
		threads, _ := proc.NumThreads()

		var rss uint64
		if mem, err := proc.MemoryInfo(); err == nil && mem != nil {
			rss = mem.RSS
		}

		// Times() may return nil + permission error for system processes the
		// caller can't introspect; treat that as zero CPU rather than skipping
		// the row entirely (we still want to display PID / role / mem).
		var cpuSec float64
		if t, err := proc.Times(); err == nil && t != nil {
			cpuSec = t.User + t.System
		}

		out = append(out, RawProcess{
			PID:        proc.Pid,
			PPID:       ppid,
			Name:       name,
			Cmdline:    cmdline,
			Threads:    threads,
			MemRSS:     rss,
			CPUSeconds: cpuSec,
		})
	}
	return out, nil
}
