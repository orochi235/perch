package spec

import "testing"

func parseOne(t *testing.T, watchBody string) Watch {
	t.Helper()
	s, err := Parse([]byte(`
app: {name: a, id: b, icon: c, interval: 1s}
watch:
  w:
` + watchBody + `
menu: [{text: Quit, quit: true}]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s.Watches[0]
}

func TestParseShapeNestedListOfObjects(t *testing.T) {
	w := parseOne(t, `    run: [onto, top]
    json: true
    shape:
      nodes: [{name: string, up: bool}]
      jobs: [{id: string, node: string, cmd: string}]`)
	if w.Shape == nil {
		t.Fatal("Shape = nil, want an object type")
	}
	if w.Shape.Kind != TypeObject {
		t.Fatalf("Shape.Kind = %v, want object", w.Shape.Kind)
	}
	if len(w.Shape.Fields) != 2 {
		t.Fatalf("Shape has %d fields, want 2", len(w.Shape.Fields))
	}
	if w.Shape.Fields[0].Name != "nodes" || w.Shape.Fields[1].Name != "jobs" {
		t.Errorf("field order = %q, %q; want nodes, jobs", w.Shape.Fields[0].Name, w.Shape.Fields[1].Name)
	}
	nodes := w.Shape.Fields[0].Type
	if nodes.Kind != TypeList {
		t.Fatalf("nodes.Kind = %v, want list", nodes.Kind)
	}
	if nodes.Elem.Kind != TypeObject || len(nodes.Elem.Fields) != 2 {
		t.Fatalf("nodes element = %v with %d fields, want object with 2", nodes.Elem.Kind, len(nodes.Elem.Fields))
	}
	if nodes.Elem.Fields[1].Name != "up" || nodes.Elem.Fields[1].Type.Kind != TypeBool {
		t.Errorf("nodes[].up = %v, want bool", nodes.Elem.Fields[1].Type.Kind)
	}
}

func TestParseShapeScalarVocabulary(t *testing.T) {
	w := parseOne(t, `    run: [x]
    json: true
    shape: {s: string, i: int, d: double, b: bool, a: any}`)
	want := []TypeKind{TypeString, TypeInt, TypeDouble, TypeBool, TypeAny}
	for i, k := range want {
		if got := w.Shape.Fields[i].Type.Kind; got != k {
			t.Errorf("field %d kind = %v, want %v", i, got, k)
		}
	}
}

func TestParseShapeRejectsUnknownScalar(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: a, id: b, icon: c, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: float}}}
menu: [{text: Quit, quit: true}]
`))
	if err == nil {
		t.Fatal("want error for unknown scalar type 'float', got nil")
	}
}

func TestParseShapeRejectsMultiElementList(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: a, id: b, icon: c, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: [string, int]}}}
menu: [{text: Quit, quit: true}]
`))
	if err == nil {
		t.Fatal("want error for a list type with two elements, got nil")
	}
}

func TestParseShapeAbsentIsNil(t *testing.T) {
	w := parseOne(t, `    run: [x]
    json: true`)
	if w.Shape != nil {
		t.Errorf("Shape = %v, want nil when undeclared", w.Shape)
	}
}
