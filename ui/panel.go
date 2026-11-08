package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// panel groups controls on a neutral surface. An empty title omits the header.
func panel(title string, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colPanel)
	bg.CornerRadius = 6
	bg.StrokeColor = colPanelStroke
	bg.StrokeWidth = 1

	inner := content
	if title != "" {
		head := canvas.NewText(title, colMuted)
		head.TextStyle = fyne.TextStyle{Bold: true}
		head.TextSize = 13
		inner = container.NewBorder(head, nil, nil, nil, content)
	}
	return container.NewStack(bg, container.NewPadded(inner))
}
