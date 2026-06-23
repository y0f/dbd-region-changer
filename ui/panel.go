package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// panel wraps content in a rounded, translucent wine surface with a hairline rim
// so each section floats over the backdrop gradient. An empty title omits the header.
func panel(title string, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colPanel)
	bg.CornerRadius = 10
	bg.StrokeColor = colPanelStroke
	bg.StrokeWidth = 1

	inner := content
	if title != "" {
		head := canvas.NewText(title, colAccent)
		head.TextStyle = fyne.TextStyle{Bold: true}
		head.TextSize = 13
		inner = container.NewVBox(head, content)
	}
	return container.NewStack(bg, container.NewPadded(inner))
}
