package spec

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Use is one use of a template: its own watches and its own ordered states,
// reached in expressions by Name, and as self from inside the template.
type Use struct {
	Name     string
	Template string
	File     string // where the template came from, for errors
	Watches  []Watch
	States   []State

	status []statusFragment
	menu   []menuFragment
}

// where names the use and its template file, to prefix an error.
func (u Use) where() string { return fmt.Sprintf("use.%s (%s)", u.Name, u.File) }

type rawTemplate struct {
	Params yaml.Node `yaml:"params"`
	Watch  yaml.Node `yaml:"watch"`
	State  yaml.Node `yaml:"state"`
	Status yaml.Node `yaml:"status"`
	Menu   yaml.Node `yaml:"menu"`
}

// fileOnlyKeys are the top-level keys a menubar.yaml takes and a template does not.
var fileOnlyKeys = []string{"app", "window", "use"}

func parseUses(n *yaml.Node, src TemplateSource) ([]Use, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("use: want a mapping of names to templates, e.g. daemon: {service: {label: dev.example.daemon}}")
	}
	var out []Use
	for i := 0; i+1 < len(n.Content); i += 2 {
		name, body := n.Content[i].Value, n.Content[i+1]
		path := "use." + name
		if err := checkName("use", path, name); err != nil {
			return nil, err
		}
		if err := checkBound(path, name, "use"); err != nil {
			return nil, err
		}
		if body.Kind != yaml.MappingNode || len(body.Content) != 2 {
			return nil, fmt.Errorf("%s: want exactly one template and its arguments, e.g. service: {label: dev.example.daemon}", path)
		}
		u, err := expandUse(name, body.Content[0].Value, body.Content[1], src)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

func expandUse(name, tmpl string, args *yaml.Node, src TemplateSource) (Use, error) {
	path := "use." + name
	if err := checkTemplateName(path, tmpl); err != nil {
		return Use{}, err
	}
	body, file, ok, err := src.Template(tmpl)
	if err != nil {
		return Use{}, fmt.Errorf("%s: %w", path, err)
	}
	if !ok {
		return Use{}, fmt.Errorf("%s: no template named %q; the templates are %s", path, tmpl, strings.Join(src.Names(), ", "))
	}
	u := Use{Name: name, Template: tmpl, File: file}
	where := u.where()

	var doc yaml.Node
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	if len(doc.Content) == 0 {
		return Use{}, fmt.Errorf("%s: the template is empty", where)
	}
	root, err := resolveAliases(doc.Content[0])
	if err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	if err := refuseNesting(root, where); err != nil {
		return Use{}, err
	}
	var raw rawTemplate
	if err := decodeStrict(root, &raw, where); err != nil {
		return Use{}, err
	}
	values, err := bindParams(&raw.Params, args, where)
	if err != nil {
		return Use{}, err
	}
	for _, n := range []*yaml.Node{&raw.Watch, &raw.State, &raw.Status, &raw.Menu} {
		if err := substitute(n, values, where); err != nil {
			return Use{}, err
		}
	}

	if u.Watches, err = parseWatches(&raw.Watch); err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	for i := range u.Watches {
		u.Watches[i].Scope = name
	}
	if u.States, err = parseStates(&raw.State); err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	if u.status, err = parseStatusFragments(&raw.Status, where); err != nil {
		return Use{}, err
	}
	if u.menu, err = parseMenuFragments(&raw.Menu, where); err != nil {
		return Use{}, err
	}
	return u, nil
}

func refuseNesting(root *yaml.Node, where string) error {
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: want a mapping of params, watch, state, status and menu", where)
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if key := root.Content[i].Value; slices.Contains(fileOnlyKeys, key) {
			return fmt.Errorf("%s: %s: belongs to a menubar.yaml; a template takes params, watch, state, status and menu, and templates do not nest", where, key)
		}
	}
	return nil
}

// menuFragment is what one template puts at one outlet. "" is the default.
type menuFragment struct {
	outlet string
	items  []Item
}

type statusFragment struct {
	outlet string
	rules  []StatusRule
}

// outletKey is how a template names the default outlet.
func outletKey(key string) string {
	if key == "default" {
		return ""
	}
	return key
}

func outletLabel(name string) string {
	if name == "" {
		return "the default outlet"
	}
	return fmt.Sprintf("the outlet %q", name)
}

func parseMenuFragments(n *yaml.Node, where string) ([]menuFragment, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: menu: in a template, menu: maps outlet names to lists of items, e.g. default: [...]", where)
	}
	var out []menuFragment
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		path := "menu." + key
		if err := checkName("outlet", path, key); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		items, err := parseMenu(n.Content[i+1], path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		if err := refuseMenuMarks(items, path); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		out = append(out, menuFragment{outlet: outletKey(key), items: items})
	}
	return out, nil
}

