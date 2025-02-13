package gtkui

import (
	_ "embed"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/data"
	"nyiyui.ca/jts/database"
)

//go:embed new_session.ui
var NewSessionXML string

type NewSessionWindow struct {
	Window             *gtk.Window
	SaveButton         *gtk.Button
	SessionDescription *gtk.Entry
	SessionNotes       *gtk.TextView
	db                 *database.Database
}

func NewNewSessionWindow(db *database.Database) *NewSessionWindow {
	builder := gtk.NewBuilderFromString(NewSessionXML)
	nsw := new(NewSessionWindow)
	nsw.Window = builder.GetObject("NewSessionWindow").Cast().(*gtk.Window)
	nsw.SaveButton = builder.GetObject("SaveButton").Cast().(*gtk.Button)
	nsw.SessionDescription = builder.GetObject("SessionDescription").Cast().(*gtk.Entry)
	nsw.SessionNotes = builder.GetObject("SessionNotes").Cast().(*gtk.TextView)
	nsw.SaveButton.ConnectClicked(nsw.save)
	nsw.db = db
	return nsw
}

func (nsw *NewSessionWindow) save() {
	buf := nsw.SessionNotes.Buffer()
	_, err := nsw.db.AddSession(data.Session{
		Description: nsw.SessionDescription.Buffer().Text(),
		Notes:       buf.Text(buf.StartIter(), buf.EndIter(), false),
		Timeframes: []data.Timeframe{
			{
				Start: time.Now(),
				End:   time.Now(),
			},
		},
	})
	if err != nil {
		panic(err)
	}
	nsw.Window.Close()
}
