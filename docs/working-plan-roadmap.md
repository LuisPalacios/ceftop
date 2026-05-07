# Working plan and roadmap

Living implementation plan for CefTop while the repository is pre-publication. It records the phase-by-phase work, scope per phase, files touched, exit criteria, decisions taken, and questions still open. Future Claude Code or Codex sessions should treat it as the canonical answer to "what is the next thing to build, and why".

Last updated: 2026-05-06 (Phases 1–4 complete + post-MVP iteration pass).

> [!NOTE]
> This document retires when the repository moves to GitHub with issues enabled. At that point each open phase and each open question becomes an issue, and `readme.md` swaps the link for a pointer to the issue tracker.

## Goal

Bring CefTop to feature parity with the validated PowerShell prototype (`ceftop-cli.ps1`) on Windows, macOS, and Linux, behind a Wails v2 + Svelte GUI. "Done" means:

- Cross-platform process discovery, role parsing, telemetry (PID / PPID / threads / RSS), and per-process kill — driven by the configured target executable name.
- First-run onboarding flow that captures the target name into `<UserConfigDir>/ceftop/ceftop.json`, with the main monitoring view as the second screen.
- Color-coded role rendering that matches the prototype's `$RoleColors` table, with snapshots refreshed live without user action.

## Operating principles

- One phase at a time. Each phase ends at a hard review checkpoint — no cascading work into the next phase without explicit go-ahead.
- Cross-platform first. Windows-only behavior is a bug; reserve `runtime.GOOS` gates for genuine implementation differences (kill syscall, cmdline reader fallback), never for required behavior.
- `github.com/shirou/gopsutil/v4/process` is the default provider. Reach for platform-specific code only where gopsutil cannot answer.
- Snapshots are immutable on the frontend. The backend emits a fully-built tree; the Svelte store replaces it, never mutates.
- Backend kill operations return structured results (`KillResult{Killed bool, Err string, Errno string}`), not booleans — the UI needs to distinguish permission-denied from process-gone.

## Phase 1 — `pkg/config/`

Done — 2026-05-06. `pkg/config/` ships `Config{AppName string}`, `DefaultPath()` resolving to `~/.config/ceftop/ceftop.json` (XDG-aware, gitbox-style on every OS), `Load` (missing-file → empty config, not error), and `Save` (creates parent dir, 4-space indent + trailing newline). `cmd/gui/app.go` loads config on startup and exposes `App.GetConfig()` (returns `{AppName, Path, LoadError}`) and `App.SetTargetApp(name)`. The scaffold's `Greet` method is kept as a stub until Phase 4 replaces the Svelte template.

Exit criteria met: `go test ./pkg/config/...` passes 7 tests, `go vet ./...` clean, `go build ./cmd/gui` clean.

## Phase 2 — `pkg/process/` (read-only snapshot)

Done — 2026-05-06. `pkg/process/` ships a `Provider` interface with a `GopsutilProvider` implementation, role parsing (`ExtractRole` + the 11-value `Role` constant set tracking `$RoleColors`), and `BuildSnapshot` that assembles raw enumerations into a memory-sorted parent / child tree (orphans surface as additional roots). 10-test suite covers regex edge cases, single / multi-child trees, orphan promotion, root sort order, target-name normalization, and a live integration smoke against the test process. `cmd/gui/app.go` exposes `App.Snapshot() (*ProcessSnapshot, error)`. `go test ./pkg/process/...`, `go vet ./...`, and `go build ./cmd/gui` all clean.

> [!NOTE]
> The Windows integration smoke confirms `gopsutil.Cmdline()` returns the running process's command line intact. The same test will run on macOS and Linux when those builds happen — failure there is the trigger to add a platform-specific cmdline reader (Linux: `/proc/<pid>/cmdline`; macOS: `sysctl KERN_PROCARGS2`).

## Phase 3 — Live updates and kill

Done — 2026-05-06. `pkg/process/kill.go` ships `Kill(pid int32) KillResult` with the `EINVAL` / `ESRCH` / `EPERM` Errno categorization the UI needs. `cmd/gui/app.go` runs a `time.Ticker`-driven snapshot loop in a background goroutine, emits each snapshot on the `snapshot` Wails event (and parse failures on `snapshot:error`), and exposes `App.KillProcess(pid int) KillResult`. The loop reads `cfg.TickInterval()` once at start; an `OnShutdown` hook closes a quit channel so `wails dev` hot reloads do not leak goroutines. The `TickIntervalSeconds` field landed in `pkg/config` with `omitempty` and a `DefaultTickIntervalSeconds = 2` constant — onboarding still hides the knob, power users edit the JSON directly. 24-test suite total (11 config + 13 process, including a real-subprocess kill round-trip).

## Phase 4 — Frontend port

Done — 2026-05-06. Stock svelte-ts template replaced with the real CefTop UI: `src/lib/{types,bridge,stores,roleTheme}.ts` plus `Onboarding.svelte`, `ProcessTree.svelte`, `ProcessNode.svelte`, `StatusBar.svelte`. `App.svelte` routes between onboarding and the monitor view via the derived `onboardingNeeded` store; the monitor subscribes once to `bridge.onSnapshot` / `bridge.onSnapshotError` in `onMount` and tears the listeners down in `onDestroy`. The snapshot store is hard-replaced on each event — no in-place mutation. `roleTheme.ts` is the single color source of truth, mapping the prototype's `$RoleColors` PowerShell names to dark-theme-friendly CSS hex while preserving hue ordering. `Greet` is gone from `app.go`; the wailsjs bindings were regenerated via `wails generate module`. `style.css` ships the `--bg / --bg-elevated / --border / --fg / --fg-muted / --accent / --danger` token set; `main.go` window options match (820×640, dark `#0b1220` background). Verified: `npm run check` 0 errors / 0 warnings, `npm run build` clean (22 modules, 19 KiB JS), `go vet ./...` clean, `go test ./...` 24 / 24 pass, `go build ./cmd/gui` produces a 7.3 MB binary. A live `wails dev` smoke is the user's call.

