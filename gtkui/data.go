package gtkui

import (
	"github.com/diamondburned/gotk4/pkg/core/gioutil"
	"nyiyui.ca/jts/data"
)

var SessionListModelType = gioutil.NewListModelType[data.Session]()

type SessionListModel struct {
	*gioutil.ListModel[data.Session]
}

func NewSessionListModel() *SessionListModel {
	return &SessionListModel{SessionListModelType.New()}
}

func (m *SessionListModel) FillFromSlice(sessions []data.Session) {
	for _, s := range sessions {
		m.Append(s)
	}
}
