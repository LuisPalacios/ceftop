package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/LuisPalacios/ceftop/pkg/config"
	"github.com/LuisPalacios/ceftop/pkg/icons"
	"github.com/LuisPalacios/ceftop/pkg/process"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Wails event names used by the snapshot and discovery loops. The frontend
// subscribes via EventsOn("snapshot", ...), EventsOn("snapshot:error", ...),
// EventsOn("discovery", ...), and EventsOn("discovery:error", ...).
const (
	eventSnapshot       = "snapshot"
	eventSnapshotError  = "snapshot:error"
	eventDiscovery      = "discovery"
	eventDiscoveryError = "discovery:error"
)

// discoveryInterval is the fixed cadence at which the host is rescanned for
// CEF / Chromium / Electron applications. Decoupled from the snapshot tick
// (which the user can stretch up to 999 s) so the apps bar stays responsive
// even when the watched target's tick is slow.
const discoveryInterval = 5 * time.Second

// App is the Wails application struct. Exported methods become frontend
// bindings via window.go.main.App.<Method>().
type App struct {
	ctx context.Context

	mu           sync.Mutex
	cfg          *config.Config
	cfgPath      string
	cfgLoadError string // non-empty when an existing config exists but failed to parse

	provider process.Provider
	monitor  *process.Monitor

	// iconFS is the embedded directory holding the bundled app-*.svg icons
	// (frontend/dist/app-icons). nil means "no bundled icons" — every
	// target then resolves to the default icon unless a private one matches.
	iconFS fs.FS

	// quit is closed by shutdown to stop the snapshot loop. shutdownOnce keeps
	// double-close panics out of the picture if Wails ever calls the shutdown
	// hook twice (e.g. on graceful + forced close).
	quit         chan struct{}
	shutdownOnce sync.Once

	// intervalChange carries a new tick duration into the snapshot loop so a
	// SetTickInterval call takes effect on the next tick instead of next
	// launch. Buffered(1) so the API call never blocks; if the loop is busy
	// and the channel is full, the in-flight value still wins on next read.
	intervalChange chan time.Duration

	// refresh asks the snapshot loop for an out-of-band tick. SetTargetApp
	// pushes here so the new target's tree lands immediately instead of
	// after the remainder of the current interval, and FrontendReady pushes
	// here for the first paint. Buffered(1): a second request while one is
	// pending is redundant and is dropped.
	refresh chan struct{}

	// discoveryRefresh is the discovery loop's counterpart of refresh.
	// FrontendReady pushes here so the apps bar is populated on first paint
	// instead of after the first discoveryInterval.
	discoveryRefresh chan struct{}
}

// NewApp creates a new App instance. iconFS is the embedded bundled-icon
// directory (may be nil).
func NewApp(iconFS fs.FS) *App {
	provider := process.NewGopsutilProvider()
	return &App{
		cfgPath:          config.DefaultPath(),
		provider:         provider,
		monitor:          process.NewMonitor(provider),
		iconFS:           iconFS,
		quit:             make(chan struct{}),
		intervalChange:   make(chan time.Duration, 1),
		refresh:          make(chan struct{}, 1),
		discoveryRefresh: make(chan struct{}, 1),
	}
}

// loadIcons builds a fresh icon index from the bundled set plus whatever
// app-*.svg files currently sit next to the config JSON. It is cheap (two
// directory listings and a handful of small file reads) and is rebuilt on
// every discovery tick so a freshly dropped private icon shows up within
// seconds. A private-directory read failure degrades to bundled-only.
func (a *App) loadIcons() *icons.Index {
	a.mu.Lock()
	dir := filepath.Dir(a.cfgPath)
	a.mu.Unlock()

	idx, err := icons.Load(a.iconFS, dir)
	if err != nil {
		log.Println("[ceftop] private icons:", err)
		idx, _ = icons.Load(a.iconFS, "")
	}
	return idx
}

// startup is the Wails OnStartup hook. The context is captured so runtime
// methods can be called later, the user config is loaded eagerly so the
// frontend can decide between onboarding and the monitor view on first paint,
// and the snapshot loop is launched in a background goroutine.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.loadConfig()
	go a.snapshotLoop()
	go a.discoveryLoop()
}

// shutdown is the Wails OnShutdown hook. It closes the quit channel so the
// snapshot goroutine returns, preventing a leaked goroutine across hot
// reloads of `wails dev`.
func (a *App) shutdown(_ context.Context) {
	a.shutdownOnce.Do(func() {
		close(a.quit)
	})
}

