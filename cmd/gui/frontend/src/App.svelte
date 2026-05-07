<script lang="ts">
	import { onMount, onDestroy, tick } from "svelte";
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
	import { refreshPrivateIcons } from "./lib/iconResolver";

	// ── Window auto-fit ──
	// Width tracks content tightly on every snapshot tick / zoom / gear
	// toggle. Height grows when content gets taller than the current window
	// but never shrinks (so user-dragged extra space is preserved). Both
	// axes are clamped at the OS level by main.go's MinWidth / MinHeight;
	// the ceiling is the monitor work area so deep trees stay on-screen.

	// Sticky chrome delta. outerWidth/outerHeight occasionally read 0 right
	// after a WindowSetSize (Wails Windows in particular). Once we get a sane
	// reading we keep it — the OS title bar / borders don't change at runtime
	// barring a DPI swap, which Wails would restart for anyway.
	let chromeW = 0;
	let chromeH = 0;
	function chromeDelta(): { w: number; h: number } {
		const dw = window.outerWidth - window.innerWidth;
		const dh = window.outerHeight - window.innerHeight;
		if (Number.isFinite(dw) && dw > 0 && dw < 200) chromeW = dw;
		if (Number.isFinite(dh) && dh > 0 && dh < 200) chromeH = dh;
		return { w: chromeW, h: chromeH };
	}

	function measureContentSize(): { w: number; h: number } | null {
		const main = document.querySelector("main");
		if (!main) return null;
		const header = main.querySelector("header") as HTMLElement | null;
		const appsBar = main.querySelector(".apps-bar") as HTMLElement | null;
		const settings = main.querySelector(".settings") as HTMLElement | null;
		const treePane = main.querySelector(".tree-pane") as HTMLElement | null;
		const bar = main.querySelector(".bar") as HTMLElement | null;

		// Width: read from the INNER tree element, not the tree-pane.
		// The tree-pane stretches to fill <main>, so its scrollWidth grows with
		// the window itself — measuring it would feed back into the fit and
		// produce oscillation between two widths every tick. The inner .tree
		// gets a natural max-content width from the rows' min-width:max-content
		// rule, which is stable regardless of how wide the pane is.
		// Header / settings / bar are flex layouts that stretch to <main> too,
		// so we explicitly skip them: in practice they're always narrower than
		// the tree, and including them re-introduces the same feedback bug.
		const innerTree = treePane?.querySelector(".tree") as HTMLElement | null;
		if (!innerTree) {
			// No rows rendered (waiting for first snapshot, or no matching
			// processes). The placeholder has no stable natural width — it
			// fills its parent, so measuring it produces a recursive 10 px
			// shrink each tick (the custom scrollbar steals 10 px from the
			// pane's content width). Bail entirely; the window keeps its
			// current size until a real tree shows up.
			return null;
		}
		let w = Math.max(0, innerTree.scrollWidth);

		// Height: header + settings (when open) + tree's INNER natural height
		// + statusbar. We can't use treePane.scrollHeight directly because the
		// pane has flex:1 — when the window already fits, scrollHeight equals
		// offsetHeight equals the allocated size, which would create a feedback
		// loop. The inner .tree / .placeholder / .banner reflects the actual
		// content height regardless of pane allocation.
		let h = 0;
		if (header) h += header.offsetHeight;
		if (appsBar) h += appsBar.offsetHeight;
		if (settings) h += settings.offsetHeight;
		if (treePane) {
			const banner = treePane.querySelector(".banner") as HTMLElement | null;
			const tree = treePane.querySelector(".tree") as HTMLElement | null;
			const placeholder = treePane.querySelector(".placeholder") as HTMLElement | null;
			if (banner) h += banner.offsetHeight;
			if (tree) h += tree.offsetHeight;
			else if (placeholder) h += placeholder.offsetHeight;
		}
		if (bar) h += bar.offsetHeight;

		// One-row breathing space at the bottom: avoids sub-pixel scrollbar
		// flicker AND gives a visual cue that the list ends here. We sample
		// an actual rendered row so the buffer scales with zoom; falling
		// back to a constant when no rows are present (onboarding, empty).
		const sampleRow = treePane?.querySelector(".row") as HTMLElement | null;
		const buffer = sampleRow ? sampleRow.offsetHeight : 28;
		h += buffer;

		return { w: Math.ceil(w), h: Math.ceil(h) };
	}

	// Distinguish the native Wails webview from a browser tab pointed at the
	// Wails dev runtime URL (localhost:34115) — both have window.go injected,
	// so just checking `window.go` is not enough. The native WebView2 reports
	// outerWidth == innerWidth (chrome=0x0); a real browser has at least a
	// few px of window border + tab strip + address bar. A browser firing
	// fits would call WindowSetSize against the actual Wails native window
	// and fight the native instance for control of its size.
	function inWailsRuntime(): boolean {
		if (typeof (window as unknown as { go?: unknown }).go === "undefined") return false;
		const chromeW = window.outerWidth - window.innerWidth;
		const chromeH = window.outerHeight - window.innerHeight;
		// Tab bar + URL bar in any browser is at least 60px tall. The native
		// WebView2 reports 0–1 here.
		if (chromeW > 5 || chromeH > 60) return false;
		return true;
	}

	let fitScheduled = false;
	function scheduleFit() {
		if (fitScheduled) return;
		if (!inWailsRuntime()) return;
		if ($onboardingNeeded) return; // let user resize freely during onboarding
		fitScheduled = true;
		// Two RAFs: first lets Svelte commit the DOM change, second lets the
		// browser apply layout so scrollWidth/offsetHeight read the new sizes.
		requestAnimationFrame(() => {
			requestAnimationFrame(async () => {
				fitScheduled = false;
				await tick();
				fitToContent();
			});
		});
	}

	// Cooldown between *applied* fits. Cheap insurance against runaway fit
	// cascades — even if some measurement disagrees with itself across ticks,
	// the window can resize at most twice per second.
	let lastFitAt = 0;
	const FIT_COOLDOWN_MS = 500;

	// Set to true to re-enable per-fit telemetry (one line per snapshot tick
	// in the wails dev terminal + browser console). Useful when debugging
	// auto-fit / DPI / chrome-delta issues; off by default to keep the log
	// quiet during normal use.
	const DEBUG_FIT = false;

	async function fitToContent() {
		const m = measureContentSize();
		const measureBad = !m || m.w <= 0 || m.h <= 0;

		// Read the OS window's outer size from Wails. This is the only
		// reliable source on Windows: WebView2's window.outerWidth reports the
		// inner viewport, and Wails' WindowSetSize takes "Wails units" that
		// turn out to be (cssInner * dpi + nativeChromePhysical) — at 125%
		// scaling on a Windows host with a typical title bar, that's about
		// 1.27× the CSS inner. We derive the conversion empirically from the
		// current observation rather than trusting devicePixelRatio (the user
		// reported 100% scale but the data clearly shows ~1.27, so a different
		// effective scale is in play and devicePixelRatio cannot be trusted).
		let wailsW = 0;
		let wailsH = 0;
		try {
			const sz = await bridge.windowGetSize();
			wailsW = sz[0] ?? 0;
			wailsH = sz[1] ?? 0;
		} catch {
			/* bridge unavailable — fall back to JS outer */
		}

		// Empirical conversion: 1 CSS inner pixel == this many Wails units.
		// For initial bootstrap when wails size isn't known yet, default to 1
		// (just sends raw CSS px, which matches non-Windows behavior).
		const scaleW =
			wailsW > 0 && window.innerWidth > 0 ? wailsW / window.innerWidth : 1;
		const scaleH =
			wailsH > 0 && window.innerHeight > 0 ? wailsH / window.innerHeight : 1;

		// Both axes tight to content. No client-side min floor — Wails'
		// MinWidth / MinHeight in main.go enforces an OS-level safety net
		// for tiny / onboarding states; everything above that fits content
		// exactly. Manual drags get auto-corrected on the next snapshot tick.
		const targetCssW = measureBad
			? 0
			: Math.min(window.screen.availWidth, m!.w);
		const targetCssH = measureBad
			? 0
			: Math.min(window.screen.availHeight, m!.h);
		const targetW = Math.round(targetCssW * scaleW);
		const targetH = Math.round(targetCssH * scaleH);

		// Both drifts bidirectional now: any difference between current
		// Wails outer and the computed target triggers a fit.
		const tol = Math.max(2, Math.round(2 * Math.max(scaleW, scaleH)));
		const driftW = Math.abs(wailsW - targetW);
		const driftH = Math.abs(wailsH - targetH);
		const now = Date.now();
		const cooldown = now - lastFitAt < FIT_COOLDOWN_MS;
		const stable = driftW <= tol && driftH <= tol;
		const disposition = measureBad
			? "skip-bad-measure"
			: stable
				? "skip-stable"
				: cooldown
					? "skip-cooldown"
					: "fit";
		if (DEBUG_FIT) {
			const line =
				`fit inner=${window.innerWidth}x${window.innerHeight}` +
				` jsouter=${window.outerWidth}x${window.outerHeight}` +
				` wailsouter=${wailsW}x${wailsH}` +
				` scale=${scaleW.toFixed(3)}x${scaleH.toFixed(3)}` +
				` content=${m ? `${m.w}x${m.h}` : "null"}` +
				` -> ${targetW}x${targetH}` +
				` drift=${driftW}x${driftH} tol=${tol} [${disposition}]`;
			console.log("[ceftop]", line);
			bridge.log(line).catch(() => {
				/* bridge unavailable in plain vite dev — console.log above suffices */
			});
		}
		if (disposition !== "fit") return;
		lastFitAt = now;
		bridge.windowSetSize(targetW, targetH).catch(() => {
			/* bridge unavailable (plain vite dev) — drop silently */
		});
	}


	// ── Zoom (Ctrl + / Ctrl - / Ctrl 0) ──
	// Persisted across launches in localStorage; applied to <html> font-size.
	// 1.0 == 14px base. Bounds keep the UI usable: 0.7 → 10px, 1.6 → ~22px.
	const ZOOM_KEY = "ceftop:zoom";
	const ZOOM_MIN = 0.7;
	const ZOOM_MAX = 1.6;
	const ZOOM_STEP = 0.1;
	const BASE_FONT_PX = 14;

	let zoom = 1.0;

	function applyZoom() {
		document.documentElement.style.fontSize = `${(BASE_FONT_PX * zoom).toFixed(2)}px`;
		try {
			localStorage.setItem(ZOOM_KEY, zoom.toFixed(2));
		} catch {
			/* private browsing or quota — ignore */
		}
		scheduleFit();
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

		try {
			const cfg = await bridge.getConfig();
			configStore.set(cfg);
		} catch (e) {
			snapshotErrorStore.set(`could not read config: ${String(e)}`);
		}

		// Initial private-icon scan so the first paint already prefers user
		// overrides over the bundled SVGs. Subsequent refreshes happen on each
		// discovery tick (see onDiscovery handler below).
		await refreshPrivateIcons();

		unsubSnapshot = bridge.onSnapshot((snap) => {
			snapshotStore.set(snap);
			snapshotErrorStore.set("");
			lastUpdateAtStore.set(Date.now());
			scheduleFit();
		});

		unsubError = bridge.onSnapshotError((err) => {
			snapshotErrorStore.set(err);
		});

		unsubDiscovery = bridge.onDiscovery((apps) => {
			discoveredAppsStore.set(apps ?? []);
			// Piggyback on the 5 s discovery cadence: any private icon the user
			// drops into the config directory shows up within one tick without
			// having to wire a refresh through SetTargetApp / OpenConfigInEditor.
			refreshPrivateIcons();
		});

		unsubDiscoveryError = bridge.onDiscoveryError(() => {
			// Treat a transient enumeration failure as "no apps known yet"
			// rather than surfacing it — the snapshot error channel already
			// covers persistent host-enumeration problems.
			discoveredAppsStore.set([]);
		});

		// Pull the first snapshot synchronously. The backend's snapshotLoop
		// emits an event immediately at startup, but Wails events are fire-
		// and-forget — the emit happens before this onMount has wired the
		// subscription, so without this pull the user would wait one full
		// tick interval (up to 999 s with hand-edited config) for the first
		// paint. Errors are swallowed: the next ticker emit will populate.
		try {
			const snap = await bridge.snapshot();
			snapshotStore.set(snap);
			snapshotErrorStore.set("");
			lastUpdateAtStore.set(Date.now());
			scheduleFit();
		} catch {
			/* ignore — next tick will refill */
		}

		// Same fire-and-forget race for discovery — pull once so the toggle
		// button lights up on first paint instead of after the 5 s tick.
		try {
			const apps = await bridge.discoverApps();
			discoveredAppsStore.set(apps ?? []);
		} catch {
			/* ignore — next tick will refill */
		}

		window.addEventListener("keydown", onKeydown);
	});

	onDestroy(() => {
		unsubSnapshot?.();
		unsubError?.();
		unsubDiscovery?.();
		unsubDiscoveryError?.();
		window.removeEventListener("keydown", onKeydown);
	});

	function toggleSettings() {
		showSettings.update((v) => !v);
		// Settings panel uses a 150ms slide transition; measuring during the
		// slide would catch a half-collapsed height. Wait for it to finish
		// before re-fitting, otherwise the gear close leaves trailing space.
		setTimeout(scheduleFit, 200);
	}

	function toggleDiscoveredApps() {
		// Mirrors toggleSettings; same 150ms slide → same 200ms re-fit delay.
		showDiscoveredApps.update((v) => !v);
		setTimeout(scheduleFit, 200);
	}

	$: appsButtonDisabled =
		Array.isArray($discoveredAppsStore) && $discoveredAppsStore.length === 0;
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
			<div class="header-actions">
				<button
					class="btn-gear"
					class:active={$showDiscoveredApps}
					on:click={toggleDiscoveredApps}
					disabled={appsButtonDisabled}
					title={appsButtonDisabled
						? "No CEF/Chromium apps detected"
						: "Discovered apps"}
					aria-label="Discovered apps"
				>
					&#9638;
				</button>
				<button
					class="btn-gear"
					class:active={$showSettings}
					on:click={toggleSettings}
					title="Settings"
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
	header {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 0.6rem 1rem;
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
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
		margin-left: auto;
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
