// roleTheme — single source of color truth for Chromium process roles.
//
// Mappings track $RoleColors in the validated PowerShell prototype; the
// hex values translate the prototype's PowerShell ConsoleColor names into
// dark-theme-friendly CSS while preserving the same hue ordering so a user
// who learned the CLI palette can read the GUI without retraining.
//
//   PowerShell name  → CSS hex (intent)
//   Green            → bright green   (Main / Browser is the parent)
//   DarkYellow       → mustard        (renderers — most numerous)
//   Magenta          → pink-magenta   (gpu-process — visible from across the room)
//   DarkCyan         → teal           (utility sandboxes)
//   Red              → bright red     (crashpad-handler — alarming on purpose)
//   DarkRed          → muted red      (watcher)
//   Cyan             → cyan           (plugin / ppapi family)
//   Yellow           → bright yellow  (extension)
//   DarkMagenta      → purple         (zygote, mostly Linux)

import type { ProcessNode } from "./types";

export const ROLE_MAIN = "Main / Browser";

export const ROLE_DEFAULT = "default";

const palette: Record<string, string> = {
	[ROLE_MAIN]: "#34d399",
	"renderer": "#f59e0b",
	"gpu-process": "#ec4899",
	"utility": "#0d9488",
	"crashpad-handler": "#ef4444",
	"watcher": "#9f1239",
	"plugin": "#22d3ee",
	"ppapi": "#22d3ee",
	"ppapi-broker": "#22d3ee",
	"extension": "#facc15",
	"zygote": "#a855f7",
	[ROLE_DEFAULT]: "#94a3b8",
};

export function colorForRole(role: string): string {
	return palette[role] ?? palette[ROLE_DEFAULT];
}

export function colorForNode(node: ProcessNode): string {
	return colorForRole(node.role);
}

// Stable ordering for the role legend in the status bar. Roles not in this
// list fall to the end alphabetically.
export const ROLE_LEGEND_ORDER = [
	ROLE_MAIN,
	"renderer",
	"gpu-process",
	"utility",
	"crashpad-handler",
	"watcher",
	"extension",
	"plugin",
	"ppapi",
	"ppapi-broker",
	"zygote",
];
