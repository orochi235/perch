// Package shape infers a watch's shape declaration from one sample run, so the
// optional typing that normally goes unused because authoring it is a chore
// costs one command instead.
package shape

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/orochi235/perch/internal/spec"
)

// plainKey is the shape of a JSON key that can be written bare in YAML. Anything
// else is quoted, so a key holding a colon or a leading digit still parses, and
// perch build can then reject it against the line it came from.
var plainKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// yamlKey renders name as a YAML double-quoted scalar unless it can be written
// bare. YAML refuses a raw control character even inside quotes, and a command
// is free to print one in a key.
func yamlKey(name string) string {
	if plainKey.MatchString(name) {
		return name
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range name {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			fmt.Fprintf(&b, `\x%02x`, r)
		case r == 0xfffe || r == 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Infer reads one JSON document and renders the YAML body of a shape: block.
func Infer(doc []byte) (string, error) { return InferAll([][]byte{doc}) }

// InferAll unions several samples. One healthy sample omits every field that
// only appears when something is wrong, which is disproportionately what a
// widget branches on, so more samples make a better declaration.
func InferAll(docs [][]byte) (string, error) {
	if len(docs) == 0 {
		return "", fmt.Errorf("need at least one sample")
	}
	var t *spec.Type
	for _, doc := range docs {
		one, err := parseDocument(doc)
		if err != nil {
			return "", err
		}
		t = unify(t, one)
	}
	if t.Kind != spec.TypeObject {
		return "", fmt.Errorf("a shape describes an object; this command printed a %v", t.Kind)
	}
	return renderObject(t), nil
}

// renderObject writes the YAML body of a shape: block. An object with no fields
// is written {} rather than as nothing at all: a command whose result set is
// empty still has to yield a block that parses.
func renderObject(t *spec.Type) string {
	if len(t.Fields) == 0 {
		return "{}\n"
	}
	var b strings.Builder
	for _, f := range t.Fields {
		writeField(&b, f, 0)
	}
	return b.String()
}

func parseDocument(doc []byte) (*spec.Type, error) {
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	t, err := parse(dec)
	if err != nil {
		return nil, err
	}
	// Not dec.Token(): it reports trailing bytes that are not valid JSON as an
	// error, which reads as "no more tokens" and lets them through.
	if rest := bytes.TrimSpace(doc[dec.InputOffset():]); len(rest) > 0 {
		return nil, fmt.Errorf("trailing content after the JSON document: %q", clip(string(rest), 40))
	}
	return t, nil
}

func parse(dec *json.Decoder) (*spec.Type, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			return parseObject(dec)
		case '[':
			return parseArray(dec)
		}
		return nil, fmt.Errorf("unexpected %v", v)
	case string:
		return &spec.Type{Kind: spec.TypeString}, nil
	case bool:
		return &spec.Type{Kind: spec.TypeBool}, nil
	case json.Number:
		if _, err := v.Int64(); err == nil {
			return &spec.Type{Kind: spec.TypeInt}, nil
		}
		return &spec.Type{Kind: spec.TypeDouble}, nil
	case nil:
		return &spec.Type{Kind: spec.TypeAny}, nil
	}
	return nil, fmt.Errorf("unexpected JSON token %v", tok)
}

func parseObject(dec *json.Decoder) (*spec.Type, error) {
	t := &spec.Type{Kind: spec.TypeObject}
	seen := map[string]int{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("object key is not a string")
		}
		// A shape: block is written in YAML's simple-key form, which tops out at
		// 1024 characters. Rendering a longer key emits a block that does not
		// parse back, so refuse it here and name the key.
		if rendered := yamlKey(name); len(rendered) > 1024 {
			return nil, fmt.Errorf("key %s… is too long for a shape field; YAML keys stop at 1024 characters", clip(rendered, 32))
		}
		ft, err := parse(dec)
		if err != nil {
			return nil, err
		}
		// JSON allows a key twice; YAML's simple-key form does not, and emitting
		// it twice puts the whole shape out of reach. Two occurrences are two
		// observations of the same position, so they unify.
		if i, ok := seen[name]; ok {
			t.Fields[i].Type = unify(t.Fields[i].Type, ft)
			continue
		}
		seen[name] = len(t.Fields)
		t.Fields = append(t.Fields, spec.Field{Name: name, Type: ft})
	}
	_, err := dec.Token() // closing }
	return t, err
}