// snapshotLoop emits a snapshot on each tick of the configured interval.
// The interval can be re-tuned at runtime via the intervalChange channel —
// SetTickInterval pushes the new value here — and an out-of-band tick can
// be requested via refresh. A refresh restarts the ticker so the next
// regular tick is a full interval away, rather than landing right behind
// the refreshed one.
//
// There is deliberately no emit at loop start: Wails events are fire-and-
// forget and the frontend has not subscribed yet when startup runs, so an
// early emit would only be wasted work. The first paint is driven by
// FrontendReady instead.
func (a *App) snapshotLoop() {
	a.mu.Lock()
	interval := a.cfg.TickInterval()
	a.mu.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-a.quit:
			return
		case interval = <-a.intervalChange:
			ticker.Reset(interval)
		case <-a.refresh:
			a.emitSnapshot()
			ticker.Reset(interval)
		case <-ticker.C:
			a.emitSnapshot()
		}
	}
}

// requestRefresh asks snapshotLoop for an immediate tick without blocking
// the caller. A request that finds one already queued is dropped: the
// pending tick will read the latest config anyway.
func (a *App) requestRefresh() {
	select {
	case a.refresh <- struct{}{}:
	default:
	}
}

// emitSnapshot takes one snapshot and forwards it to the frontend. A failed
// snapshot is published on the error event rather than silently dropped, so
// the UI can surface persistent enumeration failures. A snapshot whose
// target no longer matches the config — the user switched apps while it was
// being taken — is discarded: the refresh queued by SetTargetApp is about to
// replace it, and publishing it would flash the old tree under the new name.
func (a *App) emitSnapshot() {
	if a.ctx == nil {
		return
	}
	snap, err := a.Snapshot()
	if err != nil {
		wailsrt.EventsEmit(a.ctx, eventSnapshotError, err.Error())
		return
	}
	if a.currentTarget() != snap.Target {
		return
	}
	wailsrt.EventsEmit(a.ctx, eventSnapshot, snap)
}

// currentTarget returns the configured target name, or "" during
// onboarding. Trimmed the same way pkg/process trims it, so it compares
// equal to ProcessSnapshot.Target.
func (a *App) currentTarget() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg == nil {
		return ""
	}
	return strings.TrimSpace(a.cfg.AppName)
}

// discoveryLoop emits a discovery scan on each tick of discoveryInterval and
// on request via discoveryRefresh. It runs in its own goroutine so a slow
// host enumeration cannot starve the snapshot loop (and vice versa). Like
// snapshotLoop it does not emit at start; see FrontendReady.
func (a *App) discoveryLoop() {
	ticker := time.NewTicker(discoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.quit:
			return
		case <-a.discoveryRefresh:
			a.emitDiscovery()
			ticker.Reset(discoveryInterval)
		case <-ticker.C:
			a.emitDiscovery()
		}
	}
}

// requestDiscovery asks discoveryLoop for an immediate scan without
// blocking the caller; a request that finds one already queued is dropped.
func (a *App) requestDiscovery() {
	select {
	case a.discoveryRefresh <- struct{}{}:
	default:
	}
}

// FrontendReady is called by the frontend once its event subscriptions are
// wired. It requests an immediate snapshot and discovery scan so the first
// paint does not wait for the next scheduled tick. This replaces the
// frontend pulling Snapshot() and DiscoverApps() itself, which duplicated
// the loops' work and ran four host enumerations concurrently at startup.
// Both loops fire within the same instant, so the provider's short-lived
// enumeration cache lets the second one reuse the first one's walk.
func (a *App) FrontendReady() {
	a.requestRefresh()
	a.requestDiscovery()
}

// emitDiscovery runs one discovery scan and forwards the result to the
// frontend. Failures publish on the discovery error event so the UI can
// surface a persistent host-enumeration problem; an empty list is a valid
// success ("no CEF apps running") and is emitted as such.
func (a *App) emitDiscovery() {
	if a.ctx == nil {
		return
	}
	apps, err := a.discoverAppsMerged()
	if err != nil {
		wailsrt.EventsEmit(a.ctx, eventDiscoveryError, err.Error())
		return
	}
	wailsrt.EventsEmit(a.ctx, eventDiscovery, apps)
}

// DiscoveredAppView is what the frontend receives per discovered app: the
// process-level facts from pkg/process plus the icon already resolved by
// pkg/icons, so the UI never has to guess file names. IconSrc is either a
// bundled URL (/app-icons/app-<key>.svg) or a data URI for a private icon.
type DiscoveredAppView struct {
	Name       string `json:"name"`
	ChildCount int    `json:"childCount"`
	IconSrc    string `json:"iconSrc"`
}

