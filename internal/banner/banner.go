// Package banner renders the startup banner (console/debug mode).
package banner

import (
	"fmt"
	"io"
)

const art = `
            ██╗   ██╗ ██████╗  ███████╗
            ╚██╗ ██╔╝ ██╔═══██╗██╔════╝
             ╚████╔╝  ██║   ██║█████╗
              ╚██╔╝   ██║   ██║██╔══╝
               ██║    ╚██████╔╝██║
               ╚═╝     ╚═════╝ ╚═╝
          dbd region changer - by y0f
`

const (
	colorReset = "\033[0m"
	colorCyan  = "\033[96m"
)

func String() string { return art }

// Print writes the banner to w in cyan.
func Print(w io.Writer) {
	fmt.Fprint(w, colorCyan+art+colorReset+"\n")
}
