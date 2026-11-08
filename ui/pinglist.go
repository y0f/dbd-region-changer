package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/y0f/dbd-region-changer/internal/ping"
	"github.com/y0f/dbd-region-changer/internal/region"
)

func pingColor(ms int) color.Color {
	if ms < 0 {
		return colMuted
	}
	return colFg
}

func latencyText(ms int) string {
	if ms < 0 {
		return "—"
	}
	return fmt.Sprintf("%d ms", ms)
}

type PingRow struct {
	Name, Code, Latency, Loss string
	Color                     color.Color
}

func pingRows(regions []region.Server, results []ping.Result) []PingRow {
	rows := make([]PingRow, len(regions))
	for i, r := range regions {
		res := ping.Result{}
		if i < len(results) {
			res = results[i]
		}
		latency, loss := latencyText(res.DisplayPing()), "No response"
		if res.OK {
			loss = fmt.Sprintf("%.0f%% loss", res.DisplayLossPct())
			if res.Fallback {
				loss = "TCP estimate"
			}
		}
		rows[i] = PingRow{r.Pretty, r.Code, latency, loss, pingColor(res.DisplayPing())}
	}
	return rows
}

// PingList reuses its text objects between samples. The region set is small
// and fixed after discovery, so a virtualized list adds no useful complexity.
type PingList struct {
	box  *fyne.Container
	rows [][4]*canvas.Text
}

func NewPingList() *PingList { return &PingList{box: container.NewVBox()} }

func (p *PingList) Object() fyne.CanvasObject { return p.box }

// SetRows must run on the UI goroutine.
func (p *PingList) SetRows(rows []PingRow) {
	if len(rows) != len(p.rows) {
		p.rows = make([][4]*canvas.Text, len(rows))
		p.box.Objects = make([]fyne.CanvasObject, len(rows))
		for i := range rows {
			name := newColoredText("", colFg)
			name.TextSize = 14
			code := newMonoText("", colMuted)
			code.TextSize = 11
			latency := newMonoText("", colFg)
			latency.Alignment = fyne.TextAlignTrailing
			loss := newColoredText("", colMuted)
			loss.TextSize = 11
			loss.Alignment = fyne.TextAlignTrailing
			p.rows[i] = [4]*canvas.Text{name, code, latency, loss}
			p.box.Objects[i] = container.NewBorder(nil, nil, nil,
				container.NewVBox(latency, loss), container.NewVBox(name, code))
		}
	}
	for i, row := range rows {
		for j, value := range []string{row.Name, row.Code, row.Latency, row.Loss} {
			obj := p.rows[i][j]
			col := color.Color(colMuted)
			if j == 0 || j == 2 {
				col = row.Color
			}
			if obj.Text != value || obj.Color != col {
				obj.Text, obj.Color = value, col
				obj.Refresh()
			}
		}
	}
	p.box.Refresh()
}
