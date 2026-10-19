// Frontend DTOs that mirror the Go structs in pkg/process and pkg/config.
//
// We define these by hand instead of importing from wailsjs/go/models.ts
// because the auto-generated models include constructor wrapping logic that
// fights with reactive Svelte stores (the wrapper rebuilds nested objects
// every time a snapshot arrives, which would invalidate identity-based
// stable keys in {#each} blocks). Keeping a thin local type also stabilizes
// the surface against future Wails generator output changes.

export interface ConfigState {
	appName: string;
	path: string;
	loadError: string;
	tickIntervalSeconds: number;
	// The user's app catalog (apps.json next to ceftop.json; may not exist
	// yet) and the last error reading it, if any.
	catalogPath: string;
	catalogError: string;
}

export interface ProcessNode {
	pid: number;
	ppid: number;
	role: string;
	name: string;
	threads: number;
	memMB: number;
	cpuPercent: number; // 0..100, share of total CPU capacity over the last tick
	cpuTimeMs: number; // accumulated CPU time since process start
	// Set only on the main process of an embedded runtime group (e.g. one
	// Microsoft Edge WebView2 group): the app that hosts it, discovered by
	// the backend. host is the exe name, hostLabel its display name.
	host?: string;
	hostPid?: number;
	hostLabel?: string;
	children?: ProcessNode[];
}

export interface ProcessSnapshot {
	target: string;
	roots: ProcessNode[] | null;
	captured: string; // RFC3339 string from time.Time
	total: number;
}

export interface KillResult {
	killed: boolean;
	err?: string;
	errno?: string;
}

export interface DiscoveredApp {
	name: string;
	// Backend-resolved display name (app catalog, then the OS's description
	// of the executable, then a heuristic). Never empty.
	friendlyName: string;
	childCount: number;
	// Backend-resolved <img src>: a bundled /app-icons/ URL or a private
	// data URI. Never empty — the backend falls back to the default icon.
	iconSrc: string;
	// Embedded runtimes only (Microsoft Edge WebView2): the apps hosting
	// one of its process groups right now.
	hosts?: HostView[];
}

export interface HostView {
	exe: string;
	label: string;
}

// How the backend presents one app: display name and icon, both resolved
// server-side (see DescribeApp in app.go).
export interface AppInfo {
	name: string;
	friendlyName: string;
	iconSrc: string;
}
