package gtkui

import (
	_ "embed"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

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

//go:embed merge_timeframe.ui
var mergeTimeframeXML string

var sessionConflictsListModelType = gioutil.NewListModelType[mergeConflictRef]()

type mergeConflictType int

const (
	mergeConflictTypeSession mergeConflictType = iota
	mergeConflictTypeTimeframe
	mergeConflictTypeTask
)

type mergeConflictRef struct {
	mergeConflictType
	Index int
}

func makeMergeConflictRefs(mcs sync.MergeConflicts) []mergeConflictRef {
	result := make([]mergeConflictRef, 0, len(mcs.Sessions)+len(mcs.Timeframes)+len(mcs.Tasks))
	for i := range mcs.Sessions {
		result = append(result, mergeConflictRef{
			mergeConflictTypeSession,
			i,
		})
	}
	for i := range mcs.Timeframes {
		result = append(result, mergeConflictRef{
			mergeConflictTypeTimeframe,
			i,
		})
	}
	for i := range mcs.Tasks {
		result = append(result, mergeConflictRef{
			mergeConflictTypeTask,
			i,
		})
	}
	return result
}

type MergeWindow struct {
	mc      sync.MergeConflicts
	changes sync.Changes
	saved   bool

	Window                        *adw.Window
	saveButton                    *gtk.Button
	sessionConflictsListView      *gtk.ListView
	sessionConflictsListModel     *gioutil.ListModel[mergeConflictRef]
	sessionConflictsListSelection *gtk.SingleSelection
	splitView                     *adw.OverlaySplitView
	mergeSession                  *MergeSession
	mergeTimeframe                *MergeTimeframe
}

func NewMergeWindow(mc sync.MergeConflicts, changes chan<- sync.Changes, errs chan<- error) *MergeWindow {
	builder := gtk.NewBuilderFromString(mergeWindowXML)
	mw := new(MergeWindow)
	mw.mc = mc
	mw.changes = sync.Changes{
		Sessions:   make([]sync.Change[data.Session], len(mc.Sessions)),
		Timeframes: make([]sync.Change[data.Timeframe], len(mc.Timeframes)),
	}
	log.Printf("mc %v", mc)
	mw.Window = builder.GetObject("MergeWindow").Cast().(*adw.Window)
	mw.splitView = builder.GetObject("split_view").Cast().(*adw.OverlaySplitView)
	mw.saveButton = builder.GetObject("save_button").Cast().(*gtk.Button)
	{
		mw.sessionConflictsListView = builder.GetObject("SessionConflictsListView").Cast().(*gtk.ListView)
		mw.sessionConflictsListModel = sessionConflictsListModelType.New()
		mw.sessionConflictsListModel.Splice(0, 0, makeMergeConflictRefs(mc)...)
		mw.sessionConflictsListSelection = gtk.NewSingleSelection(mw.sessionConflictsListModel)
		mw.sessionConflictsListView.SetModel(mw.sessionConflictsListSelection)
		factory := NewGenericConflictsListItemFactory(&mw.Window.Window, func(v mergeConflictRef) string {
			switch v.mergeConflictType {
			case mergeConflictTypeSession:
				return mw.mc.Sessions[v.Index].Original.Description
			case mergeConflictTypeTimeframe:
				timeframe := mw.mc.Timeframes[v.Index].Original
				session := mw.mc.Sessions[slices.IndexFunc(mw.mc.Sessions, func(s sync.MergeConflict[data.Session]) bool {
					return s.Original.ID == timeframe.SessionID
				})]
				return fmt.Sprintf("Timeframe for %s", session.Original.Description)
			case mergeConflictTypeTask:
				return mw.mc.Tasks[v.Index].Original.Description
			default:
				panic(fmt.Sprintf("unknown merge conflict type %d", v.mergeConflictType))
			}
		}, sessionConflictsListModelType)
		mw.sessionConflictsListView.SetFactory(&factory.ListItemFactory)
		mw.sessionConflictsListView.ConnectActivate(func(uint) {
			mw.renderSelected()
		})
		mw.sessionConflictsListSelection.SetSelected(0)
		glib.IdleAdd(func() {
			mw.renderSelected()
		})
	}
	mw.mergeSession = NewMergeSession()
	mw.mergeTimeframe = NewMergeTimeframe()

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

func (mw *MergeWindow) renderSelected() {
	index := mw.sessionConflictsListSelection.Selected()
	listItem := mw.sessionConflictsListSelection.SelectedItem()
	mcRef := sessionConflictsListModelType.ObjectValue(listItem)
	switch mcRef.mergeConflictType {
	case mergeConflictTypeSession:
		mw.mergeSession.SetMergeConflict(mw.mc.Sessions[mcRef.Index])
		mw.mergeSession.onSave = func(c sync.Change[data.Session]) {
			mw.changes.Sessions[index] = c
		}
		mw.splitView.SetContent(mw.mergeSession.Main)
	case mergeConflictTypeTimeframe:
		mw.mergeTimeframe.SetMergeConflict(mw.mc.Timeframes[mcRef.Index])
		mw.mergeTimeframe.onSave = func(c sync.Change[data.Timeframe]) {
			mw.changes.Timeframes[index] = c
		}
		mw.splitView.SetContent(mw.mergeTimeframe.Main)
	default:
		panic(fmt.Sprintf("unknown merge conflict type %d", mcRef.mergeConflictType))
	}
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
	builder := gtk.NewBuilderFromString(mergeSessionXML)
	ms := new(MergeSession)
	ms.Main = builder.GetObject("main_box").Cast().(*gtk.Box)
	ms.useLocal = builder.GetObject("UseLocal").Cast().(*gtk.Button)
	ms.useRemote = builder.GetObject("UseRemote").Cast().(*gtk.Button)
	ms.sessionDescriptionLocal = builder.GetObject("SessionDescriptionLocal").Cast().(*gtk.Entry)
	ms.sessionDescriptionResult = builder.GetObject("SessionDescriptionResult").Cast().(*gtk.Entry)
	ms.sessionDescriptionRemote = builder.GetObject("SessionDescriptionRemote").Cast().(*gtk.Entry)
	ms.sessionNotesLocal = builder.GetObject("SessionNotesLocal").Cast().(*gtk.TextView)
	ms.sessionNotesResult = builder.GetObject("SessionNotesResult").Cast().(*gtk.TextView)
	ms.sessionNotesRemote = builder.GetObject("SessionNotesRemote").Cast().(*gtk.TextView)

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
	if ms.mc.Local.Notes == ms.mc.Remote.Notes {
		ms.sessionNotesResult.Buffer().SetText(ms.mc.Local.Notes)
	}
	if ms.mc.Local.Description == ms.mc.Remote.Description {
		ms.sessionDescriptionResult.SetText(ms.mc.Local.Description)
	}
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

type MergeTimeframe struct {
	mc     sync.MergeConflict[data.Timeframe]
	c      sync.Change[data.Timeframe]
	onSave func(sync.Change[data.Timeframe])

	Main                 *gtk.Box
	useLocal             *gtk.Button
	useRemote            *gtk.Button
	timeframeStartLocal  *gtk.Entry
	timeframeStartResult *gtk.Entry
	timeframeStartRemote *gtk.Entry
	timeframeEndLocal    *gtk.Entry
	timeframeEndResult   *gtk.Entry
	timeframeEndRemote   *gtk.Entry
	timeframeDoneLocal   *gtk.CheckButton
	timeframeDoneResult  *gtk.CheckButton
	timeframeDoneRemote  *gtk.CheckButton
	errorMessage         *gtk.Label
}

func timeFormat(t time.Time) string {
	return t.Local().Format(TimeFormat)
}

func NewMergeTimeframe() *MergeTimeframe {
	builder := gtk.NewBuilderFromString(mergeTimeframeXML)
	mt := new(MergeTimeframe)
	mt.Main = builder.GetObject("main_box").Cast().(*gtk.Box)
	mt.useLocal = builder.GetObject("UseLocal").Cast().(*gtk.Button)
	mt.useRemote = builder.GetObject("UseRemote").Cast().(*gtk.Button)

	mt.timeframeStartLocal = builder.GetObject("TimeframeStartLocal").Cast().(*gtk.Entry)
	mt.timeframeStartResult = builder.GetObject("TimeframeStartResult").Cast().(*gtk.Entry)
	mt.timeframeStartRemote = builder.GetObject("TimeframeStartRemote").Cast().(*gtk.Entry)
	mt.timeframeEndLocal = builder.GetObject("TimeframeEndLocal").Cast().(*gtk.Entry)
	mt.timeframeEndResult = builder.GetObject("TimeframeEndResult").Cast().(*gtk.Entry)
	mt.timeframeEndRemote = builder.GetObject("TimeframeEndRemote").Cast().(*gtk.Entry)
	mt.timeframeDoneLocal = builder.GetObject("TimeframeDoneLocal").Cast().(*gtk.CheckButton)
	mt.timeframeDoneResult = builder.GetObject("TimeframeDoneResult").Cast().(*gtk.CheckButton)
	mt.timeframeDoneRemote = builder.GetObject("TimeframeDoneRemote").Cast().(*gtk.CheckButton)
	mt.errorMessage = builder.GetObject("ErrorMessage").Cast().(*gtk.Label)

	mt.useLocal.ConnectClicked(func() {
		mt.timeframeStartResult.SetText(timeFormat(mt.mc.Local.Start))
		mt.timeframeEndResult.SetText(timeFormat(mt.mc.Local.End))
		mt.timeframeDoneResult.SetActive(mt.mc.Local.Done)
		mt.saveChanges()
	})
	mt.useRemote.ConnectClicked(func() {
		mt.timeframeStartResult.SetText(timeFormat(mt.mc.Remote.Start))
		mt.timeframeEndResult.SetText(timeFormat(mt.mc.Remote.End))
		mt.timeframeDoneResult.SetActive(mt.mc.Remote.Done)
		mt.saveChanges()
	})
	return mt
}

func (mt *MergeTimeframe) SetMergeConflict(mc sync.MergeConflict[data.Timeframe]) {
	if mc.Local.SessionID != mc.Remote.SessionID {
		panic("bail")
	}
	mt.mc = mc
	mt.timeframeStartLocal.SetText(timeFormat(mc.Local.Start))
	mt.timeframeStartRemote.SetText(timeFormat(mc.Remote.Start))
	mt.timeframeEndLocal.SetText(timeFormat(mc.Local.End))
	mt.timeframeEndRemote.SetText(timeFormat(mc.Remote.End))
	mt.timeframeDoneLocal.SetActive(mc.Local.Done)
	mt.timeframeDoneRemote.SetActive(mc.Remote.Done)
	if mt.mc.Local.Start.Equal(mt.mc.Remote.Start) {
		mt.timeframeStartResult.SetText(timeFormat(mc.Local.Start))
	}
	if mt.mc.Local.End.Equal(mt.mc.Remote.End) {
		mt.timeframeEndResult.SetText(timeFormat(mc.Local.End))
	}
	if mt.mc.Local.Done == mt.mc.Remote.Done {
		mt.timeframeDoneResult.SetActive(mc.Local.Done)
	}
	mt.saveChanges()
}

const TimeFormat = "2006-01-02 15:04"

func (mt *MergeTimeframe) saveChanges() {
	mt.errorMessage.SetLabel("")
	start, err := time.ParseInLocation(TimeFormat, mt.timeframeStartResult.Text(), time.Local)
	if err != nil {
		mt.errorMessage.SetLabel(fmt.Sprintf("Invalid start time: %v", err))
		return
	}
	end, err := time.ParseInLocation(TimeFormat, mt.timeframeEndResult.Text(), time.Local)
	if err != nil {
		mt.errorMessage.SetLabel(fmt.Sprintf("Invalid end time: %v", err))
		return
	}
	mt.c = sync.Change[data.Timeframe]{sync.ChangeOperationExist, data.Timeframe{
		ID:        mt.mc.Original.ID,
		SessionID: mt.mc.Original.SessionID,
		Start:     start,
		End:       end,
		Done:      mt.timeframeDoneResult.Active(),
	}}
}
