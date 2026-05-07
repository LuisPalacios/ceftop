package process

import "sort"

// DiscoveredApp is a CEF / Chromium / Electron application detected on the
// host. Name is the normalized executable basename (matches what
// SetTargetApp expects); ChildCount is the number of `--type=` children
// rolled up under it in the latest scan.
type DiscoveredApp struct {
	Name       string `json:"name"`
	ChildCount int    `json:"childCount"`
}

// DiscoverApps scans every process on the host, finds the ones carrying a
// `--type=<role>` Chromium flag, walks PPID up to the first ancestor without
// that flag, and groups those ancestors by normalized exe basename.
//
// The result is one DiscoveredApp per distinct browser process tree currently
// running — so a host with Chrome, VS Code and a personal CEF app open
// returns three entries. Children whose ancestry can't be resolved (orphan,
// re-parented, or PPID cycle) are silently skipped: discovery is best-effort
// and a transient resolution failure must not corrupt the list.
func DiscoverApps(provider Provider) ([]DiscoveredApp, error) {
	raws, err := provider.Snapshot("")
	if err != nil {
		return nil, err
	}

	index := make(map[int32]RawProcess, len(raws))
	for _, r := range raws {
		index[r.PID] = r
	}

	counts := make(map[string]int)
	for _, r := range raws {
		if !typeFlagRe.MatchString(r.Cmdline) {
			continue
		}
		root, ok := walkToBrowser(r, index)
		if !ok {
			continue
		}
		name := NormalizeTargetName(root.Name)
		if name == "" {
			continue
		}
		counts[name]++
	}

	out := make([]DiscoveredApp, 0, len(counts))
	for name, n := range counts {
		out = append(out, DiscoveredApp{Name: name, ChildCount: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// walkToBrowser climbs the PPID chain from a CEF child until it lands on a
// process whose cmdline has no `--type=` flag — that's the Main / Browser.
// Returns ok=false if the chain dead-ends (PPID missing from snapshot) or
// loops on itself before finding a browser. The depth cap is a defense
// against pathological cycles; real Chromium trees are 1-2 hops deep.
func walkToBrowser(start RawProcess, index map[int32]RawProcess) (RawProcess, bool) {
	const maxHops = 32
	cur := start
	for hop := 0; hop < maxHops; hop++ {
		parent, ok := index[cur.PPID]
		if !ok {
			return RawProcess{}, false
		}
		if parent.PID == cur.PID {
			return RawProcess{}, false
		}
		if !typeFlagRe.MatchString(parent.Cmdline) {
			return parent, true
		}
		cur = parent
	}
	return RawProcess{}, false
}
