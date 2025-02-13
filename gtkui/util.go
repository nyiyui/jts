package gtkui

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

func PresentDialog(mainWindow, dialog *gtk.Window) {
	dialog.SetTransientFor(mainWindow)
	dialog.SetDestroyWithParent(true)
	dialog.SetApplication(mainWindow.Application())
	dialog.Present()
}
