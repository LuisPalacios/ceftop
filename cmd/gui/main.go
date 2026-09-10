package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// Build-time variables (set via -ldflags). CI passes the tag and commit SHA;
// local `go build` leaves the defaults, in which case GetAppVersion falls
// back to `git describe` / `git rev-parse` at runtime.
var (
	version = "dev"
	commit  = "none"
)

//go:embed all:frontend/dist
var assets embed.FS

// bundledIconDir is where the frontend build places the synced app-*.svg
// icons inside the embedded dist tree. The backend lists this directory to
// learn which bundled icons exist, so icon resolution (fuzzy name matching)
// happens in exactly one place — see pkg/icons.
const bundledIconDir = "frontend/dist/app-icons"

func main() {
	iconFS, err := fs.Sub(assets, bundledIconDir)
	if err != nil {
		// Only happens if the embed pattern changes; the app still runs,
		// every target just gets the default icon.
		log.Println("[ceftop] bundled icons unavailable:", err)
		iconFS = nil
	}

	// Create an instance of the app structure
	app := NewApp(iconFS)

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "CefTop",
		Width:  920,
		Height: 640,
		// Boot-time floor only. Once the monitor view is up, the frontend
		// pins min and max height to its fixed 20-row budget and caps the
		// width at the monitor's work area (see the window-fit section in
		// App.svelte); MinWidth is mirrored there as MIN_WIDTH. This just
		// stops the window from collapsing to 0 during onboarding or a
		// layout glitch.
		MinWidth:  200,
		MinHeight: 150,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 11, G: 18, B: 32, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
