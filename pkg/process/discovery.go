package process

import "sort"

// DiscoveredApp is a CEF / Chromium / Electron application detected on the
// host. Name is the normalized executable basename (matches what
// SetTargetApp expects); ChildCount is the number of `--type=` children
// rolled up under it in the latest scan. PID is one of its main processes
// (the lowest, so it is stable across scans), handy for asking the OS about
// the executable; a multiGroup runtime has several and any of them will do.
//
// Hosts lists, for an embedded runtime such as Microsoft Edge WebView2, the
// apps currently hosting one of its process groups (see resolveHost),
// sorted by exe name and de-duplicated; it is empty for ordinary apps.
type DiscoveredApp struct {
	Name       string    `json:"name"`
	ChildCount int       `json:"childCount"`
	PID        int32     `json:"pid"`
	Hosts      []HostRef `json:"hosts,omitempty"`
}

// HostRef is one app hosting an embedded runtime: its exe name as the OS
// reports it, and its PID (0 when known by name only).
type HostRef struct {
	Exe string `json:"exe"`
	PID int32  `json:"pid"`
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
//
// isMultiGroup is the same optional catalog hint Monitor.SetMultiGroup takes;
// it only affects Hosts.
func DiscoverApps(provider Provider, isMultiGroup func(string) bool) ([]DiscoveredApp, error) {
	raws, err := provider.Snapshot("")
	if err != nil {
		return nil, err
	}

	index := make(map[int32]RawProcess, len(raws))
	for _, r := range raws {
		index[r.PID] = r
	}

	counts := make(map[string]int)
	pids := make(map[string]int32)
	hosts := make(map[string]map[string]HostRef) // app → normalized host exe → ref
	seenRoots := make(map[int32]bool)
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
		if p, ok := pids[name]; !ok || root.PID < p {
			pids[name] = root.PID
		}
		if seenRoots[root.PID] {
			continue
		}
		seenRoots[root.PID] = true
		if exe, hostPID := resolveHost(root, isMultiGroup); exe != "" {
			if hosts[name] == nil {
				hosts[name] = map[string]HostRef{}
			}
			hosts[name][NormalizeTargetName(exe)] = HostRef{Exe: exe, PID: hostPID}
		}
	}

	out := make([]DiscoveredApp, 0, len(counts))
	for name, n := range counts {
		app := DiscoveredApp{Name: name, ChildCount: n, PID: pids[name]}
		for _, h := range hosts[name] {
			app.Hosts = append(app.Hosts, h)
		}
		sort.Slice(app.Hosts, func(i, j int) bool {
			return NormalizeTargetName(app.Hosts[i].Exe) < NormalizeTargetName(app.Hosts[j].Exe)
		})
		out = append(out, app)
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
