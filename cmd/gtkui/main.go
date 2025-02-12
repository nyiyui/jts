package main

import (
	"log"
	"os"

	_ "embed"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/gtkui"
)

func main() {
	db, err := database.NewDatabase()
	if err != nil {
		log.Fatalf("new db: %s", err)
	}
	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate db: %s", err)
	}

	app := gtk.NewApplication("ca.nyiyui.jts", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() { activate(app, db) })

	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}

func activate(app *gtk.Application, db *database.Database) {
	builder := gtk.NewBuilderFromString(gtkui.MainWindowXML)
	window := builder.GetObject("MainWindow").Cast().(*gtk.ApplicationWindow)

	newSessionButton := builder.GetObject("NewSessionButton").Cast().(*gtk.Button)
	newSessionButton.ConnectClicked(func() {
		nsw := gtkui.NewNewSessionWindow(db)
		nsw.Window.SetTransientFor(&window.Window)
		nsw.Window.SetDestroyWithParent(true) // TODO: dialog lives on (after MainWindow is closed) somehow
		nsw.Window.SetApplication(app)
		nsw.Window.Show()
	})

	currentListView := builder.GetObject("CurrentListView").Cast().(*gtk.ListView)
	m := gtkui.NewSessionListModel(db)
	m2 := gtk.NewNoSelection(m)
	currentListView.SetModel(m2)

	factory := gtkui.NewSessionListItemFactory(&window.Window, db)
	currentListView.SetFactory(&factory.ListItemFactory)

	window.SetApplication(app)
	window.Show()
}
