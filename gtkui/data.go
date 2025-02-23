package gtkui

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"golang.org/x/sync/semaphore"
	"nyiyui.ca/jts/data"
	"nyiyui.ca/jts/database"
)

var SessionListModelType = gioutil.NewListModelType[data.Session]()

type SessionListModel struct {
	*gioutil.ListModel[data.Session]
	db   *database.Database
	sema *semaphore.Weighted
}

func NewSessionListModel(db *database.Database) *SessionListModel {
	m := &SessionListModel{SessionListModelType.New(), db, semaphore.NewWeighted(1)}
	m.FillFromDatabase()
	db.Notify(m.updateHook) // TODO: leak?
	return m
}

func (m *SessionListModel) FillFromDatabase() {
	ok := m.sema.TryAcquire(1)
	if !ok {
		// this probably means we are calling FillFromDatabase too fast
		// there is no backpressure, so we should not add more to the metaphorical backlog
		return
	}
	sessions, err := m.db.GetLatestSessions(100, 0)
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}
	log.Printf("idleadd")
	if err == sql.ErrNoRows {
		glib.IdleAdd(func() {
			defer m.sema.Release(1)
			log.Printf("splice0")
			m.Splice(0, m.Len())
		})
	} else {
		glib.IdleAdd(func() {
			defer m.sema.Release(1)
			log.Printf("splice")
			m.Splice(0, m.Len(), sessions...)
		})
	}
}

func (m *SessionListModel) updateHook(op int, db string, table string, rowid int64) {
	if table != "sessions" && table != "time_frames" {
		return
	}
	m.FillFromDatabase()
}

func NewSessionListItemFactory(parent *gtk.Window, db *database.Database, changed chan<- struct{}) *gtk.SignalListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	// we can't use builder factory as it doesn't support introspection of Go objects
	factory.ConnectSetup(func(object *glib.Object) {
		listItem := object.Cast().(*gtk.ListItem)
		label := gtk.NewLabel("")
		label.SetHExpand(true)
		timeframes := gtk.NewLabel("")
		actions := gtk.NewBox(gtk.OrientationHorizontal, 0)
		extend := gtk.NewButtonWithLabel("打刻延長")
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
			if changed != nil {
				changed <- struct{}{}
			}
		})
		edit := extend.NextSibling().(*gtk.Button)
		edit.ConnectClicked(func() {
			esw := NewEditSessionWindow(db, session.ID, changed)
			esw.Window.SetTransientFor(parent)
			esw.Window.SetApplication(parent.Application())
			esw.Window.Show()
		})
	})
	// nothing to do for unbind and teardown
	return factory
}

var TaskListModelType = gioutil.NewListModelType[data.Task]()

type TaskListModel struct {
	*gioutil.ListModel[data.Task]
	db   *database.Database
	sema *semaphore.Weighted
}

func NewTaskListModel(db *database.Database) *TaskListModel {
	m := &TaskListModel{TaskListModelType.New(), db, semaphore.NewWeighted(1)}
	m.FillFromDatabase()
	db.Notify(m.updateHook) // TODO: leak?
	return m
}

func (m *TaskListModel) FillFromDatabase() {
	ok := m.sema.TryAcquire(1)
	if !ok {
		// this probably means we are calling FillFromDatabase too fast
		// there is no backpressure, so we should not add more to the metaphorical backlog
		return
	}
	tasks, err := m.db.GetUndoneTasks()
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}
	log.Printf("there are %d tasks", len(tasks))
	if err == sql.ErrNoRows {
		glib.IdleAdd(func() {
			defer m.sema.Release(1)
			m.Splice(0, m.Len())
		})
	} else {
		glib.IdleAdd(func() {
			defer m.sema.Release(1)
			m.Splice(0, m.Len(), tasks...)
		})
	}
}

func (m *TaskListModel) updateHook(op int, db string, table string, rowid int64) {
	if table != "tasks" && table != "sessions" && table != "time_frames" {
		return
	}
	m.FillFromDatabase()
}

func NewTaskListItemFactory(parent *gtk.Window, db *database.Database, changed chan<- struct{}) *gtk.SignalListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	// we can't use builder factory as it doesn't support introspection of Go objects
	factory.ConnectSetup(func(object *glib.Object) {
		listItem := object.Cast().(*gtk.ListItem)
		label := gtk.NewLabel("")
		label.SetHExpand(true)
		box := gtk.NewBox(gtk.OrientationVertical, 0)
		actions := gtk.NewBox(gtk.OrientationHorizontal, 0)
		newSession := gtk.NewButtonWithLabel("セッション作成")
		actions.Append(newSession)
		actions.SetHAlign(gtk.AlignEnd)
		box.Append(label)
		box.Append(actions)
		listItem.SetChild(box)
	})
	factory.ConnectBind(func(object *glib.Object) {
		listItem := object.Cast().(*gtk.ListItem)
		box := listItem.Child().(*gtk.Box)
		label := box.FirstChild().(*gtk.Label)
		actions := label.NextSibling().(*gtk.Box)
		newSession := actions.FirstChild().(*gtk.Button)

		task := TaskListModelType.ObjectValue(listItem.Item())
		label.SetText(task.Description)
		newSession.ConnectClicked(func() {
			nsw := NewNewSessionWindow(db, changed)
			nsw.SetTask(task)
			nsw.Window.SetTransientFor(parent)
			nsw.Window.SetDestroyWithParent(true) // TODO: dialog lives on (after MainWindow is closed) somehow
			nsw.Window.SetApplication(parent.Application())
			nsw.Window.Show()
		})
	})
	// nothing to do for unbind and teardown
	return factory
}
