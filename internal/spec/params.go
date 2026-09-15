package spec

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// bindParams pairs what a template declares with what a use passes. A param
// declared with ~ is required; anything else is its default.
func bindParams(declared, args *yaml.Node, where string) (map[string]string, error) {
	values := map[string]string{}
	required := map[string]bool{}
	if declared.Kind != 0 {
		if declared.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s: params: want a mapping of names to defaults, with ~ for a required one", where)
		}
		for i := 0; i+1 < len(declared.Content); i += 2 {
			name, def := declared.Content[i].Value, declared.Content[i+1]
			if err := checkParamName(where+": params", name); err != nil {
				return nil, err
			}
			switch {
			case isNull(def):
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

	if isNull(args) {
		args = &yaml.Node{Kind: yaml.MappingNode}
	}
	if args.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: want the template's arguments as a mapping", where)
	}
	for i := 0; i+1 < len(args.Content); i += 2 {
		name, v := args.Content[i].Value, args.Content[i+1]
		switch {
		case !slices.Contains(takes, name):
			return nil, fmt.Errorf("%s: %q is not a parameter of this template; it takes %s", where, name, takesList(takes))
		case isNull(v) && required[name]:
			return nil, fmt.Errorf("%s: %s: pass a value; the template requires it", where, name)
		case isNull(v):
			return nil, fmt.Errorf("%s: %s: pass a value, or leave it out to take the default", where, name)
		case v.Kind != yaml.ScalarNode:
			return nil, fmt.Errorf("%s: %s: want a string", where, name)
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
		return nil, fmt.Errorf("%s: the template requires %s, which this use does not pass", where, strings.Join(missing, ", "))
	}
	return values, nil
}

func isNull(n *yaml.Node) bool { return n.Kind == yaml.ScalarNode && n.Tag == "!!null" }

// plainNull are the plain scalars YAML resolves as null.
var plainNull = map[string]bool{"": true, "~": true, "null": true, "Null": true, "NULL": true}

// substitute fills ${name} into every string value of a parsed template, so a
// value holding ": " or a newline stays one string and cannot change the file's
// structure. Keys are left alone: they are names the template declares.
func substitute(n *yaml.Node, values map[string]string, where string) error {
	if n == nil || n.Kind == 0 {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if !strings.Contains(n.Value, "${") {
			return nil
		}
		out, err := expand(n.Value, values, where)
		if err != nil {
			return err
		}
		n.Value = out
		// An unquoted, untagged scalar is resolved again, so json: ${decode}
		// reads as a bool once it holds "true" — but an empty one stays a string.
		if n.Style&(yaml.TaggedStyle|yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) == 0 && !plainNull[out] {
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

// expand fills ${name}, and $${ writes a literal ${. Any other $ is left as
// written, so shell text like $HOME or $$ passes through a template untouched.
func expand(s string, values map[string]string, where string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "$${"):
			b.WriteString("${")
			i += 3
		case strings.HasPrefix(s[i:], "${"):
			end := strings.IndexByte(s[i+2:], '}')
			// A space or $ before the } means this ${ was never closed.
			if end < 0 || strings.ContainsFunc(s[i+2:i+2+end], func(r rune) bool { return r == '$' || unicode.IsSpace(r) }) {
				line, _, _ := strings.Cut(s[i:], "\n")
				return "", fmt.Errorf("%s: %q has a ${ with no closing }, or a name with a space in it", where, line)
			}
			name := s[i+2 : i+2+end]
			switch {
			case name == "":
				return "", fmt.Errorf("%s: ${} names no parameter", where)
			case !identifier.MatchString(name):
				return "", fmt.Errorf("%s: ${%s} is not a parameter name; write $${ for a literal ${", where, name)
			}
			v, ok := values[name]
			if !ok {
				return "", unknownParam(name, values, where)
			}
			b.WriteString(v)
			i += end + 3
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String(), nil
}

func unknownParam(name string, values map[string]string, where string) error {
	names := make([]string, 0, len(values))
	for k := range values {
		names = append(names, k)
	}
	sort.Strings(names)
	return fmt.Errorf("%s: ${%s} is not a parameter of this template; it takes %s; write $${ for a literal ${", where, name, takesList(names))
}

func takesList(names []string) string {
	if len(names) == 0 {
		return "no parameters"
	}
	return strings.Join(names, ", ")
}
