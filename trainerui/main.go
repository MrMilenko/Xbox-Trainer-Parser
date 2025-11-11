package main

import (
	"embed"

	"trainerui/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	appObj := app.NewApp()

	err := wails.Run(&options.App{
		Title:            "Trainer Viewer",
		Width:            1200,
		Height:           800,
		Assets:           assets,
		BackgroundColour: &options.RGBA{R: 22, G: 22, B: 22, A: 255},
		OnStartup:        appObj.Startup,
		Bind:             []interface{}{appObj},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
