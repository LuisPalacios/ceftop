package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PrivateIconPrefix is the filename prefix used for user-supplied SVG icons
// that live next to the config JSON. A file named "app-<name>.svg" in the
// config directory overrides the bundled icon for the same <name>.
const PrivateIconPrefix = "app-"

// PrivateIconExt is the extension we recognize for private icons. SVG only —
// the bundled icons are also SVG, and keeping a single format avoids the
// guesswork around mime-types in the data URI.
const PrivateIconExt = ".svg"

// PrivateIconDefaultKey is the reserved <name> used as the icon fallback by
// the frontend resolver. It must never surface as a selectable app.
const PrivateIconDefaultKey = "default"

// LoadPrivateIcons walks the directory that contains the config JSON and
// returns a map of <name> → data URI for every "app-<name>.svg" file found.
// Missing directories yield an empty map without an error: a fresh install
// has no private icons and that's fine. I/O errors on individual files are
// silently skipped so one unreadable icon does not poison the whole map.
func LoadPrivateIcons(configDir string) (map[string]string, error) {
	out := map[string]string{}
	if configDir == "" {
		return out, nil
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, PrivateIconPrefix) || !strings.HasSuffix(name, PrivateIconExt) {
			continue
		}
		key := strings.TrimSuffix(strings.TrimPrefix(name, PrivateIconPrefix), PrivateIconExt)
		if key == "" {
			continue
		}
		full := filepath.Join(configDir, name)
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		out[key] = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(data)
	}
	return out, nil
}

// PrivateIconNames returns the <name> portion of every "app-<name>.svg" file
// in configDir, sorted and de-duplicated. Used to surface user-declared apps
// in the discovery bar even when no matching process is currently running —
// dropping a file like "app-sumwall.browser.svg" alongside the config JSON
// is the canonical way to register a personal CEF app. The reserved
// "default" key (icon fallback) is excluded.
func PrivateIconNames(configDir string) ([]string, error) {
	if configDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, PrivateIconPrefix) || !strings.HasSuffix(name, PrivateIconExt) {
			continue
		}
		key := strings.TrimSuffix(strings.TrimPrefix(name, PrivateIconPrefix), PrivateIconExt)
		if key == "" || key == PrivateIconDefaultKey {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out, nil
}
