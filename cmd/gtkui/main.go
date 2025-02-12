package main

import (
	"fmt"
	"log"
	"os"
	"time"

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
	m := gtkui.NewSessionListModel()
	sessions, err := db.GetLatestSessions(10, 0)
	if err != nil {
		log.Fatalf("get sessions: %s", err)
	}
	log.Printf("loaded %d sessions", len(sessions))
	m.FillFromSlice(sessions)
	log.Print("a")
	m2 := gtk.NewNoSelection(m)
	log.Print("b")
	currentListView.SetModel(m2)
	log.Print("c")
	factory := gtk.NewSignalListItemFactory()
	// we can't use builder factory as it doesn't support introspection of Go objects
	factory.ConnectSetup(func(object *glib.Object) {
		listItem := object.Cast().(*gtk.ListItem)
		label := gtk.NewLabel("")
		label.SetHExpand(true)
		timeframes := gtk.NewLabel("")
		actions := gtk.NewBox(gtk.OrientationHorizontal, 0)
		extend := gtk.NewButtonWithLabel("最新は現在")
		edit := gtk.NewButtonWithLabel("修正")
		actions.Append(extend)
		actions.Append(edit)
		actions.SetHAlign(gtk.AlignEnd)
		box := gtk.NewBox(gtk.OrientationVertical, 0)
		box.Append(label)
		box.Append(timeframes)
		box.Append(actions)
		listItem.SetChild(box)
	})
	factory.ConnectBind(func(object *glib.Object) {
		listItem := object.Cast().(*gtk.ListItem)
		box := listItem.Child().(*gtk.Box)
		label := box.FirstChild().(*gtk.Label)
		timeframes := label.NextSibling().(*gtk.Label)
		actions := timeframes.NextSibling().(*gtk.Box)
		session := gtkui.SessionListModelType.ObjectValue(listItem.Item())
		label.SetText(session.Description)
		text := ""
		for _, tf := range session.Timeframes {
			text += fmt.Sprintf("%s - %s", tf.Start.Local(), tf.End.Local())
		}
		timeframes.SetText(text)
		extend := actions.FirstChild().(*gtk.Button)
		extend.ConnectClicked(func() {
			err := db.ExtendSession(session.ID, time.Now())
			if err != nil {
				panic(err)
			}
		})
		edit := extend.NextSibling().(*gtk.Button)
		edit.ConnectClicked(func() {
			esw := gtkui.NewEditSessionWindow(db, session.ID)
			esw.Window.SetTransientFor(&window.Window)
			esw.Window.SetApplication(app)
			esw.Window.Show()
		})
	})
	factory.ConnectUnbind(func(object *glib.Object) {
		// nothing to do
	})
	factory.ConnectTeardown(func(object *glib.Object) {
		listItem := object.Cast().(*gtk.ListItem)
		label := listItem.Child().(*gtk.Box)
		label.Unparent()
	})
	log.Print("d")
	currentListView.SetFactory(&factory.ListItemFactory)
	log.Print("e")

	window.SetApplication(app)
	window.Show()
}
