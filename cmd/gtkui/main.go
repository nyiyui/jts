package main

import (
	"log"
	"os"

	_ "embed"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
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
	builder := gtk.NewBuilderFromString(gtkui.UIXML)
	window := builder.GetObject("MainWindow").Cast().(*gtk.Window)
	currentListView := builder.GetObject("CurrentListView").Cast().(*gtk.ListView)
	m := gtkui.NewSessionListModel()
	sessions, err := db.GetLatestSessions(10, 0)
	if err != nil {
		log.Fatalf("get sessions: %s", err)
	}
	log.Printf("loaded %d sessions", len(sessions))
	m.FillFromSlice(sessions)
	log.Print("a")
	m2 := gtk.NewSingleSelection(m)
	log.Print("b")
	currentListView.SetModel(m2)
	//m := gtk.NewNoSelection(gtk.NewStringList([]string{"one", "two", "three"}))
	//currentListView.SetModel(m)
	log.Print("c")
	factory := gtk.NewBuilderListItemFactoryFromBytes(nil, glib.NewBytes([]byte(gtkui.SessionRowXML)))
	log.Print("d")
	currentListView.SetFactory(&factory.ListItemFactory)
	log.Print("e")
	//lv2 := gtk.NewListView(m, &factory.ListItemFactory)
	//log.Print("f")
	//window.SetChild(lv2)
	//log.Print("g")

	app.AddWindow(window)
	window.Show()
}
