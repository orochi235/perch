package spec

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Badges are the pills drawn beside a menu item's label. They are written out
// as a list, or named by an expression whose elements carry the same fields, so
// the data can decide how many a row gets.
type Badges struct {
	List []Badge
	Expr string
}

func (b Badges) IsZero() bool { return len(b.List) == 0 && b.Expr == "" }

// Badge is one pill: Symbol and Text in FG on a rounded patch of BG. Each takes
// {{ }} holes; a badge whose text and symbol both resolve to nothing is not drawn.
type Badge struct {
	Text   string
	Symbol string
	FG     string
	BG     string
	At     BadgeAt
	// Plain draws the text as a run of the label, in FG, with no pill.
	Plain bool
}

// BadgeAt is which side of the label a badge sits on.
type BadgeAt string

const (
	BadgeEnd   BadgeAt = "end"
	BadgeStart BadgeAt = "start"
)

type rawBadge struct {
	Text   string `yaml:"text"`
	Symbol string `yaml:"symbol"`
	FG     string `yaml:"fg"`
	BG     string `yaml:"bg"`
	At     string `yaml:"at"`
	Plain  bool   `yaml:"plain"`
}

func parseBadges(n *yaml.Node, path string) (Badges, error) {
	switch n.Kind {
	case 0:
		return Badges{}, nil
	case yaml.ScalarNode:
		expr := strings.TrimSpace(n.Value)
		if expr == "" {
			return Badges{}, fmt.Errorf("%s: want a list of badges, or an expression naming one", path)
		}
		if strings.Contains(expr, "{{") {
			return Badges{}, fmt.Errorf("%s: a string here is an expression naming a list, so it takes no {{ }}; write it bare, as it.badges", path)
		}
		return Badges{Expr: expr}, nil
	case yaml.SequenceNode:
		out := make([]Badge, 0, len(n.Content))
		for i, c := range n.Content {
			b, err := parseBadge(c, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return Badges{}, err
			}
			out = append(out, b)
		}
		return Badges{List: out}, nil
	}
	return Badges{}, fmt.Errorf("%s: want a list of {text, symbol, fg, bg, at}, or an expression naming one", path)
}

func parseBadge(n *yaml.Node, path string) (Badge, error) {
	if n.Kind != yaml.MappingNode {
		return Badge{}, fmt.Errorf("%s: want {text: <text>, symbol: <SF Symbol>, fg: <color>, bg: <color>, at: start|end}", path)
	}
	var raw rawBadge
	if err := decodeStrict(n, &raw, path); err != nil {
		return Badge{}, err
	}
	if raw.Text == "" && raw.Symbol == "" {
		return Badge{}, fmt.Errorf("%s: needs text:, symbol:, or both", path)
	}
	for key, c := range map[string]string{"fg": raw.FG, "bg": raw.BG} {
		if err := checkColor(c); err != nil {
			return Badge{}, fmt.Errorf("%s.%s: %w", path, key, err)
		}
	}
	if raw.Plain && raw.BG != "" {
		return Badge{}, fmt.Errorf("%s.bg: a plain: badge is a run of text with no pill to fill", path)
	}
	at := BadgeAt(raw.At)
	switch at {
	case "":
		at = BadgeEnd
	case BadgeEnd, BadgeStart:
	default:
		return Badge{}, fmt.Errorf("%s.at: %q is neither start nor end", path, raw.At)
	}
	return Badge{Text: raw.Text, Symbol: raw.Symbol, FG: raw.FG, BG: raw.BG, At: at, Plain: raw.Plain}, nil
}
