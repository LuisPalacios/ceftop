package catalog

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/LuisPalacios/ceftop/pkg/process"
)

// FriendlyName returns the label to show for an executable. Resolution order:
//
//  1. The catalog entry's friendlyName (user override, then embedded).
//  2. The OS's own description of the running executable, when pid > 0 and
//     the platform has one (the version resource's FileDescription /
//     ProductName on Windows). This is what makes unknown apps, such as a
//     new WebView2 host, read well without a catalog entry.
//  3. A heuristic on the basename: split on ".", "-" and "_", drop generic
//     leading modifiers ("my", "super", "the") and trailing categories
//     ("browser", "app", "helper", "client"), capitalize each part.
//
// The result is never empty for a non-empty name.
func (c *Catalog) FriendlyName(name string, pid int32) string {
	if e, ok := c.Lookup(name); ok && e.FriendlyName != "" {
		return e.FriendlyName
	}
	if pid > 0 {
		if d := describeProcess(pid, process.NormalizeTargetName(name)); d != "" {
			return d
		}
	}
	return Heuristic(name)
}

var (
	separatorRe      = regexp.MustCompile(`[.\-_]+`)
	genericPrefixes  = map[string]bool{"my": true, "super": true, "the": true}
	genericSuffixes  = map[string]bool{"browser": true, "app": true, "helper": true, "client": true}
	maxDescribedName = 40
)

// Heuristic derives a label from an executable name alone. Case is kept
// apart from the first letter of each part, so "SearchHost.exe" reads
// "SearchHost" while "my.super-program" reads "Program".
func Heuristic(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		name = name[:len(name)-len(".exe")]
	}
	var parts []string
	for _, p := range separatorRe.Split(name, -1) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	// Drop generic leading and trailing tokens, but never to empty.
	for len(parts) > 1 && genericPrefixes[strings.ToLower(parts[0])] {
		parts = parts[1:]
	}
	for len(parts) > 1 && genericSuffixes[strings.ToLower(parts[len(parts)-1])] {
		parts = parts[:len(parts)-1]
	}
	for i, p := range parts {
		parts[i] = capitalize(p)
	}
	return strings.Join(parts, " ")
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// usableDescription filters what the OS hands back. Rejected: blank, overly
// long (a sentence rather than a name), a bare file name ("Foo.exe"),
// generic framework names that describe the runtime instead of the app
// ("Electron", "Chromium"), and the product name Windows' own binaries
// share ("Microsoft® Windows® Operating System").
func usableDescription(d string) bool {
	d = strings.TrimSpace(d)
	if d == "" || utf8.RuneCountInString(d) > maxDescribedName {
		return false
	}
	lower := strings.ToLower(d)
	if strings.HasSuffix(lower, ".exe") || strings.Contains(lower, "operating system") {
		return false
	}
	switch lower {
	case "electron", "chromium", "cef", "chromium embedded framework":
		return false
	}
	return true
}
