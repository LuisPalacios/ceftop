// Package process discovers Chromium / CEF / Electron multi-process trees,
// extracts the role from each child's --type=<role> command-line flag, and
// reports per-process telemetry (PID, PPID, threads, RSS).
//
// The package is split into:
//
//   - types.go            : DTOs and the Provider interface (this file).
//   - roles.go            : --type= parsing and role constants.
//   - monitor.go          : tree assembly from a flat Provider snapshot.
//   - provider_gopsutil.go: cross-platform Provider implementation backed by
//                           github.com/shirou/gopsutil/v4/process.
//
// The frontend never mutates ProcessSnapshot or ProcessNode in place; each
// tick replaces the snapshot wholesale.
package process

import "time"

// RawProcess is the OS-level data point a Provider emits for one process.
// Memory is in bytes — conversion to MB happens in monitor.go so callers can
// report exact numbers when they want them. CPUSeconds is the cumulative
// user+system CPU time since process start; deriving a tick-relative percent
// from it is the Monitor's job.
type RawProcess struct {
	PID        int32
	PPID       int32
	Name       string
	Cmdline    string
	Threads    int32
	MemRSS     uint64
	CPUSeconds float64
}

// Provider enumerates the processes that belong to a target's subtree.
//
// For a non-empty target the result includes every process whose basename
// (case-insensitive, with any ".exe" suffix stripped) equals the target's
// normalized form, plus every transitive OS-level child by PPID. Helpers
// whose binary name differs from the target — e.g. an externally-named
// crashpad_handler launched via plain CreateProcess by upstream Crashpad —
// and helpers that do not carry a Chromium --type=<role> flag are still
// included; the OS-level parent / child relationship is the source of
// truth, not the binary name or the command-line role flag.
//
// An empty target returns every process on the host — that's the "give me
// everything" path DiscoverApps relies on.
//
// Errors should be transient — the caller is expected to retry on the next
// tick rather than treating a failure as fatal.
type Provider interface {
	Snapshot(target string) ([]RawProcess, error)
}

// ProcessNode is a single process in the snapshot tree.
//
// CPUPercent is the share of total CPU capacity (across all cores) the
// process consumed during the last refresh interval, in the range [0, 100].
// It is zero on the first tick after a process appears (no prior sample to
// diff against) and stays zero for snapshots produced by stateless callers
// such as BuildSnapshot. CPUTimeMs is total CPU time since process start.
type ProcessNode struct {
	PID        int32          `json:"pid"`
	PPID       int32          `json:"ppid"`
	Role       Role           `json:"role"`
	Name       string         `json:"name"`
	Threads    int32          `json:"threads"`
	MemMB      float64        `json:"memMB"`
	CPUPercent float64        `json:"cpuPercent"`
	CPUTimeMs  uint64         `json:"cpuTimeMs"`
	Cmdline    string         `json:"cmdline,omitempty"`
	Children   []*ProcessNode `json:"children,omitempty"`
}

// ProcessSnapshot is the immutable payload sent to the frontend on each tick.
// Roots are sorted heaviest-first by MemMB (matching the prototype's layout);
// each node's Children are sorted the same way.
type ProcessSnapshot struct {
	Target   string         `json:"target"`
	Roots    []*ProcessNode `json:"roots"`
	Captured time.Time      `json:"captured"`
	Total    int            `json:"total"`
}
