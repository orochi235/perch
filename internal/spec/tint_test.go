package spec

import (
	"strings"
	"testing"
)

func tinted(tint string) string {
	return "app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s, tint: " + tint + "}\n" + tail
}

func TestTintAloneIsADot(t *testing.T) {
	got := parse(t, tinted("teal")).App.Tint
	want := Tint{Color: "teal", Alpha: 1, Style: TintDot, Size: "small", Corner: "bottom-right"}
	if got != want {
		t.Errorf("tint = %+v, want %+v", got, want)
	}
}

func TestTintTakesAStyleAndPlacesADot(t *testing.T) {
	for doc, want := range map[string]Tint{
		`{color: "#eeb48d", style: glyph}`:               {Color: "#eeb48d", Alpha: 1, Style: TintGlyph, Size: "small", Corner: "bottom-right"},
		`{color: pink, style: accent}`:                   {Color: "pink", Alpha: 1, Style: TintAccent, Size: "small", Corner: "bottom-right"},
		`{color: orange, size: large, corner: top-left}`: {Color: "orange", Alpha: 1, Style: TintDot, Size: "large", Corner: "top-left"},
		`{color: blue, style: chip}`:                     {Color: "blue", Alpha: chipAlpha, Style: TintChip, Size: "small", Corner: "bottom-right"},
		`{color: "#eeb48d80", style: chip}`:              {Color: "#eeb48d", Alpha: 128.0 / 255, Style: TintChip, Size: "small", Corner: "bottom-right"},
	} {
		if got := parse(t, tinted(doc)).App.Tint; got != want {
			t.Errorf("%s: tint = %+v, want %+v", doc, got, want)
		}
	}
}

func TestTintRefusesWhatItCannotDraw(t *testing.T) {
	for doc, msg := range map[string]string{
		`magenta`:                                "neither #rrggbb",
		`"#eeb"`:                                 "neither #rrggbb",
		`{style: dot}`:                           "color: required",
		`{color: red, style: halo}`:              "not one of",
		`{color: red, size: huge}`:               "not one of",
		`{color: red, corner: middle}`:           "not one of",
		`{color: red, style: chip, size: large}`: "place a dot",
		`{color: red, shade: dark}`:              "shade",
	} {
		_, err := Parse([]byte(tinted(doc)))
		if err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("%s: err = %v, want one saying %q", doc, err, msg)
		}
	}
}

func TestStatusRuleReplacesTheTint(t *testing.T) {
	s := parse(t, tinted("teal")+"status: [{when: 'true', tint: red}]\n")
	if got := s.Status[0].Tint.Color; got != "red" {
		t.Errorf("status[0].tint.color = %q, want red", got)
	}
}

// Artwork has no layers, so the two styles that recolor layers are refused on
// it, wherever the icon and the tint each came from.
func TestLayerStylesAreRefusedOnArtwork(t *testing.T) {
	for _, doc := range []string{
		"app: {name: onto, id: dev.onto.menubar, icon: {asset: o-0}, interval: 1s, tint: {color: red, style: glyph}}\n" + tail,
		tinted("{color: red, style: accent}") + "status: [{icon: {asset: o-0}}]\n",
		"app: {name: onto, id: dev.onto.menubar, icon: {asset: o-0}, interval: 1s}\n" + tail + "status: [{tint: {color: red, style: glyph}}]\n",
	} {
		if _, err := Parse([]byte(doc)); err == nil || !strings.Contains(err.Error(), "use dot or chip") {
			t.Errorf("err = %v, want a refusal\n%s", err, doc)
		}
	}
	for _, style := range []string{"dot", "chip"} {
		parse(t, "app: {name: onto, id: dev.onto.menubar, icon: {asset: o-0}, interval: 1s, tint: {color: red, style: "+style+"}}\n"+tail)
	}
}
