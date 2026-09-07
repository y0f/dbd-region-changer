package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/y0f/dbd-region-changer/internal/ping"
)

var (
	colGreen  = color.NRGBA{R: 0x4C, G: 0xAF, B: 0x50, A: 0xFF}
	colOrange = color.NRGBA{R: 0xFF, G: 0x98, B: 0x00, A: 0xFF}
	colRed    = color.NRGBA{R: 0xF4, G: 0x43, B: 0x36, A: 0xFF}
	colGrey   = color.NRGBA{R: 0x9E, G: 0x9E, B: 0x9E, A: 0xFF}
)

func pingColor(ms int) color.Color {
	switch ping.ColorFor(ms) {
	case ping.Green:
		return colGreen
	case ping.Orange:
		return colOrange
	case ping.Red:
		return colRed
	default:
		return colGrey
	}
}

type PingRow struct {
	Text  string
	Color color.Color
}

// PingList stacks color-coded text rows (canvas.Text allows arbitrary RGB, unlike widget.RichText).
type PingList struct {
	box *fyne.Container
}

func NewPingList() *PingList {
	return &PingList{box: container.NewVBox()}
}

func (p *PingList) Object() fyne.CanvasObject {
	return p.box
}

// SetRows replaces the rows; must run on the UI goroutine (via fyne.Do).
func (p *PingList) SetRows(rows []PingRow) {
	objs := make([]fyne.CanvasObject, 0, len(rows))
	for _, r := range rows {
		t := newMonoText(r.Text, r.Color)
		objs = append(objs, t)
	}
	p.box.Objects = objs
	p.box.Refresh()
}
