// friendlyName — display label for a CEF / Chromium target.
//
// Resolution order:
//   1. KNOWN: hand-curated map of common executable names → short labels.
//   2. Heuristic fallback: split on `.`, `-`, `_`; drop generic leading
//      modifiers ("my", "super", ...) and generic trailing categories
//      ("browser", "app", ...); title-case what's left.
//
// Examples (heuristic):
//   "program.browser"   → "Program"
//   "my.super-program"  → "Program"
//   "browser.cool"      → "Browser Cool"
//   "foo.bar"           → "Foo Bar"
//
// The lookup is case-insensitive on input; the canonical target name (as
// stored in config and shown in the StatusBar editor) is left unchanged —
// only the visible label below the icon (DiscoveredAppsBar) and to the
// right of the icon in the ProcessTree header gets the friendly form.

const KNOWN: Record<string, string> = {
	adobecollabsync: "Adobe Sync",
	msedgewebview2: "MS Edge",
	slack: "Slack",
	code: "VSCode",
	chrome: "Chrome",
};

const GENERIC_PREFIXES = new Set(["my", "super", "the"]);
const GENERIC_SUFFIXES = new Set(["browser", "app", "helper", "client"]);

function titleCase(s: string): string {
	if (!s) return s;
	return s[0].toUpperCase() + s.slice(1);
}

export function friendlyName(target: string): string {
	if (!target) return "";
	const key = target.toLowerCase();
	if (KNOWN[key]) return KNOWN[key];

	const parts = key.split(/[.\-_]+/).filter(Boolean);
	if (parts.length === 0) return "";

	// Drop generic leading tokens, but never to empty.
	while (parts.length > 1 && GENERIC_PREFIXES.has(parts[0])) {
		parts.shift();
	}
	// Drop generic trailing tokens, but never to empty.
	while (parts.length > 1 && GENERIC_SUFFIXES.has(parts[parts.length - 1])) {
		parts.pop();
	}

	return parts.map(titleCase).join(" ");
}