func parseArray(dec *json.Decoder) (*spec.Type, error) {
	var elem *spec.Type
	for dec.More() {
		et, err := parse(dec)
		if err != nil {
			return nil, err
		}
		elem = unify(elem, et)
	}
	if _, err := dec.Token(); err != nil { // closing ]
		return nil, err
	}
	if elem == nil {
		elem = &spec.Type{Kind: spec.TypeAny}
	}
	return &spec.Type{Kind: spec.TypeList, Elem: elem}, nil
}

// unify widens two observations of the same position into one declaration.
// Objects union their fields; anything else that disagrees becomes any.
func unify(a, b *spec.Type) *spec.Type {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case a.Kind == spec.TypeAny || b.Kind == spec.TypeAny:
		return &spec.Type{Kind: spec.TypeAny}
	case a.Kind == spec.TypeObject && b.Kind == spec.TypeObject:
		return unifyObjects(a, b)
	case a.Kind == spec.TypeList && b.Kind == spec.TypeList:
		return &spec.Type{Kind: spec.TypeList, Elem: unify(a.Elem, b.Elem)}
	case a.Kind == b.Kind:
		return a
	case isNumber(a.Kind) && isNumber(b.Kind):
		return &spec.Type{Kind: spec.TypeDouble}
	}
	return &spec.Type{Kind: spec.TypeAny}
}

func isNumber(k spec.TypeKind) bool { return k == spec.TypeInt || k == spec.TypeDouble }

func unifyObjects(a, b *spec.Type) *spec.Type {
	out := &spec.Type{Kind: spec.TypeObject}
	seen := map[string]int{}
	for _, f := range a.Fields {
		seen[f.Name] = len(out.Fields)
		out.Fields = append(out.Fields, f)
	}
	for _, f := range b.Fields {
		if i, ok := seen[f.Name]; ok {
			out.Fields[i].Type = unify(out.Fields[i].Type, f.Type)
			continue
		}
		out.Fields = append(out.Fields, f)
	}
	return out
}

// flowable reports whether a type fits on one line, which is how the design
// doc's own example is written.
func flowable(t *spec.Type) bool {
	switch t.Kind {
	case spec.TypeObject:
		for _, f := range t.Fields {
			if f.Type.Kind == spec.TypeObject || f.Type.Kind == spec.TypeList {
				return false
			}
		}
		return true
	case spec.TypeList:
		return flowable(t.Elem)
	}
	return true
}

func flow(t *spec.Type) string {
	switch t.Kind {
	case spec.TypeObject:
		parts := make([]string, 0, len(t.Fields))
		for _, f := range t.Fields {
			parts = append(parts, yamlKey(f.Name)+": "+flow(f.Type))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case spec.TypeList:
		return "[" + flow(t.Elem) + "]"
	}
	return t.Kind.String()
}

func writeField(b *strings.Builder, f spec.Field, indent int) {
	pad := strings.Repeat(" ", indent)
	if flowable(f.Type) {
		fmt.Fprintf(b, "%s%s: %s\n", pad, yamlKey(f.Name), flow(f.Type))
		return
	}
	switch f.Type.Kind {
	case spec.TypeObject:
		fmt.Fprintf(b, "%s%s:\n", pad, yamlKey(f.Name))
		for _, sub := range f.Type.Fields {
			writeField(b, sub, indent+2)
		}
	case spec.TypeList:
		fmt.Fprintf(b, "%s%s:\n", pad, yamlKey(f.Name))
		writeElement(b, f.Type.Elem, indent+2)
	}
}

func writeElement(b *strings.Builder, t *spec.Type, indent int) {
	pad := strings.Repeat(" ", indent)
	if flowable(t) {
		fmt.Fprintf(b, "%s- %s\n", pad, flow(t))
		return
	}
	fmt.Fprintf(b, "%s-\n", pad)
	switch t.Kind {
	case spec.TypeObject:
		for _, f := range t.Fields {
			writeField(b, f, indent+2)
		}
	case spec.TypeList:
		writeElement(b, t.Elem, indent+2)
	}
}

// clip shortens a string for a diagnostic without splitting a rune, which would
// put a broken character in the message.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
