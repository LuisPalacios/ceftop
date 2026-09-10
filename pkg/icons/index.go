package icons

import (
	"io/fs"
	"path"
	"strings"

	"github.com/LuisPalacios/ceftop/pkg/config"
)

// DefaultKey is the reserved icon key used as the fallback when no candidate
// matches. It never takes part in matching, so a process that happens to be
// called "default" still resolves through the normal rules.
const DefaultKey = config.PrivateIconDefaultKey

// BundledURLPrefix is where the frontend serves the bundled icon set from.
// The sync script copies assets/app-*.svg into public/app-icons/ and Vite
// ships that directory verbatim in dist/, which main.go embeds.
const BundledURLPrefix = "/app-icons/"

// entry is one candidate icon: the key parsed from "app-<key>.svg" and the
// src the frontend can drop straight into <img src=...> — a URL for bundled
// icons, a base64 data URI for private ones.
type entry struct {
	key     string
	src     string
	private bool
}

// Index holds every icon candidate known at one point in time and answers
// "which icon for this process name?". Build a fresh one whenever the private
// directory may have changed; construction is cheap (one ReadDir on each
// source) and the App rebuilds it on every discovery tick.
type Index struct {
	entries []entry
	private map[string]string
}

// Load builds an Index from the bundled icon directory (any fs.FS whose root
// contains app-*.svg files — nil is allowed and simply contributes nothing)
// and from the private icons next to the user's config JSON (an empty
// privateDir skips that source). I/O errors from the private directory are
// returned; a missing private directory is not an error.
func Load(bundled fs.FS, privateDir string) (*Index, error) {
	idx := &Index{private: map[string]string{}}

	if bundled != nil {
		for _, key := range bundledKeys(bundled) {
			idx.entries = append(idx.entries, entry{
				key: key,
				src: BundledURLPrefix + config.PrivateIconPrefix + key + config.PrivateIconExt,
			})
		}
	}

	if privateDir != "" {
		private, err := config.LoadPrivateIcons(privateDir)
		if err != nil {
			return nil, err
		}
		idx.private = private
		for _, key := range sortedKeys(private) {
			idx.entries = append(idx.entries, entry{key: key, src: private[key], private: true})
		}
	}
	return idx, nil
}

// bundledKeys lists the <key> of every app-<key>.svg at the root of fsys.
// Unreadable directories yield nothing: a dev build without a synced icon
// directory should degrade to the default icon, not fail startup.
func bundledKeys(fsys fs.FS) []string {
	dirEntries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}
	var keys []string
	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}
		name := path.Base(e.Name())
		if !strings.HasPrefix(name, config.PrivateIconPrefix) || !strings.HasSuffix(name, config.PrivateIconExt) {
			continue
		}
		key := strings.TrimSuffix(strings.TrimPrefix(name, config.PrivateIconPrefix), config.PrivateIconExt)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

// Keys returns every matchable icon key (bundled and private, minus the
// reserved default) in a stable order. Handy for diagnostics and tests.
func (x *Index) Keys() []string {
	out := make([]string, 0, len(x.entries))
	for _, e := range x.entries {
		if e.key == DefaultKey {
			continue
		}
		out = append(out, e.key)
	}
	return out
}

// PrivateKeys returns the keys of user-supplied icons, minus the reserved
// default, in lexical order. The App uses them to surface offline apps in the
// discovery bar.
func (x *Index) PrivateKeys() []string {
	out := make([]string, 0, len(x.private))
	for _, k := range sortedKeys(x.private) {
		if k == DefaultKey {
			continue
		}
		out = append(out, k)
	}
	return out
}

// DefaultSrc is the fallback icon: the user's private app-default.svg when
// present, otherwise the bundled one.
func (x *Index) DefaultSrc() string {
	if src, ok := x.private[DefaultKey]; ok {
		return src
	}
	return BundledURLPrefix + config.PrivateIconPrefix + DefaultKey + config.PrivateIconExt
}

// Resolve returns the src of the best-matching icon for name, or DefaultSrc
// when nothing scores at least MinScore. When a bundled and a private icon
// tie, the private one wins — a user who ships app-chrome.svg next to the
// config JSON expects it to replace the bundled Chrome logo.
func (x *Index) Resolve(name string) string {
	src, _, ok := x.Match(name)
	if !ok {
		return x.DefaultSrc()
	}
	return src
}

// Match is Resolve with the details exposed: the winning src, its key, and
// whether anything matched at all.
func (x *Index) Match(name string) (src, key string, ok bool) {
	if strings.TrimSpace(name) == "" {
		return "", "", false
	}
	best := -1
	var win entry
	for _, e := range x.entries {
		if e.key == DefaultKey {
			continue
		}
		s := Score(name, e.key)
		if s < MinScore {
			continue
		}
		switch {
		case s > best:
			best, win = s, e
		case s == best && e.private && !win.private:
			win = e
		case s == best && e.private == win.private && betterKey(e.key, win.key):
			win = e
		}
	}
	if best < 0 {
		return "", "", false
	}
	return win.src, win.key, true
}
