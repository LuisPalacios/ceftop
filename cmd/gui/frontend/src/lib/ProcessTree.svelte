<script lang="ts">
	import { snapshotStore, snapshotErrorStore, privateIconsStore } from "./stores";
	import { resolveIconSrc, makeIconErrorHandler } from "./iconResolver";
	import { friendlyName } from "./friendlyName";
	import ProcessNode from "./ProcessNode.svelte";
	import type { ProcessNode as PNode } from "./types";

	// Max nesting depth across the snapshot. Used to size the PID column
	// just wide enough for the deepest indent — for a flat tree (depth=0)
	// this saves ~6rem of horizontal space versus the previous fixed 12.5rem.
	function maxTreeDepth(nodes: PNode[] | null | undefined, current = 0): number {
		if (!nodes || nodes.length === 0) return current;
		let m = current;
		for (const n of nodes) {
			if (n.children && n.children.length > 0) {
				m = Math.max(m, maxTreeDepth(n.children, current + 1));
			}
		}
		return m;
	}

	// When the tree is fully flat (no node has any children), the
	// expand/collapse toggle is unused dead weight — drop it entirely.
	// Saves ~1.65 rem on the left column for the single-process case.
	$: depth = maxTreeDepth($snapshotStore?.roots);
	$: showToggle = depth > 0;
	// Base width: toggle (1.25) + gap (0.4) + "[123456]" PID (~5rem) = 6.65,
	// or just PID without the toggle gap = 5rem. Plus 1rem per nesting level.
	// Also widen to fit the target name + 24px icon in the header row so long
	// names like "msedgewebview2" don't crash into the Type column.
	$: target = $snapshotStore?.target ?? "";
	$: targetLabel = friendlyName(target);
	$: targetMinRem = targetLabel.length * 0.5 + 2.5;
	$: pidMinRem = (showToggle ? 6.65 : 5) + depth;
	$: leftCol = `${Math.max(pidMinRem, targetMinRem)}rem`;

	$: privates = $privateIconsStore;
	$: onIconError = makeIconErrorHandler(privates);
</script>

{#if $snapshotErrorStore}
	<div class="banner error">{$snapshotErrorStore}</div>
{/if}

{#if !$snapshotStore}
	<p class="placeholder">Waiting for first snapshot…</p>
{:else if !$snapshotStore.roots || $snapshotStore.roots.length === 0}
	<p class="placeholder">
		No running instances of <code>{$snapshotStore.target}</code>.
	</p>
{:else}
	<div class="tree" style="--left-col: {leftCol}">
		<div class="header-row">
			<span class="header-target" title={target}>
				<img
					class="header-target-icon"
					src={resolveIconSrc(target, privates)}
					alt=""
					on:error={onIconError}
				/>
				<span class="header-target-name">{targetLabel}</span>
			</span>
			<span>Type</span>
			<span class="num">Thr</span>
			<span class="num">Memory</span>
			<span class="num">CPU</span>
			<span class="num">Age</span>
			<span></span>
		</div>
		{#each $snapshotStore.roots as root (root.pid)}
			<ProcessNode node={root} depth={0} {showToggle} />
		{/each}
	</div>
{/if}

<style>
	.banner.error {
		background: var(--danger);
		color: var(--bg);
		padding: 0.5rem 0.75rem;
		font-family: var(--font-mono);
		font-size: 0.85rem;
	}
	.placeholder {
		padding: 2rem;
		text-align: center;
		color: var(--fg-muted);
		font-style: italic;
	}
	.placeholder code {
		font-style: normal;
		background: var(--bg-elevated);
		padding: 0 0.25rem;
		border-radius: 3px;
	}
	.tree {
		padding: 0.5rem 0;
		/* Shrink-wrap the tree to its widest row so the App-level fit
		   measurement gets a stable, parent-independent natural width.
		   Without this, .tree stretches to fill .tree-pane, scrollWidth
		   reads the parent's width, and the fit feedback-loops between
		   two sizes every snapshot tick. */
		width: max-content;
	}
	/* Header row uses the SAME grid template as ProcessNode .row so the
	   column titles line up exactly with the per-row values. If you tweak
	   the row template in ProcessNode.svelte, mirror it here. */
	.header-row {
		display: grid;
		grid-template-columns: var(--left-col, 12.5rem) 10rem 3rem 6rem 3rem 5rem auto;
		gap: 0.4rem;
		padding: 0.25rem 0.4rem 0.5rem 0.4rem;
		margin-bottom: 0.25rem;
		border-bottom: 1px solid var(--border);
		font-family: var(--font-mono);
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--header);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		min-width: max-content;
		align-items: center;
	}
	.header-target {
		text-transform: none;
		letter-spacing: 0;
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		min-width: 0;
	}
	.header-target-icon {
		width: 24px;
		height: 24px;
		display: block;
		flex-shrink: 0;
		pointer-events: none;
	}
	.header-target-name {
		font-size: 0.78rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.header-row .num {
		justify-self: end;
	}
</style>
