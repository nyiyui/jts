package gtkui

import (
	"slices"
	"time"

	_ "embed"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/data"
	"nyiyui.ca/jts/database"
)

//go:embed edit_timeframe.ui
var EditTimeframeXML string

type EditTimeframeWindow struct {
	Window             *gtk.Window
	sessionID          string
	timeframeID        string
	TimeframeId        *gtk.Label
	SaveButton         *gtk.Button
	DeleteButton       *gtk.Button
	TimeframeStart     *gtk.Entry
	TimeframeStartHint *gtk.Label
	TimeframeEnd       *gtk.Entry
	TimeframeEndHint   *gtk.Label
	timeframe          data.Timeframe
	timeFormat         string
	db                 *database.Database
}

func NewEditTimeframeWindow(db *database.Database, sessionID, timeframeID string) *EditTimeframeWindow {
	builder := gtk.NewBuilderFromString(EditTimeframeXML)
	etw := new(EditTimeframeWindow)
	etw.timeFormat = "2006-01-02 15:04"
	etw.Window = builder.GetObject("EditTimeframeWindow").Cast().(*gtk.Window)
	etw.TimeframeId = builder.GetObject("TimeframeId").Cast().(*gtk.Label)
	etw.SaveButton = builder.GetObject("SaveButton").Cast().(*gtk.Button)
	etw.DeleteButton = builder.GetObject("DeleteButton").Cast().(*gtk.Button)
	etw.TimeframeStart = builder.GetObject("TimeframeStart").Cast().(*gtk.Entry)
	etw.TimeframeStartHint = builder.GetObject("TimeframeStartHint").Cast().(*gtk.Label)
	etw.TimeframeEnd = builder.GetObject("TimeframeEnd").Cast().(*gtk.Entry)
	etw.TimeframeEndHint = builder.GetObject("TimeframeEndHint").Cast().(*gtk.Label)

	etw.TimeframeId.SetLabel(timeframeID)
	etw.SaveButton.ConnectClicked(etw.save)
	etw.DeleteButton.ConnectClicked(etw.delete_)
	etw.TimeframeStart.ConnectChanged(etw.update)
	etw.TimeframeEnd.ConnectChanged(etw.update)

	etw.db = db
	etw.sessionID = sessionID
	etw.timeframeID = timeframeID
	session, err := db.GetSession(sessionID)
	if err == nil {
		etw.Window.SetTitle("打刻を修正")
		i := slices.IndexFunc(session.Timeframes, func(tf data.Timeframe) bool {
			return tf.ID == timeframeID
		})
		etw.timeframe = session.Timeframes[i]
		etw.TimeframeStart.SetText(etw.timeframe.Start.Local().Format(etw.timeFormat))
		etw.TimeframeEnd.SetText(etw.timeframe.End.Local().Format(etw.timeFormat))
		etw.update()
	}
	return etw
}

func (etw *EditTimeframeWindow) update() {
	start, err := time.ParseInLocation(etw.timeFormat, etw.TimeframeStart.Text(), time.Local)
	if err != nil {
		etw.TimeframeStartHint.SetLabel(err.Error())
	} else {
		etw.TimeframeStartHint.SetLabel(time.Until(start).Round(1 * time.Minute).String())
	}
	end, err := time.ParseInLocation(etw.timeFormat, etw.TimeframeEnd.Text(), time.Local)
	if err != nil {
		etw.TimeframeEndHint.SetLabel(err.Error())
	} else {
		etw.TimeframeEndHint.SetLabel(time.Until(end).Round(1 * time.Minute).String())
	}
}

func (etw *EditTimeframeWindow) save() {
	start, err := time.ParseInLocation(etw.timeFormat, etw.TimeframeStart.Text(), time.Local)
	if err != nil {
		return
	}
	end, err := time.ParseInLocation(etw.timeFormat, etw.TimeframeEnd.Text(), time.Local)
	if err != nil {
		return
	}
	err = etw.db.EditTimeframe(etw.sessionID, etw.timeframe.ID, data.Timeframe{
		Start: start,
		End:   end,
	})
	if err != nil {
		panic(err)
	}
	etw.Window.Destroy()
}

func (etw *EditTimeframeWindow) delete_() {
	err := etw.db.DeleteTimeframe(etw.sessionID, etw.timeframe.ID)
	if err != nil {
		panic(err)
	}
	etw.Window.Destroy()
}
