<script lang="ts">
	import { afterUpdate } from "svelte";
	import { snapshotStore, snapshotErrorStore } from "./stores";
	import ProcessNode from "./ProcessNode.svelte";
	import type { ProcessNode as PNode } from "./types";

	// Every column is always on screen inside the fixed 950-wide window, at
	// any zoom. Each column has a design width (px at the default zoom):
	//
	//   Process 250 · Type 250 · Thr 70 · Memory 150 · CPU 90 · Age 110
	//
	// used as its share of the row: the six columns split whatever the gaps
	// leave, in those proportions, so together they always fill the pane
	// exactly and never exceed it (~241 · 241 · 67 · 144 · 87 · 106 px in
	// the 950 window at the default zoom). minmax(0, …) forbids any column
	// from growing past its share: text that does not fit ellipsizes (Type)
	// and the values in Thr / Memory / CPU / Age fit with room to spare even
	// at the maximum zoom.
	//
	// Process is the one column whose content can outgrow it: each nesting
	// level indents the PID by INDENT_REM. When the deepest row does not fit,
	// that column alone becomes scrollable: a thin scrollbar appears under
	// its title (sticky, so always reachable) and Shift + wheel over the
	// column works too. Scrolling sets --proc-scroll, which slides the
	// Process content of every row together (see .left-inner in
	// ProcessNode.svelte); the other columns never move.
	const INDENT_REM = 0.6;
	const COLUMN_WIDTHS = [250, 250, 70, 150, 90, 110]; // Process … Age
	// No kill column: the kill button lives in the empty right side of the
	// Process cell (see ProcessNode.svelte), so the six columns get the
	// whole row. Header and rows have the same six tracks, which keeps every
	// title aligned with its values.
	const TREE_COLS = COLUMN_WIDTHS.map((w) => `minmax(0, ${w}fr)`).join(" ");

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
	$: depth = maxTreeDepth($snapshotStore?.roots);
	$: showToggle = depth > 0;

	let treeEl: HTMLDivElement | undefined;
	let scrollerEl: HTMLDivElement | undefined;
	let procColPx = 0; // the Process column's width (bound to its header cell)
	let procContentPx = 0; // the widest row's Process content
	let procScroll = 0; // how far the Process column is scrolled

	// Measure the real content after every render rather than estimating it
	// from the depth: exact at any zoom and for any PID length. Assigning
	// only on change keeps this from re-triggering itself.
	afterUpdate(() => {
		if (!treeEl) return;
		let widest = 0;
		for (const el of treeEl.querySelectorAll<HTMLElement>(".left-inner")) {
			widest = Math.max(widest, el.offsetWidth);
		}
		if (widest !== procContentPx) procContentPx = widest;
	});

	$: procOverflow = Math.max(0, Math.ceil(procContentPx - procColPx));
	$: if (procScroll > procOverflow) procScroll = procOverflow;

	function onProcScroll() {
		procScroll = scrollerEl?.scrollLeft ?? 0;
	}

	// Shift + wheel (or a horizontal trackpad swipe) over the Process column
	// scrolls it; everywhere else the wheel keeps scrolling the rows.
	function onWheel(e: WheelEvent) {
		if (procOverflow <= 0 || !scrollerEl) return;
		const target = e.target as HTMLElement | null;
		if (!target?.closest(".left, .proc-head")) return;
		const dx = e.deltaX || (e.shiftKey ? e.deltaY : 0);
		if (!dx) return;
		e.preventDefault();
		scrollerEl.scrollLeft += dx;
	}
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
	<div
		class="tree"
		bind:this={treeEl}
		style="--tree-cols: {TREE_COLS}; --indent: {INDENT_REM}rem; --proc-scroll: {procScroll}px"
		on:wheel|nonpassive={onWheel}
	>
		<div class="header-row">
			<div class="proc-head" bind:clientWidth={procColPx}>
				<span>Process</span>
				{#if procOverflow > 0}
					<div
						class="proc-scroll"
						bind:this={scrollerEl}
						on:scroll={onProcScroll}
						title="Scroll the Process column (or Shift + wheel over it)"
					>
						<div style="width: {procContentPx}px; height: 1px"></div>
					</div>
				{/if}
			</div>
			<span>Type</span>
			<span class="num">Thr</span>
			<span class="num">Memory</span>
			<span class="num">CPU</span>
			<span class="num">Age</span>
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
	}
	/* Header row uses the SAME grid template as ProcessNode .row (both read
	   --tree-cols from .tree) so the column titles line up exactly with the
	   per-row values. It sticks to the top of the pane while the rows
	   scroll under it, so the titles and the Process scrollbar never leave
	   the screen. */
	.header-row {
		display: grid;
		grid-template-columns: var(--tree-cols);
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
		align-items: center;
		position: sticky;
		top: 0;
		z-index: 1;
		background: var(--bg);
	}
	.header-row .num {
		justify-self: end;
	}
	.proc-head {
		min-width: 0;
	}
	/* A narrow Process column clips its title instead of letting it run
	   over Type's. */
	.proc-head > span {
		display: block;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.proc-scroll {
		margin-top: 0.2rem;
		overflow-x: auto;
		overflow-y: hidden;
	}
	.proc-scroll::-webkit-scrollbar {
		height: 6px;
	}
	.proc-scroll::-webkit-scrollbar-thumb {
		background: var(--border);
		border-radius: 3px;
	}
	.proc-scroll::-webkit-scrollbar-thumb:hover {
		background: var(--accent);
	}
</style>
