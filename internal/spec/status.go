package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// StatusRule decides the status item's appearance. The first rule whose when:
// holds wins; a rule with no when: always matches.
type StatusRule struct {
	When  string
	Icon  Icon
	Dim   bool
	Badge string
	// Scope is the use this rule came from, or "" for the file's own.
	Scope string

	outlet   string
	isOutlet bool
	path     string // where the author wrote it, for errors after placement
}

type rawStatusRule struct {
	When  string    `yaml:"when"`
	Icon  yaml.Node `yaml:"icon"`
	Dim   bool      `yaml:"dim"`
	Badge string    `yaml:"badge"`
}

func parseStatus(n *yaml.Node, path string) ([]StatusRule, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: want a list of rules", path)
	}
	out := make([]StatusRule, 0, len(n.Content))
	for i, c := range n.Content {
		at := fmt.Sprintf("%s[%d]", path, i)
		if name, ok, err := outletMark(c, at); err != nil {
			return nil, err
		} else if ok {
			out = append(out, StatusRule{outlet: name, isOutlet: true, path: at})
			continue
		}
		var raw rawStatusRule
		if err := decodeStrict(c, &raw, at); err != nil {
			return nil, err
		}
		icon, err := parseIcon(&raw.Icon, at+".icon")
		if err != nil {
			return nil, err
		}
		out = append(out, StatusRule{When: raw.When, Icon: icon, Dim: raw.Dim, Badge: raw.Badge, path: at})
	}
	return out, nil
}
