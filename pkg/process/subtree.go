package process

import "strings"

// procRef is the minimal per-process record needed to attribute a process to
// the target's subtree: identity, parentage, and binary basename. It exists
// so the gopsutil provider can do a cheap first-pass enumeration before
// paying for the expensive lookups (cmdline, memory, CPU) on the target's
// subtree only — on Windows those lookups call NtQueryInformationProcess
// and read the PEB, and the cost adds up quickly when there are hundreds of
// processes on the host.
type procRef struct {
	PID  int32
	PPID int32
	Name string
}

// selectSubtreePIDs returns the set of PIDs that belong to the target's
// process subtree: every process whose normalized basename equals the target
// (the seed set), plus every transitive child by PPID.
//
// This intentionally pulls in helpers whose binary name differs from the
// target — e.g. an externally-named crashpad_handler launched via plain
// CreateProcess by upstream Crashpad — and helpers that do not carry a
// Chromium --type=<role> flag. The OS-level parent / child relationship is
// the source of truth; the binary name is just a seed selector and the
// --type= flag is a labelling hint, not an inclusion filter.
//
// An empty (whitespace-only) target returns the full PID set; the gopsutil
// provider relies on that path when DiscoverApps asks for every process on
// the host.
func selectSubtreePIDs(refs []procRef, target string) map[int32]struct{} {
	target = strings.TrimSpace(target)
	keep := make(map[int32]struct{}, len(refs))
	if target == "" {
		for _, r := range refs {
			keep[r.PID] = struct{}{}
		}
		return keep
	}

	want := NormalizeTargetName(target)
	children := make(map[int32][]int32, len(refs))
	for _, r := range refs {
		children[r.PPID] = append(children[r.PPID], r.PID)
	}

	var queue []int32
	for _, r := range refs {
		if NormalizeTargetName(r.Name) != want {
			continue
		}
		if _, dup := keep[r.PID]; dup {
			continue
		}
		keep[r.PID] = struct{}{}
		queue = append(queue, r.PID)
	}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, child := range children[pid] {
			if _, seen := keep[child]; seen {
				continue
			}
			keep[child] = struct{}{}
			queue = append(queue, child)
		}
	}
	return keep
}
