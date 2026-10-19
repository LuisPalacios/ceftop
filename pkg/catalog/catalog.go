// Package catalog is CefTop's knowledge about executables: the friendly name
// to show for each one and a few attributes that change how its process tree
// is read (for example, multiGroup runtimes such as Microsoft Edge WebView2,
// which other apps embed to draw their windows).
//
// The catalog is a refinement, never a requirement. Everything CefTop
// discovers works without an entry: unknown executables get their name from
// the OS (the exe's version resource on Windows) or, failing that, from a
// heuristic on the basename. See FriendlyName.
//
// Two sources are merged, key by key and field by field:
//
//   - apps.json embedded in the binary (this package), curated in the repo;
//   - an optional apps.json next to the user's ceftop.json, which adds or
//     overrides entries. It is never created automatically.
package catalog

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/LuisPalacios/ceftop/pkg/process"
)

// FileName is the name of the user override file, which lives in the same
// directory as ceftop.json.
const FileName = "apps.json"

//go:embed apps.json
var embeddedJSON []byte

// Entry is everything the catalog knows about one executable. Only
// FriendlyName is meaningful for every entry; the rest is optional.
type Entry struct {
	// FriendlyName is the label shown in the UI instead of the exe name.
	FriendlyName string `json:"friendlyName"`
	// MultiGroup marks a runtime that other apps launch to draw their own
	// window, so one executable runs several independent process groups,
	// one per host app. CefTop labels each group's main process with its
	// host. Runtimes that announce their host on the command line (WebView2
	// does) are detected without this flag; it is the hint for the rest.
	MultiGroup bool `json:"multiGroup,omitempty"`
	// Icon is an explicit icon key (app-<icon>.svg) that bypasses fuzzy
	// icon matching.
	Icon string `json:"icon,omitempty"`
	// Vendor, Kind and Description are informational. Kind is one of
	// browser, electron, cef, runtime or app.
	Vendor      string `json:"vendor,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Description string `json:"description,omitempty"`
	// Hosts names the apps that embed this runtime, as they should read
	// inside its tree ("Main / WhatsApp"). Keys are host executable names,
	// normalized like every other key, and may be glob patterns
	// ("whatsapp*"); an exact key beats a pattern and a longer pattern beats
	// a shorter one. This is a naming list, not a filter: hosts missing
	// from it are still discovered and named by FriendlyName. See HostName.
	Hosts map[string]string `json:"hosts,omitempty"`
}

// rawEntry is the on-disk shape. Pointer fields tell "absent" apart from
// "set to the zero value", so an override can switch multiGroup off or blank
// a field without restating the rest of the entry.
type rawEntry struct {
	FriendlyName *string `json:"friendlyName"`
	MultiGroup   *bool   `json:"multiGroup"`
	Icon         *string `json:"icon"`
	Vendor       *string `json:"vendor"`
	Kind         *string `json:"kind"`
	Description  *string `json:"description"`
	// Hosts merges key by key; an empty value in the override removes the
	// embedded name for that host.
	Hosts map[string]string `json:"hosts"`
}

type rawFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Apps          map[string]rawEntry `json:"apps"`
}

// Catalog is an immutable, merged view of both sources. Build a fresh one
// whenever the override file may have changed; Load is cheap.
type Catalog struct {
	apps map[string]Entry
}

// Load returns the embedded catalog merged with the override at userPath.
// An empty userPath or a missing file means "no override". A file that
// cannot be read or parsed is reported through the error, but the returned
// Catalog is still usable (embedded entries only): a typo in the user's
// apps.json must never stop CefTop from starting.
func Load(userPath string) (*Catalog, error) {
	c := &Catalog{apps: map[string]Entry{}}
	if err := c.merge(embeddedJSON); err != nil {
		return c, fmt.Errorf("embedded catalog: %w", err)
	}
	if userPath == "" {
		return c, nil
	}
	data, err := os.ReadFile(userPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil
		}
		return c, err
	}
	// Parse into a scratch copy so a half-applied override cannot leak.
	next := c.clone()
	if err := next.merge(data); err != nil {
		return c, fmt.Errorf("%s: %w", userPath, err)
	}
	return next, nil
}

func (c *Catalog) clone() *Catalog {
	return &Catalog{apps: maps.Clone(c.apps)}
}

// merge applies one JSON document on top of the current entries. Keys are
// normalized like process names, so "SearchHost.exe" and "searchhost" name
// the same entry.
func (c *Catalog) merge(data []byte) error {
	var f rawFile
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	for key, raw := range f.Apps {
		k := process.NormalizeTargetName(key)
		if k == "" {
			continue
		}
		e := c.apps[k]
		if raw.FriendlyName != nil {
			e.FriendlyName = *raw.FriendlyName
		}
		if raw.MultiGroup != nil {
			e.MultiGroup = *raw.MultiGroup
		}
		if raw.Icon != nil {
			e.Icon = *raw.Icon
		}
		if raw.Vendor != nil {
			e.Vendor = *raw.Vendor
		}
		if raw.Kind != nil {
			e.Kind = *raw.Kind
		}
		if raw.Description != nil {
			e.Description = *raw.Description
		}
		if raw.Hosts != nil {
			// Copy before writing: clone() shares the inner map with the
			// catalog this one was cloned from.
			hosts := maps.Clone(e.Hosts)
			if hosts == nil {
				hosts = make(map[string]string, len(raw.Hosts))
			}
			for hk, label := range raw.Hosts {
				hk = process.NormalizeTargetName(hk)
				label = strings.TrimSpace(label)
				if hk == "" {
					continue
				}
				if label == "" {
					delete(hosts, hk)
					continue
				}
				hosts[hk] = label
			}
			e.Hosts = hosts
		}
		c.apps[k] = e
	}
	return nil
}

// Lookup returns the entry for an executable name, given in any form a
// process name arrives in ("SearchHost.exe", a full path, "searchhost").
func (c *Catalog) Lookup(name string) (Entry, bool) {
	if c == nil {
		return Entry{}, false
	}
	e, ok := c.apps[process.NormalizeTargetName(name)]
	return e, ok
}

// IsMultiGroup reports whether the catalog marks name as a multiGroup
// runtime. It has the signature process.Monitor.SetMultiGroup expects.
func (c *Catalog) IsMultiGroup(name string) bool {
	e, ok := c.Lookup(name)
	return ok && e.MultiGroup
}

// Keys returns every catalog key in lexical order. Handy for tests and
// diagnostics.
func (c *Catalog) Keys() []string {
	return slices.Sorted(maps.Keys(c.apps))
}
