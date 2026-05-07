package main

import (
	"embed"

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

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "CefTop",
		Width:  920,
		Height: 640,
		// Floor low enough that the frontend auto-fit can shrink to match
		// content size. Previous 900x420 was a hard floor that left the
		// window 100+ px wider than the rows actually needed (the row grid
		// was retuned to be much tighter — ~588px at default zoom for a
		// typical Chromium tree). The auto-fit clamps to monitor work area
		// from above; this just stops the window from collapsing to 0 in
		// pathological cases (empty target, layout glitch).
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
