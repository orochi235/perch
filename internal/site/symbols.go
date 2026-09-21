package site

import (
	"embed"
	"regexp"
	"slices"
	"strings"

	"github.com/orochi235/perch/v2/internal/preview"
)

// symbols holds a Lucide look-alike for every SF Symbol a page's previews draw,
// named <sf symbol>.svg. SF Symbols are licensed for apps on Apple's platforms,
// so a web page draws a stand-in and names the real one.
//
//go:embed symbols/*.svg
var symbols embed.FS

var svgComment = regexp.MustCompile(`(?s)<!--.*?-->`)

func symbolSVG(name string) (string, bool) {
	b, err := symbols.ReadFile("symbols/" + name + ".svg")
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(svgComment.ReplaceAllString(string(b), "")), true
}

// missingSymbols is every symbol the frames draw that has no look-alike. It
// fails the build rather than drawing a blank, which is what an unknown name
// would draw in the app too.
func missingSymbols(frames []preview.Frame) []string {
	var out []string
	note := func(i *preview.Icon) {
		if i == nil || i.Symbol == "" {
			return
		}
		if _, ok := symbolSVG(i.Symbol); !ok && !slices.Contains(out, i.Symbol) {
			out = append(out, i.Symbol)
		}
	}
	var walk func([]preview.Node)
	walk = func(nodes []preview.Node) {
		for _, n := range nodes {
			note(n.Icon)
			walk(n.Items)
		}
	}
	for _, f := range frames {
		note(&f.Face.Icon)
		walk(f.Menu)
	}
	return out
}
