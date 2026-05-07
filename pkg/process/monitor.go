package process

import (
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// BuildSnapshot enumerates processes via the supplied Provider, assembles
// them into a tree by parent / child PIDs, and returns the immutable payload
// for the frontend.
//
// An empty target returns an empty snapshot rather than enumerating every
// process on the host — the GUI uses that as the signal to keep showing the
// onboarding view.
func BuildSnapshot(provider Provider, target string) (*ProcessSnapshot, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return &ProcessSnapshot{Target: "", Roots: nil, Captured: time.Now(), Total: 0}, nil
	}

	raws, err := provider.Snapshot(target)
	if err != nil {
		return nil, err
	}
	return assembleTree(raws, target, time.Now()), nil
}

// assembleTree wires raw process records into a parent / child tree.
// A node whose PPID is not present in the snapshot is treated as a root —
// this matches the prototype's behavior and correctly handles re-parented
// orphans (e.g. when the OS lifts a child to PID 1 after its parent dies).
//
// Children are sorted heaviest-first by MemMB at every level. Sort is stable
// so equal-memory siblings keep enumeration order.
func assembleTree(raws []RawProcess, target string, capturedAt time.Time) *ProcessSnapshot {
	nodes := make(map[int32]*ProcessNode, len(raws))
	pids := make(map[int32]struct{}, len(raws))

	for _, r := range raws {
		pids[r.PID] = struct{}{}
		nodes[r.PID] = &ProcessNode{
			PID:       r.PID,
			PPID:      r.PPID,
			Role:      ExtractRole(r.Cmdline),
			Name:      r.Name,
			Threads:   r.Threads,
			MemMB:     bytesToMB(r.MemRSS),
			CPUTimeMs: uint64(r.CPUSeconds * 1000),
			Cmdline:   r.Cmdline,
		}
	}

	var roots []*ProcessNode
	for _, n := range nodes {
		if _, ok := pids[n.PPID]; ok {
			parent := nodes[n.PPID]
			parent.Children = append(parent.Children, n)
			continue
		}
		roots = append(roots, n)
	}

	sortByMem(roots)
	for _, r := range roots {
		sortRecursive(r)
	}

	return &ProcessSnapshot{
		Target:   target,
		Roots:    roots,
		Captured: capturedAt,
		Total:    len(raws),
	}
}

func sortByMem(nodes []*ProcessNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].MemMB > nodes[j].MemMB
	})
}

func sortRecursive(n *ProcessNode) {
	sortByMem(n.Children)
	for _, c := range n.Children {
		sortRecursive(c)
	}
}

// bytesToMB rounds bytes to MB with two decimal places, matching the
// prototype's `[math]::Round($bytes / 1MB, 2)` output.
func bytesToMB(b uint64) float64 {
	mb := float64(b) / (1024.0 * 1024.0)
	rounded := float64(int64(mb*100+0.5)) / 100.0
	return rounded
}

// NormalizeTargetName lowercases a target executable name and strips any
// trailing ".exe" so callers can match against process names emitted in
// either form. Used by Provider implementations and exported for tests.
//
// We can't rely on filepath.Base here: it only recognises the host OS's
// separator. A Windows-style path like "C:\\Path\\app.exe" passed on a
// Linux runner (e.g. CI) would come back unchanged. Splitting on either
// separator keeps the function host-agnostic, so the same input produces
// the same output on every platform.
func NormalizeTargetName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	lower := strings.ToLower(name)
	return strings.TrimSuffix(lower, ".exe")
}

// cpuSample is one PID's CPU-time snapshot, captured at a wall-clock instant.
// The Monitor diffs the previous sample against the current one to derive a
// per-tick CPU percentage.
type cpuSample struct {
	cpuSeconds float64
	at         time.Time
}

// Monitor is BuildSnapshot's stateful sibling: it tracks per-PID CPU samples
// across ticks so it can populate ProcessNode.CPUPercent with the share of
// total CPU capacity (across all cores) consumed during the last interval.
//
// On the first tick after a process appears, CPUPercent is 0 because there
// is no prior sample to diff against. Stateless callers should keep using
// BuildSnapshot; both produce identical CPUTimeMs.
type Monitor struct {
	provider Provider
	numCPU   float64

	mu   sync.Mutex
	prev map[int32]cpuSample
}

// NewMonitor returns a Monitor that uses the supplied Provider. NumCPU is
// captured once at construction — CefTop is short-lived per-session so we
// don't need to handle hotplug. A floor of 1 keeps the percentage formula
// safe on hosts that report 0 (shouldn't happen, but cheap to guard).
func NewMonitor(p Provider) *Monitor {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	return &Monitor{
		provider: p,
		numCPU:   float64(n),
		prev:     map[int32]cpuSample{},
	}
}

// Snapshot enumerates the target's processes and returns the immutable tree
// payload, with each node's CPUPercent filled from the delta against the
// previous tick's sample.
//
// An empty target clears the sample state and returns an empty snapshot —
// matching BuildSnapshot's onboarding contract — so the next non-empty tick
// starts fresh instead of comparing against stale values.
func (m *Monitor) Snapshot(target string) (*ProcessSnapshot, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		m.mu.Lock()
		m.prev = map[int32]cpuSample{}
		m.mu.Unlock()
		return &ProcessSnapshot{Target: "", Roots: nil, Captured: time.Now(), Total: 0}, nil
	}

	raws, err := m.provider.Snapshot(target)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	m.mu.Lock()
	next := make(map[int32]cpuSample, len(raws))
	pcts := make(map[int32]float64, len(raws))
	for _, r := range raws {
		next[r.PID] = cpuSample{cpuSeconds: r.CPUSeconds, at: now}
		prev, ok := m.prev[r.PID]
		if !ok {
			continue
		}
		dCPU := r.CPUSeconds - prev.cpuSeconds
		dWall := now.Sub(prev.at).Seconds()
		if dWall <= 0 || dCPU < 0 {
			continue
		}
		pct := (dCPU / (dWall * m.numCPU)) * 100.0
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		pcts[r.PID] = pct
	}
	m.prev = next
	m.mu.Unlock()

	snap := assembleTree(raws, target, now)
	applyCPUPercent(snap.Roots, pcts)
	return snap, nil
}

// applyCPUPercent walks the tree and stamps each node with its computed
// percentage. Nodes without a prior sample are left at 0.
func applyCPUPercent(nodes []*ProcessNode, pcts map[int32]float64) {
	for _, n := range nodes {
		if pct, ok := pcts[n.PID]; ok {
			n.CPUPercent = pct
		}
		applyCPUPercent(n.Children, pcts)
	}
}
