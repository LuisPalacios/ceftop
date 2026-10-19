package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LuisPalacios/ceftop/pkg/process"
)

// The user's apps.json is meant to be edited by hand, so the writers below
// change only what they were asked to and keep everything else the user put
// there, including fields this version of CefTop does not know. Key order
// is not preserved (Go sorts object keys on output); content is.

// SetHostName records how host should read inside runtime's tree, in the
// user's apps.json at userPath: apps[runtime].hosts[host] = label. An empty
// label removes the line, so the name falls back to the embedded catalog or
// to discovery, and empty objects left behind are pruned. The file is
// created when missing. A file that is not valid JSON is never overwritten:
// the error asks the user to fix it first.
func SetHostName(userPath, runtime, host, label string) error {
	rt := process.NormalizeTargetName(runtime)
	hk := process.NormalizeTargetName(host)
	if rt == "" || hk == "" {
		return fmt.Errorf("runtime and host names cannot be empty")
	}
	label = strings.TrimSpace(label)

	doc, err := readDoc(userPath)
	if err != nil {
		return err
	}
	apps, err := object(doc, "apps", label != "")
	if err != nil || apps == nil {
		return err
	}
	rtKey := keyFor(apps, rt)
	entry, err := object(apps, rtKey, label != "")
	if err != nil || entry == nil {
		return err
	}
	hosts, err := object(entry, "hosts", label != "")
	if err != nil || hosts == nil {
		return err
	}
	hostKey := keyFor(hosts, hk)

	if label != "" {
		hosts[hostKey] = label
	} else {
		delete(hosts, hostKey)
		if len(hosts) == 0 {
			delete(entry, "hosts")
		}
		if len(entry) == 0 {
			delete(apps, rtKey)
		}
	}
	return writeDoc(userPath, doc)
}

// CreateUserFile writes a starter apps.json at userPath listing the hosts
// seen right now, runtime → host → current label, so the user opens a file
// that already names every app on screen and only has to edit labels. It
// does nothing when the file exists (created reports which). Runtimes with
// no hosts are left out.
func CreateUserFile(userPath string, hosts map[string]map[string]string) (created bool, err error) {
	if _, err := os.Stat(userPath); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	apps := map[string]any{}
	for rt, hs := range hosts {
		if len(hs) == 0 {
			continue
		}
		list := map[string]any{}
		for h, label := range hs {
			list[process.NormalizeTargetName(h)] = label
		}
		apps[process.NormalizeTargetName(rt)] = map[string]any{"hosts": list}
	}
	doc := map[string]any{"schemaVersion": 1, "apps": apps}
	if err := writeDoc(userPath, doc); err != nil {
		return false, err
	}
	return true, nil
}

func readDoc(userPath string) (map[string]any, error) {
	data, err := os.ReadFile(userPath)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{"schemaVersion": 1}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON; fix it before saving names from CefTop: %w", userPath, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	if _, ok := doc["schemaVersion"]; !ok {
		doc["schemaVersion"] = 1
	}
	return doc, nil
}

// object returns parent[key] as a JSON object, creating it when create is
// set. A missing key without create returns nil, nil (nothing to remove).
func object(parent map[string]any, key string, create bool) (map[string]any, error) {
	v, ok := parent[key]
	if !ok {
		if !create {
			return nil, nil
		}
		m := map[string]any{}
		parent[key] = m
		return m, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%q in apps.json must be an object", key)
	}
	return m, nil
}

// keyFor returns the existing key in m that names want once normalized
// ("MSEdgeWebView2.exe" for "msedgewebview2"), or want itself. Reusing the
// user's spelling avoids two keys for the same app, whose merge order would
// then be undefined.
func keyFor(m map[string]any, want string) string {
	if _, ok := m[want]; ok {
		return want
	}
	for k := range m {
		if process.NormalizeTargetName(k) == want {
			return k
		}
	}
	return want
}

// writeDoc writes atomically (temp file + rename) so a crash mid-write can
// never leave a truncated apps.json behind.
func writeDoc(userPath string, doc map[string]any) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		return err
	}
	tmp := userPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, userPath); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
