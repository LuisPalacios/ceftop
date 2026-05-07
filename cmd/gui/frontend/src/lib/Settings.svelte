<script lang="ts">
	import { onMount } from "svelte";
	import { slide } from "svelte/transition";
	import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
	import { bridge } from "./bridge";
	import { configStore } from "./stores";

	const REPO_URL = "https://github.com/LuisPalacios/ceftop";

	let appVersion = "";

	onMount(async () => {
		try {
			appVersion = await bridge.getAppVersion();
		} catch {
			appVersion = "";
		}
	});

	// Slider covers the everyday range; the text input accepts the full
	// legal range (1..999). The backend clamps anything outside that to
	// the default and the UI re-syncs from the persisted value.
	const TICK_SLIDER_MIN = 1;
	const TICK_SLIDER_MAX = 30;
	const TICK_INPUT_MIN = 1;
	const TICK_INPUT_MAX = 999;
	const TICK_DEFAULT = 5;

	let editingTarget = false;
	let targetDraft = "";
	let targetSaving = false;
	let targetError = "";

	let tickSaving = false;
	let tickError = "";

	// Local UI state for the tick controls, kept in sync with the store.
	// Both controls write to the backend; reading the persisted value back
	// drives the redraw, so a clamped value (out of range -> 5) is reflected
	// in both controls automatically.
	let sliderValue = TICK_DEFAULT;
	let inputDraft = String(TICK_DEFAULT);

	$: cfg = $configStore;
	$: syncTickFromStore(cfg?.tickIntervalSeconds ?? TICK_DEFAULT);

	function syncTickFromStore(v: number) {
		// Slider clamps to its own range; input shows the actual stored value.
		sliderValue = Math.min(TICK_SLIDER_MAX, Math.max(TICK_SLIDER_MIN, v));
		inputDraft = String(v);
	}

	function startEditTarget() {
		targetDraft = cfg?.appName ?? "";
		targetError = "";
		editingTarget = true;
	}

	async function saveTarget() {
		const trimmed = targetDraft.trim();
		if (!trimmed) {
			targetError = "Target cannot be empty.";
			return;
		}
		targetSaving = true;
		targetError = "";
		try {
			await bridge.setTargetApp(trimmed);
			const next = await bridge.getConfig();
			configStore.set(next);
			editingTarget = false;
		} catch (e) {
			targetError = String(e);
		} finally {
			targetSaving = false;
		}
	}

	function cancelTarget() {
		editingTarget = false;
		targetError = "";
	}

	async function persistTick(seconds: number) {
		// The backend clamps anything outside [1..999] to the default. We
		// always re-fetch so the UI reflects whatever actually got saved.
		tickSaving = true;
		tickError = "";
		try {
			await bridge.setTickInterval(seconds);
			const next = await bridge.getConfig();
			configStore.set(next);
		} catch (e) {
			tickError = String(e);
		} finally {
			tickSaving = false;
		}
	}

	function onSliderChange() {
		// Slider value is always in [TICK_SLIDER_MIN..TICK_SLIDER_MAX].
		if (cfg?.tickIntervalSeconds === sliderValue) return;
		persistTick(sliderValue);
	}

	function commitInput() {
		// maxlength=3 caps at 999; non-digit input is filtered onInput.
		const n = parseInt(inputDraft, 10);
		if (!Number.isFinite(n) || n < TICK_INPUT_MIN || n > TICK_INPUT_MAX) {
			// Out of range: backend will clamp to the default and the store
			// re-sync will redraw both controls.
			persistTick(TICK_DEFAULT);
			return;
		}
		if (cfg?.tickIntervalSeconds === n) return;
		persistTick(n);
	}

	function onInputKeydown(e: KeyboardEvent) {
		if (e.key === "Enter") {
			(e.target as HTMLInputElement).blur();
		}
	}

	function onInputDigitsOnly(e: Event) {
		const el = e.target as HTMLInputElement;
		// Strip anything non-numeric live so the user can't accumulate junk
		// past maxlength (e.g. paste). Keep empty intermediate state usable.
		const cleaned = el.value.replace(/[^0-9]/g, "").slice(0, 3);
		if (cleaned !== el.value) {
			el.value = cleaned;
			inputDraft = cleaned;
		}
	}

	async function openInEditor() {
		try {
			await bridge.openConfigInEditor();
		} catch (e) {
			tickError = String(e);
		}
	}

	function onTargetKeydown(e: KeyboardEvent) {
		if (e.key === "Enter") saveTarget();
		else if (e.key === "Escape") cancelTarget();
	}
</script>

