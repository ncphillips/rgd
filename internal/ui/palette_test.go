package ui

import (
	"math"
	"strconv"
	"testing"
)

func luminance(hex string) float64 {
	components := [3]float64{}
	for i := range components {
		value, _ := strconv.ParseUint(hex[1+i*2:3+i*2], 16, 8)
		x := float64(value) / 255
		if x <= 0.04045 {
			components[i] = x / 12.92
		} else {
			components[i] = math.Pow((x+0.055)/1.055, 2.4)
		}
	}
	return components[0]*0.2126 + components[1]*0.7152 + components[2]*0.0722
}

func TestDarkPaletteTextContrast(t *testing.T) {
	for _, background := range []string{"#011627", selectionColor} {
		for _, foreground := range []string{textColor, secondaryColor, accentColor, addedColor, removedColor} {
			ratio := (luminance(foreground) + 0.05) / (luminance(background) + 0.05)
			if ratio < 7 {
				t.Errorf("%s against %s has insufficient text contrast: %.2f", foreground, background, ratio)
			}
		}
	}
}
