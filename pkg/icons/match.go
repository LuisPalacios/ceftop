// Package icons resolves which app icon to show for a given process name.
//
// Icon files are named "app-<key>.svg" and live either bundled with the
// frontend (served at /app-icons/) or privately next to the user's config
// JSON. The <key> is authored by a human and the process name comes from
// the OS, so the two rarely agree byte-for-byte: "Docker Desktop.exe" vs
// "app-docker-desktop.svg", "Google Chrome" vs "app-chrome.svg", or a
// user-supplied "app-Docker Desktop for Mac.svg". This package scores the
// similarity between a process name and every candidate key and picks the
// best one, so the file name only has to be *recognizably* the same app.
//
// The matcher is deliberately deterministic and rule-based (no fuzzy edit
// distance): every rule is explainable in one sentence and testable with a
// table. Rules are tried from strictest to loosest and the first hit wins:
//
//	100  identical after basic normalization (lowercase, .exe stripped)
//	 95  identical once every separator is removed ("docker-desktop" ==
//	     "Docker Desktop" == "dockerdesktop")
//	 85  identical once noise tokens are removed as well ("docker desktop
//	     for mac" == "win docker desktop" == "docker desktop")
//	50..80 one side's meaningful tokens are all present in the other
//	     ("chrome" ⊂ "google chrome"); scaled by how much of the longer
//	     side is covered, so a two-token match beats a one-token match
//	 40  one side's joined tokens are a prefix of the other's (>= 4 chars),
//	     which catches glued-together names like "msedge" / "msedgewebview2"
//	  0  no relation
package icons

import (
	"sort"
	"strings"
	"unicode"
)

// MinScore is the lowest Score that still counts as a match. Anything below
// it is treated as "unrelated" by BestMatch and Index.Resolve.
const MinScore = 40

// noiseTokens are words that describe the platform, the packaging, or plain
// English glue rather than the application. They are stripped from BOTH
// sides before the looser comparisons so "Docker Desktop for Mac" and
// "Docker Desktop" collapse to the same tokens. Keep this list tight: a word
// that is part of a real product name ("desktop" in Docker Desktop, "browser"
// in Brave Browser) must not be here — the containment rule handles those.
var noiseTokens = map[string]struct{}{
	// platform / OS
	"win": {}, "win32": {}, "win64": {}, "windows": {},
	"mac": {}, "macos": {}, "osx": {}, "darwin": {}, "apple": {},
	"linux": {}, "unix": {},
	// architecture
	"x64": {}, "x86": {}, "amd64": {}, "arm64": {}, "aarch64": {}, "ia32": {}, "i386": {},
	// packaging / executable suffixes that leak into names
	"exe": {}, "bin": {}, "app": {},
	// English glue
	"for": {}, "the": {}, "a": {}, "an": {}, "of": {}, "and": {}, "by": {}, "on": {},
}

// normalize lowercases a name, strips any directory prefix and a trailing
// ".exe", and trims whitespace. It is the byte-exact comparison baseline.
func normalize(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.TrimSuffix(name, ".exe")
}

// Tokens splits a name into lowercase alphanumeric runs. Every other
// character (space, hyphen, underscore, dot, parentheses, ...) is a
// separator. "Docker-Desktop for Mac (x64).exe" → [docker desktop for mac
// x64 exe].
func Tokens(name string) []string {
	name = strings.ToLower(name)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// meaningful drops noise tokens. If that would leave nothing (the name was
// entirely noise, e.g. a process literally called "app"), the original
// tokens are returned so the name still has an identity.
func meaningful(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, noise := noiseTokens[t]; noise {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return tokens
	}
	return out
}

// Canonical returns the identity a name collapses to once separators and
// noise are removed: "Docker Desktop for Mac.exe" → "dockerdesktop". Two
// names with the same Canonical form score at least 85 against each other.
func Canonical(name string) string {
	return strings.Join(meaningful(Tokens(name)), "")
}

// Score rates how strongly two names refer to the same application, from 0
// (unrelated) to 100 (identical). It is symmetric: Score(a, b) == Score(b, a).
func Score(a, b string) int {
	na, nb := normalize(a), normalize(b)
	if na == "" || nb == "" {
		return 0
	}
	if na == nb {
		return 100
	}

	ta, tb := Tokens(na), Tokens(nb)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	if strings.Join(ta, "") == strings.Join(tb, "") {
		return 95
	}

	ma, mb := meaningful(ta), meaningful(tb)
	ja, jb := strings.Join(ma, ""), strings.Join(mb, "")
	if ja == jb {
		return 85
	}

	// Containment: every meaningful token of the shorter side appears in
	// the longer side. Order does not matter ("desktop docker" still finds
	// "docker desktop"). Coverage scales the score so that matching 2 of 3
	// tokens beats matching 1 of 3.
	small, large := ma, mb
	if len(small) > len(large) {
		small, large = large, small
	}
	if containsAll(large, small) {
		return 50 + (30*len(small))/len(large)
	}

	// Glued prefix: "msedge" vs "msedgewebview2". Require a few characters
	// so that a lone "a" or "go" never latches onto everything.
	short, long := ja, jb
	if len(short) > len(long) {
		short, long = long, short
	}
	if len(short) >= 4 && strings.HasPrefix(long, short) {
		return MinScore
	}

	return 0
}

func containsAll(haystack, needles []string) bool {
	set := make(map[string]struct{}, len(haystack))
	for _, h := range haystack {
		set[h] = struct{}{}
	}
	for _, n := range needles {
		if _, ok := set[n]; !ok {
			return false
		}
	}
	return true
}

// Matches reports whether two names refer to the same application, i.e.
// Score(a, b) >= MinScore.
func Matches(a, b string) bool {
	return Score(a, b) >= MinScore
}

// BestMatch picks the candidate key that best matches name. Ties are broken
// by preferring the longer key (more specific) and then lexical order, so the
// result is stable across runs. ok is false when nothing reaches MinScore.
func BestMatch(name string, keys []string) (key string, score int, ok bool) {
	best := -1
	for _, k := range keys {
		s := Score(name, k)
		if s < MinScore {
			continue
		}
		if s > best || (s == best && betterKey(k, key)) {
			best, key = s, k
		}
	}
	if best < 0 {
		return "", 0, false
	}
	return key, best, true
}

func betterKey(candidate, current string) bool {
	if len(candidate) != len(current) {
		return len(candidate) > len(current)
	}
	return candidate < current
}

// sortedKeys returns the map's keys in lexical order so BestMatch sees a
// deterministic candidate sequence regardless of map iteration.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