<div class="settings" transition:slide={{ duration: 150 }}>
	<div class="settings-row">
		<span class="settings-label">Target</span>
		{#if editingTarget}
			<input
				class="settings-input"
				type="text"
				autocomplete="off"
				spellcheck="false"
				bind:value={targetDraft}
				on:keydown={onTargetKeydown}
				disabled={targetSaving}
			/>
			<button class="settings-btn" on:click={saveTarget} disabled={targetSaving}>
				{targetSaving ? "Saving…" : "Save"}
			</button>
			<button class="settings-btn ghost" on:click={cancelTarget}>Cancel</button>
		{:else}
			<span class="settings-value">{cfg?.appName || "(not set)"}</span>
			<button class="settings-btn" on:click={startEditTarget}>Edit</button>
		{/if}
	</div>
	{#if targetError}
		<p class="settings-error">{targetError}</p>
	{/if}

	<div class="settings-row">
		<span class="settings-label">Tick</span>
		<input
			type="range"
			class="tick-slider"
			min={TICK_SLIDER_MIN}
			max={TICK_SLIDER_MAX}
			step="1"
			bind:value={sliderValue}
			on:input={() => (inputDraft = String(sliderValue))}
			on:change={onSliderChange}
			disabled={tickSaving}
			title="Refresh cadence: {sliderValue}s"
		/>
		<input
			type="text"
			class="tick-input"
			inputmode="numeric"
			pattern="[0-9]{'{1,3}'}"
			maxlength="3"
			bind:value={inputDraft}
			on:input={onInputDigitsOnly}
			on:keydown={onInputKeydown}
			on:blur={commitInput}
			disabled={tickSaving}
			title="1..999 seconds; out of range resets to {TICK_DEFAULT}"
		/>
		<span class="tick-unit">s</span>
	</div>

	<div class="settings-row">
		<span class="settings-label">Config</span>
		<span class="settings-value path">{cfg?.path || ""}</span>
		<button class="settings-btn" on:click={openInEditor}>Edit</button>
	</div>

	<div class="settings-row">
		<span class="settings-label">Version</span>
		<span class="settings-value">{appVersion || "—"}</span>
	</div>

	<div class="settings-row">
		<span class="settings-label">Author</span>
		<span class="settings-value">
			Luis Palacios Derqui &mdash;
			<a
				href={REPO_URL}
				on:click|preventDefault={() => BrowserOpenURL(REPO_URL)}
			>github.com/LuisPalacios/ceftop</a>
		</span>
	</div>

	{#if cfg?.loadError}
		<div class="settings-row">
			<span class="settings-label warn">Load error</span>
			<span class="settings-value warn">{cfg.loadError}</span>
		</div>
	{/if}

	{#if tickError}
		<p class="settings-error">{tickError}</p>
	{/if}
</div>

<style>
	.settings {
		padding: 12px 16px;
		border-bottom: 1px solid var(--border);
		background: var(--bg-elevated);
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.settings-row {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	.settings-label {
		font-size: 11px;
		font-weight: 600;
		color: var(--fg-muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		width: 80px;
		flex-shrink: 0;
	}
	.settings-label.warn {
		color: var(--danger);
	}
	.settings-value {
		font-size: 12px;
		color: var(--fg);
		font-family: var(--font-mono);
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.settings-value.warn {
		color: var(--danger);
	}
	.settings-value.path {
		direction: rtl;
		text-align: left;
	}
	.settings-value a {
		color: var(--accent);
		text-decoration: none;
	}
	.settings-value a:hover {
		text-decoration: underline;
	}
	.settings-input {
		flex: 1;
		min-width: 0;
		font-family: var(--font-mono);
		font-size: 12px;
		padding: 4px 8px;
		border-radius: 4px;
		border: 1px solid var(--border);
		background: var(--bg);
		color: var(--fg);
	}
	.settings-input:focus {
		outline: 2px solid var(--accent);
		outline-offset: -1px;
	}
	.settings-btn {
		padding: 3px 10px;
		font-size: 11px;
		font-weight: 500;
		background: transparent;
		border: 1px solid var(--border);
		color: var(--fg);
		border-radius: 4px;
		cursor: pointer;
		white-space: nowrap;
	}
	.settings-btn:hover:not(:disabled) {
		background: var(--bg);
		border-color: var(--accent);
		color: var(--accent);
	}
	.settings-btn.ghost {
		color: var(--fg-muted);
	}
	.settings-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
	.tick-slider {
		flex: 1;
		min-width: 8rem;
		max-width: 24rem;
		accent-color: var(--accent);
		cursor: pointer;
	}
	.tick-slider:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.tick-input {
		width: 4rem;
		padding: 3px 8px;
		font-family: var(--font-mono);
		font-size: 12px;
		text-align: right;
		background: var(--bg);
		color: var(--fg);
		border: 1px solid var(--border);
		border-radius: 4px;
	}
	.tick-input:focus {
		outline: 2px solid var(--accent);
		outline-offset: -1px;
	}
	.tick-input:disabled {
		opacity: 0.5;
	}
	.tick-unit {
		font-family: var(--font-mono);
		font-size: 12px;
		color: var(--fg-muted);
	}
	.settings-error {
		margin: 0;
		padding: 0 0 0 92px;
		color: var(--danger);
		font-size: 11px;
	}
</style>
