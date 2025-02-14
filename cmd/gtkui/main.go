package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	_ "embed"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/database/sync"
	"nyiyui.ca/jts/gtkui"
	"nyiyui.ca/jts/tokens"
)

func main() {
	var token tokens.Token
	var err error
	rawToken := os.Getenv("JTS_SERVER_TOKEN")
	if rawToken != "" {
		token, err = tokens.ParseToken(rawToken)
		if err != nil {
			log.Fatalf("parse token: %s", err)
		}
	}

	db, err := database.NewDatabase("")
	if err != nil {
		log.Fatalf("new db: %s", err)
	}
	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate db: %s", err)
	}

	app := gtk.NewApplication("ca.nyiyui.jts", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() { activate(app, db, token) })

	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}

func activate(app *gtk.Application, db *database.Database, token tokens.Token) {
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
	toastOverlay := builder.GetObject("ToastOverlay").Cast().(*adw.ToastOverlay)
	syncStatus := builder.GetObject("SyncStatus").Cast().(*gtk.Box)
	syncButton := builder.GetObject("SyncButton").Cast().(*gtk.Button)
	syncButton.ConnectClicked(func() {
		syncStatus.SetVisible(true)
		go func() {
			defer syncStatus.SetVisible(false)
			baseURL, err := url.Parse("https://jts.kiyuri.ca")
			if err != nil {
				panic(err)
			}
			sc := sync.NewServerClient(&http.Client{Timeout: 5 * time.Second}, baseURL, token)
			status := make(chan string)
			go func() {
				syncStatusLabel := builder.GetObject("SyncStatusLabel").Cast().(*gtk.Label)
				for s := range status {
					log.Println("sync status: ", s)
					syncStatusLabel.SetLabel(s)
				}
			}()
			changes, err := sc.SyncDatabase(context.Background(), sync.ExportedDatabase{}, db, status)
			if err != nil {
				log.Println("sync: ", err)
				toast := adw.NewToast(fmt.Sprintf("同期に失敗しました。 %s", err))
				toast.SetPriority(adw.ToastPriorityHigh)
				toastOverlay.AddToast(toast)
				return
			}
			log.Printf("done: sessions=%d, timeframes=%d", len(changes.Sessions), len(changes.Timeframes))
			toast := adw.NewToast(fmt.Sprintf("同期しました。 セッション: %d, 打刻: %d", len(changes.Sessions), len(changes.Timeframes)))
			toast.SetPriority(adw.ToastPriorityHigh)
			toastOverlay.AddToast(toast)
		}()
	})
	mergeButton := builder.GetObject("merge").Cast().(*gtk.Button)
	mergeButton.ConnectClicked(func() {
		nsw := gtkui.NewMergeWindow()
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
