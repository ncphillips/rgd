package ui

// Keep secondary text readable, not dim. These colors target the dark reading
// surface used by Rex; unchanged code must not inherit a low-contrast terminal
// foreground. Use weight and spacing, rather than faint text, for hierarchy.
const (
	textColor      = "#F1F5F9"
	secondaryColor = "#C0CCD8"
	accentColor    = "#B9DEFF"
	addedColor     = "#ADF0B4"
	removedColor   = "#FFBAC2"
	selectionColor = "#1B3245"
)
