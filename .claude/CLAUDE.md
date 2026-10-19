# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

CefTop is a cross-platform Wails v2 + Go + Svelte desktop monitor for CEF / Electron / Chromium-style multi-process applications. The repo is in early bootstrap: the stock Wails scaffold is in place under `cmd/gui/`, but no CefTop business logic has been written yet. The validated behavior baseline lives at the repo root in `ceftop-cli.ps1` (PowerShell prototype) plus `ceftop-cli.txt` / `ceftop-cli.png` (sample output). The Go backend must reproduce that prototype's semantics cross-platform.

Two reference documents take precedence over inferred patterns:

- `context.md` — product brief: features, config schema, target acquisition, role parsing, kill semantics.
- `gitbox/` — read-only reference clone of an analogous Wails + Go + Svelte project. Borrow its architecture; never edit its files. It is gitignored and is its own git checkout.

## Dual Claude / Codex setup

This file is the canonical agent guidance. `AGENTS.md` at the repo root is a symlink to `.claude/CLAUDE.md` so Codex (which discovers `AGENTS.md` at the project root) reads the same content. Skills under `.claude/skills/` are exposed to Codex through `.agents/skills` (symlink to `../.claude/skills`). See [`.claude/context/guideline_codex.md`](context/guideline_codex.md) for the repair procedure if the symlinks ever go missing or get checked out as plain files.

## Continuity ledger

Last updated: 2026-10-19 (WebView2 host discovery + `pkg/catalog` app catalog; earlier: fuzzy icon matching, releases v1.0.0–v1.0.2)

