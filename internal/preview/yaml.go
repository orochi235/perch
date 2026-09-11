package preview

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// A state fence is YAML, but every value in it is read the way the watch that
// binds it is written: out: may be given as a mapping or a list, which is what
// a JSON-printing command actually returns, and is carried as the text that
// command would have printed.

type entry struct {
	key   string
	value value
}

type node struct {
	entries []entry
}

type value struct {
	n *yaml.Node
}

func unmarshal(src []byte, into *node) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return err
	}
	if len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("want a mapping of watch names to what one poll returned")
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		into.entries = append(into.entries, entry{
			key:   root.Content[i].Value,
			value: value{n: root.Content[i+1]},
		})
	}
	return nil
}

func (v value) fields() ([]entry, error) {
	if v.n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("want a mapping")
	}
	var out []entry
	for i := 0; i+1 < len(v.n.Content); i += 2 {
		out = append(out, entry{key: v.n.Content[i].Value, value: value{n: v.n.Content[i+1]}})
	}
	return out, nil
}

func (v value) bool() (bool, error) {
	if v.n.Kind != yaml.ScalarNode {
		return false, fmt.Errorf("want true or false")
	}
	switch v.n.Value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("want true or false")
}

func (v value) number() (int, error) {
	if v.n.Kind != yaml.ScalarNode {
		return 0, fmt.Errorf("want a number")
	}
	n, err := numberFrom(v.n.Value)
	if err != nil {
		return 0, fmt.Errorf("want a number, got %q", v.n.Value)
	}
	return n, nil
}

// text is what the command printed. A scalar is taken literally; a mapping or a
// list is encoded as the JSON such a command would have written.
func (v value) text() (string, error) {
	if v.n.Kind == yaml.ScalarNode {
		return v.n.Value, nil
	}
	var any any
	if err := v.n.Decode(&any); err != nil {
		return "", err
	}
	b, err := json.Marshal(any)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
