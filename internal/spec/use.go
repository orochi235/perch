package spec

import (
	"fmt"
	"os"
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
}

type rawTemplate struct {
	Params yaml.Node `yaml:"params"`
	Watch  yaml.Node `yaml:"watch"`
	State  yaml.Node `yaml:"state"`
	Status yaml.Node `yaml:"status"`
	Menu   yaml.Node `yaml:"menu"`
}

// nested are the top-level keys a menubar.yaml takes and a template does not.
var nested = []string{"app", "window", "use"}

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

// checkBound refuses the two names perch binds itself: it inside an each:, and
// self inside a template.
func checkBound(path, name, kind string) error {
	binder := map[string]string{"it": "each: binds its element to it", "self": "a template binds its own use to self"}[name]
	if binder == "" {
		return nil
	}
	return fmt.Errorf("%s: %q is bound by perch itself (%s), so a %s cannot take that name", path, name, binder, kind)
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
	where := fmt.Sprintf("%s (%s)", path, file)

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
	values, err := bindParams(&raw.Params, args, path, where)
	if err != nil {
		return Use{}, err
	}
	for _, n := range []*yaml.Node{&raw.Watch, &raw.State, &raw.Status, &raw.Menu} {
		if err := substitute(n, values, where); err != nil {
			return Use{}, err
		}
	}

	u := Use{Name: name, Template: tmpl, File: file}
	if u.Watches, err = parseWatches(&raw.Watch); err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	for i := range u.Watches {
		u.Watches[i].Scope = name
	}
	if u.States, err = parseStates(&raw.State); err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	return u, nil
}

func refuseNesting(root *yaml.Node, where string) error {
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: want a mapping of params, watch, state, status and menu", where)
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if key := root.Content[i].Value; slices.Contains(nested, key) {
			return fmt.Errorf("%s: %s: belongs to a menubar.yaml; a template takes params, watch, state, status and menu, and templates do not nest", where, key)
		}
	}
	return nil
}

// bindParams pairs what a template declares with what a use passes. A param
// declared with ~ is required; anything else is its default.
func bindParams(declared, args *yaml.Node, path, where string) (map[string]string, error) {
	values := map[string]string{}
	required := map[string]bool{}
	if declared.Kind != 0 {
		if declared.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s: params: want a mapping of names to defaults, with ~ for a required one", where)
		}
		for i := 0; i+1 < len(declared.Content); i += 2 {
			name, def := declared.Content[i].Value, declared.Content[i+1]
			if err := checkName("param", where+": params", name); err != nil {
				return nil, err
			}
			switch {
			case def.Kind == yaml.ScalarNode && def.Tag == "!!null":
				required[name] = true
			case def.Kind == yaml.ScalarNode:
				values[name] = def.Value
			default:
				return nil, fmt.Errorf("%s: params.%s: want a default as a string, or ~ to make it required", where, name)
			}
		}
	}
	takes := make([]string, 0, len(values)+len(required))
	for name := range values {
		takes = append(takes, name)
	}
	for name := range required {
		takes = append(takes, name)
	}
	sort.Strings(takes)

	if args.Kind == yaml.ScalarNode && args.Tag == "!!null" {
		args = &yaml.Node{Kind: yaml.MappingNode}
	}
	if args.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: want the template's arguments as a mapping", path)
	}
	for i := 0; i+1 < len(args.Content); i += 2 {
		name, v := args.Content[i].Value, args.Content[i+1]
		if !slices.Contains(takes, name) {
			return nil, fmt.Errorf("%s: %q is not a parameter of this template; it takes %s", path, name, strings.Join(takes, ", "))
		}
		if v.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s: %s: want a string", path, name)
		}
		values[name] = v.Value
		delete(required, name)
	}
	if len(required) > 0 {
		var missing []string
		for name := range required {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("%s: the template requires %s, which this use does not pass", path, strings.Join(missing, ", "))
	}
	return values, nil
}

// substitute fills ${name} into every string value of a parsed template, so a
// value holding ": " or a newline stays one string and cannot change the file's
// structure. Keys are left alone: they are names the template declares.
func substitute(n *yaml.Node, values map[string]string, where string) error {
	if n == nil || n.Kind == 0 {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if !strings.Contains(n.Value, "$") {
			return nil
		}
		var unknown []string
		out := os.Expand(n.Value, func(name string) string {
			if name == "$" {
				return "$"
			}
			v, ok := values[name]
			if !ok {
				unknown = append(unknown, name)
			}
			return v
		})
		if len(unknown) > 0 {
			return unknownParam(unknown[0], values, where)
		}
		n.Value = out
		// An unquoted scalar is resolved again, so json: ${decode} still reads
		// as a bool once it holds "true".
		if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) == 0 {
			n.Tag = ""
		}
	case yaml.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			if err := substitute(n.Content[i], values, where); err != nil {
				return err
			}
		}
	default:
		for _, c := range n.Content {
			if err := substitute(c, values, where); err != nil {
				return err
			}
		}
	}
	return nil
}

func unknownParam(name string, values map[string]string, where string) error {
	if !identifier.MatchString(name) {
		return fmt.Errorf("%s: $%s is not a parameter; write $$ for a literal $", where, name)
	}
	names := make([]string, 0, len(values))
	for k := range values {
		names = append(names, k)
	}
	sort.Strings(names)
	takes := "no parameters"
	if len(names) > 0 {
		takes = strings.Join(names, ", ")
	}
	return fmt.Errorf("%s: ${%s} is not a parameter of this template; it takes %s", where, name, takes)
}
