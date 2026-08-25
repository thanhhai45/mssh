package main

import (
	"context"
	"fmt"

	"mssh/internal/store"
)

// App is the bridge between the frontend and the Go packages that do the real
// work. Methods here stay thin on purpose: logic lives in internal/.
type App struct {
	ctx   context.Context
	store *store.Store
}

// NewApp creates a new App application struct
func NewApp(st *store.Store) *App {
	return &App{store: st}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
