// Icon resolution for CEF / Chromium target apps.
//
// Lookup order for a given <name>:
//   1. User-private SVG next to the config JSON, served as a base64 data URI
//      under the "private" key. Refreshed via refreshPrivateIcons().
//   2. Bundled /app-icons/app-<name>.svg.
//   3. Default fallback: the user's private app-default.svg if present,
//      otherwise the bundled /app-icons/app-default.svg.
//
// The two consumers (DiscoveredAppsBar and the ProcessTree header) share
// this module so the lookup order is defined in exactly one place.

import { bridge } from "./bridge";
import { privateIconsStore } from "./stores";

export const BUNDLED_DEFAULT_ICON = "/app-icons/app-default.svg";
export const DEFAULT_ICON_KEY = "default";

export function bundledIconSrc(name: string): string {
	return `/app-icons/app-${name}.svg`;
}

export function resolveIconSrc(
	name: string,
	privates: Record<string, string>,
): string {
	if (name && privates[name]) return privates[name];
	return bundledIconSrc(name);
}

export function resolveDefaultIcon(privates: Record<string, string>): string {
	return privates[DEFAULT_ICON_KEY] ?? BUNDLED_DEFAULT_ICON;
}

// onIconError handler: called when an <img> 404s on either the private data
// URI (corrupted base64? unlikely but cheap to guard) or the bundled file.
// Falls through to the default icon, then disables further error retries to
// avoid infinite loops if the default itself is missing.
export function makeIconErrorHandler(privates: Record<string, string>) {
	return function onIconError(e: Event) {
		const el = e.currentTarget as HTMLImageElement;
		el.onerror = null;
		el.src = resolveDefaultIcon(privates);
	};
}

export async function refreshPrivateIcons(): Promise<void> {
	try {
		const m = await bridge.getPrivateIcons();
		privateIconsStore.set(m ?? {});
	} catch {
		privateIconsStore.set({});
	}
}
