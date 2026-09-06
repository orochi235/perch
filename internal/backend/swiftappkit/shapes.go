package swiftappkit

import (
	"fmt"

	"github.com/orochi235/perch/internal/spec"
)

// structNames assigns every object in every declared shape a Swift struct name
// that is unique across the emitted file. Deriving a name from the field path
// alone is not enough: a field a_b and a field b nested under a both read as
// FleetDataAB, and the second declaration is what the compiler complains about.
type structNames struct {
	shape  map[*spec.Type]string
	result map[string]string // watch name -> its result struct
}

func nameStructs(s *spec.Spec) structNames {
	n := structNames{shape: map[*spec.Type]string{}, result: map[string]string{}}
	taken := map[string]bool{}
	// Result names are claimed first, so a shape never takes one out from under
	// the watch it belongs to. Watch names differ but exported() can flatten two
	// of them together: a_b and aB both read as AB.
	for _, w := range s.Watches {
		n.result[w.Name] = unique(exported(w.Name)+"Result", taken)
	}
	for _, w := range s.Watches {
		n.assign(w.Shape, exported(w.Name)+"Data", taken)
	}
	return n
}

func unique(base string, taken map[string]bool) string {
	name := base
	for i := 2; taken[name]; i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	taken[name] = true
	return name
}

// assign walks in the order emitObjects writes, so a suffixed name lands on the
// same struct every build.
func (n structNames) assign(t *spec.Type, base string, taken map[string]bool) {
	if t == nil {
		return
	}
	switch t.Kind {
	case spec.TypeList:
		n.assign(t.Elem, base, taken)
	case spec.TypeObject:
		name := unique(base, taken)
		n.shape[t] = name
		for _, f := range t.Fields {
			n.assign(f.Type, name+exported(f.Name), taken)
		}
	}
}

// swiftType names the Swift type a declared shape lowers to.
func (n structNames) swiftType(t *spec.Type) string {
	switch t.Kind {
	case spec.TypeString:
		return "String"
	case spec.TypeInt:
		return "Int"
	case spec.TypeDouble:
		return "Double"
	case spec.TypeBool:
		return "Bool"
	case spec.TypeList:
		return "[" + n.swiftType(t.Elem) + "]"
	case spec.TypeObject:
		return n.shape[t]
	}
	return "JSONValue"
}

func (n structNames) swiftZero(t *spec.Type) string {
	switch t.Kind {
	case spec.TypeString:
		return `""`
	case spec.TypeInt, spec.TypeDouble:
		return "0"
	case spec.TypeBool:
		return "false"
	case spec.TypeList:
		return "[]"
	case spec.TypeObject:
		return n.shape[t] + "()"
	}
	return "JSONValue.null"
}

// emitShapes writes one Decodable struct per object in every declared shape.
// Each field falls back to its zero value, so a command that prints a partial
// document still opens a menu.
func emitShapes(s *spec.Spec, n structNames) string {
	b := &buf{}
	var any bool
	for _, w := range s.Watches {
		if w.Shape == nil {
			continue
		}
		if !any {
			b.line("%s", header)
			b.line("import Foundation")
			b.line("")
			any = true
		}
		emitObjects(b, w.Shape, n)
	}
	if !any {
		return ""
	}
	return b.String()
}

// emitObjects writes t's struct and every struct nested inside it, deepest last.
func emitObjects(b *buf, t *spec.Type, n structNames) {
	switch t.Kind {
	case spec.TypeList:
		emitObjects(b, t.Elem, n)
		return
	case spec.TypeObject:
	default:
		return
	}

	b.line("struct %s: Decodable {", n.shape[t])
	b.in()
	for _, f := range t.Fields {
		b.line("var %s: %s = %s", decl(f.Name), n.swiftType(f.Type), n.swiftZero(f.Type))
	}
	b.line("")
	b.line("init() {}")
	b.line("")
	b.line("private enum CodingKeys: String, CodingKey {")
	b.in()
	for _, f := range t.Fields {
		b.line("case %s", decl(f.Name))
	}
	b.out()
	b.line("}")
	b.line("")
	b.line("init(from decoder: Decoder) throws {")
	b.in()
	b.line("let c = try decoder.container(keyedBy: CodingKeys.self)")
	for _, f := range t.Fields {
		b.line("%s = (try? c.decode(%s.self, forKey: .%s)) ?? %s",
			decl(f.Name), n.swiftType(f.Type), decl(f.Name), n.swiftZero(f.Type))
	}
	b.out()
	b.line("}")
	b.out()
	b.line("}")
	b.line("")

	for _, f := range t.Fields {
		emitObjects(b, f.Type, n)
	}
}

// resultTypeName is the struct holding one watch's poll outcome.
func (n structNames) resultTypeName(w spec.Watch) string { return n.result[w.Name] }

func (n structNames) dataTypeAndZero(w spec.Watch) (string, string, bool) {
	if !w.JSON {
		return "", "", false
	}
	if w.Shape == nil {
		return "JSONValue", "JSONValue.null", true
	}
	return n.swiftType(w.Shape), n.swiftZero(w.Shape), true
}
