package main

import (
	"os"

	_ "embed"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"nyiyui.ca/jts/gtkui"
)

func main() {
	app := gtk.NewApplication("com.github.diamondburned.gotk4-examples.gtk4.builder", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() { activate(app) })

	if code := app.Run(os.Args); code > 0 {
		os.Exit(code)
	}
}

func activate(app *gtk.Application) {
	// You can build UIs using Cambalache (https://flathub.org/apps/details/ar.xjuan.Cambalache)
	builder := gtk.NewBuilderFromString(gtkui.UIXML)
	window := builder.GetObject("CurrentWindow").Cast().(*gtk.Window)

	app.AddWindow(window)
	window.Show()
}