// discoverAppsMerged combines live host discovery with names declared by
// user-supplied "app-<name>.svg" files in the config directory. Names that
// exist as icons but have no running process show up with ChildCount == 0
// so the user can still pick them as targets — useful for an app the user
// only launches occasionally, or one that's currently down.
//
// "Already running" is decided by the same fuzzy matcher that resolves
// icons, so a private app-docker-desktop.svg does not spawn a duplicate
// offline entry next to the live "Docker Desktop" process.
func (a *App) discoverAppsMerged() ([]DiscoveredAppView, error) {
	apps, err := process.DiscoverApps(a.provider)
	if err != nil {
		return nil, err
	}
	idx := a.loadIcons()

	out := make([]DiscoveredAppView, 0, len(apps))
	for _, app := range apps {
		out = append(out, DiscoveredAppView{
			Name:       app.Name,
			ChildCount: app.ChildCount,
			IconSrc:    idx.Resolve(app.Name),
		})
	}

	for _, key := range idx.PrivateKeys() {
		running := false
		for _, app := range apps {
			if icons.Matches(key, app.Name) {
				running = true
				break
			}
		}
		if running {
			continue
		}
		out = append(out, DiscoveredAppView{Name: key, ChildCount: 0, IconSrc: idx.Resolve(key)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (a *App) loadConfig() {
	a.mu.Lock()
	defer a.mu.Unlock()

	cfg, err := config.Load(a.cfgPath)
	if err != nil {
		a.cfg = &config.Config{}
		a.cfg.Normalize()
		a.cfgLoadError = err.Error()
		return
	}
	cfg.Normalize()
	a.cfg = cfg
	a.cfgLoadError = ""
}

// ConfigState is the snapshot returned to the frontend on each GetConfig call.
// Path lets the UI tell the user where its config lives; LoadError surfaces a
// parse failure so onboarding can warn before overwriting a corrupted file.
type ConfigState struct {
	AppName             string `json:"appName"`
	Path                string `json:"path"`
	LoadError           string `json:"loadError"`
	TickIntervalSeconds int    `json:"tickIntervalSeconds"`
}

// GetConfig returns the current configuration state.
func (a *App) GetConfig() ConfigState {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.cfg
	if cfg == nil {
		cfg = &config.Config{}
	}
	return ConfigState{
		AppName:             cfg.AppName,
		Path:                a.cfgPath,
		LoadError:           a.cfgLoadError,
		TickIntervalSeconds: int(cfg.TickInterval() / time.Second),
	}
}

// SetTargetApp persists a new target executable name and asks the snapshot
// loop for an immediate tick, so the new tree replaces the old one right
// away instead of after the remainder of the current interval. An empty /
// whitespace name is rejected; the tick interval is preserved across the
// rewrite.
func (a *App) SetTargetApp(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("target app name cannot be empty")
	}

	a.mu.Lock()
	next := &config.Config{AppName: trimmed}
	if a.cfg != nil {
		next.TickIntervalSeconds = a.cfg.TickIntervalSeconds
	}
	next.Normalize()
	if err := config.Save(a.cfgPath, next); err != nil {
		a.mu.Unlock()
		return err
	}
	a.cfg = next
	a.cfgLoadError = ""
	a.mu.Unlock()

	a.requestRefresh()
	return nil
}

// SetTickInterval updates the snapshot tick interval and persists it.
// Out-of-range values (anything outside [Min..Max]) are silently clamped
// to the default — the frontend reads the persisted value back via
// GetConfig, so a user who typed 9999 or pasted "-1" sees the UI snap to
// the default instead of a confusing error. The running ticker resets on
// the next select, so the new cadence takes effect almost immediately.
func (a *App) SetTickInterval(seconds int) error {
	if seconds < config.MinTickIntervalSeconds || seconds > config.MaxTickIntervalSeconds {
		seconds = config.DefaultTickIntervalSeconds
	}

	a.mu.Lock()
	if a.cfg == nil {
		a.cfg = &config.Config{}
	}
	next := *a.cfg
	next.TickIntervalSeconds = seconds
	if err := config.Save(a.cfgPath, &next); err != nil {
		a.mu.Unlock()
		return err
	}
	a.cfg = &next
	a.mu.Unlock()

	// Non-blocking: if a previous interval change is still queued, drop it
	// and replace with the latest. Buffered(1) keeps this safe.
	newDur := time.Duration(seconds) * time.Second
	select {
	case a.intervalChange <- newDur:
	default:
		select {
		case <-a.intervalChange:
		default:
		}
		a.intervalChange <- newDur
	}
	return nil
}

// OpenConfigInEditor opens the on-disk config file with the OS default
// application registered for `.json`. On Windows we route through
// `cmd /c start ""` so the spawned editor inherits its own console; the
// HideWindow flag prevents a flashing cmd.exe window.
func (a *App) OpenConfigInEditor() error {
	a.mu.Lock()
	path := a.cfgPath
	a.mu.Unlock()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// `start` uses ShellExecute, which honors the user's default
		// editor for .json files. The empty "" is the title argument.
		native := filepath.FromSlash(path)
		cmd = exec.Command("cmd", "/c", "start", "", native)
		hideWindow(cmd)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// hideWindow sets the Windows-only SysProcAttr that prevents a console
// flash when the GUI process spawns a child. On non-Windows it is a no-op.
func hideWindow(cmd *exec.Cmd) {
	if runtime.GOOS != "windows" {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	setHideWindow(cmd.SysProcAttr)
}

// Snapshot returns the current process tree for the configured target. When
// no target is configured (onboarding hasn't run yet), it returns an empty
// snapshot rather than an error so the frontend can route to onboarding
// without special-casing the call.
func (a *App) Snapshot() (*process.ProcessSnapshot, error) {
	a.mu.Lock()
	target := ""
	if a.cfg != nil {
		target = a.cfg.AppName
	}
	a.mu.Unlock()
	return a.monitor.Snapshot(target)
}

// KillProcess terminates the process identified by pid and returns a
// structured result the UI can render directly. Wails marshals JS numbers
// to int — narrowing to int32 keeps the public API aligned with the rest
// of pkg/process.
func (a *App) KillProcess(pid int) process.KillResult {
	return process.Kill(int32(pid))
}

// DiscoverApps scans every process on the host, identifies the
// CEF / Chromium / Electron applications currently running (any process
// tree containing children with --type=<role> flags), and returns one
// entry per distinct browser process, each with its icon resolved.
// User-declared apps (any "app-<name>.svg" next to the config JSON) are
// merged in with ChildCount == 0 so they remain selectable when not
// running. The frontend receives this on the "discovery" event emitted by
// discoveryLoop (first paint is triggered via FrontendReady); the binding
// stays exported for ad-hoc callers and tooling.
func (a *App) DiscoverApps() ([]DiscoveredAppView, error) {
	return a.discoverAppsMerged()
}

// ResolveIcon returns the <img src> for the icon that best matches an app
// or process name: a private icon next to the config JSON when one
// matches, otherwise a bundled one, otherwise the default. Matching is
// fuzzy — see pkg/icons — so "Docker Desktop.exe", "docker-desktop" and
// "Docker Desktop for Mac" all land on app-docker-desktop.svg. The result
// is always a usable src; this never errors.
func (a *App) ResolveIcon(name string) string {
	return a.loadIcons().Resolve(name)
}

// WindowSetSize sets the OS window's outer size. The frontend uses this to
// fit-to-content on snapshot ticks and zoom changes; clamping against the
// MinWidth / MinHeight set in main.go is enforced by Wails itself, so this
// stays a thin pass-through.
func (a *App) WindowSetSize(width, height int) {
	if a.ctx == nil {
		return
	}
	wailsrt.WindowSetSize(a.ctx, width, height)
}

// WindowGetSize returns the OS window's current outer size as [width, height].
// Used by the auto-fit pipeline as the ground truth for drift detection: in
// Wails v2 on Windows, WebView2's window.outerWidth reports the webview's
// inner viewport rather than the OS window outer, so the JS-side comparison
// is unreliable. Wails' runtime answer comes from the OS itself.
func (a *App) WindowGetSize() [2]int {
	if a.ctx == nil {
		return [2]int{0, 0}
	}
	w, h := wailsrt.WindowGetSize(a.ctx)
	return [2]int{w, h}
}

// Log forwards a frontend diagnostic line to the backend stderr so it shows
// up in the `wails dev` terminal. Wails v2 on Windows does not forward
// webview console.log to the terminal, and right-click → Inspect is not
// always available, leaving the user no other channel for diagnosing
// runtime behavior. log.Println prefixes date+time so the user can see when
// each fit fired without guessing from terminal scroll order.
func (a *App) Log(msg string) {
	log.Println("[ceftop]", msg)
}
