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
	cmdline?: string;
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
	childCount: number;
}
