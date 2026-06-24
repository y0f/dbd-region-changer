package ui

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/y0f/dbd-region-changer/internal/config"
	"github.com/y0f/dbd-region-changer/internal/version"
)

func (a *App) openURL(raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		a.log.Error("bad url", "url", raw, "err", err)
		return
	}
	if err := a.fyneApp.OpenURL(u); err != nil {
		a.log.Error("open url failed", "err", err)
	}
}

func (a *App) buildTitle() fyne.CanvasObject {
	title := canvas.NewText("DBD Region Changer", colAccent)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20

	ver := canvas.NewText("v"+version.Current.Display(), colMuted)
	ver.TextSize = 12

	gh := widget.NewButton("GitHub", func() { a.openURL(config.GithubRepoURL) })
	gh.Importance = widget.LowImportance
	menu := widget.NewButton("...", a.showMenu)
	menu.Importance = widget.LowImportance

	left := container.NewHBox(title, container.NewCenter(ver))
	right := container.NewHBox(gh, menu)
	return container.NewBorder(nil, nil, left, right)
}

func (a *App) showMenu() {
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("Set DBD path (firewall)", a.onSetDBDPath),
		fyne.NewMenuItem("Check for updates", a.onCheckUpdates),
		fyne.NewMenuItem("Dump logs", a.onDumpLogs),
	)
	size := a.win.Canvas().Size()
	widget.ShowPopUpMenuAtPosition(menu, a.win.Canvas(), fyne.NewPos(size.Width-160, 40))
}
