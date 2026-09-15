package spec

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// fragment is what one template puts at one outlet. "" is the default outlet.
type fragment[T any] struct {
	outlet  string
	entries []T
}

// outletKey maps the name default to the default outlet.
func outletKey(name string) string {
	if name == "default" {
		return ""
	}
	return name
}

func outletLabel(name string) string {
	if name == "" {
		return "the default outlet"
	}
	return fmt.Sprintf("the outlet %q", name)
}

// outletMark reads `- outlet`, the default outlet, or `- outlet: <name>`.
func outletMark(n *yaml.Node, path string) (string, bool, error) {
	if n.Kind == yaml.ScalarNode && n.Value == "outlet" {
		return "", true, nil
	}
	if n.Kind != yaml.MappingNode {
		return "", false, nil
	}
	at := -1
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "outlet" {
			at = i
			break
		}
	}
	if at < 0 {
		return "", false, nil
	}
	if len(n.Content) != 2 {
		return "", false, fmt.Errorf("%s: an outlet takes nothing but its name", path)
	}
	v := n.Content[1]
	if isNull(v) {
		return "", true, nil
	}
	if v.Kind != yaml.ScalarNode {
		return "", false, fmt.Errorf("%s: an outlet's name is a string", path)
	}
	if err := checkOutletName(path, v.Value); err != nil {
		return "", false, err
	}
	return outletKey(v.Value), true, nil
}

func parseMenuFragments(n *yaml.Node, where string) ([]fragment[Item], error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: menu: a template's menu maps outlet names to lists of items, e.g. default: [...]", where)
	}
	var out []fragment[Item]
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		path := where + ": menu." + key
		if err := checkOutletName(path, key); err != nil {
			return nil, err
		}
		items, err := parseMenu(n.Content[i+1], path)
		if err != nil {
			return nil, err
		}
		if err := refuseMenuMarks(items); err != nil {
			return nil, err
		}
		out = append(out, fragment[Item]{outlet: outletKey(key), entries: items})
	}
	return out, nil
}

func refuseMenuMarks(items []Item) error {
	for _, it := range items {
		if it.isOutlet {
			return fmt.Errorf("%s: a template fills outlets; it cannot declare one", it.path)
		}
		if err := refuseMenuMarks(it.Menu); err != nil {
			return err
		}
	}
	return nil
}

func parseStatusFragments(n *yaml.Node, where string) ([]fragment[StatusRule], error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: status: a template's status maps outlet names to lists of rules, e.g. default: [...]", where)
	}
	var out []fragment[StatusRule]
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		path := where + ": status." + key
		if err := checkOutletName(path, key); err != nil {
			return nil, err
		}
		rules, err := parseStatus(n.Content[i+1], path)
		if err != nil {
			return nil, err
		}
		for _, r := range rules {
			switch {
			case r.isOutlet:
				return nil, fmt.Errorf("%s: a template fills outlets; it cannot declare one", r.path)
			case r.When == "":
				return nil, fmt.Errorf("%s: a rule with no when: would land ahead of the file's own rules and shadow every one of them", r.path)
			}
		}
		out = append(out, fragment[StatusRule]{outlet: outletKey(key), entries: rules})
	}
	return out, nil
}

// mark is an outlet a file declares, and where.
type mark struct{ name, path string }

func declare(marks []mark, m mark) ([]mark, error) {
	if slices.ContainsFunc(marks, func(p mark) bool { return p.name == m.name }) {
		return nil, fmt.Errorf("%s: %s is declared twice", m.path, outletLabel(m.name))
	}
	return append(marks, m), nil
}

func declaresDefault(marks []mark) bool {
	return slices.ContainsFunc(marks, func(m mark) bool { return m.name == "" })
}

