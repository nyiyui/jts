package main

import (
	"os"
	"runtime"
	"time"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/data"
	"nyiyui.ca/jts/database/sync"
	"nyiyui.ca/jts/gtkui"
)

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func main() {
	runtime.LockOSThread() // for gtk
	app := gtk.NewApplication("ca.nyiyui.jts-test", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() {
		mccs := sync.MergeConflicts{
			Sessions: []sync.MergeConflict[data.Session]{
				{
					Original: data.Session{Rowid: 1, ID: "1", Description: "Session 1", Notes: "Notes 1"},
					Local:    data.Session{Rowid: 1, ID: "1", Description: "Session 1local", Notes: "Notes 2"},
					Remote:   data.Session{Rowid: 1, ID: "1", Description: "Session 1remote", Notes: "Notes 2"},
				},
				{
					Original: data.Session{Rowid: 2, ID: "2", Description: "Session 2", Notes: "Notes 1"},
					Local:    data.Session{Rowid: 2, ID: "2", Description: "Session 2local", Notes: "Notes 2"},
					Remote:   data.Session{Rowid: 2, ID: "2", Description: "Session 2remote", Notes: "Notes 2"},
				},
			},
			Timeframes: []sync.MergeConflict[data.Timeframe]{
				{
					Original: data.Timeframe{Rowid: 1, ID: "1", SessionID: "1", Start: mustParseTime("2025-04-01T00:00:00Z"), End: mustParseTime("2025-04-02T00:00:00Z")},
					Local:    data.Timeframe{Rowid: 1, ID: "1", SessionID: "1", Start: mustParseTime("2025-04-01T00:00:00Z"), End: mustParseTime("2025-04-04T00:00:00Z")},
					Remote:   data.Timeframe{Rowid: 1, ID: "1", SessionID: "1", Start: mustParseTime("2025-04-01T00:00:00Z"), End: mustParseTime("2025-04-03T00:00:00Z")},
				},
			},
		}
		mergeWindow := gtkui.NewMergeWindow(mccs, nil, make(chan error, 1))
		mergeWindow.Window.SetApplication(app)
		mergeWindow.Window.Show()
	})

	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}
