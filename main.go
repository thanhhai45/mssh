package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"mssh/internal/session"
	"mssh/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dbPath, err := store.DefaultPath()
	if err != nil {
		log.Fatalf("mssh: %v", err)
	}

	// Without the database there is nothing to show, so failing here is fatal
	// rather than something the UI tries to recover from.
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("mssh: %v", err)
	}
	defer st.Close()

	log.Printf("mssh: database at %s", dbPath)

	passwords := st.ConnectionPasswords()

	// Passwords saved by earlier versions are in the OS keychain. Move them
	// here once after that the keychain is never read again.
	moveKeychainPasswords(st, passwords)

	// Create an instance of the app structure
	app := NewApp(st, passwords, session.NewManager(st))

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "mssh",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
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
