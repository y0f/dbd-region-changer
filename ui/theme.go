package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

var (
	colBg          = gray(0x0c)
	colPanel       = gray(0x16)
	colPanelStroke = gray(0x30)
	colAccent      = color.NRGBA{R: 0x58, G: 0xa6, B: 0xff, A: 0xff}
	colFg          = gray(0xed)
	colMuted       = gray(0xa0)
	colGrey        = colMuted
)

func gray(v uint8) color.NRGBA { return color.NRGBA{R: v, G: v, B: v, A: 0xff} }

type darkTheme struct{ fyne.Theme }

func newTheme() fyne.Theme { return &darkTheme{Theme: theme.DefaultTheme()} }

func (t *darkTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary, theme.ColorNameHyperlink, theme.ColorNameSuccess:
		return colAccent
	case theme.ColorNameFocus, theme.ColorNameSelection:
		return color.NRGBA{R: 0x58, G: 0xa6, B: 0xff, A: 0x40}
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnSuccess,
		theme.ColorNameForegroundOnWarning, theme.ColorNameForegroundOnError:
		return colBg
	case theme.ColorNameBackground:
		return colBg
	case theme.ColorNameHeaderBackground, theme.ColorNameInputBackground,
		theme.ColorNameDisabledButton, theme.ColorNameScrollBarBackground:
		return colPanel
	case theme.ColorNameButton, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return gray(0x22)
	case theme.ColorNameDisabled:
		return gray(0x70)
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return colPanelStroke
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x12}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x24}
	case theme.ColorNameForeground, theme.ColorNameError, theme.ColorNameWarning:
		return colFg
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNameScrollBar:
		return gray(0x60)
	case theme.ColorNameShadow:
		return color.NRGBA{A: 0x99}
	}
	return t.Theme.Color(name, theme.VariantDark)
}

func (t *darkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 12
	case theme.SizeNameText:
		return 14
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 5
	case theme.SizeNameScrollBar:
		return 6
	case theme.SizeNameScrollBarRadius:
		return 3
	}
	return t.Theme.Size(name)
}
