<script lang="ts">
	import { bridge } from "./bridge";
	import { killFlashStore } from "./stores";
	import { colorForNode } from "./roleTheme";
	import type { ProcessNode } from "./types";

	export let node: ProcessNode;
	export let depth = 0;
	// Tree-wide flag from ProcessTree: when no node anywhere has children,
	// the expand/collapse toggle is omitted entirely so flat trees don't
	// reserve space for an interaction that's never available.
	export let showToggle = true;

	let expanded = true;
	let killing = false;

	$: roleColor = colorForNode(node);
	$: hasChildren = !!node.children && node.children.length > 0;

	// CPU% formatter: always one decimal. Non-consuming / first-tick / NaN
	// all collapse to "0.0" so the column never renders an em-dash placeholder
	// (the visual ambiguity made it look like the column was failing to load).
	function fmtCpuPct(pct: number): string {
		if (!Number.isFinite(pct) || pct <= 0) return "0.0";
		return pct.toFixed(1);
	}

	// CPU time formatter — always HH:MM:SS, no sub-second precision.
	// Total CPU time consumed by the process since start (user + system),
	// matching Process Explorer's "CPU Time" column. Note: this is NOT
	// wall-clock age; an idle process barely ticks while a busy one accrues.
	function fmtCpuTime(ms: number): string {
		if (!Number.isFinite(ms) || ms <= 0) return "00:00:00";
		const totalSec = Math.floor(ms / 1000);
		const hours = Math.floor(totalSec / 3600);
		const minutes = Math.floor((totalSec % 3600) / 60);
		const seconds = totalSec % 60;
		const pad = (n: number) => n.toString().padStart(2, "0");
		return `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`;
	}

	// Memory thresholds (MB): <250 green, 250–450 yellow, ≥450 red.
	$: memClass =
		!Number.isFinite(node.memMB) || node.memMB <= 0
			? ""
			: node.memMB < 250
				? "good"
				: node.memMB < 450
					? "warn"
					: "bad";

	// CPU thresholds (%): <15 default, 15–50 yellow, ≥50 red.
	$: cpuClass =
		!Number.isFinite(node.cpuPercent) || node.cpuPercent < 15
			? ""
			: node.cpuPercent < 50
				? "warn"
				: "bad";

	async function kill() {
		if (killing) return;
		killing = true;
		try {
			const result = await bridge.killProcess(node.pid);
			killFlashStore.set({
				pid: node.pid,
				role: node.role,
				result,
				at: Date.now(),
			});
		} catch (e) {
			killFlashStore.set({
				pid: node.pid,
				role: node.role,
				result: { killed: false, err: String(e) },
				at: Date.now(),
			});
		} finally {
			killing = false;
		}
	}
</script>

<div class="row" style="--depth: {depth}">
	<div class="left">
		{#if depth > 0}
			<span class="indent" aria-hidden="true"></span>
		{/if}
		{#if showToggle}
			<button
				class="toggle"
				class:invisible={!hasChildren}
				on:click={() => (expanded = !expanded)}
				aria-label={expanded ? "Collapse" : "Expand"}
				tabindex={hasChildren ? 0 : -1}
			>
				{expanded ? "▾" : "▸"}
			</button>
		{/if}
		<span class="pid">[{node.pid}]</span>
	</div>

	<span class="role" style="color: {roleColor}">{node.role}</span>
	<span class="metric">{node.threads}</span>
	<span class="metric {memClass}">{node.memMB.toFixed(2)} MB</span>
	<span class="metric {cpuClass}"><span class="num cpu-num">{fmtCpuPct(node.cpuPercent)}</span>%</span>
	<span class="metric">{fmtCpuTime(node.cpuTimeMs)}</span>

	<button class="kill" on:click={kill} disabled={killing} title="Terminate PID {node.pid}">
		{killing ? "…" : "kill"}
	</button>
</div>

{#if expanded && hasChildren}
	{#each node.children ?? [] as child (child.pid)}
		<svelte:self node={child} depth={depth + 1} {showToggle} />
	{/each}
{/if}

<style>
	/* Every row uses the SAME explicit grid template, so columns line up
	   across nesting levels regardless of how the left region is filled.
	   The `--depth` CSS variable drives the indent inside the left column
	   without disturbing the rest of the row. */
	.row {
		display: grid;
		grid-template-columns: var(--left-col, 12.5rem) 10rem 3rem 6rem 3rem 5rem auto;
		gap: 0.4rem;
		align-items: center;
		padding: 0.25rem 0.4rem;
		font-family: var(--font-mono);
		font-size: 0.875rem;
		border-bottom: 1px solid var(--bg-elevated);
		min-width: max-content;
	}
	.row:hover {
		background: var(--bg-elevated);
	}

	.left {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		min-width: 0;
		overflow: hidden;
	}
	.indent {
		display: inline-block;
		width: calc(var(--depth, 0) * 1rem);
		flex-shrink: 0;
	}
	.toggle {
		width: 1.25rem;
		height: 1.25rem;
		padding: 0;
		border: none;
		background: transparent;
		color: var(--fg-muted);
		cursor: pointer;
		font-size: 0.75rem;
		flex-shrink: 0;
	}
	.toggle.invisible {
		visibility: hidden;
		cursor: default;
	}
	.pid {
		color: var(--fg-muted);
		flex-shrink: 0;
	}

	.role {
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.metric {
		color: var(--fg);
		white-space: nowrap;
		/* Position the span at the right edge of its grid cell. text-align
		   alone wouldn't work — the span is content-sized as a grid item,
		   so it sits at justify-self: start by default and the "right
		   alignment" is invisible. Pinning justify-self: end pushes the
		   value flush against the column's right edge, so the only space
		   between the AGE digits and the kill button is the 0.4rem grid
		   gap (plus the kill button's own 0.35rem left padding). */
		justify-self: end;
	}
	.metric.good {
		color: var(--accent);
	}
	.metric.warn {
		color: var(--warn);
	}
	.metric.bad {
		color: var(--danger);
	}
	/* Right-aligned fixed-width number so single-digit and double-digit values
	   share the same trailing edge before their unit suffix. The mono font
	   means 0.55em per glyph; "100.0" needs five glyphs ≈ 2.75em + breathing. */
	.metric .num {
		display: inline-block;
		min-width: 2.75em;
		text-align: right;
	}

	.kill {
		min-width: 2.5rem;
		padding: 0.1rem 0.35rem;
		border-radius: 3px;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--danger);
		font-family: inherit;
		font-size: 0.75rem;
		text-align: center;
		cursor: pointer;
		opacity: 0;
		transition: opacity 0.1s;
	}
	.row:hover .kill,
	.kill:focus {
		opacity: 1;
	}
	.kill:hover:not(:disabled) {
		background: var(--danger);
		color: var(--bg);
		border-color: var(--danger);
	}
	.kill:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
</style>
