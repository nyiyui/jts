package gtkui

import (
	_ "embed"
	"errors"
	"log"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/data"
	"nyiyui.ca/jts/database/sync"
)

//go:embed merge_window.ui
var mergeWindowXML string

//go:embed merge_session.ui
var mergeSessionXML string

var sessionConflictsListModelType = gioutil.NewListModelType[sync.MergeConflict[data.Session]]()

type MergeWindow struct {
	mc      sync.MergeConflicts
	changes sync.Changes
	saved   bool

	Window                        *adw.Window
	saveButton                    *gtk.Button
	sessionConflictsListView      *gtk.ListView
	sessionConflictsListModel     *gioutil.ListModel[sync.MergeConflict[data.Session]]
	sessionConflictsListSelection *gtk.SingleSelection
	splitView                     *adw.OverlaySplitView
	mergeSession                  *MergeSession
}

func NewMergeWindow(mc sync.MergeConflicts, changes chan<- sync.Changes, errs chan<- error) *MergeWindow {
	builder := gtk.NewBuilderFromString(mergeWindowXML)
	mw := new(MergeWindow)
	mw.mc = mc
	mw.changes = sync.Changes{
		Sessions:   make([]sync.Change[data.Session], len(mc.Sessions)),
		Timeframes: make([]sync.Change[data.Timeframe], len(mc.Timeframes)),
	}
	mw.Window = builder.GetObject("MergeWindow").Cast().(*adw.Window)
	mw.splitView = builder.GetObject("split_view").Cast().(*adw.OverlaySplitView)
	mw.saveButton = builder.GetObject("save_button").Cast().(*gtk.Button)
	{
		mw.sessionConflictsListView = builder.GetObject("SessionConflictsListView").Cast().(*gtk.ListView)
		mw.sessionConflictsListModel = sessionConflictsListModelType.New()
		mw.sessionConflictsListModel.Splice(0, 0, mc.Sessions...)
		mw.sessionConflictsListSelection = gtk.NewSingleSelection(mw.sessionConflictsListModel)
		mw.sessionConflictsListSelection.SetAutoselect(false)
		mw.sessionConflictsListView.SetModel(mw.sessionConflictsListSelection)
		factory := NewGenericConflictsListItemFactory(&mw.Window.Window, func(v sync.MergeConflict[data.Session]) string {
			return v.Original.Description
		}, sessionConflictsListModelType)
		mw.sessionConflictsListView.SetFactory(&factory.ListItemFactory)
		mw.sessionConflictsListView.ConnectActivate(func(uint) {
			mw.renderSession()
		})
		mw.sessionConflictsListSelection.SetSelected(0)
	}
	mw.mergeSession = NewMergeSession()

	mw.saveButton.ConnectClicked(func() {
		changes <- mw.changes
		mw.saved = true
		mw.Window.Close()
	})
	mw.Window.ConnectCloseRequest(func() bool {
		if !mw.saved {
			errs <- errors.New("user refused to resolve conflicts")
		}
		return false
	})
	return mw
}

func (mw *MergeWindow) renderSession() {
	index := mw.sessionConflictsListSelection.Selected()
	listItem := mw.sessionConflictsListSelection.SelectedItem()
	mc := sessionConflictsListModelType.ObjectValue(listItem)
	mw.mergeSession.SetMergeConflict(mc)
	mw.mergeSession.onSave = func(c sync.Change[data.Session]) {
		mw.changes.Sessions[index] = c
	}
	mw.splitView.SetContent(mw.mergeSession.Main)
}

func NewGenericConflictsListItemFactory[T any](parent *gtk.Window, toString func(T) string, modelType gioutil.ListModelType[T]) *gtk.SignalListItemFactory {
	factory := gtk.NewSignalListItemFactory()
	// we can't use builder factory as it doesn't support introspection of Go objects
	factory.ConnectSetup(func(object *glib.Object) {
		listItem := object.Cast().(*gtk.ListItem)
		label := gtk.NewLabel("")
		label.SetHExpand(true)
		listItem.SetChild(label)
	})
	factory.ConnectBind(func(object *glib.Object) {
		listItem := object.Cast().(*gtk.ListItem)
		label := listItem.Child().(*gtk.Label)
		value := modelType.ObjectValue(listItem.Item())
		label.SetText(toString(value))
	})
	// nothing to do for unbind and teardown
	return factory
}

type MergeSession struct {
	mc     sync.MergeConflict[data.Session]
	c      sync.Change[data.Session]
	onSave func(sync.Change[data.Session])

	Main                     *gtk.Box
	useLocal                 *gtk.Button
	useRemote                *gtk.Button
	sessionDescriptionLocal  *gtk.Entry
	sessionDescriptionResult *gtk.Entry
	sessionDescriptionRemote *gtk.Entry
	sessionNotesLocal        *gtk.TextView
	sessionNotesResult       *gtk.TextView
	sessionNotesRemote       *gtk.TextView
}

func NewMergeSession() *MergeSession {
	log.Printf("creating merge session")
	builder := gtk.NewBuilderFromString(mergeSessionXML)
	log.Printf("=== a")
	ms := new(MergeSession)
	log.Printf("=== b")
	ms.Main = builder.GetObject("main_box").Cast().(*gtk.Box)
	ms.useLocal = builder.GetObject("UseLocal").Cast().(*gtk.Button)
	ms.useRemote = builder.GetObject("UseRemote").Cast().(*gtk.Button)
	ms.sessionDescriptionLocal = builder.GetObject("SessionDescriptionLocal").Cast().(*gtk.Entry)
	ms.sessionDescriptionResult = builder.GetObject("SessionDescriptionResult").Cast().(*gtk.Entry)
	ms.sessionDescriptionRemote = builder.GetObject("SessionDescriptionRemote").Cast().(*gtk.Entry)
	ms.sessionNotesLocal = builder.GetObject("SessionNotesLocal").Cast().(*gtk.TextView)
	ms.sessionNotesResult = builder.GetObject("SessionNotesResult").Cast().(*gtk.TextView)
	ms.sessionNotesRemote = builder.GetObject("SessionNotesRemote").Cast().(*gtk.TextView)
	log.Printf("=== c")

	ms.useLocal.ConnectClicked(func() {
		ms.sessionDescriptionResult.SetText(ms.mc.Local.Description)
		ms.sessionNotesResult.Buffer().SetText(ms.mc.Local.Notes)
	})
	ms.useRemote.ConnectClicked(func() {
		ms.sessionDescriptionResult.SetText(ms.mc.Remote.Description)
		ms.sessionNotesResult.Buffer().SetText(ms.mc.Remote.Notes)
	})
	return ms
}

func (ms *MergeSession) SetMergeConflict(mc sync.MergeConflict[data.Session]) {
	ms.mc = mc
	ms.sessionDescriptionLocal.SetText(mc.Local.Description)
	ms.sessionDescriptionRemote.SetText(mc.Remote.Description)
	ms.sessionNotesLocal.Buffer().SetText(mc.Local.Notes)
	ms.sessionNotesRemote.Buffer().SetText(mc.Remote.Notes)
	ms.saveChanges()
}

func (ms *MergeSession) saveChanges() {
	buf := ms.sessionNotesResult.Buffer()
	ms.c = sync.Change[data.Session]{sync.ChangeOperationExist, data.Session{
		ID:          ms.mc.Original.ID,
		Description: ms.sessionDescriptionResult.Text(),
		Notes:       buf.Text(buf.StartIter(), buf.EndIter(), false),
	}}
}
