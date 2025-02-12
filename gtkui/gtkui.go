package gtkui

import (
	_ "embed"
)

//go:embed main_window.ui
var MainWindowXML string

//go:embed session_row.ui
var SessionRowXML string
