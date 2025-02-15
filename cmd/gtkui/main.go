package main

import (
	"log"
	"os"
	"path/filepath"

	_ "embed"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/kirsle/configdir"
	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/gtkui"
	"nyiyui.ca/jts/tokens"
)

func main() {
	path := configdir.LocalConfig("jts")
	if err := configdir.MakePath(path); err != nil {
		log.Fatalf("create config dir: %s", err)
	}

	var token tokens.Token
	var err error
	rawToken, err := os.ReadFile(filepath.Join(path, "server-token"))
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("read token: %s", err)
	}
	if err == nil {
		token, err = tokens.ParseToken(string(rawToken))
		if err != nil {
			log.Fatalf("parse token: %s", err)
		}
	}

	db, err := database.NewDatabase("")
	if err != nil {
		log.Fatalf("new db: %s", err)
	}
	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate db: %s", err)
	}

	app := gtk.NewApplication("ca.nyiyui.jts", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() {
		mw := gtkui.NewMainWindow(db, token, filepath.Join(path, "original.json"))
		mw.Window.SetApplication(app)
		mw.Window.Show()
	})

	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}
