package spec

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Tint colors the status item so that one app can be told from another in a
// crowded menu bar. A tinted icon is no longer a template, so it is drawn the
// same whether the bar is light or dark; the styles that keep most of the
// glyph in the bar's own text color read on both.
type Tint struct {
	// Color is a SystemColors name or #rrggbb, as written.
	Color string
	// Alpha is the color's opacity: from #rrggbbaa, otherwise the style's own.
	Alpha  float64
	Style  TintStyle
	Size   TintSize
	Corner TintCorner
}

func (t Tint) IsZero() bool { return t.Color == "" }

type TintStyle string

const (
	// TintGlyph colors the whole symbol, its secondary layers fainter.
	TintGlyph TintStyle = "glyph"
	// TintAccent colors a symbol's secondary layer and leaves the rest in
	// the bar's text color.
	TintAccent TintStyle = "accent"
	// TintDot draws a dot on one corner of the icon.
	TintDot TintStyle = "dot"
	// TintChip draws the icon on a faint rounded patch of the color.
	TintChip TintStyle = "chip"
)

var tintStyles = []TintStyle{TintDot, TintGlyph, TintAccent, TintChip}

// SymbolOnly is a style that recolors a symbol's layers, which artwork does
// not have.
func (s TintStyle) SymbolOnly() bool { return s == TintGlyph || s == TintAccent }

type TintSize string

var tintSizes = []TintSize{"small", "medium", "large"}

type TintCorner string

var tintCorners = []TintCorner{"bottom-right", "bottom-left", "top-right", "top-left"}

// SystemColors are the names a tint takes besides a hex code. Each is one of
// macOS's system colors, which shift between the light and dark menu bar.
var SystemColors = []string{
	"red", "orange", "yellow", "green", "mint", "teal", "cyan",
	"blue", "indigo", "purple", "pink", "brown", "gray",
}

// chipAlpha is how strongly a chip shows a color given without its own alpha:
// at full strength the patch would bury the glyph on it.
const chipAlpha = 0.35

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$`)

type rawTint struct {
	Color  string `yaml:"color"`
	Style  string `yaml:"style"`
	Size   string `yaml:"size"`
	Corner string `yaml:"corner"`
}

// parseTint takes a color alone, which draws a dot, or a mapping that picks
// the style.
func parseTint(n *yaml.Node, path string) (Tint, error) {
	var raw rawTint
	switch {
	case n == nil || n.Kind == 0:
		return Tint{}, nil
	case n.Kind == yaml.ScalarNode:
		if err := n.Decode(&raw.Color); err != nil {
			return Tint{}, fmt.Errorf("%s: %w", path, err)
		}
	case n.Kind == yaml.MappingNode:
		if err := decodeStrict(n, &raw, path); err != nil {
			return Tint{}, err
		}
	default:
		return Tint{}, fmt.Errorf("%s: want a color, or {color: <color>, style: <style>}", path)
	}

	t := Tint{Color: raw.Color, Alpha: 1, Style: TintDot, Size: "small", Corner: "bottom-right"}
	if raw.Style != "" {
		t.Style = TintStyle(raw.Style)
	}
	if !slices.Contains(tintStyles, t.Style) {
		return Tint{}, fmt.Errorf("%s.style: %q is not one of %v", path, raw.Style, tintStyles)
	}
	if t.Style == TintChip {
		t.Alpha = chipAlpha
	}
	switch {
	case raw.Color == "":
		return Tint{}, fmt.Errorf("%s.color: required (#rrggbb, or one of %v)", path, SystemColors)
	case hexColor.MatchString(raw.Color):
		if len(raw.Color) == 9 {
			a, _ := strconv.ParseUint(raw.Color[7:], 16, 8)
			t.Color, t.Alpha = raw.Color[:7], float64(a)/255
		}
	case !slices.Contains(SystemColors, raw.Color):
		return Tint{}, fmt.Errorf("%s.color: %q is neither #rrggbb, #rrggbbaa, nor one of %v", path, raw.Color, SystemColors)
	}
	if raw.Size != "" || raw.Corner != "" {
		if t.Style != TintDot {
			return Tint{}, fmt.Errorf("%s: size: and corner: place a dot; style %s has none", path, t.Style)
		}
	}
	if raw.Size != "" {
		t.Size = TintSize(raw.Size)
		if !slices.Contains(tintSizes, t.Size) {
			return Tint{}, fmt.Errorf("%s.size: %q is not one of %v", path, raw.Size, tintSizes)
		}
	}
	if raw.Corner != "" {
		t.Corner = TintCorner(raw.Corner)
		if !slices.Contains(tintCorners, t.Corner) {
			return Tint{}, fmt.Errorf("%s.corner: %q is not one of %v", path, raw.Corner, tintCorners)
		}
	}
	return t, nil
}

// checkTint refuses a style that has nothing to recolor: artwork has no layers.
func checkTint(icon Icon, t Tint, path string) error {
	if icon.Asset != "" && t.Style.SymbolOnly() {
		return fmt.Errorf("%s: tint style %s recolors an SF Symbol's layers, and {asset: %s} is artwork; use dot or chip", path, t.Style, icon.Asset)
	}
	return nil
}
