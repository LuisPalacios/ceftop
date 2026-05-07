<script lang="ts">
	import { onMount, onDestroy, tick } from "svelte";
	import { bridge } from "./bridge";
	import {
		configStore,
		snapshotStore,
		lastUpdateAtStore,
		killFlashStore,
	} from "./stores";

	const FLASH_DURATION_MS = 4000;

	// Drives a 1Hz refresh so "updated Ns ago" stays fresh without a separate
	// timer in every consumer.
	let now = Date.now();
	let interval: ReturnType<typeof setInterval>;
	onMount(() => {
		interval = setInterval(() => (now = Date.now()), 500);
	});
	onDestroy(() => clearInterval(interval));

	$: secondsAgo = $lastUpdateAtStore
		? Math.max(0, Math.floor((now - $lastUpdateAtStore) / 1000))
		: -1;

	$: total = $snapshotStore?.total ?? 0;
	$: target = $snapshotStore?.target || $configStore?.appName || "—";
	$: tickSeconds = $configStore?.tickIntervalSeconds ?? 2;

	$: flash = $killFlashStore;
	$: flashAge = flash ? now - flash.at : Infinity;
	$: showFlash = flash && flashAge < FLASH_DURATION_MS;
	$: flashLabel = (() => {
		if (!flash) return "";
		const { result, pid, role } = flash;
		if (result.killed) return `killed [${pid}] ${role}`;
		const code = result.errno || "ERR";
		return `${code}: could not kill [${pid}] ${role}`;
	})();

	// Inline target edit
	let editingTarget = false;
	let targetDraft = "";
	let targetSaving = false;
	let targetError = "";
	let targetInputEl: HTMLInputElement | undefined;

	async function startEditTarget() {
		targetDraft = $configStore?.appName ?? "";
		targetError = "";
		editingTarget = true;
		await tick();
		targetInputEl?.select();
	}

	async function saveTarget() {
		const trimmed = targetDraft.trim();
		if (!trimmed) {
			targetError = "empty";
			return;
		}
		targetSaving = true;
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

	function targetKey(e: KeyboardEvent) {
		if (e.key === "Enter") saveTarget();
		else if (e.key === "Escape") cancelTarget();
	}

	// Inline tick edit
	let editingTick = false;
	let tickDraft = "2";
	let tickSaving = false;
	let tickError = "";
	let tickInputEl: HTMLInputElement | undefined;

	async function startEditTick() {
		tickDraft = String(tickSeconds);
		tickError = "";
		editingTick = true;
		await tick();
		tickInputEl?.select();
	}

	async function saveTick() {
		const n = Number(tickDraft);
		if (!Number.isFinite(n) || n <= 0 || !Number.isInteger(n)) {
			tickError = "1+";
			return;
		}
		tickSaving = true;
		try {
			await bridge.setTickInterval(n);
			const next = await bridge.getConfig();
			configStore.set(next);
			editingTick = false;
		} catch (e) {
			tickError = String(e);
		} finally {
			tickSaving = false;
		}
	}

	function cancelTick() {
		editingTick = false;
		tickError = "";
	}

	function tickKey(e: KeyboardEvent) {
		if (e.key === "Enter") saveTick();
		else if (e.key === "Escape") cancelTick();
	}
</script>

<div class="bar">
	<span class="metric target">
		<span class="label">target</span>
		{#if editingTarget}
			<input
				bind:this={targetInputEl}
				bind:value={targetDraft}
				on:keydown={targetKey}
				on:blur={saveTarget}
				disabled={targetSaving}
				class="inline-input"
				autocomplete="off"
				spellcheck="false"
			/>
			{#if targetError}<span class="inline-error" title={targetError}>!</span>{/if}
		{:else}
			<button class="value-button" on:click={startEditTarget} title="Click to change target">
				<code>{target}</code>
			</button>
		{/if}
	</span>

	<span class="metric">
		<span class="label">processes</span>
		{total}
	</span>

	<span class="metric">
		<span class="label">tick</span>
		{#if editingTick}
			<input
				bind:this={tickInputEl}
				bind:value={tickDraft}
				on:keydown={tickKey}
				on:blur={saveTick}
				disabled={tickSaving}
				class="inline-input narrow"
				type="number"
				min="1"
				step="1"
			/>s
			{#if tickError}<span class="inline-error" title={tickError}>!</span>{/if}
		{:else}
			<button class="value-button" on:click={startEditTick} title="Click to change tick interval">
				{tickSeconds}s
			</button>
		{/if}
	</span>

	<span class="metric updated">
		{#if secondsAgo < 0}
			<span class="label">waiting…</span>
		{:else}
			<span class="label">updated</span>
			{secondsAgo}s ago
		{/if}
	</span>

	{#if showFlash && flash}
		<span class="flash" class:bad={!flash.result.killed} role="status">
			{flashLabel}
		</span>
	{/if}
</div>

<style>
	.bar {
		display: flex;
		align-items: center;
		gap: 1.25rem;
		padding: 0.5rem 1rem;
		background: var(--bg-elevated);
		border-top: 1px solid var(--border);
		font-family: var(--font-mono);
		font-size: 0.8rem;
		color: var(--fg);
		flex-shrink: 0;
	}
	.metric {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		white-space: nowrap;
	}
	.label {
		color: var(--fg-muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		font-size: 0.7rem;
	}
	code {
		color: var(--accent);
	}
	.value-button {
		background: transparent;
		border: 1px dashed transparent;
		border-radius: 3px;
		padding: 1px 4px;
		margin: -1px -4px;
		color: inherit;
		font: inherit;
		cursor: pointer;
	}
	.value-button:hover {
		border-color: var(--border);
		background: var(--bg);
	}
	.inline-input {
		font-family: var(--font-mono);
		font-size: 0.8rem;
		padding: 1px 4px;
		border-radius: 3px;
		border: 1px solid var(--accent);
		background: var(--bg);
		color: var(--fg);
		min-width: 8rem;
	}
	.inline-input.narrow {
		min-width: 0;
		width: 4rem;
	}
	.inline-input:focus {
		outline: none;
	}
	.inline-error {
		color: var(--danger);
		font-weight: 700;
		margin-left: 0.25rem;
		cursor: help;
	}
	.updated {
		margin-left: auto;
	}
	.flash {
		padding: 0.15rem 0.6rem;
		border-radius: 3px;
		background: var(--accent);
		color: var(--bg);
		font-weight: 600;
	}
	.flash.bad {
		background: var(--danger);
	}
</style>
