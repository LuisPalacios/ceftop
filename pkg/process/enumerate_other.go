//go:build !windows

package process

import (
	"fmt"

	psprocess "github.com/shirou/gopsutil/v4/process"
)

// Linux reads /proc/<pid>/{stat,status} and macOS issues one sysctl per
// process for Name() / Ppid(); both are O(1) per PID, so gopsutil's own
// accessors are fine here. Thread counts are left to the enrichment pass.
const enumerationReportsThreads = false

// enumerateLite lists every process with PID, PPID and basename via
// gopsutil. Processes whose name cannot be read (gone between listing and
// lookup, or permission denied) are skipped — a transient failure must not
// corrupt the tree.
func enumerateLite() ([]liteProc, error) {
	procs, err := psprocess.Processes()
	if err != nil {
		return nil, fmt.Errorf("enumerating processes: %w", err)
	}
	out := make([]liteProc, 0, len(procs))
	for _, pr := range procs {
		name, err := pr.Name()
		if err != nil {
			continue
		}
		ppid, _ := pr.Ppid()
		out = append(out, liteProc{procRef: procRef{PID: pr.Pid, PPID: ppid, Name: name}})
	}
	return out, nil
}
