package main

import (
	"context"
	"fmt"
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
}

// NewApp creates a new App instance.
func NewApp() *App {
	provider := process.NewGopsutilProvider()
	return &App{
		cfgPath:        config.DefaultPath(),
		provider:       provider,
		monitor:        process.NewMonitor(provider),
		quit:           make(chan struct{}),
		intervalChange: make(chan time.Duration, 1),
	}
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

// snapshotLoop emits a snapshot immediately at startup and then on each tick
// of the configured interval. The interval can be re-tuned at runtime via
// the intervalChange channel — SetTickInterval pushes the new value here.
func (a *App) snapshotLoop() {
	a.emitSnapshot()

	a.mu.Lock()
	interval := a.cfg.TickInterval()
	a.mu.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-a.quit:
			return
		case newInterval := <-a.intervalChange:
			ticker.Reset(newInterval)
		case <-ticker.C:
			a.emitSnapshot()
		}
	}
}

// emitSnapshot takes one snapshot and forwards it to the frontend. A failed
// snapshot is published on the error event rather than silently dropped, so
// the UI can surface persistent enumeration failures.
func (a *App) emitSnapshot() {
	if a.ctx == nil {
		return
	}
	snap, err := a.Snapshot()
	if err != nil {
		wailsrt.EventsEmit(a.ctx, eventSnapshotError, err.Error())
		return
	}
	wailsrt.EventsEmit(a.ctx, eventSnapshot, snap)
}

// discoveryLoop emits a discovery scan immediately at startup and then on
// each tick of discoveryInterval. It runs in its own goroutine so a slow
// host enumeration cannot starve the snapshot loop (and vice versa).
func (a *App) discoveryLoop() {
	a.emitDiscovery()

	ticker := time.NewTicker(discoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.quit:
			return
		case <-ticker.C:
			a.emitDiscovery()
		}
	}
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

// discoverAppsMerged combines live host discovery with names declared by
// user-supplied "app-<name>.svg" files in the config directory. Names that
// exist as icons but have no running process show up with ChildCount == 0
// so the user can still pick them as targets — useful for an app the user
// only launches occasionally, or one that's currently down.
func (a *App) discoverAppsMerged() ([]process.DiscoveredApp, error) {
	apps, err := process.DiscoverApps(a.provider)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	dir := filepath.Dir(a.cfgPath)
	a.mu.Unlock()

	names, iconErr := config.PrivateIconNames(dir)
	if iconErr != nil {
		log.Println("[ceftop] private icon names:", iconErr)
		return apps, nil
	}
	if len(names) == 0 {
		return apps, nil
	}

	seen := make(map[string]struct{}, len(apps))
	for _, app := range apps {
		seen[app.Name] = struct{}{}
	}
	for _, n := range names {
		if _, ok := seen[n]; ok {
			continue
		}
		apps = append(apps, process.DiscoveredApp{Name: n, ChildCount: 0})
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })
	return apps, nil
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

// SetTargetApp persists a new target executable name. An empty / whitespace
// name is rejected; the tick interval is preserved across the rewrite.
func (a *App) SetTargetApp(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("target app name cannot be empty")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	next := &config.Config{AppName: trimmed}
	if a.cfg != nil {
		next.TickIntervalSeconds = a.cfg.TickIntervalSeconds
	}
	next.Normalize()
	if err := config.Save(a.cfgPath, next); err != nil {
		return err
	}
	a.cfg = next
	a.cfgLoadError = ""
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
// entry per distinct browser process. User-declared apps (any
// "app-<name>.svg" next to the config JSON) are merged in with
// ChildCount == 0 so they remain selectable when not running. The
// frontend uses this for the one-shot pull at mount time; subsequent
// updates arrive on the "discovery" event emitted by discoveryLoop.
func (a *App) DiscoverApps() ([]process.DiscoveredApp, error) {
	return a.discoverAppsMerged()
}

// GetPrivateIcons returns user-supplied SVG icons that live alongside the
// config JSON, keyed by <name> (the part between "app-" and ".svg"). The
// frontend prefers these over the bundled icons in /app-icons/. An empty
// map is a normal "no overrides yet" result — never an error.
func (a *App) GetPrivateIcons() map[string]string {
	a.mu.Lock()
	dir := filepath.Dir(a.cfgPath)
	a.mu.Unlock()
	icons, err := config.LoadPrivateIcons(dir)
	if err != nil {
		log.Println("[ceftop] private icons:", err)
		return map[string]string{}
	}
	return icons
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
