package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

func newColoredText(s string, c color.Color) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = 13
	return t
}

// newMonoText is a monospaced colored row, so ping columns line up.
func newMonoText(s string, c color.Color) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = 13
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}
