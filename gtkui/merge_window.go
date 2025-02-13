package gtkui

import (
	_ "embed"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

//go:embed merge_window.ui
var MergeWindowXML string

type MergeWindow struct {
	Window *adw.Window
}

func NewMergeWindow() *MergeWindow {
	builder := gtk.NewBuilderFromString(MergeWindowXML)
	mw := new(MergeWindow)
	mw.Window = builder.GetObject("MergeWindow").Cast().(*adw.Window)
	return mw
}