// group scopes every use's fragments and gathers them by the outlet they land
// at: their own if the file declares it, the default otherwise.
func group[T any](marks []mark, uses []Use, fragments func(Use) []fragment[T], scope func([]T, string) []T) (map[string][]T, error) {
	declared := map[string]bool{}
	for _, m := range marks {
		declared[m.name] = true
	}
	groups := map[string][]T{}
	var offered []string
	for _, u := range uses {
		for _, f := range fragments(u) {
			if name := cmp.Or(f.outlet, "default"); !slices.Contains(offered, name) {
				offered = append(offered, name)
			}
			at := f.outlet
			if !declared[at] {
				at = ""
			}
			groups[at] = append(groups[at], scope(f.entries, u.Name)...)
		}
	}
	for _, m := range marks {
		if m.name != "" && len(groups[m.name]) == 0 {
			fills := "the uses fill no outlets"
			switch {
			case len(uses) == 0:
				fills = "this file has no use: block"
			case len(offered) > 0:
				fills = "the uses fill " + strings.Join(offered, ", ")
			}
			return nil, fmt.Errorf("%s: no use fills %s; %s", m.path, outletLabel(m.name), fills)
		}
	}
	return groups, nil
}

// placeMenu puts every use's menu fragments at the outlets the file declares.
// With no default declared, the default is the end of the menu.
func placeMenu(items []Item, uses []Use) ([]Item, error) {
	marks, err := menuMarks(items, nil)
	if err != nil {
		return nil, err
	}
	groups, err := group(marks, uses, func(u Use) []fragment[Item] { return u.menu }, scopeItems)
	if err != nil {
		return nil, err
	}
	placed := expandMenuOutlets(items, groups)
	if !declaresDefault(marks) {
		placed = append(placed, groups[""]...)
	}
	return placed, nil
}

func menuMarks(items []Item, marks []mark) ([]mark, error) {
	var err error
	for _, it := range items {
		if it.isOutlet {
			marks, err = declare(marks, mark{it.outlet, it.path})
		} else {
			marks, err = menuMarks(it.Menu, marks)
		}
		if err != nil {
			return nil, err
		}
	}
	return marks, nil
}

func expandMenuOutlets(items []Item, groups map[string][]Item) []Item {
	if items == nil {
		return nil
	}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.isOutlet {
			out = append(out, groups[it.outlet]...)
			continue
		}
		if len(it.Menu) > 0 {
			it.Menu = expandMenuOutlets(it.Menu, groups)
			// A submenu of outlets nothing filled would open onto nothing.
			if len(it.Menu) == 0 {
				continue
			}
		}
		out = append(out, it)
	}
	return out
}

func scopeItems(items []Item, scope string) []Item {
	if items == nil {
		return nil
	}
	out := make([]Item, len(items))
	for i, it := range items {
		it.Scope = scope
		it.Menu = scopeItems(it.Menu, scope)
		out[i] = it
	}
	return out
}

func scopeRules(rules []StatusRule, scope string) []StatusRule {
	out := slices.Clone(rules)
	for i := range out {
		out[i].Scope = scope
	}
	return out
}

// placeStatus is placeMenu for status rules. With no default declared, the
// default is the start of the list, where a use's rules can still match.
func placeStatus(rules []StatusRule, uses []Use) ([]StatusRule, error) {
	var marks []mark
	catchAll := false
	for _, r := range rules {
		var err error
		switch {
		case r.isOutlet && catchAll:
			return nil, fmt.Errorf("%s: an outlet after a rule with no when: could never apply", r.path)
		case r.isOutlet:
			marks, err = declare(marks, mark{r.outlet, r.path})
		case r.When == "":
			catchAll = true
		}
		if err != nil {
			return nil, err
		}
	}
	groups, err := group(marks, uses, func(u Use) []fragment[StatusRule] { return u.status }, scopeRules)
	if err != nil {
		return nil, err
	}
	var placed []StatusRule
	if !declaresDefault(marks) {
		placed = append(placed, groups[""]...)
	}
	for _, r := range rules {
		if r.isOutlet {
			placed = append(placed, groups[r.outlet]...)
			continue
		}
		placed = append(placed, r)
	}
	return placed, nil
}
