<script lang="ts">
	import { onMount, onDestroy } from "svelte";
	import { bridge } from "./lib/bridge";
	import {
		configStore,
		snapshotStore,
		snapshotErrorStore,
		lastUpdateAtStore,
		onboardingNeeded,
		showSettings,
		discoveredAppsStore,
		showDiscoveredApps,
	} from "./lib/stores";
	import Onboarding from "./lib/Onboarding.svelte";
	import ProcessTree from "./lib/ProcessTree.svelte";
	import StatusBar from "./lib/StatusBar.svelte";
	import Settings from "./lib/Settings.svelte";
	import DiscoveredAppsBar from "./lib/DiscoveredAppsBar.svelte";
	import { refreshTarget, targetIconStore, targetLabelStore, onIconError } from "./lib/iconResolver";

	// Monitored app, shown centred in the top bar so the eye lands on it
	// straight away. Config-driven (not snapshot-driven) so it flips the
	// instant the user picks another app, before the new tree arrives: the
	// raw name shows at once and the backend's display name replaces it.
	$: targetName = $configStore?.appName ?? "";
	$: targetLabel = targetName ? $targetLabelStore || targetName : "";

	// ── Window size ──
	// The window has one fixed size (main.go windowWidth × windowHeight,
	// DisableResize): neither the user nor the layout resizes it. The
	// process tree is the only part whose size varies, so .tree-pane is the
	// only part that scrolls, vertically and, for very deep trees or a large
	// zoom, horizontally.
	//
	// The one exception: the window must never be larger than the monitor it
	// is on. We report that monitor's work area (screen.* follows the window
	// across monitors and excludes the taskbar) and the backend caps the
	// window to it, growing it back on a larger monitor. Moving a window to
	// another monitor fires no event, hence the light 1 s poll; the backend
	// is only called when the work area actually changes.
	const SCREEN_POLL_MS = 1000;
	let lastScreen = "";
	let screenTimer: ReturnType<typeof setInterval> | undefined;

	function fitToScreen() {
		const w = window.screen.availWidth;
		const h = window.screen.availHeight;
		const key = `${w}x${h}`;
		if (key === lastScreen) return;
		lastScreen = key;
		bridge.fitWindowToScreen(w, h).catch(() => {
			/* plain vite dev — no native window */
		});
	}

	// ── Zoom (Ctrl + / Ctrl - / Ctrl 0) ──
	// Persisted across launches in localStorage; applied to <html> font-size.
	// 1.0 == 13.5px base (a touch under the browser's usual 14px; Ctrl+0
	// returns here). Bounds keep the UI usable: 0.7 → ~9.5px, 1.6 → ~21.6px.
	const ZOOM_KEY = "ceftop:zoom";
	const ZOOM_MIN = 0.7;
	const ZOOM_MAX = 1.6;
	const ZOOM_STEP = 0.1;
	const BASE_FONT_PX = 13.5;

	let zoom = 1.0;

	function applyZoom() {
		document.documentElement.style.fontSize = `${(BASE_FONT_PX * zoom).toFixed(2)}px`;
		try {
			localStorage.setItem(ZOOM_KEY, zoom.toFixed(2));
		} catch {
			/* private browsing or quota — ignore */
		}
	}

	function changeZoom(delta: number) {
		zoom = Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, zoom + delta));
		applyZoom();
	}

	function resetZoom() {
		zoom = 1.0;
		applyZoom();
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === "Escape" && !e.defaultPrevented && !isTextField(e.target)) {
			if (closeLastPanel()) e.preventDefault();
			return;
		}
		if (!(e.ctrlKey || e.metaKey)) return;
		// "+" arrives as either "+", "=", or " Plus"; "-" as "-" or "Minus".
		if (e.key === "+" || e.key === "=") {
			e.preventDefault();
			changeZoom(ZOOM_STEP);
		} else if (e.key === "-" || e.key === "_") {
			e.preventDefault();
			changeZoom(-ZOOM_STEP);
		} else if (e.key === "0") {
			e.preventDefault();
			resetZoom();
		}
	}

	// ── Bridge wiring ──
	let unsubSnapshot: (() => void) | undefined;
	let unsubError: (() => void) | undefined;
	let unsubDiscovery: (() => void) | undefined;
	let unsubDiscoveryError: (() => void) | undefined;

	onMount(async () => {
		// Restore zoom before paint so there is no flash of unzoomed UI.
		const saved = Number(localStorage.getItem(ZOOM_KEY));
		if (Number.isFinite(saved) && saved >= ZOOM_MIN && saved <= ZOOM_MAX) {
			zoom = saved;
		}
		applyZoom();

		// Reopen the apps bar if it was open when the app last closed.
		try {
			if (localStorage.getItem(APPS_BAR_KEY) === "1") setPanel("apps", true);
		} catch {
			/* storage unavailable — start closed */
		}

		try {
			const cfg = await bridge.getConfig();
			configStore.set(cfg);
		} catch (e) {
			snapshotErrorStore.set(`could not read config: ${String(e)}`);
		}

		unsubSnapshot = bridge.onSnapshot((snap) => {
			snapshotStore.set(snap);
			snapshotErrorStore.set("");
			lastUpdateAtStore.set(Date.now());
		});

		unsubError = bridge.onSnapshotError((err) => {
			snapshotErrorStore.set(err);
		});

		unsubDiscovery = bridge.onDiscovery((apps) => {
			discoveredAppsStore.set(apps ?? []);
			// Piggyback on the 5 s discovery cadence: any private icon the user
			// drops into the config directory, or an edit to apps.json, shows
			// up within one tick without having to wire a refresh through
			// SetTargetApp / OpenConfigInEditor.
			refreshTarget($configStore?.appName ?? "");
		});

		unsubDiscoveryError = bridge.onDiscoveryError(() => {
			// Treat a transient enumeration failure as "no apps known yet"
			// rather than surfacing it — the snapshot error channel already
			// covers persistent host-enumeration problems.
			discoveredAppsStore.set([]);
		});

		// Subscriptions are wired; tell the backend so it emits the first
		// snapshot and discovery scan now. Wails events are fire-and-forget,
		// so the backend deliberately does not emit at startup (nobody is
		// listening yet) and instead waits for this signal. Without it the
		// user would wait one full tick interval (up to 999 s with hand-
		// edited config) for the first paint. Errors are swallowed: the
		// next ticker emit will populate.
		try {
			await bridge.frontendReady();
		} catch {
			/* ignore — next tick will refill */
		}

		window.addEventListener("keydown", onKeydown);

		fitToScreen();
		screenTimer = setInterval(fitToScreen, SCREEN_POLL_MS);
	});

	onDestroy(() => {
		unsubSnapshot?.();
		unsubError?.();
		unsubDiscovery?.();
		unsubDiscoveryError?.();
		window.removeEventListener("keydown", onKeydown);
		clearInterval(screenTimer);
	});

	// ── Panels (apps bar, settings) ──
	// Both are docked slide-downs, not popovers: they stay open until the
	// user closes them with their header toggle or Esc, never on a pick or
	// a click elsewhere. Esc closes the most recently opened one first, one
	// per press, and leaves Esc inside a text field to that field (it
	// cancels the edit). The apps bar remembers its state across launches
	// because it is a working surface; Settings always starts closed.
	type Panel = "apps" | "settings";
	const APPS_BAR_KEY = "ceftop:appsbar";
	let openOrder: Panel[] = [];

	function setPanel(panel: Panel, open: boolean) {
		const store = panel === "apps" ? showDiscoveredApps : showSettings;
		store.set(open);
		openOrder = openOrder.filter((p) => p !== panel);
		if (open) openOrder = [...openOrder, panel];
		if (panel === "apps") {
			try {
				localStorage.setItem(APPS_BAR_KEY, open ? "1" : "0");
			} catch {
				/* private browsing or quota — ignore */
			}
		}
	}

	function toggleSettings() {
		setPanel("settings", !$showSettings);
	}

	function toggleDiscoveredApps() {
		setPanel("apps", !$showDiscoveredApps);
	}

	function isTextField(t: EventTarget | null): boolean {
		return (
			t instanceof HTMLInputElement ||
			t instanceof HTMLTextAreaElement ||
			t instanceof HTMLSelectElement ||
			(t instanceof HTMLElement && t.isContentEditable)
		);
	}

	// Returns true when Esc closed a panel.
	function closeLastPanel(): boolean {
		const last = openOrder[openOrder.length - 1];
		if (!last) return false;
		setPanel(last, false);
		return true;
	}

	$: appsButtonDisabled =
		Array.isArray($discoveredAppsStore) && $discoveredAppsStore.length === 0;

	// The header icon follows the configured target: re-resolve whenever the
	// user picks a different app (apps bar, settings, status bar editor).
	$: refreshTarget($configStore?.appName ?? "");
