package gtkui

import (
	_ "embed"
	"fmt"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/data"
	"nyiyui.ca/jts/database"
)

//go:embed edit_session.ui
var EditSessionXML string

type EditSessionWindow struct {
	Window             *gtk.Window
	sessionID          string
	SessionId          *gtk.Label
	SaveButton         *gtk.Button
	DeleteButton       *gtk.Button
	SessionDescription *gtk.Entry
	SessionNotes       *gtk.TextView
	db                 *database.Database
}

func NewEditSessionWindow(db *database.Database, sessionID string) *EditSessionWindow {
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
	esw.db = db
	esw.sessionID = sessionID
	session, err := db.GetSession(sessionID)
	if err == nil {
		esw.Window.SetTitle(fmt.Sprintf("%sを修正", session.Description))
		esw.SessionDescription.SetText(session.Description)
		esw.SessionNotes.Buffer().SetText(session.Notes)
	}
	return esw
}

func (esw *EditSessionWindow) save() {
	buf := esw.SessionNotes.Buffer()
	err := esw.db.EditSessionProperties(data.Session{
		Description: esw.SessionDescription.Buffer().Text(),
		Notes:       buf.Text(buf.StartIter(), buf.EndIter(), false),
	})
	if err != nil {
		panic(err)
	}
	esw.Window.Close()
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
