package gtkui

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/database/sync"
	"nyiyui.ca/jts/tokens"
)

//go:embed main_window.ui
var MainWindowXML string

type MainWindow struct {
	Window                *gtk.ApplicationWindow
	newSessionButton      *gtk.Button
	toastOverlay          *adw.ToastOverlay
	syncStatus            *gtk.Box
	syncButton            *gtk.Button
	syncStatusLabel       *gtk.Label
	syncConflictButtonBox *gtk.Box
	currentListView       *gtk.ListView
	token                 tokens.Token
	db                    *database.Database
	originalEDPath        string
}

func NewMainWindow(db *database.Database, token tokens.Token, originalEDPath string) *MainWindow {
	mw := new(MainWindow)
	builder := gtk.NewBuilderFromString(MainWindowXML)
	mw.db = db
	mw.token = token
	mw.originalEDPath = originalEDPath

	mw.Window = builder.GetObject("MainWindow").Cast().(*gtk.ApplicationWindow)
	mw.newSessionButton = builder.GetObject("NewSessionButton").Cast().(*gtk.Button)
	mw.toastOverlay = builder.GetObject("ToastOverlay").Cast().(*adw.ToastOverlay)
	mw.syncStatus = builder.GetObject("SyncStatus").Cast().(*gtk.Box)
	mw.syncButton = builder.GetObject("SyncButton").Cast().(*gtk.Button)
	mw.syncStatusLabel = builder.GetObject("SyncStatusLabel").Cast().(*gtk.Label)
	mw.syncConflictButtonBox = builder.GetObject("SyncConflictButtonBox").Cast().(*gtk.Box)

	mw.newSessionButton.ConnectClicked(func() {
		nsw := NewNewSessionWindow(db)
		nsw.Window.SetTransientFor(&mw.Window.Window)
		nsw.Window.SetDestroyWithParent(true) // TODO: dialog lives on (after MainWindow is closed) somehow
		nsw.Window.SetApplication(mw.Window.Application())
		nsw.Window.Show()
	})
	mw.syncButton.ConnectClicked(mw.sync)
	mergeButton := builder.GetObject("merge").Cast().(*gtk.Button)
	mergeButton.ConnectClicked(func() {
		nsw := NewMergeWindow()
		nsw.Window.SetTransientFor(&mw.Window.Window)
		nsw.Window.SetDestroyWithParent(true) // TODO: dialog lives on (after MainWindow is closed) somehow
		nsw.Window.SetApplication(mw.Window.Application())
		nsw.Window.Show()
	})

	mw.currentListView = builder.GetObject("CurrentListView").Cast().(*gtk.ListView)
	m := NewSessionListModel(db)
	m2 := gtk.NewNoSelection(m)
	mw.currentListView.SetModel(m2)

	factory := NewSessionListItemFactory(&mw.Window.Window, db)
	mw.currentListView.SetFactory(&factory.ListItemFactory)

	return mw
}

func (mw *MainWindow) resolveConflicts(mc sync.MergeConflicts) (sync.Changes, error) {
	for i, c := range mc.Sessions {
		log.Printf("conflict %d: session: %s", i, c)
	}
	for i, c := range mc.Timeframes {
		log.Printf("conflict %d: timeframe: %s", i, c)
	}
	return sync.Changes{}, errors.New("not implemented")
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

func (mw *MainWindow) sync() {
	mw.syncStatus.SetVisible(true)
	go func() {
		defer mw.syncStatus.SetVisible(false)
		baseURL, err := url.Parse("https://jts.kiyuri.ca")
		if err != nil {
			panic(err)
		}
		sc := sync.NewServerClient(&http.Client{Timeout: 5 * time.Second}, baseURL, mw.token)
		status := make(chan string)
		go func() {
			for s := range status {
				log.Println("sync status: ", s)
				mw.syncStatusLabel.SetLabel(s)
			}
		}()
		original, err := mw.readOriginalED()
		if err != nil {
			log.Printf("read original ed: %s", err)
		}
		changes, newED, err := sc.SyncDatabase(context.Background(), original, mw.db, mw.resolveConflicts, status)
		if err != nil {
			log.Println("sync: ", err)
			toast := adw.NewToast(fmt.Sprintf("同期に失敗しました。 %s", err))
			toast.SetPriority(adw.ToastPriorityHigh)
			mw.toastOverlay.AddToast(toast)
			return
		}
		if err = mw.updateOriginalED(newED); err != nil {
			log.Printf("update original ed: %s", err)
		}
		log.Printf("done: sessions=%d, timeframes=%d", len(changes.Sessions), len(changes.Timeframes))
		toast := adw.NewToast(fmt.Sprintf("同期しました。 セッション: %d, 打刻: %d", len(changes.Sessions), len(changes.Timeframes)))
		toast.SetPriority(adw.ToastPriorityHigh)
		mw.toastOverlay.AddToast(toast)
	}()
}
