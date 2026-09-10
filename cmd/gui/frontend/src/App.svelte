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
	import { refreshTargetIcon } from "./lib/iconResolver";

	// ── Window fit ──
	// Height is fixed: room for the header, the optional apps / settings
	// panels, the status bar, the column header and exactly VISIBLE_ROWS
	// tree rows. It changes only with zoom or a panel toggle, never with
	// the process count — deeper trees scroll inside .tree-pane. The OS
	// min and max height are pinned to that value so a drag cannot change
	// it. Width still tracks the widest row (we cannot know how deep a tree
	// will nest) between MIN_WIDTH and the monitor's work area; a tree wider
	// than the screen scrolls horizontally inside .tree-pane.
	const VISIBLE_ROWS = 20;
	// Mirrors MinWidth in main.go. Outer size in the runtime's own units.
	const MIN_WIDTH = 200;

	// Row and column-header heights in rem, measured from the real DOM
	// whenever rows are on screen and persisted, so the "no rows" states
	// (waiting for a snapshot, target not running) and the next launch get
	// the exact height instead of an estimate. Stored in rem so one value
	// serves every zoom level. The fallbacks approximate the .row and
	// .header-row layouts in ProcessNode.svelte / ProcessTree.svelte and
	// only matter until the first tree has ever rendered.
	const METRICS_KEY = "ceftop:rowmetrics";
	// rowRem: height of one .row. rowsTopRem: distance from the top of
	// .tree to the top of its first row (tree padding + column header +
	// its margin). Both fractional, measured with getBoundingClientRect.
	const FALLBACK_ROW_REM = 1.82;
	const FALLBACK_ROWS_TOP_REM = 3.1;
	// .tree padding-bottom.
	const TREE_PADDING_BOTTOM_REM = 0.5;
	// Matches ::-webkit-scrollbar width in style.css; used until the pane
	// has actually shown a scrollbar we can measure.
	const SCROLLBAR_PX = 10;

	interface RowMetrics {
		rowRem: number;
		rowsTopRem: number;
	}

	function loadMetrics(): RowMetrics {
		try {
			const raw = localStorage.getItem(METRICS_KEY);
			if (raw) {
				const parsed = JSON.parse(raw) as Partial<RowMetrics>;
				const rowRem = Number(parsed.rowRem);
				const rowsTopRem = Number(parsed.rowsTopRem);
				if (rowRem > 0 && rowRem < 10 && rowsTopRem > 0 && rowsTopRem < 10) {
					return { rowRem, rowsTopRem };
				}
			}
		} catch {
			/* storage unavailable or corrupt — fall back */
		}
		return { rowRem: FALLBACK_ROW_REM, rowsTopRem: FALLBACK_ROWS_TOP_REM };
	}

	let metrics: RowMetrics = loadMetrics();

	function saveMetrics() {
		try {
			localStorage.setItem(METRICS_KEY, JSON.stringify(metrics));
		} catch {
			/* private browsing or quota — ignore */
		}
	}

	function remPx(): number {
		const v = parseFloat(getComputedStyle(document.documentElement).fontSize);
		return Number.isFinite(v) && v > 0 ? v : 14;
	}

	// Wails reports and sets the OUTER window size in its own units, and
	// those are not always CSS px: on this Windows host with a 125 % display
	// WindowGetSize says 920 while the webview's innerWidth says 724, i.e.
	// Wails units are CSS px × devicePixelRatio plus the OS chrome. Whether
	// that holds depends on the process's DPI awareness, so we do not hard-
	// code it. Model: wails = inner × k + chrome. k is learned exactly from
	// two observations with different inner sizes (our own resizes provide
	// them); until then it is guessed from devicePixelRatio, keeping
	// whichever of {dpr, 1} implies a plausible chrome. Chrome (title bar +
	// borders) is sticky once measured — it does not change at runtime.
	let scaleK = 0; // 0 = not learned yet
	let chromeW = 0;
	let chromeH = 0;
	let lastObs: { w: number; iw: number } | null = null;

	function guessScale(wailsW: number, iw: number): number {
		const dpr = window.devicePixelRatio || 1;
		for (const k of [dpr, 1]) {
			const cw = wailsW - iw * k;
			if (cw >= 0 && cw < 60 * k) return k;
		}
		return 1;
	}

	function calibrate(wailsW: number, wailsH: number) {
		const iw = window.innerWidth;
		const ih = window.innerHeight;
		if (wailsW <= 0 || wailsH <= 0 || iw <= 0 || ih <= 0) return;
		if (scaleK === 0 && lastObs && Math.abs(iw - lastObs.iw) >= 20) {
			const k = (wailsW - lastObs.w) / (iw - lastObs.iw);
			if (k > 0.5 && k < 4) scaleK = k;
		}
		lastObs = { w: wailsW, iw };
		const k = scaleK || guessScale(wailsW, iw);
		const cw = wailsW - iw * k;
		const ch = wailsH - ih * k;
		// Readings occasionally come back stale for one frame right after
		// a resize (outer already new, inner not yet); only accept sane ones.
		if (cw >= 0 && cw < 60 * k && ch >= 0 && ch < 150 * k) {
			chromeW = cw;
			chromeH = ch;
		}
	}

	function currentScale(): number {
		return scaleK || (lastObs ? guessScale(lastObs.w, lastObs.iw) : window.devicePixelRatio || 1);
	}

	// w is null when no tree is rendered: the placeholder has no natural
	// width, so the window keeps its current width until rows show up.
	interface ContentSize {
		w: number | null;
		h: number;
	}

	function measureContentSize(): ContentSize | null {
		const main = document.querySelector("main");
		if (!main) return null;
		const header = main.querySelector("header") as HTMLElement | null;
		const appsBar = main.querySelector(".apps-bar") as HTMLElement | null;
		const settings = main.querySelector(".settings") as HTMLElement | null;
		const treePane = main.querySelector(".tree-pane") as HTMLElement | null;
		const bar = main.querySelector(".bar") as HTMLElement | null;

		// Fractional heights throughout: offsetHeight rounds to whole px and
		// rows are ~25.45px at the default zoom, so per-row rounding would
		// drift by half a row across 20 of them.
		const hOf = (el: HTMLElement | null): number => (el ? el.getBoundingClientRect().height : 0);

		const rem = remPx();
		const innerTree = treePane?.querySelector(".tree") as HTMLElement | null;
		const row = treePane?.querySelector(".row") as HTMLElement | null;
		if (innerTree && row) {
			const treeRect = innerTree.getBoundingClientRect();
			const rowRect = row.getBoundingClientRect();
			const next = { rowRem: rowRect.height / rem, rowsTopRem: (rowRect.top - treeRect.top) / rem };
			if (
				next.rowRem > 0 &&
				next.rowsTopRem > 0 &&
				(Math.abs(next.rowRem - metrics.rowRem) > 1e-3 ||
					Math.abs(next.rowsTopRem - metrics.rowsTopRem) > 1e-3)
			) {
				metrics = next;
				saveMetrics();
			}
		}
		const paneBudget =
			(metrics.rowsTopRem + VISIBLE_ROWS * metrics.rowRem + TREE_PADDING_BOTTOM_REM) * rem;

		// Width comes from the INNER .tree element (width: max-content), not
		// the pane: the pane stretches to fill <main>, so measuring it would
		// feed the window width back into itself. When the tree is taller
		// than the pane budget a vertical scrollbar appears and takes its
		// width from the pane; add it back or the pane ends up narrower than
		// the tree and grows a spurious horizontal scrollbar too.
		let w: number | null = null;
		if (innerTree) {
			w = Math.ceil(innerTree.scrollWidth);
			if (treePane && hOf(innerTree) > paneBudget + 0.5) {
				const sb = treePane.offsetWidth - treePane.clientWidth;
				w += sb > 0 ? sb : SCROLLBAR_PX;
			}
		}

		const banner = treePane?.querySelector(".banner") as HTMLElement | null;
		const h = hOf(header) + hOf(appsBar) + hOf(settings) + hOf(banner) + paneBudget + hOf(bar);

		// Floor, not ceil: a pane 1px short clips the 20th row's bottom
		// border, a pane 1px long shows a sliver of the 21st row.
		return { w, h: Math.floor(h) };
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
		const dw = window.outerWidth - window.innerWidth;
		const dh = window.outerHeight - window.innerHeight;
		// Tab bar + URL bar in any browser is at least 60px tall. The native
		// WebView2 reports 0–1 here.
		if (dw > 5 || dh > 60) return false;
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
	// in the wails dev terminal + browser console). Off by default.
	const DEBUG_FIT = false;

	// Last height / max width we pinned at the OS level, so the min / max
	// calls only go out when the lock actually changes (zoom, panel toggle,
	// monitor change), not on every tick.
	let lockedH = 0;
	let lockedMaxW = 0;

	async function fitToContent() {
		const m = measureContentSize();
		if (!m || m.h <= 0) return;

		let wailsW = 0;
		let wailsH = 0;
		try {
			const sz = await bridge.windowGetSize();
			wailsW = sz[0] ?? 0;
			wailsH = sz[1] ?? 0;
		} catch {
			/* bridge unavailable — chrome stays at its last known value */
		}
		calibrate(wailsW, wailsH);
		const k = currentScale();

		// Everything from here on is in Wails units: CSS px × k plus chrome.
		const maxW = Math.max(MIN_WIDTH, Math.floor(window.screen.availWidth * k));
		const targetH = Math.round(m.h * k + chromeH);
		let targetW: number;
		if (m.w === null) {
			if (wailsW <= 0) return; // nothing to measure and nothing known
			targetW = wailsW;
		} else {
			targetW = Math.min(maxW, Math.max(MIN_WIDTH, Math.round(m.w * k + chromeW)));
		}

		// Pin the height at the OS level before resizing: Wails clamps
		// SetSize against the current min / max, so the lock must move first.
		if (targetH !== lockedH || maxW !== lockedMaxW) {
			lockedH = targetH;
			lockedMaxW = maxW;
			bridge.windowSetMinSize(MIN_WIDTH, targetH).catch(() => {
				/* plain vite dev — no native window */
			});
			bridge.windowSetMaxSize(maxW, targetH).catch(() => {
				/* plain vite dev — no native window */
			});
		}

		const tol = Math.max(2, Math.round(2 * k));
		const driftW = Math.abs(wailsW - targetW);
		const driftH = Math.abs(wailsH - targetH);
		const now = Date.now();
		const stable = driftW <= tol && driftH <= tol;
		const cooldown = now - lastFitAt < FIT_COOLDOWN_MS;
		const disposition = stable ? "skip-stable" : cooldown ? "skip-cooldown" : "fit";
		if (DEBUG_FIT) {
			const line =
				`fit inner=${window.innerWidth}x${window.innerHeight}` +
				` jsouter=${window.outerWidth}x${window.outerHeight}` +
				` dpr=${window.devicePixelRatio} screen=${window.screen.width}x${window.screen.height}` +
				` wailsouter=${wailsW}x${wailsH}` +
				` k=${k.toFixed(3)}${scaleK ? "" : "?"} chrome=${chromeW.toFixed(1)}x${chromeH.toFixed(1)}` +
				` content=${m.w ?? "null"}x${m.h}` +
				` rowRem=${metrics.rowRem.toFixed(3)} rowsTopRem=${metrics.rowsTopRem.toFixed(3)}` +
				` -> ${targetW}x${targetH} (maxW ${maxW})` +
				` drift=${driftW}x${driftH} [${disposition}]`;
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
			refreshTargetIcon($configStore?.appName ?? "");
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

	// The header icon follows the configured target: re-resolve whenever the
	// user picks a different app (apps bar, settings, status bar editor).
	$: refreshTargetIcon($configStore?.appName ?? "");
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