func refuseMenuMarks(items []Item, path string) error {
	for i, it := range items {
		if it.isOutlet {
			return fmt.Errorf("%s[%d]: a template fills outlets; it cannot declare one", path, i)
		}
		if err := refuseMenuMarks(it.Menu, fmt.Sprintf("%s[%d].menu", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func parseStatusFragments(n *yaml.Node, where string) ([]statusFragment, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: status: in a template, status: maps outlet names to lists of rules, e.g. default: [...]", where)
	}
	var out []statusFragment
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		if err := checkName("outlet", where+": status."+key, key); err != nil {
			return nil, err
		}
		rules, err := parseStatus(n.Content[i+1])
		if err != nil {
			return nil, fmt.Errorf("%s: status.%s: %w", where, key, err)
		}
		for j, r := range rules {
			switch {
			case r.isOutlet:
				return nil, fmt.Errorf("%s: status.%s[%d]: a template fills outlets; it cannot declare one", where, key, j)
			case r.When == "":
				return nil, fmt.Errorf("%s: status.%s[%d]: a rule with no when: would land ahead of the file's own rules and shadow every one of them", where, key, j)
			}
		}
		out = append(out, statusFragment{outlet: outletKey(key), rules: rules})
	}
	return out, nil
}

// landing is where a fragment goes: its own outlet if the file declares it,
// the default outlet otherwise.
func landing(outlet string, declared map[string]bool) string {
	if declared[outlet] {
		return outlet
	}
	return ""
}

func unfilled(declared map[string]bool, filled func(string) bool, list string) error {
	var names []string
	for name := range declared {
		if name != "" && !filled(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		return fmt.Errorf("%s: no use fills the outlet %q", list, names[0])
	}
	return nil
}

// placeMenu puts every use's menu fragments at the outlets the file declares.
func placeMenu(items []Item, uses []Use) ([]Item, error) {
	declared := map[string]bool{}
	if err := collectMenuOutlets(items, "menu", declared); err != nil {
		return nil, err
	}
	fill := func(outlet string) []Item {
		var out []Item
		for _, u := range uses {
			for _, f := range u.menu {
				if landing(f.outlet, declared) == outlet {
					out = append(out, scopeItems(f.items, u.Name)...)
				}
			}
		}
		return out
	}
	if err := unfilled(declared, func(n string) bool { return len(fill(n)) > 0 }, "menu"); err != nil {
		return nil, err
	}
	placed := expandMenuOutlets(items, fill)
	if !declared[""] {
		placed = append(placed, fill("")...)
	}
	return placed, nil
}

func collectMenuOutlets(items []Item, path string, declared map[string]bool) error {
	for i, it := range items {
		p := fmt.Sprintf("%s[%d]", path, i)
		if it.isOutlet {
			if declared[it.outlet] {
				return fmt.Errorf("%s: %s is declared twice", p, outletLabel(it.outlet))
			}
			declared[it.outlet] = true
			continue
		}
		if err := collectMenuOutlets(it.Menu, p+".menu", declared); err != nil {
			return err
		}
	}
	return nil
}

func expandMenuOutlets(items []Item, fill func(string) []Item) []Item {
	if items == nil {
		return nil
	}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.isOutlet {
			out = append(out, fill(it.outlet)...)
			continue
		}
		it.Menu = expandMenuOutlets(it.Menu, fill)
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

// placeStatus is placeMenu for status rules. With no default declared, the
// default is the start of the list.
func placeStatus(rules []StatusRule, uses []Use) ([]StatusRule, error) {
	declared := map[string]bool{}
	for i, r := range rules {
		if !r.isOutlet {
			continue
		}
		if declared[r.outlet] {
			return nil, fmt.Errorf("status[%d]: %s is declared twice", i, outletLabel(r.outlet))
		}
		declared[r.outlet] = true
	}
	fill := func(outlet string) []StatusRule {
		var out []StatusRule
		for _, u := range uses {
			for _, f := range u.status {
				if landing(f.outlet, declared) != outlet {
					continue
				}
				for _, r := range f.rules {
					r.Scope = u.Name
					out = append(out, r)
				}
			}
		}
		return out
	}
	if err := unfilled(declared, func(n string) bool { return len(fill(n)) > 0 }, "status"); err != nil {
		return nil, err
	}
	var placed []StatusRule
	if !declared[""] {
		placed = append(placed, fill("")...)
	}
	for _, r := range rules {
		if r.isOutlet {
			placed = append(placed, fill(r.outlet)...)
			continue
		}
		placed = append(placed, r)
	}
	return placed, nil
}
