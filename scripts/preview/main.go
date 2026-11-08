// Command preview captures the running app after startup and a round of pings.
// Run in an isolated user profile: startup restores any saved firewall lock.
//
//	LIBGL_ALWAYS_SOFTWARE=1 xvfb-run -a go run ./scripts/preview docs/img/preview.png
package main

import (
	"image/png"
	"log"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/software"

	"github.com/y0f/dbd-region-changer/internal/applog"
	"github.com/y0f/dbd-region-changer/ui"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: preview <output.png>")
	}
	// The preview must never pick up a saved region or game path.
	profile, err := os.MkdirTemp("", "dbd-preview-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(profile)
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "APPDATA", "LOCALAPPDATA"} {
		if err := os.Setenv(key, profile); err != nil {
			log.Fatal(err)
		}
	}
	logger, err := applog.New(false)
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()
	a := ui.New(logger)
	go func() {
		time.Sleep(50 * time.Second)
		fyne.Do(func() {
			fa := fyne.CurrentApp()
			f, err := os.Create(os.Args[1])
			if err != nil {
				log.Fatal(err)
			}
			live := fa.Driver().AllWindows()[0].Canvas()
			capture := software.NewCanvas()
			capture.SetContent(live.Content())
			capture.Resize(live.Size())
			err = png.Encode(f, capture.Capture())
			closeErr := f.Close()
			if err != nil {
				log.Fatal(err)
			}
			if closeErr != nil {
				log.Fatal(closeErr)
			}
			fa.Quit()
		})
	}()
	a.Run()
}
