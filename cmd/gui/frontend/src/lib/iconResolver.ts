// Icon and display-name resolution for CEF / Chromium target apps.
//
// All of it lives in the backend: pkg/icons knows both the bundled icon set
// and the user's private app-<name>.svg files, and scores names fuzzily so
// "Docker Desktop.exe" finds app-docker-desktop.svg; pkg/catalog turns exe
// names into display names (app catalog, then the OS's description of the
// executable, then a heuristic). The frontend never builds either itself:
//
//   - Discovered apps arrive with `iconSrc` and `friendlyName` resolved.
//   - The current target's icon and name are fetched via
//     bridge.describeApp() and cached in targetIconStore / targetLabelStore;
//     App.svelte refreshes them whenever the configured target changes and
//     on every discovery tick (which is how a freshly dropped private icon
//     or an edited apps.json shows up without a restart).
//
// The only client-side fallbacks are the raw target name, shown until the
// backend answers, and the <img on:error> handler, which swaps in the
// bundled default if a src ever fails to load.

import { writable } from "svelte/store";
import type { Writable } from "svelte/store";
import { bridge } from "./bridge";

export const BUNDLED_DEFAULT_ICON = "/app-icons/app-default.svg";

// Resolved <img src> for the currently configured target. Starts on the
// bundled default so the header never renders a broken image.
export const targetIconStore: Writable<string> = writable(BUNDLED_DEFAULT_ICON);

// Display name for the currently configured target.
export const targetLabelStore: Writable<string> = writable("");

let lastRequested = "";

// refreshTarget asks the backend how to show `name` and publishes the icon
// and label. The label flips to the raw name at once on a target change, so
// the header never shows the previous app's name under the new icon.
// Out-of-order responses are dropped so a slow answer for a previous target
// cannot overwrite the current one.
export async function refreshTarget(name: string): Promise<void> {
	const wanted = name ?? "";
	if (wanted !== lastRequested) {
		targetLabelStore.set(wanted);
	}
	lastRequested = wanted;
	if (!wanted) {
		targetIconStore.set(BUNDLED_DEFAULT_ICON);
		return;
	}
	try {
		const info = await bridge.describeApp(wanted);
		if (lastRequested !== wanted) return;
		targetIconStore.set(info.iconSrc || BUNDLED_DEFAULT_ICON);
		targetLabelStore.set(info.friendlyName || wanted);
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
