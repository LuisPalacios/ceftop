package catalog

import (
	"path"
	"strings"

	"github.com/LuisPalacios/ceftop/pkg/process"
)

// HostName returns the label for host when it embeds runtime, e.g.
// HostName("msedgewebview2.exe", "WhatsApp.Root.exe", pid) → "WhatsApp".
// Resolution order:
//
//  1. runtime's hosts list, exact key;
//  2. runtime's hosts list, the most specific matching glob pattern;
//  3. FriendlyName(host, hostPID): the catalog entry for the host, the OS's
//     description of its executable, the heuristic.
//
// So the list only ever refines names; any host it does not mention is
// still named, from the process itself.
func (c *Catalog) HostName(runtime, host string, hostPID int32) string {
	if label, ok := c.hostListName(runtime, host); ok {
		return label
	}
	return c.FriendlyName(host, hostPID)
}

func (c *Catalog) hostListName(runtime, host string) (string, bool) {
	e, ok := c.Lookup(runtime)
	if !ok || len(e.Hosts) == 0 {
		return "", false
	}
	h := process.NormalizeTargetName(host)
	if label, ok := e.Hosts[h]; ok {
		return label, true
	}
	best, bestLabel := "", ""
	for pattern, label := range e.Hosts {
		if !isGlob(pattern) {
			continue
		}
		if ok, err := path.Match(pattern, h); err != nil || !ok {
			continue
		}
		// Most specific wins: the longer pattern, then lexical order so
		// the choice never depends on map iteration.
		if len(pattern) > len(best) || (len(pattern) == len(best) && pattern < best) {
			best, bestLabel = pattern, label
		}
	}
	return bestLabel, best != ""
}

func isGlob(s string) bool {
	return strings.ContainsAny(s, "*?[")
}
