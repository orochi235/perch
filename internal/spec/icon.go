package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Icon is what the status item shows. An SF Symbol is a name macOS resolves and
// tints for the menu bar; an asset is a file the consuming repo drew itself,
// which macOS will not tint and which therefore carries its own color.
type Icon struct {
	Symbol string
	Asset  string
}

func (i Icon) IsZero() bool { return i.Symbol == "" && i.Asset == "" }

type rawIconAsset struct {
	Asset string `yaml:"asset"`
}

// parseIcon takes either form: a bare name is a symbol, because that is what
// every spec written before assets existed says.
func parseIcon(n *yaml.Node, path string) (Icon, error) {
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
		var raw rawIconAsset
		if err := decodeStrict(n, &raw, path); err != nil {
			return Icon{}, err
		}
		if raw.Asset == "" {
			return Icon{}, fmt.Errorf("%s: want an SF Symbol name or {asset: <file in menubar/Icons, without its extension>}", path)
		}
		if err := checkAssetName(raw.Asset); err != nil {
			return Icon{}, fmt.Errorf("%s.asset: %w", path, err)
		}
		return Icon{Asset: raw.Asset}, nil
	}
	return Icon{}, fmt.Errorf("%s: want an SF Symbol name or {asset: <name>}", path)
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
