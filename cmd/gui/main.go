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

// The window's one and only size, as an outer size in Wails units (DIPs:
// Wails scales them with the monitor DPI, so they equal CSS px at any display
// scale). On Windows 11 the frame adds an invisible 7 px border left, right
// and bottom, so this shows as 936×993. The user cannot resize the window;
// the only exception is FitWindowToScreen shrinking it to a monitor whose
// work area is smaller. Content that does not fit (a very deep tree, a large
// zoom, a shrunk window) scrolls inside the process tree pane.
const (
	windowWidth  = 950
	windowHeight = 1000
)

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
		// Fixed size, never resized by the user; see windowWidth. DisableResize
		// also removes the maximize button.
		Width:         windowWidth,
		Height:        windowHeight,
		DisableResize: true,
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
