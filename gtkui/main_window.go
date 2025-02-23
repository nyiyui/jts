package gtkui

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"golang.org/x/sync/semaphore"
	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/database/sync"
	"nyiyui.ca/jts/tokens"
)

//go:embed main_window.ui
var MainWindowXML string

type MainWindow struct {
	token            tokens.Token
	db               *database.Database
	originalEDPath   string
	syncSemaphore    *semaphore.Weighted
	syncBackgroundCh chan<- struct{}

	Window                *gtk.ApplicationWindow
	newSessionButton      *gtk.Button
	toastOverlay          *adw.ToastOverlay
	syncStatus            *gtk.Box
	syncButton            *gtk.Button
	syncStatusLabel       *gtk.Label
	syncConflictButtonBox *gtk.Box
	currentListView       *gtk.ListView
}

func NewMainWindow(db *database.Database, token tokens.Token, originalEDPath string) *MainWindow {
	mw := new(MainWindow)
	builder := gtk.NewBuilderFromString(MainWindowXML)
	mw.db = db
	mw.token = token
	mw.originalEDPath = originalEDPath
	mw.syncSemaphore = semaphore.NewWeighted(1)

	mw.Window = builder.GetObject("MainWindow").Cast().(*gtk.ApplicationWindow)
	mw.newSessionButton = builder.GetObject("NewSessionButton").Cast().(*gtk.Button)
	mw.toastOverlay = builder.GetObject("ToastOverlay").Cast().(*adw.ToastOverlay)
	mw.syncStatus = builder.GetObject("SyncStatus").Cast().(*gtk.Box)
	mw.syncButton = builder.GetObject("SyncButton").Cast().(*gtk.Button)
	mw.syncStatusLabel = builder.GetObject("SyncStatusLabel").Cast().(*gtk.Label)
	mw.syncConflictButtonBox = builder.GetObject("SyncConflictButtonBox").Cast().(*gtk.Box)

	mw.newSessionButton.ConnectClicked(func() {
		nsw := NewNewSessionWindow(db, mw.syncBackgroundCh)
		nsw.Window.SetTransientFor(&mw.Window.Window)
		nsw.Window.SetDestroyWithParent(true) // TODO: dialog lives on (after MainWindow is closed) somehow
		nsw.Window.SetApplication(mw.Window.Application())
		nsw.Window.Show()
	})
	mw.syncButton.ConnectClicked(func() {
		go mw.sync(true)
	})
	syncBackgroundCh := make(chan struct{})
	mw.syncBackgroundCh = syncBackgroundCh
	go func() {
		for range syncBackgroundCh {
			go mw.sync(false)
		}
	}()

	mw.currentListView = builder.GetObject("CurrentListView").Cast().(*gtk.ListView)
	m := NewSessionListModel(db)
	m2 := gtk.NewNoSelection(m)
	mw.currentListView.SetModel(m2)

	factory := NewSessionListItemFactory(&mw.Window.Window, db, mw.syncBackgroundCh)
	mw.currentListView.SetFactory(&factory.ListItemFactory)

	return mw
}

func (mw *MainWindow) resolveConflicts(mc sync.MergeConflicts) (sync.Changes, error) {
	for i, c := range mc.Sessions {
		log.Printf("conflict %d: session: %v", i, c)
	}
	for i, c := range mc.Timeframes {
		log.Printf("conflict %d: timeframe: %v", i, c)
	}
	for i, c := range mc.Tasks {
		log.Printf("conflict %d: task: %v", i, c)
	}
	changes := make(chan sync.Changes)
	errs := make(chan error)
	glib.IdleAdd(func() {
		mw2 := NewMergeWindow(mc, changes, errs)
		mw2.Window.SetTransientFor(&mw.Window.Window)
		mw2.Window.SetDestroyWithParent(true) // TODO: dialog lives on (after MainWindow is closed) somehow
		mw2.Window.SetApplication(mw.Window.Application())
		mw2.Window.Show()
	})
	select {
	case c := <-changes:
		return c, nil
	case err := <-errs:
		return sync.Changes{}, err
	}
}

func (mw *MainWindow) readOriginalED() (sync.ExportedDatabase, error) {
	file, err := os.Open(mw.originalEDPath)
	if err != nil {
		return sync.ExportedDatabase{}, err
	}
	defer file.Close()
	var ed sync.ExportedDatabase
	err = json.NewDecoder(file).Decode(&ed)
	return ed, err
}

func (mw *MainWindow) updateOriginalED(ed sync.ExportedDatabase) error {
	file, err := os.Create(mw.originalEDPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(ed)
}

// sync synchronizes the local database with the server database.
// Does not have to be called from the UI goroutine.
func (mw *MainWindow) sync(interactive bool) {
	ok := mw.syncSemaphore.TryAcquire(1)
	if !ok {
		// do not allow multiple syncs to happen at the same time
		// as that is pretty much useless
		return
	}
	defer mw.syncSemaphore.Release(1)
	glib.IdleAdd(func() {
		mw.syncButton.SetSensitive(false)
		mw.syncStatus.SetVisible(true)
	})
	defer glib.IdleAdd(func() {
		mw.syncButton.SetSensitive(true)
		mw.syncStatus.SetVisible(false)
	})
	baseURL, err := url.Parse("https://jts.kiyuri.ca")
	if err != nil {
		panic(err)
	}
	sc := sync.NewServerClient(&http.Client{Timeout: 5 * time.Second}, baseURL, mw.token)
	status := make(chan string)
	defer close(status)
	go func() {
		for s := range status {
			log.Println("sync status: ", s)
			glib.IdleAdd(func() {
				mw.syncStatusLabel.SetLabel(s)
			})
		}
	}()
	original, err := mw.readOriginalED()
	if err != nil {
		log.Printf("read original ed: %s", err)
	}
	_, _ = original, sc
	resolver := mw.resolveConflicts
	if !interactive {
		resolver = nil
	}
	// TODO: SyncDatabase call causes choppiness in GTK
	changes, newED, err := sc.SyncDatabase(context.Background(), original, mw.db, resolver, status)
	if err != nil {
		log.Println("sync: ", err)
		glib.IdleAdd(func() {
			toast := adw.NewToast(fmt.Sprintf("同期に失敗しました。 %s", err))
			toast.SetPriority(adw.ToastPriorityHigh)
			mw.toastOverlay.AddToast(toast)
		})
		return
	}
	_ = changes
	_ = newED
	if err = mw.updateOriginalED(newED); err != nil {
		log.Printf("update original ed: %s", err)
		glib.IdleAdd(func() {
			toast := adw.NewToast(fmt.Sprintf("元データベース更新に失敗しました。 %s", err))
			toast.SetPriority(adw.ToastPriorityHigh)
			mw.toastOverlay.AddToast(toast)
		})
	}
	log.Printf("done: sessions=%d, timeframes=%d", len(changes.Sessions), len(changes.Timeframes))
	glib.IdleAdd(func() {
		toast := adw.NewToast(fmt.Sprintf("同期しました。 セッション: %d, 打刻: %d", len(changes.Sessions), len(changes.Timeframes)))
		toast.SetPriority(adw.ToastPriorityNormal)
		mw.toastOverlay.AddToast(toast)
	})
}