- Goal: bring CefTop to feature parity with the validated PowerShell prototype on Windows / macOS / Linux, behind a Wails v2 + Svelte GUI. Current thread: grow the bundled icon set (`assets/APP-ICON-SPEC.md` is the design contract; the user downloads vendor SVGs, the agent normalizes them to spec).
- State: released (v1.0.2), usable end-to-end. Icon resolution is backend-side: `pkg/icons` scores process names against `app-<key>.svg` keys (bundled set listed from the embedded `frontend/dist/app-icons`, private set from the config dir); `DiscoverApps` returns `DiscoveredAppView` with `IconSrc` and `FriendlyName` pre-resolved and `DescribeApp(name)` (name + icon) serves the tree header. Display names come from `pkg/catalog` (embedded `apps.json` + optional user `apps.json` next to `ceftop.json`, then the exe's Windows version resource, then a heuristic; the frontend's `friendlyName.ts` is gone). WebView2 host discovery (2026-10-19): `pkg/process.resolveHost` sets `ProcessNode.Host`/`HostPID` from `--webview-exe-name=` or, for embedded / catalog-`multiGroup` mains, the parent process; `App.Snapshot` fills `HostLabel`; `App.Snapshot` labels via `Catalog.HostName(runtime, host, pid)`: the runtime entry's `hosts` map (exact key, then longest glob such as `whatsapp*`), then `FriendlyName`. `roleTheme.roleLabel` renders `Main / <host>`, `ProcessTree` widens the Type column (`--role-col`) to fit, and the host name is editable in place (`RenameHost` binding writes the user's `apps.json` via `catalog.SetHostName`). Settings → App names → Edit (`OpenCatalogInEditor`) seeds a missing `apps.json` with the last scan's hosts. Discovery is dynamic; `hosts` only refines names. Backend bindings: `GetConfig`, `SetTargetApp`, `SetTickInterval`, `OpenConfigInEditor`, `Snapshot`, `KillProcess`, `DiscoverApps`, `DescribeApp`, `RenameHost`, `OpenCatalogInEditor`, `GetAppVersion`, `FrontendReady`, `Log`. `pkg/config` always materializes every supported key on disk: `omitempty` is off, `Normalize()` fills defaults, `Load` rewrites when something changed. Snapshot ticker resets on the fly via a buffered `intervalChange` channel. Frontend: gear-button → `Settings.svelte` panel mirroring gitbox (Target / Tick presets / Config path + Edit button); `StatusBar` lets users click TARGET and TICK values to inline-edit. Ctrl+Plus / Ctrl+Minus / Ctrl+0 zoom the UI (persisted in `localStorage["ceftop:zoom"]`). Window size (2026-10-19): fixed 950×1000 outer (`windowWidth`/`windowHeight` consts in `main.go`, 936×993 visible on Win11; base font 13.5px at zoom 1.0), `DisableResize: true` (no drag, no maximize; Wails scales it by monitor DPI so units are CSS px at any scale). Never larger than the monitor: `App.svelte` polls `screen.availWidth/availHeight` every 1 s and calls `FitWindowToScreen`, which caps the size to the work area (re-centring when capped) and grows it back on a bigger monitor. The old auto-fit pipeline (20-row height lock, scale calibration, `ceftop:rowmetrics`, `WindowSetSize/MinSize/MaxSize/GetSize` bindings) is gone. Tree columns (`ProcessTree.svelte` `COLUMN_WIDTHS`): design widths Process 250 · Type 250 · Thr 70 · Memory 150 · CPU 90 · Age 110 px used as `minmax(0, Nfr)` shares (no kill column), all in one `--tree-cols` shared by the header and every row, so the six columns always fill the pane exactly and never exceed the window at any zoom (~241/241/67/144/87/106 px at zoom 1). Only the Process column scrolls horizontally: when the deepest row's measured `.left-inner` is wider than the column, a thin scrollbar appears under its sticky title (Shift + wheel works too) and drives `--proc-scroll`, which slides every row's Process content together; the other columns never move. Kill lives in the empty right side of the Process cell (absolute, outside the sliding `.left-inner`), shown on row hover, and takes two clicks: the first arms it (red "kill?", 2 s, disarmed on mouseleave/blur), the second calls `KillProcess`. Header and rows must keep the same tracks (an `auto` track once misaligned them). Lesson: an earlier fixed 63rem layout pushed CPU/Age off-screen at the user's Ctrl+ 1.4× zoom. `.tree-pane` scrolls vertically; the column header row is sticky. Top bar is a `1fr auto 1fr` grid: brand left, the monitored app's icon (2.5rem) + friendly name centred (config-driven, so it flips before the new tree arrives), action buttons right; the tree's first column header is a plain "Process". Tests: `pkg/config` 13 incl. auto-fill, `pkg/process` 13. `npm run check` clean, `go test ./...` clean, `go build ./cmd/gui` produces ~7.3 MB binary. SVG app icon at `assets/appicon.svg` (PNG conversion still TODO for Wails build assets). Perf (2026-09-10): host enumeration goes through `pkg/process/enumerate_{windows,other}.go` — one Toolhelp walk on Windows instead of gopsutil's per-process `Ppid()`/`NumThreads()` (each was a full Toolhelp scan; a tick was ~3 s, now ~10 ms, discovery ~5 s → ~15 ms). `SetTargetApp` queues an immediate tick via `App.refresh`; `emitSnapshot` drops a snapshot whose target no longer matches config; `Monitor` resets CPU samples on target change. Neither loop emits at startup: the frontend calls the `FrontendReady` binding once its subscriptions are wired and the backend answers with an immediate snapshot + discovery emit (the old `bridge.snapshot()` / `bridge.discoverApps()` mount-time pulls are gone). `GopsutilProvider` caches the host enumeration pass for 100 ms so the two loops share one walk when they coincide. `ProcessNode.Cmdline` is `json:"-"` (frontend never read it; it was ~80 % of the tick payload).
- Next: Phase 5 (deferred polish) — window-state persistence, single-instance lock, PNG / .ico / .icns icon generation from `assets/appicon.svg`, CI workflow, ship script. Macros / keyboard shortcuts beyond zoom only when asked.
- Working set: `pkg/catalog/`, `pkg/config/`, `pkg/icons/`, `pkg/process/`, `cmd/gui/{main.go, app.go, hidewindow_*.go}`, `cmd/gui/frontend/src/{App.svelte, style.css, lib/*}`, `cmd/gui/frontend/wailsjs/` (generated), `assets/app-*.svg`, `assets/APP-ICON-SPEC.md`, `docs/working-plan-roadmap.md`, `.claude/CLAUDE.md`.

## Stack and module layout

- Wails v2.11 + Go 1.26.1 + Svelte 3 (svelte-ts template, Vite).
- Module: `github.com/LuisPalacios/ceftop`. `go.mod` lives at the repo root.
- GUI app under `cmd/gui/` (`main.go`, `app.go`, `wails.json`, `frontend/`, `build/`). Reusable domain code belongs under `pkg/` (planned, not yet created).
- `cmd/gui/frontend/wailsjs/` is auto-generated by `wails dev` / `wails build`. Wrap those generated bindings in `cmd/gui/frontend/src/lib/bridge.ts` rather than importing them from components.

## Common commands

```bash
# Frontend (run from cmd/gui/frontend/)
npm install                      # once after a fresh clone
npm run dev                      # vite dev server (used by `wails dev`)
npm run build                    # populates frontend/dist/, required before `go build ./cmd/gui`
npm run check                    # svelte-check type check
```

