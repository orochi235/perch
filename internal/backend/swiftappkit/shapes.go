package swiftappkit

import (
	"github.com/orochi235/perch/internal/spec"
)

// swiftType names the Swift type a declared shape lowers to. Objects and lists
// of objects get a generated struct named after their path.
func swiftType(t *spec.Type, path string) string {
	switch t.Kind {
	case spec.TypeString:
		return "String"
	case spec.TypeInt:
		return "Int"
	case spec.TypeDouble:
		return "Double"
	case spec.TypeBool:
		return "Bool"
	case spec.TypeAny:
		return "JSONValue"
	case spec.TypeList:
		return "[" + swiftType(t.Elem, path) + "]"
	case spec.TypeObject:
		return path
	}
	return "JSONValue"
}

func swiftZero(t *spec.Type, path string) string {
	switch t.Kind {
	case spec.TypeString:
		return `""`
	case spec.TypeInt, spec.TypeDouble:
		return "0"
	case spec.TypeBool:
		return "false"
	case spec.TypeAny:
		return "JSONValue.null"
	case spec.TypeList:
		return "[]"
	case spec.TypeObject:
		return path + "()"
	}
	return "JSONValue.null"
}

// emitShapes writes one Decodable struct per object in every declared shape.
// Each field falls back to its zero value, so a command that prints a partial
// document still opens a menu.
func emitShapes(s *spec.Spec) string {
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
		emitObjects(b, w.Shape, exported(w.Name)+"Data")
	}
	if !any {
		return ""
	}
	return b.String()
}

// emitObjects writes t's struct and every struct nested inside it, deepest last.
func emitObjects(b *buf, t *spec.Type, path string) {
	switch t.Kind {
	case spec.TypeList:
		emitObjects(b, t.Elem, path)
		return
	case spec.TypeObject:
	default:
		return
	}

	b.line("struct %s: Decodable {", path)
	b.in()
	for _, f := range t.Fields {
		fp := path + exported(f.Name)
		b.line("var %s: %s = %s", decl(f.Name), swiftType(f.Type, fp), swiftZero(f.Type, fp))
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
		fp := path + exported(f.Name)
		b.line("%s = (try? c.decode(%s.self, forKey: .%s)) ?? %s",
			decl(f.Name), swiftType(f.Type, fp), decl(f.Name), swiftZero(f.Type, fp))
	}
	b.out()
	b.line("}")
	b.out()
	b.line("}")
	b.line("")

	for _, f := range t.Fields {
		emitObjects(b, f.Type, path+exported(f.Name))
	}
}

// resultTypeName is the struct holding one watch's poll outcome.
func resultTypeName(w spec.Watch) string { return exported(w.Name) + "Result" }

func dataTypeAndZero(w spec.Watch) (string, string, bool) {
	if !w.JSON {
		return "", "", false
	}
	if w.Shape == nil {
		return "JSONValue", "JSONValue.null", true
	}
	name := exported(w.Name) + "Data"
	return swiftType(w.Shape, name), swiftZero(w.Shape, name), true
}
