package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// StatusRule decides the status item's appearance. The first rule whose when:
// holds wins; a rule with no when: always matches.
type StatusRule struct {
	When  string
	Icon  string
	Dim   bool
	Badge string
}

type rawStatusRule struct {
	When  string `yaml:"when"`
	Icon  string `yaml:"icon"`
	Dim   bool   `yaml:"dim"`
	Badge string `yaml:"badge"`
}

func parseStatus(n *yaml.Node) ([]StatusRule, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("status: want a list of rules")
	}
	out := make([]StatusRule, 0, len(n.Content))
	for i, c := range n.Content {
		path := fmt.Sprintf("status[%d]", i)
		var raw rawStatusRule
		if err := decodeStrict(c, &raw, path); err != nil {
			return nil, err
		}
		out = append(out, StatusRule{When: raw.When, Icon: raw.Icon, Dim: raw.Dim, Badge: raw.Badge})
	}
	return out, nil
}
