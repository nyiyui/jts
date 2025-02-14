package main

import (
	"log"
	"os"

	_ "embed"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/gtkui"
	"nyiyui.ca/jts/tokens"
)

func main() {
	var token tokens.Token
	var err error
	rawToken := os.Getenv("JTS_SERVER_TOKEN")
	if rawToken != "" {
		token, err = tokens.ParseToken(rawToken)
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
		mw := gtkui.NewMainWindow(db, token)
		mw.Window.SetApplication(app)
		mw.Window.Show()
	})

	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}
