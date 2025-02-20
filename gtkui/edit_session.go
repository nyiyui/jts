package gtkui

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"golang.org/x/sync/semaphore"
	"nyiyui.ca/jts/data"
	"nyiyui.ca/jts/database"
)

//go:embed edit_session.ui
var EditSessionXML string

type EditSessionWindow struct {
	Window             *gtk.Window
	SessionId          *gtk.Label
	SaveButton         *gtk.Button
	DeleteButton       *gtk.Button
	SessionDescription *gtk.Entry
	SessionNotes       *gtk.TextView
	Timeframes         *gtk.ColumnView

	sessionID string
	db        *database.Database
	changed   chan<- struct{}
}

func NewEditSessionWindow(db *database.Database, sessionID string, changed chan<- struct{}) *EditSessionWindow {
	builder := gtk.NewBuilderFromString(EditSessionXML)
	esw := new(EditSessionWindow)
	esw.Window = builder.GetObject("EditSessionWindow").Cast().(*gtk.Window)
	esw.SessionId = builder.GetObject("SessionId").Cast().(*gtk.Label)
	esw.SessionId.SetLabel(sessionID)
	esw.SaveButton = builder.GetObject("SaveButton").Cast().(*gtk.Button)
	esw.DeleteButton = builder.GetObject("DeleteButton").Cast().(*gtk.Button)
	esw.SessionDescription = builder.GetObject("SessionDescription").Cast().(*gtk.Entry)
	esw.SessionNotes = builder.GetObject("SessionNotes").Cast().(*gtk.TextView)
	esw.SaveButton.ConnectClicked(esw.save)
	esw.DeleteButton.ConnectClicked(esw.delete_)

	esw.sessionID = sessionID
	esw.db = db
	esw.changed = changed

	session, err := db.GetSession(sessionID)
	if err == nil {
		esw.Window.SetTitle(fmt.Sprintf("%sを修正", session.Description))
		esw.SessionDescription.SetText(session.Description)
		esw.SessionNotes.Buffer().SetText(session.Notes)
	}
	esw.Timeframes = builder.GetObject("Timeframes").Cast().(*gtk.ColumnView)
	m := NewTimeframeListModel(esw.db, sessionID)
	esw.Timeframes.SetModel(gtk.NewNoSelection(m))

	factoryStart := NewTimeframeListItemFactory(func(tf data.Timeframe) string {
		return tf.StringStart()
	})
	esw.Timeframes.AppendColumn(gtk.NewColumnViewColumn("開始", &factoryStart.ListItemFactory))
	factoryEnd := NewTimeframeListItemFactory(func(tf data.Timeframe) string {
		return tf.StringEnd()
	})
	esw.Timeframes.AppendColumn(gtk.NewColumnViewColumn("終了", &factoryEnd.ListItemFactory))
	factoryDuration := NewTimeframeListItemFactory(func(tf data.Timeframe) string {
		return tf.Duration().Round(1 * time.Second).String()
	})
	esw.Timeframes.AppendColumn(gtk.NewColumnViewColumn("時間", &factoryDuration.ListItemFactory))
	factoryEdit := NewTimeframeEditListItemFactory(esw.Window, esw.db, sessionID, changed)
	esw.Timeframes.AppendColumn(gtk.NewColumnViewColumn("操作", &factoryEdit.ListItemFactory))

	return esw
}

func (esw *EditSessionWindow) save() {
	buf := esw.SessionNotes.Buffer()
	err := esw.db.EditSessionProperties(data.Session{
		Description: esw.SessionDescription.Buffer().Text(),
		Notes:       buf.Text(buf.StartIter(), buf.EndIter(), false),
		ID:          esw.sessionID,
	})
	if err != nil {
		panic(err)
	}
	esw.Window.Close()
	esw.changed <- struct{}{}
}

func (esw *EditSessionWindow) delete_() {
	ad := adw.NewAlertDialog("セッションを削除", "セッションを削除します。復元はできません。")
	ad.AddResponse("cancel", "削除しない")
	ad.AddResponse("delete", "削除する")
	ad.SetCloseResponse("cancel")
	ad.SetDefaultResponse("delete")
	ad.ConnectResponse(func(response string) {
		if response == "delete" {
			err := esw.db.DeleteSession(esw.sessionID)
			if err != nil {
				panic(err)
			}
		}
		esw.Window.Close()
	})
	ad.Present(esw.Window)
}

var TimeframeListModelType = gioutil.NewListModelType[data.Timeframe]()

type TimeframeListModel struct {
	*gioutil.ListModel[data.Timeframe]
	db        *database.Database
	sessionID string
	sema      *semaphore.Weighted
}

func NewTimeframeListModel(db *database.Database, sessionID string) *TimeframeListModel {
	m := &TimeframeListModel{TimeframeListModelType.New(), db, sessionID, semaphore.NewWeighted(1)}
	m.FillFromDatabase()
	db.Notify(m.updateHook)
	return m
}

func (m *TimeframeListModel) FillFromDatabase() {
	session, err := m.db.GetSession(m.sessionID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		panic(err)
	}
	err = m.sema.Acquire(context.Background(), 1)
	if err != nil {
		// this probably means we are calling FillFromDatabase too fast
		// there is no backpressure, so we should not add more to the metaphorical backlog
		return
	}
	glib.IdleAdd(func() {
		defer m.sema.Release(1)
		m.Splice(0, m.Len(), session.Timeframes...)
	})
}

func (m *TimeframeListModel) updateHook(op int, db string, table string, rowid int64) {
	if table != "time_frames" {
		return
	}
	m.FillFromDatabase()
}

func NewTimeframeListItemFactory(fn func(data.Timeframe) string) *gtk.SignalListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	// we can't use builder factory as it doesn't support introspection of Go objects
	factory.ConnectSetup(func(object *glib.Object) {
		cell := object.Cast().(*gtk.ColumnViewCell)
		label := gtk.NewLabel("")
		cell.SetChild(label)
	})
	factory.ConnectBind(func(object *glib.Object) {
		cell := object.Cast().(*gtk.ColumnViewCell)
		label := cell.Child().(*gtk.Label)
		timeframe := TimeframeListModelType.ObjectValue(cell.Item())
		label.SetText(fn(timeframe))
	})
	// nothing to do for unbind and teardown
	return factory
}

func NewTimeframeEditListItemFactory(window *gtk.Window, db *database.Database, sessionID string, changed chan<- struct{}) *gtk.SignalListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	// we can't use builder factory as it doesn't support introspection of Go objects
	factory.ConnectSetup(func(object *glib.Object) {
		cell := object.Cast().(*gtk.ColumnViewCell)
		button := gtk.NewButton()
		button.SetLabel("修正")
		cell.SetChild(button)
	})
	factory.ConnectBind(func(object *glib.Object) {
		cell := object.Cast().(*gtk.ColumnViewCell)
		button := cell.Child().(*gtk.Button)
		timeframe := TimeframeListModelType.ObjectValue(cell.Item())
		button.ConnectClicked(func() {
			PresentDialog(window, NewEditTimeframeWindow(db, sessionID, timeframe.ID, changed).Window)
		})
	})
	// nothing to do for unbind and teardown
	return factory
}
