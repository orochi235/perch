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
