package catalog

import (
	"strings"
	"sync"
)

// describeCacheMax bounds the cache. Entries are keyed by PID and name, so a
// long session with a lot of process churn would otherwise grow it forever;
// dropping everything at the bound is simpler than an LRU and costs one
// re-read per live host.
const describeCacheMax = 256

type describeKey struct {
	pid  int32
	name string
}

var (
	describeMu    sync.Mutex
	describeCache = map[describeKey]string{}
)

// describeProcess returns the OS's description of the executable running as
// pid, or "" when the platform has none or it is not usable as a label. The
// lookup opens the process and reads its image file, so results (including
// misses) are cached per PID + normalized name: a reused PID running a
// different binary gets a fresh lookup.
func describeProcess(pid int32, name string) string {
	key := describeKey{pid: pid, name: name}
	describeMu.Lock()
	if d, ok := describeCache[key]; ok {
		describeMu.Unlock()
		return d
	}
	describeMu.Unlock()

	d := strings.TrimSpace(exeDescription(pid))
	if !usableDescription(d) {
		d = ""
	}

	describeMu.Lock()
	if len(describeCache) >= describeCacheMax {
		describeCache = map[describeKey]string{}
	}
	describeCache[key] = d
	describeMu.Unlock()
	return d
}
