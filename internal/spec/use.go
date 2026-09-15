package spec

import (
	"fmt"
	"slices"
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

	status []fragment[StatusRule]
	menu   []fragment[Item]
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

// AllWatches is every watch polled: the file's, then each use's, in use: order.
func (s *Spec) AllWatches() []Watch {
	out := append([]Watch(nil), s.Watches...)
	for _, u := range s.Uses {
		out = append(out, u.Watches...)
	}
	return out
}

// Key names a watch across the file and its uses: its name, or use.name. It is
// what a preview state writes a use's watch under.
func (w Watch) Key() string {
	if w.Scope == "" {
		return w.Name
	}
	return w.Scope + "." + w.Name
}
