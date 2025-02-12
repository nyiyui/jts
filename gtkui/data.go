package gtkui

import (
	"fmt"
	"log"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/data"
	"nyiyui.ca/jts/database"
)

var SessionListModelType = gioutil.NewListModelType[data.Session]()

type SessionListModel struct {
	*gioutil.ListModel[data.Session]
	db *database.Database
}

func NewSessionListModel(db *database.Database) *SessionListModel {
	m := &SessionListModel{SessionListModelType.New(), db}
	m.FillFromDatabase()
	db.Notify(m.updateHook)
	return m
}

func (m *SessionListModel) FillFromDatabase() {
	sessions, err := m.db.GetLatestSessions(100, 0)
	if err != nil {
		panic(err)
	}
	m.Splice(0, m.Len(), sessions...)
}

func (m *SessionListModel) updateHook(op int, db string, table string, rowid int64) {
	log.Println("updateHook", op, db, table, rowid)
	if table != "sessions" && table != "time_frames" {
		return
	}
	m.FillFromDatabase()
}

func NewSessionListItemFactory(parent *gtk.Window, db *database.Database) *gtk.SignalListItemFactory {
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
		session := SessionListModelType.ObjectValue(listItem.Item())
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
			esw := NewEditSessionWindow(db, session.ID)
			esw.Window.SetTransientFor(parent)
			esw.Window.SetApplication(parent.Application())
			esw.Window.Show()
		})
	})
	// nothing to do for unbind and teardown
	return factory
}
