package main

import (
	"embed"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"mssh/internal/session"
	"mssh/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

// icon is the window's icon on Linux, where nothing else provides one: macOS
// tkes it from the bundle and Windows from executable.
//
//go:embed build/appicon.png
var icon []byte

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

	// Passwords saved by versions before 0.6 are in the macOS keychain. Move
	// them here once; after that the keychain is never read again. Those
	// versions only ever shipped for macOS, so elsewhere there is nothing to
	// move — and asking would mean a Secret Service that may not be running,
	// or a prompt to unlock a keyring the user never meant to open.
	if runtime.GOOS == "darwin" {
		moveKeychainPasswords(st, passwords)
	}

	// Sessions the last run never got to close — a crash, a force quit, the
	// battery running out — are settled before any new one can open, so the
	// log never shows a session as still open when it cannot be.
	if settled, err := st.MarkInterruptedSessions(); err != nil {
		log.Printf("mssh: session log: %v", err)
	} else if settled > 0 {
		log.Printf("mssh: session log: %d session(s) from the last run marked interrupted", settled)
	}

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
		Linux: &linux.Options{
			Icon:        icon,
			ProgramName: "mssh",
			// Wails turns GPU acceleration off when no Linux options are given,
			// for WebKitGTK's blank windows on some drives. Giving options at
			// all would quietly turn it back on, so it is said again here.
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
