package ui

import (
	"fmt"

	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/y0f/dbd-region-changer/internal/config"
	"github.com/y0f/dbd-region-changer/internal/updater"
)

const (
	permTitle   = "Permission Error!"
	permMessage = "You must ensure admin access and or add this app to the antivirus exception list"
)

func (a *App) showPermissionError() {
	dialog.NewCustom(permTitle, "OK", widget.NewLabel(permMessage), a.win).Show()
}

func versionBody(msg string, r updater.Result) string {
	return fmt.Sprintf("%s\n\nCurrent: %s\nLatest: %s", msg, r.Local, r.Remote)
}

func (a *App) showUpdateResult(r updater.Result) {
	switch r.Code {
	case updater.StatusOutdated:
		a.updateAvailableDialog("Update Available", "A New Version is Available", r)
	case updater.StatusFuture:
		a.updateAvailableDialog("Future Version", "You are using an Unreleased Version", r)
	case updater.StatusError:
		dialog.NewInformation("Cannot check for updates",
			versionBody("An error occurred while checking for updates", r), a.win).Show()
	default:
		dialog.NewInformation("No Update Found",
			versionBody("This is the latest version.", r), a.win).Show()
	}
}

func (a *App) updateAvailableDialog(title, msg string, r updater.Result) {
	dialog.NewCustomConfirm(title, "Check Github", "Cancel",
		widget.NewLabel(versionBody(msg, r)),
		func(ok bool) {
			if ok {
				a.openURL(config.UpdateURL)
			}
		}, a.win).Show()
}
