package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Candlelit-gothic plum palette. One bold accent (rose) on a wine gradient; the
// rest stays quiet. Shared across the theme, panels, and title.
var (
	colBgTop       = color.NRGBA{R: 0x2A, G: 0x13, B: 0x20, A: 0xFF} // wine glow (gradient top)
	colBgBottom    = color.NRGBA{R: 0x13, G: 0x0A, B: 0x10, A: 0xFF} // near-black (gradient bottom)
	colPanel       = color.NRGBA{R: 0x3A, G: 0x1E, B: 0x2C, A: 0xC8} // translucent panel surface
	colPanelStroke = color.NRGBA{R: 0x5C, G: 0x37, B: 0x49, A: 0xFF} // hairline panel rim
	colAccent      = color.NRGBA{R: 0xF4, G: 0x8F, B: 0xB1, A: 0xFF} // rose accent
	colAccentDeep  = color.NRGBA{R: 0xD9, G: 0x6A, B: 0x92, A: 0xFF} // pressed rose
	colOnAccent    = color.NRGBA{R: 0x2A, G: 0x13, B: 0x20, A: 0xFF} // dark text on rose
	colFg          = color.NRGBA{R: 0xF1, G: 0xD8, B: 0xE1, A: 0xFF} // primary text
	colMuted       = color.NRGBA{R: 0xA8, G: 0x82, B: 0x90, A: 0xFF} // muted text
)

// pinkTheme is a dark, premium plum theme with rose-pink accents.
type pinkTheme struct {
	fyne.Theme
}

func newTheme() fyne.Theme {
	return &pinkTheme{Theme: theme.DefaultTheme()}
}

func (t *pinkTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameFocus, theme.ColorNameHyperlink:
		return colAccent
	case theme.ColorNamePressed:
		return colAccentDeep
	case theme.ColorNameForegroundOnPrimary:
		return colOnAccent
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0xF4, G: 0x8F, B: 0xB1, A: 0x44}
	case theme.ColorNameBackground:
		return colBgBottom
	case theme.ColorNameHeaderBackground:
		return colPanel
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x3A, G: 0x20, B: 0x2D, A: 0xFF}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0x2A, G: 0x18, B: 0x22, A: 0xFF}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x6E, G: 0x52, B: 0x5E, A: 0xFF}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x2D, G: 0x18, B: 0x22, A: 0xFF}
	case theme.ColorNameInputBorder:
		return colPanelStroke
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x2A, G: 0x16, B: 0x1F, A: 0xFF}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0x47, G: 0x29, B: 0x39, A: 0xFF}
	case theme.ColorNameForeground:
		return colFg
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNameSeparator:
		return colPanelStroke
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0xF4, G: 0x8F, B: 0xB1, A: 0x55}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xAA}
	}
	return t.Theme.Color(name, theme.VariantDark)
}

func (t *pinkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 15
	case theme.SizeNameSubHeadingText:
		return 14
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 8
	case theme.SizeNameScrollBar:
		return 10
	case theme.SizeNameScrollBarRadius:
		return 5
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return t.Theme.Size(name)
}
