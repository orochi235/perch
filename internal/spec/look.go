package spec

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FontWeights are the names weight: takes.
var FontWeights = []string{"ultralight", "thin", "light", "regular", "medium", "semibold", "bold", "heavy", "black"}

// Look is how an item's label is drawn beyond its text and icon. Size and
// Indent are numbers or CEL expressions giving one; Enabled and Header are
// conditions; Weight and Subtitle take {{ }} holes.
type Look struct {
	Size     string
	Weight   string
	Indent   string
	Enabled  string
	Subtitle string
	Key      Hotkey
	Header   string
	// NoGutter drops the checkmark column of the submenu the item opens.
	NoGutter bool
}

func parseLook(f itemFields, path string) (Look, error) {
	size, err := parseNumber(&f.Size, path+".size", 6, 32, "a font size in points")
	if err != nil {
		return Look{}, err
	}
	indent, err := parseNumber(&f.Indent, path+".indent", 0, 15, "an indent level")
	if err != nil {
		return Look{}, err
	}
	if f.Weight != "" && !hole.MatchString(f.Weight) && !slices.Contains(FontWeights, f.Weight) {
		return Look{}, fmt.Errorf("%s.weight: %q is not one of %v", path, f.Weight, FontWeights)
	}
	key, err := parseHotkey(f.Key, path+".key")
	if err != nil {
		return Look{}, err
	}
	if f.Gutter != nil && f.Menu.Kind == 0 {
		return Look{}, fmt.Errorf("%s.gutter: sets the gutter of the submenu the item opens, and it has no menu:", path)
	}
	return Look{Size: size, Weight: f.Weight, Indent: indent, Enabled: f.Enabled, Subtitle: f.Subtitle, Key: key, Header: f.Header,
		NoGutter: f.Gutter != nil && !*f.Gutter}, nil
}

// checkLook refuses a look the item cannot wear: a shortcut on what has
// nothing to do, or on every row of an each:, and a header that acts.
func checkLook(f itemFields, l Look, act Action, sub []Item, path string) error {
	if l.Key.IsSet() {
		switch {
		case f.Each != "":
			return fmt.Errorf("%s.key: an each: item would give every row the same shortcut", path)
		case len(sub) > 0:
			return fmt.Errorf("%s.key: a submenu has nothing for a shortcut to do", path)
		case act.Kind == ActionNone && f.Toggle == "":
			return fmt.Errorf("%s.key: the item has no action for a shortcut to run", path)
		}
	}
	if l.Header != "" {
		switch {
		case act.Kind != ActionNone || f.Toggle != "":
			return fmt.Errorf("%s.header: a section header is a label; it takes no action or toggle:", path)
		case len(sub) > 0:
			return fmt.Errorf("%s.header: a section header is a label; it takes no submenu", path)
		case l.Key.IsSet():
			return fmt.Errorf("%s.header: a section header takes no key:", path)
		}
	}
	return nil
}

// parseNumber keeps a number as written: a literal is checked here, and an
// expression is lowered as a number where the menu is built.
func parseNumber(n *yaml.Node, path string, lo, hi float64, what string) (string, error) {
	if n.Kind == 0 {
		return "", nil
	}
	if n.Kind != yaml.ScalarNode || strings.TrimSpace(n.Value) == "" {
		return "", fmt.Errorf("%s: want %s, or a CEL expression giving one", path, what)
	}
	if v, err := strconv.ParseFloat(n.Value, 64); err == nil && (v < lo || v > hi) {
		return "", fmt.Errorf("%s: %v is outside %v to %v", path, v, lo, hi)
	}
	return n.Value, nil
}
