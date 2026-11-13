package spec

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Icon is what the status item shows. An SF Symbol is a name macOS resolves and
// tints for the menu bar; an asset is a file the consuming repo drew itself,
// which macOS will not tint and which therefore carries its own color.
type Icon struct {
	Symbol string
	Asset  string
	// FG and BG color a menu item's symbol: the symbol drawn in FG on a rounded
	// patch of BG. Either may be empty; both empty is the usual template.
	FG string
	BG string
}

func (i Icon) IsZero() bool { return i.Symbol == "" && i.Asset == "" }

// Colored is a symbol drawn in its own colors rather than as a template.
func (i Icon) Colored() bool { return i.FG != "" || i.BG != "" }

// Templated is an icon whose name holds {{ }} holes, so it is known only once
// the app has polled. Nothing can check it at build: a name that resolves to no
// symbol or file draws no icon.
func (i Icon) Templated() bool { return strings.Contains(i.Symbol+i.Asset, "{{") }

// Icon names that vary are for menu items, where a missing one costs a picture
// beside a label. On the status item it would cost the whole status item.
type iconHoles bool

const (
	holesRefused iconHoles = false
	holesAllowed iconHoles = true
)

var hole = regexp.MustCompile(`\{\{.*?\}\}`)

type rawIcon struct {
	Asset  string `yaml:"asset"`
	Symbol string `yaml:"symbol"`
	FG     string `yaml:"fg"`
	BG     string `yaml:"bg"`
}

func parseIcon(n *yaml.Node, path string, holes iconHoles) (Icon, error) {
	icon, err := parseIconName(n, path)
	if err != nil || holes == holesAllowed {
		return icon, err
	}
	if icon.Colored() {
		return Icon{}, fmt.Errorf("%s: fg: and bg: are for a menu item's icon; the status item takes tint:", path)
	}
	if icon.Templated() {
		return Icon{}, fmt.Errorf("%s: {{ }} holes are for a menu item's icon; the status item's has to be known at build", path)
	}
	return icon, nil
}

// parseIconName takes either form: a bare name is a symbol, because that is
// what every spec written before assets existed says.
func parseIconName(n *yaml.Node, path string) (Icon, error) {
	switch {
	case n == nil || n.Kind == 0:
		return Icon{}, nil
	case n.Kind == yaml.ScalarNode:
		var s string
		if err := n.Decode(&s); err != nil {
			return Icon{}, fmt.Errorf("%s: %w", path, err)
		}
		return Icon{Symbol: s}, nil
	case n.Kind == yaml.MappingNode:
		var raw rawIcon
		if err := decodeStrict(n, &raw, path); err != nil {
			return Icon{}, err
		}
		if raw.Symbol != "" {
			if raw.Asset != "" {
				return Icon{}, fmt.Errorf("%s: takes symbol: or asset:, not both", path)
			}
			for key, c := range map[string]string{"fg": raw.FG, "bg": raw.BG} {
				if err := checkColor(c); err != nil {
					return Icon{}, fmt.Errorf("%s.%s: %w", path, key, err)
				}
			}
			return Icon{Symbol: raw.Symbol, FG: raw.FG, BG: raw.BG}, nil
		}
		if raw.FG != "" || raw.BG != "" {
			return Icon{}, fmt.Errorf("%s: fg: and bg: color a symbol; an asset carries its own colors", path)
		}
		if raw.Asset == "" {
			return Icon{}, fmt.Errorf("%s: want an SF Symbol name, {symbol: <name>, fg: <color>, bg: <color>}, or {asset: <file in menubar/Icons, without its extension>}", path)
		}
		// Only the literal text is checkable here; the runtime refuses a
		// resolved name that is not a bare file name.
		if err := checkAssetName(hole.ReplaceAllString(raw.Asset, "x")); err != nil {
			return Icon{}, fmt.Errorf("%s.asset: %w", path, err)
		}
		return Icon{Asset: raw.Asset}, nil
	}
	return Icon{}, fmt.Errorf("%s: want an SF Symbol name, {symbol: <name>, fg:, bg:}, or {asset: <name>}", path)
}

// checkColor takes a tint's colors except #rrggbbaa: a patch behind a symbol has
// no bar to let through. A color with {{ }} holes is known only once the app
// polls, and one that resolves to nothing is drawn as no color.
func checkColor(c string) error {
	if c == "" || hole.MatchString(c) {
		return nil
	}
	if (hexColor.MatchString(c) && len(c) == 7) || slices.Contains(SystemColors, c) {
		return nil
	}
	return fmt.Errorf("%q is neither #rrggbb nor one of %v", c, SystemColors)
}

// checkAssetName keeps the name a single file in one directory. It is
// interpolated into a path the bundler reads and the app loads, so a name that
// climbs out of menubar/Icons is refused here rather than resolved later.
func checkAssetName(name string) error {
	if name == "." || name == ".." || name == "" {
		return fmt.Errorf("%q is not a file name", name)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_':
		default:
			return fmt.Errorf("%q: only letters, digits, - and _ (it names one file in menubar/Icons)", name)
		}
	}
	return nil
}
