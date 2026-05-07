<script lang="ts">
	import { slide } from "svelte/transition";
	import { bridge } from "./bridge";
	import {
		configStore,
		discoveredAppsStore,
		showDiscoveredApps,
		privateIconsStore,
	} from "./stores";
	import { resolveIconSrc, makeIconErrorHandler } from "./iconResolver";
	import { friendlyName } from "./friendlyName";
	import type { DiscoveredApp } from "./types";

	$: cfg = $configStore;
	$: apps = $discoveredAppsStore ?? [];
	$: privates = $privateIconsStore;
	$: onIconError = makeIconErrorHandler(privates);

	let switching = "";

	async function pick(app: DiscoveredApp) {
		if (!app.name) return;
		if (cfg?.appName === app.name) return;
		switching = app.name;
		try {
			await bridge.setTargetApp(app.name);
			const next = await bridge.getConfig();
			configStore.set(next);
			// Auto-collapse so the user lands back on the tree view.
			showDiscoveredApps.set(false);
		} catch {
			// Fall through silently — the StatusBar / Settings target editor
			// remains as the manual fallback path.
		} finally {
			switching = "";
		}
	}
</script>

{#if apps.length > 0}
	<div class="apps-bar" transition:slide={{ duration: 150 }}>
		{#each apps as app (app.name)}
			<button
				class="apps-item"
				class:active={cfg?.appName === app.name}
				class:busy={switching === app.name}
				on:click={() => pick(app)}
				disabled={switching !== "" && switching !== app.name}
				title={app.childCount === 1
					? `${app.name} — 1 child`
					: `${app.name} — ${app.childCount} children`}
			>
				<img
					class="apps-icon"
					src={resolveIconSrc(app.name, privates)}
					alt=""
					on:error={onIconError}
				/>
				<span class="apps-label">{friendlyName(app.name)}</span>
			</button>
		{/each}
	</div>
{/if}

<style>
	.apps-bar {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		padding: 0.5rem 1rem;
		border-bottom: 1px solid var(--border);
		background: var(--bg-elevated);
		flex-shrink: 0;
	}
	.apps-item {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.15rem;
		padding: 0.35rem 0.5rem;
		min-width: 4rem;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 6px;
		color: var(--fg);
		cursor: pointer;
		transition: background 0.12s, border-color 0.12s;
	}
	.apps-item:hover:not(:disabled) {
		background: var(--bg);
		border-color: var(--border);
	}
	.apps-item.active {
		border-color: var(--accent);
		background: var(--bg);
	}
	.apps-item.active .apps-label {
		color: var(--accent);
	}
	.apps-item:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.apps-item.busy {
		opacity: 0.7;
	}
	.apps-icon {
		width: 1.5rem;
		height: 1.5rem;
		display: block;
		pointer-events: none;
	}
	.apps-label {
		font-family: var(--font-mono);
		font-size: 10px;
		color: var(--fg-muted);
		max-width: 8rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
