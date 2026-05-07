// Svelte stores backing the monitor view. The snapshot store is a hard
// replace on each tick — never mutated in place — so reactive blocks
// re-render the whole tree atomically.

import { writable, derived } from "svelte/store";
import type { Readable, Writable } from "svelte/store";
import type { ConfigState, DiscoveredApp, ProcessSnapshot } from "./types";

export const configStore: Writable<ConfigState | null> = writable(null);

export const snapshotStore: Writable<ProcessSnapshot | null> = writable(null);

export const snapshotErrorStore: Writable<string> = writable("");

// Last successful tick timestamp, in millis since epoch. Used by StatusBar
// to render "updated 1.4s ago" without driving a per-component clock.
export const lastUpdateAtStore: Writable<number> = writable(0);

// Transient one-shot kill feedback. Components write a result here and the
// status bar surfaces it for a few seconds. Cleared by writing { errno: "" }.
export interface KillFlash {
	pid: number;
	role: string;
	result: { killed: boolean; errno?: string; err?: string };
	at: number;
}
export const killFlashStore: Writable<KillFlash | null> = writable(null);

// onboardingNeeded is true when no AppName is configured yet. Derived so
// every component can subscribe without having to peek at the config.
export const onboardingNeeded: Readable<boolean> = derived(
	configStore,
	($cfg) => !$cfg || !$cfg.appName,
);

// showSettings drives the gear-button slide-down panel.
export const showSettings: Writable<boolean> = writable(false);

// discoveredAppsStore holds the latest CEF / Chromium / Electron apps found
// on the host. null = "discovery hasn't reported yet"; [] = "discovery ran
// and found nothing". Refreshed by the backend on its own slow cadence.
export const discoveredAppsStore: Writable<DiscoveredApp[] | null> = writable(null);

// showDiscoveredApps drives the apps-toggle slide-down bar (hidden by default).
export const showDiscoveredApps: Writable<boolean> = writable(false);

// privateIconsStore holds user-supplied SVG icons that live alongside the
// config JSON, keyed by the <name> in app-<name>.svg. Each value is a
// base64 data URI ready to drop into <img src=...>. Refreshed on startup
// and after the user edits the config (lookup order: private → bundled →
// default). See lib/iconResolver.ts.
export const privateIconsStore: Writable<Record<string, string>> = writable({});
