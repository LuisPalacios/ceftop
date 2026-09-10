// Icon resolution for CEF / Chromium target apps.
//
// All matching lives in the backend (pkg/icons): it knows both the bundled
// icon set and the user's private app-<name>.svg files, and scores names
// fuzzily so "Docker Desktop.exe" finds app-docker-desktop.svg. The
// frontend never builds icon file names itself:
//
//   - Discovered apps arrive with `iconSrc` already resolved.
//   - The current target's icon is fetched via bridge.resolveIcon() and
//     cached in targetIconStore; App.svelte refreshes it whenever the
//     configured target changes and on every discovery tick (which is how
//     a freshly dropped private icon shows up without a restart).
//
// The only client-side fallback is the <img on:error> handler, which swaps
// in the bundled default if a src ever fails to load.

import { writable } from "svelte/store";
import type { Writable } from "svelte/store";
import { bridge } from "./bridge";

export const BUNDLED_DEFAULT_ICON = "/app-icons/app-default.svg";

// Resolved <img src> for the currently configured target. Starts on the
// bundled default so the header never renders a broken image.
export const targetIconStore: Writable<string> = writable(BUNDLED_DEFAULT_ICON);

let lastRequested = "";

// refreshTargetIcon asks the backend for the icon that matches `name` and
// publishes it to targetIconStore. Out-of-order responses are dropped so a
// slow answer for a previous target cannot overwrite the current one.
export async function refreshTargetIcon(name: string): Promise<void> {
	const wanted = name ?? "";
	lastRequested = wanted;
	if (!wanted) {
		targetIconStore.set(BUNDLED_DEFAULT_ICON);
		return;
	}
	try {
		const src = await bridge.resolveIcon(wanted);
		if (lastRequested !== wanted) return;
		targetIconStore.set(src || BUNDLED_DEFAULT_ICON);
	} catch {
		if (lastRequested !== wanted) return;
		targetIconStore.set(BUNDLED_DEFAULT_ICON);
	}
}

// onIconError: called when an <img> fails to load its src. Falls through
// to the bundled default, then disables further retries so a missing
// default cannot loop.
export function onIconError(e: Event): void {
	const el = e.currentTarget as HTMLImageElement;
	el.onerror = null;
	if (el.src.endsWith(BUNDLED_DEFAULT_ICON)) return;
	el.src = BUNDLED_DEFAULT_ICON;
}
