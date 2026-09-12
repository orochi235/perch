package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// State is one of the conditions a widget can be in. The first whose Cond
// holds wins, and the last has no Cond — so exactly one state holds at every
// poll, which is what lets an expression name one and mean it.
type State struct {
	Name string
	Cond string
}

func parseStates(n *yaml.Node) ([]State, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("state: want a list of states, each one name and the condition that reaches it")
	}
	out := make([]State, 0, len(n.Content))
	for i, c := range n.Content {
		path := fmt.Sprintf("state[%d]", i)
		if c.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s: want a name and its condition, e.g. - stopped: \"!agent.ok\"", path)
		}
		if len(c.Content) != 2 {
			return nil, fmt.Errorf("%s: names %d states; each entry takes exactly one", path, len(c.Content)/2)
		}
		st := State{Name: c.Content[0].Value}
		switch cond := c.Content[1]; {
		case cond.Kind == yaml.ScalarNode && cond.Tag == "!!null":
		case cond.Kind == yaml.ScalarNode:
			st.Cond = cond.Value
		default:
			return nil, fmt.Errorf("state.%s: want a condition as a string, or nothing at all to make it the fallback", st.Name)
		}
		out = append(out, st)
	}
	return out, nil
}

// validateStates checks what can be checked without reading an expression. A
// condition naming a state declared after it needs no check of its own: the
// backend declares states in order, so the name is simply not bound yet.
func (s *Spec) validateStates() error {
	if len(s.States) == 0 {
		return nil
	}
	watches := map[string]bool{}
	for _, w := range s.Watches {
		watches[w.Name] = true
	}
	seen := map[string]bool{}
	for i, st := range s.States {
		path := fmt.Sprintf("state[%d]", i)
		if err := checkName("state", path, st.Name); err != nil {
			return err
		}
		if st.Name == "it" {
			return fmt.Errorf("%s: each: binds its element to `it`, so a state cannot take that name", path)
		}
		if seen[st.Name] {
			return fmt.Errorf("%s: %q declared twice", path, st.Name)
		}
		seen[st.Name] = true
		if watches[st.Name] {
			return fmt.Errorf("%s: %q is already a watch; an expression names states and watches the same way, so it could not tell the two apart", path, st.Name)
		}
		if last := i == len(s.States)-1; last {
			if st.Cond != "" {
				return fmt.Errorf("%s: %q is last, so it is the fallback and takes no condition; without one state that always holds, a poll can match none of them", path, st.Name)
			}
		} else if st.Cond == "" {
			return fmt.Errorf("%s: %q has no condition, so it always holds and must be last; %d state(s) after it can never be reached", path, st.Name, len(s.States)-1-i)
		}
	}
	return nil
}