## Phase 5 — Polish (deferred, opt-in)

Tagged for later. Do not start without explicit go-ahead — none of this is required by `context.md`.

- Window state persistence (size, position, view mode) — gitbox pattern.
- Single-instance lock with focus-existing-window on second launch.
- App icons (`appicon.png`, `windows/icon.ico`, macOS `.icns`).
- CI workflow (`.github/workflows/`) that builds all three platforms and uploads artifacts.
- Cross-platform build / ship script (mirror gitbox's `scripts/ship.sh`).

## Decisions log

Append-only. Each entry: `YYYY-MM-DD — Decision — One-line rationale`.

- 2026-05-06 — Use `github.com/shirou/gopsutil/v4/process` as default process provider — most mature cross-platform option, removes WMI dependency.
- 2026-05-06 — Use `~/.config/ceftop/` on every platform (XDG-aware) instead of `os.UserConfigDir` — mirrors the gitbox sibling project so a single `~/.config/` tree hosts every Luis-Palacios tool, and config files travel cleanly between hosts via SSH or rsync.
- 2026-05-06 — Snapshots are immutable on the frontend — eliminates a class of stale-tree bugs and keeps the Svelte store trivially reactive.
- 2026-05-06 — Use Wails events for snapshot delivery — backend owns the cadence, no frontend timers, no polling races.
- 2026-05-06 — Stick with Svelte 3 (the svelte-ts template default) — matches gitbox; bump only if a concrete blocker surfaces.
- 2026-05-06 — Tick interval defaults to 5 s on first launch (was 2 s during initial development); legal range is `[MinTickIntervalSeconds=1 .. MaxTickIntervalSeconds=999]`. Anything outside that range — whether typed in the UI or hand-edited into `ceftop.json` — is silently reset to the default by `Config.Normalize` and (for `SetTickInterval`) by the binding itself. Settings panel exposes a 1–30 s slider plus a 3-digit text input that supports the full 1–999 range; both controls re-sync from the persisted value after every save.
- 2026-05-06 — Kill error categorization: `EINVAL` for non-positive PID, `ESRCH` for non-existent process (caught at `psprocess.NewProcess`), `EPERM` for any actual kill-syscall failure — collapses platform-specific error codes into three UI states. The NewProcess / Kill window is a tolerated race; a process that disappears between the two surfaces as `EPERM`.
- 2026-05-06 — Config file always materializes every supported key. `omitempty` is off; `Config.Normalize()` fills defaults for any zero / missing field; `Load` rewrites the file when normalization changed something. New supported keys land in user files automatically the next time the app launches.
- 2026-05-06 — `~/.config/ceftop/ceftop.json` is editable from the GUI: `App.SetTargetApp` and `App.SetTickInterval` persist immediately and the snapshot ticker resets on the next select via a buffered `intervalChange` channel. `App.OpenConfigInEditor` opens the file in the OS default editor (`cmd /c start ""` on Windows with `HideWindow`, `open` on macOS, `xdg-open` on Linux).
- 2026-05-06 — Frontend zoom: Ctrl+Plus / Ctrl+Minus step the html font-size in 0.1× increments between 0.7× and 1.6×; Ctrl+0 resets. Persisted in `localStorage["ceftop:zoom"]` so the choice survives launches without polluting the JSON config.
- 2026-05-06 — Responsive layout: window `MinWidth` raised to 720 px, `MinHeight` to 420 px; rows pin column widths and the tree pane scrolls horizontally for deep trees instead of letting columns collapse. The user-reported regression (columns disappearing at narrow widths) cannot recur.
- 2026-05-06 — Process tree rows use CSS Grid with explicit column widths (`12.5rem 10rem 6rem 9rem 1fr auto`). Depth indentation is rendered inside the first cell via a `--depth` CSS variable so the role / threads / mem / kill columns line up across nesting levels. Earlier flex layout drove the indent through `padding-left` on the row, which pushed every cell right by depth × indent and broke vertical alignment.

## Open questions

Parked here until they are load-bearing. When resolved, move them to the decisions log with the resolution.

- GUI-only forever, or future `cmd/cli/` like gitbox? Affects whether `pkg/process/` must stay UI-agnostic. `pkg/process/` already does — `cmd/gui/app.go` is the only consumer of Wails imports — so this stays cheap to defer.
- Target matching: exact basename only (case-insensitive), or substring / glob? Prototype matches `Win32_Process.Name` exactly. Phase 2 currently does case-insensitive exact-basename matching with optional `.exe` suffix, which mirrors the prototype.

## How this document is maintained

- Update the `Last updated` line whenever any section changes substantively.
- When a phase ships, collapse its sub-sections to a one-line `Done — see commit <sha>` entry. Do not delete — the history matters until the doc retires.
- When the repository moves to GitHub with issues enabled, copy each open phase and each open question into an issue, then delete this document and replace the README link with a pointer to the issue tracker.
