package main

import (
	"embed"
	_ "embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed ../../frontend/dist
var assets embed.FS

func main() {
	// Create app service
	appService := NewApp()

	// Create a new Wails application
	app := application.New(application.Options{
		Name:        "Fast File Deletion",
		Description: "High-performance file deletion tool",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{},
	})

	// Set the app instance on the service
	appService.SetApp(app)

	// Create the main window
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Fast File Deletion",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		Windows: application.WindowsWindow{
			BackdropType: application.Mica,
		},
		BackgroundColour: application.NewRGB(32, 32, 32),
		URL:              "/",
		Width:            1200,
		Height:           800,
		MinWidth:         800,
		MinHeight:        600,
	})

	// Run the application
	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
