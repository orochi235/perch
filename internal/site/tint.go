package site

import (
	"fmt"
	"html"
	"strconv"

	"github.com/orochi235/perch/v2/internal/preview"
)

// systemCSS is each system color as macOS draws it on a light menu bar. The
// app's own are dynamic; a page shows the one appearance.
var systemCSS = map[string]string{
	"red": "#ff3b30", "orange": "#ff9500", "yellow": "#ffcc00", "green": "#34c759",
	"mint": "#00c7be", "teal": "#30b0c7", "cyan": "#32ade6", "blue": "#007aff",
	"indigo": "#5856d6", "purple": "#af52de", "pink": "#ff2d55", "brown": "#a2845e",
	"gray": "#8e8e93",
}

// tintAttrs are the classes and the color a tinted status item carries. The
// color is data, so it rides in a custom property rather than a class per hue.
func tintAttrs(t *preview.Tint) (classes, style string) {
	if t == nil {
		return "", ""
	}
	hex := t.Color
	if css, ok := systemCSS[hex]; ok {
		hex = css
	}
	v, _ := strconv.ParseUint(hex[1:], 16, 32)
	rgba := fmt.Sprintf("rgba(%d, %d, %d, %s)", v>>16, v>>8&0xff, v&0xff, strconv.FormatFloat(t.Alpha, 'f', 3, 64))
	classes = " is-tinted tint-" + t.Style
	if t.Style == "dot" {
		classes += " dot-" + t.Size + " at-" + t.Corner
	}
	return classes, ` style="--tint: ` + html.EscapeString(rgba) + `"`
}

// cssColor is a menu icon's or badge's fg or bg as CSS, or "" where it names nothing.
func cssColor(c string) string {
	if css, ok := systemCSS[c]; ok {
		return css
	}
	if len(c) == 7 && c[0] == '#' {
		return c
	}
	return ""
}

// inkColors are the CSS colors a symbol or badge is drawn in: fg as given, or
// with only bg, white or black, whichever reads on it, as the app draws them.
func inkColors(fg, bg string) (ink, patch string) {
	ink, patch = cssColor(fg), cssColor(bg)
	if ink == "" && patch != "" {
		v, _ := strconv.ParseUint(patch[1:], 16, 32)
		r, g, b := float64(v>>16)/255, float64(v>>8&0xff)/255, float64(v&0xff)/255
		ink = "#000"
		if 0.299*r+0.587*g+0.114*b < 0.6 {
			ink = "#fff"
		}
	}
	return ink, patch
}
