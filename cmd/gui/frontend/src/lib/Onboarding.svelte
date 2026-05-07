<script lang="ts">
	import { bridge } from "./bridge";
	import { configStore } from "./stores";

	let appName = "";
	let busy = false;
	let error = "";

	async function submit() {
		if (busy) return;
		const trimmed = appName.trim();
		if (!trimmed) {
			error = "Enter the executable name (e.g. chrome, msedge, code).";
			return;
		}
		busy = true;
		error = "";
		try {
			await bridge.setTargetApp(trimmed);
			const next = await bridge.getConfig();
			configStore.set(next);
		} catch (e) {
			error = String(e);
		} finally {
			busy = false;
		}
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === "Enter") submit();
	}
</script>

<section class="onboarding">
	<h1>CefTop</h1>
	<p class="lede">
		Map the multi-process tree of any CEF, Electron, or Chromium app — and
		kill stuck workers without touching the rest.
	</p>

	<label for="appName">Target executable name</label>
	<input
		id="appName"
		type="text"
		autocomplete="off"
		spellcheck="false"
		placeholder="chrome, msedge, code"
		bind:value={appName}
		on:keydown={onKeydown}
		disabled={busy}
	/>
	<p class="hint">
		Basename only — case-insensitive, with or without <code>.exe</code>.
		The match is exact; CefTop watches every running instance.
	</p>

	<button on:click={submit} disabled={busy || !appName.trim()}>
		{busy ? "Saving…" : "Start monitoring"}
	</button>

	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}
</section>

<style>
	.onboarding {
		max-width: 30rem;
		margin: 4rem auto;
		padding: 2rem;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	h1 {
		font-size: 2rem;
		margin: 0;
		color: var(--accent);
		letter-spacing: -0.02em;
	}
	.lede {
		color: var(--fg-muted);
		margin: 0 0 1rem;
		line-height: 1.5;
	}
	label {
		font-weight: 600;
		font-size: 0.85rem;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--fg-muted);
	}
	input {
		font-size: 1rem;
		padding: 0.6rem 0.75rem;
		border-radius: 6px;
		border: 1px solid var(--border);
		background: var(--bg-elevated);
		color: var(--fg);
		font-family: inherit;
	}
	input:focus {
		outline: 2px solid var(--accent);
		outline-offset: -1px;
	}
	.hint {
		color: var(--fg-muted);
		font-size: 0.85rem;
		margin: 0;
	}
	.hint code {
		background: var(--bg-elevated);
		padding: 0 0.25rem;
		border-radius: 3px;
	}
	button {
		margin-top: 1rem;
		align-self: flex-start;
		padding: 0.6rem 1.2rem;
		border-radius: 6px;
		border: none;
		background: var(--accent);
		color: var(--bg);
		font-weight: 600;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.error {
		color: var(--danger);
		margin-top: 0.5rem;
		font-size: 0.9rem;
	}
</style>