```bash
# Go (run from repo root)
go vet ./...
go build ./cmd/gui               # requires frontend/dist/ to exist (run npm run build first)
go test ./...                    # once pkg/ exists
go test ./pkg/<package>/...      # focused package tests
```

```bash
# Wails (run from cmd/gui/)
wails dev                        # live-reload dev loop, regenerates frontend bindings
wails build                      # full GUI build into cmd/gui/build/bin/
```

### Icon assets

`assets/` holds the canonical icon set (`logo.svg`, `appicon.png` 1024×1024, `favicon.ico` multi-size). Wails consumes its copies at fixed paths under `cmd/gui/build/`, which are gitignored as build-time artifacts. **Always refresh the copies before `wails build`** — running this once after a clone, and again whenever the source icons change, keeps the embedded app icon in sync:

```bash
cp assets/appicon.png cmd/gui/build/appicon.png
cp assets/favicon.ico cmd/gui/build/windows/icon.ico
```

The frontend favicon at `cmd/gui/frontend/public/favicon.ico` is tracked in git (Vite copies it to `dist/` on build).

A fresh `go build ./cmd/gui` will fail until `npm run build` has populated `cmd/gui/frontend/dist/`, because `main.go` does `//go:embed all:frontend/dist`. `dist/` is gitignored, so this is a per-clone setup step.

## Architecture intent (from AGENTS.md)

Business logic should land incrementally with the layout below. Stop for review at each subsystem boundary; do not pre-build packages.

- `cmd/gui/main.go` — Wails entrypoint: window options, embedded frontend assets, single-instance lock.
- `cmd/gui/app.go` — Wails `App` struct, lifecycle hooks (`Startup`, `Shutdown`, ...), exported bindings. Bindings stay thin and delegate domain work to `pkg/`.
- `pkg/config/` — schema, `Load`, `Save` for `~/.config/ceftop/ceftop.json` (XDG-aware on every platform; see `path.go`). Schema: `{"AppName": "<target>"}`.
- `pkg/catalog/` — app catalog (`apps.json` embedded + optional user `apps.json` in the config dir): friendly names, `multiGroup`, explicit icon keys; `FriendlyName` falls back to the OS exe description, then a heuristic.
- `pkg/process/` — cross-platform enumeration, `--type=` parsing, role normalization, snapshot assembly, kill. Use `github.com/shirou/gopsutil/v4/process` for enumeration and telemetry; reach for platform-specific code only where gopsutil cannot answer.
- `cmd/gui/frontend/src/lib/{bridge.ts, types.ts, stores.ts, roleTheme.ts}` — typed wrapper around generated bindings, shared DTOs, Svelte stores, and the role→color table that mirrors the prototype's `$RoleColors`.

Data flow: the backend emits an immutable, fully-built process tree on each tick; the frontend replaces the snapshot store rather than mutating it in place. Kill operations on the backend must return structured errors (permission denied, process gone, etc.) suitable for direct UI rendering.

## Behavior baseline (from `ceftop-cli.ps1`)

When the spec is ambiguous, the prototype is the source of truth:

- The "Main / Browser" process is whichever instance has no `--type=` flag in its command line.
- For every other process, extract `--type=([a-zA-Z0-9\-]+)` — the captured group is the role.
- Per-process telemetry: PID, PPID, thread count, RSS in MB.
- Roots are processes whose PPID is not in the current snapshot's PID set (this is what handles re-parented orphans correctly).
- Color palette and role labels live in `$RoleColors` — preserve them exactly when porting to `roleTheme.ts`.

## Implementation rules

From `AGENTS.md` — these override anything that contradicts them:

- Cross-platform first. The Windows WMI/CIM logic from the prototype must become portable Go logic; reserve `runtime.GOOS` gates for genuine platform-specific implementation details, not for required behavior.
- Config root is `~/.config/ceftop/ceftop.json` on every platform (honors `XDG_CONFIG_HOME`), mirroring the gitbox sibling project — see [`pkg/config/path.go`](../pkg/config/path.go).
- Keep process snapshots immutable on the frontend.
- Do not commit churn in generated `cmd/gui/frontend/wailsjs/` unless it tracks a real binding change in this same task.

## Verification expectations

- Go: run focused package tests first, then `go test ./...` when the change touches shared surface. Always `go vet ./...` before declaring done.
- Frontend: `npm run check` from `cmd/gui/frontend/` for TypeScript / Svelte changes.
- GUI: `wails build` from `cmd/gui/` for binding or window-option changes — `go build ./cmd/gui` alone does not exercise Wails' bind-generation pipeline.
