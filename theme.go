package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// gameTheme wraps the default Fyne theme and substitutes the game's deep-navy
// water colour for the background. This matters on mobile, where the area
// outside the canvas's interactive region is painted with the theme background;
// without this it shows a default grey that clashes with the play surface.
type gameTheme struct {
	fyne.Theme
}

func newGameTheme() fyne.Theme {
	return &gameTheme{Theme: theme.DefaultTheme()}
}

// Color returns the game background for the background colour and otherwise
// defers to the wrapped default theme.
func (t *gameTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameBackground {
		return colWater
	}
	return t.Theme.Color(name, variant)
}
