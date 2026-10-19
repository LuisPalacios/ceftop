<script lang="ts">
	import { slide } from "svelte/transition";
	import { bridge } from "./bridge";
	import { configStore, discoveredAppsStore } from "./stores";
	import { onIconError, BUNDLED_DEFAULT_ICON } from "./iconResolver";
	import type { DiscoveredApp } from "./types";

	$: cfg = $configStore;
	$: apps = $discoveredAppsStore ?? [];

	// Tooltip: the exe, how busy it is and, for an embedded runtime such as
	// WebView2, which apps it is drawing for right now.
	function tooltip(app: DiscoveredApp): string {
		const head =
			app.childCount === 0
				? `${app.name} — not running`
				: app.childCount === 1
					? `${app.name} — 1 child`
					: `${app.name} — ${app.childCount} children`;
		const hosts = app.hosts ?? [];
		if (hosts.length === 0) return head;
		return `${head}\nHosting: ${hosts.map((h) => h.label).join(", ")}`;
	}

	let switching = "";

	// Picking an app switches the target and leaves the bar open: it is a
	// docked toolbar, not a dropdown, and hopping between apps to compare
	// them is the point. It closes only from the header toggle or Esc (see
	// the panel section in App.svelte); the active item shows what is
	// monitored and the tree below swaps in place.
	async function pick(app: DiscoveredApp) {
		if (!app.name) return;
		if (cfg?.appName === app.name) return;
		switching = app.name;
		try {
			await bridge.setTargetApp(app.name);
			const next = await bridge.getConfig();
			configStore.set(next);
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
				class:offline={app.childCount === 0}
				on:click={() => pick(app)}
				disabled={switching !== "" && switching !== app.name}
				title={tooltip(app)}
			>
				<img
					class="apps-icon"
					src={app.iconSrc || BUNDLED_DEFAULT_ICON}
					alt=""
					on:error={onIconError}
				/>
				<span class="apps-label">{app.friendlyName || app.name}</span>
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
	.apps-item.offline .apps-icon {
		filter: grayscale(1);
		opacity: 0.55;
	}
	.apps-item.offline.active .apps-icon {
		filter: none;
		opacity: 1;
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