</script>

<main>
	{#if $onboardingNeeded}
		<Onboarding />
	{:else}
		<header>
			<h1>
				<img src="/logo.svg" alt="" class="brand-logo" />
				<span>CefTop</span>
			</h1>
			<div class="header-target" title={targetName}>
				{#if targetLabel}
					<img
						class="header-target-icon"
						src={$targetIconStore}
						alt=""
						on:error={onIconError}
					/>
					<span class="header-target-name">{targetLabel}</span>
				{/if}
			</div>
			<div class="header-actions">
				<button
					class="btn-gear"
					class:active={$showDiscoveredApps}
					on:click={toggleDiscoveredApps}
					disabled={appsButtonDisabled}
					title={appsButtonDisabled
						? "No CEF/Chromium apps detected"
						: $showDiscoveredApps
							? "Hide discovered apps (Esc)"
							: "Discovered apps"}
					aria-label="Discovered apps"
				>
					&#9638;
				</button>
				<button
					class="btn-gear"
					class:active={$showSettings}
					on:click={toggleSettings}
					title={$showSettings ? "Hide settings (Esc)" : "Settings"}
					aria-label="Settings"
				>
					&#9881;
				</button>
			</div>
		</header>

		{#if $showDiscoveredApps}
			<DiscoveredAppsBar />
		{/if}

		{#if $showSettings}
			<Settings />
		{/if}

		<section class="tree-pane">
			<ProcessTree />
		</section>
		<StatusBar />
	{/if}
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		height: 100vh;
		overflow: hidden;
	}
	/* Three columns so the monitored app sits at the true centre of the
	   bar regardless of how wide the brand and the action buttons are. */
	header {
		display: grid;
		grid-template-columns: 1fr auto 1fr;
		align-items: center;
		gap: 1rem;
		padding: 0.6rem 1rem;
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
	}
	.header-target {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		min-width: 0;
		justify-self: center;
	}
	/* rem so the icon zooms with the name (the brand logo stays fixed). */
	.header-target-icon {
		width: 2.5rem;
		height: 2.5rem;
		display: block;
		flex-shrink: 0;
		pointer-events: none;
	}
	.header-target-name {
		font-family: var(--font-mono);
		font-size: 0.95rem;
		font-weight: 600;
		color: var(--fg);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	h1 {
		font-size: 1.1rem;
		font-weight: 600;
		margin: 0;
		color: var(--accent);
		letter-spacing: -0.01em;
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		line-height: 1;
	}
	.brand-logo {
		width: 48px;
		height: 48px;
		display: block;
	}
	.header-actions {
		justify-self: end;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.btn-gear {
		background: transparent;
		border: 1px solid transparent;
		color: var(--fg-muted);
		font-size: 1.1rem;
		padding: 0;
		width: 2rem;
		height: 2rem;
		border-radius: 4px;
		cursor: pointer;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		line-height: 1;
		transition: all 0.12s;
	}
	.btn-gear:hover {
		color: var(--fg);
		background: var(--bg-elevated);
	}
	.btn-gear.active {
		color: var(--accent);
		background: var(--bg-elevated);
		border-color: var(--accent);
	}
	.btn-gear:disabled {
		opacity: 0.35;
		cursor: not-allowed;
	}
	.btn-gear:disabled:hover {
		color: var(--fg-muted);
		background: transparent;
	}
	.tree-pane {
		flex: 1;
		min-height: 0;
		overflow: auto;
	}
</style>
