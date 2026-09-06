package swiftappkit

import (
	"strings"
	"testing"
)

// A command is free to print a field called default or class. Swift takes those
// at a member-access site and refuses them at a declaration site, so a shape
// that names one has to come out backticked or the whole file stops compiling.
const swiftKeywordShape = `
app: {name: keys, id: dev.keys.menubar, icon: circle, interval: 1s}
watch:
  w:
    run: [echo, x]
    json: true
    shape:
      default: string
      class: int
      func: double
      static: bool
      self: any
      nested: {repeat: string, switch: [{case: int}]}
      xs: [double]
      untyped: any
status:
  - when: "w.data.static"
    icon: circle
  - badge: "w.data.default"
menu:
  - text: "{{w.data.class}} {{w.data.func}} {{w.data.nested.repeat}}"
  - each: w.data.nested.switch
    text: "{{it.case}}"
  - each: w.data.xs
    text: "{{it}}"
  - {text: Quit, quit: true}
`

func TestEmitBackticksSwiftKeywordFieldNames(t *testing.T) {
	files := emit(t, swiftKeywordShape)
	shapes := files["Shapes.swift"]
	for _, word := range []string{"default", "class", "func", "static", "self", "repeat", "switch", "case"} {
		if !strings.Contains(shapes, "var `"+word+"`:") {
			t.Errorf("field %q is declared without backticks:\n%s", word, shapes)
		}
	}
	// Member access does not need them, and adding them there is noise.
	if strings.Contains(files["main.swift"], ".`default`") {
		t.Error("main.swift backticks a member access, which does not need it")
	}
}

// Every scalar a shape can declare has a Swift type and a zero value; a missing
// one shows up as a struct with no default, which stops the file compiling.
func TestEmitGivesEveryDeclaredScalarATypeAndAZero(t *testing.T) {
	shapes := emit(t, swiftKeywordShape)["Shapes.swift"]
	for _, want := range []string{
		"String = \"\"",
		"Int = 0",
		"Double = 0",
		"Bool = false",
		"JSONValue = JSONValue.null",
		"[Double] = []",
	} {
		if !strings.Contains(shapes, want) {
			t.Errorf("Shapes.swift has no field declared %q:\n%s", want, shapes)
		}
	}
}

// An expression perch cannot lower has to stop Emit rather than reach swiftc,
// where it would be reported against source the author never wrote.
func TestEmitStopsAtAnExpressionItCannotLower(t *testing.T) {
	for _, doc := range []string{
		`
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: int}}}
menu: [{text: "{{w.data.nope}}"}]
`,
		`
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: int}}}
status: [{when: "w.data.n", icon: circle}]
menu: [{text: Quit, quit: true}]
`,
		`
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: int}}}
menu: [{each: "w.data.n", text: x}]
`,
		`
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: int}}}
menu: [{text: x, when: "w.data.n"}]
`,
		`
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: int}}}
menu: [{text: x, run: [echo, "{{w.data.nope}}"]}]
`,
		`
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: int}}}
menu: [{text: x, open: "http://{{w.data.nope}}"}]
`,
		`
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: int}}}
menu: [{text: x, post: {url: "http://{{w.data.nope}}"}}]
`,
		`
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: int}}}
status: [{badge: "w.data.nope"}]
menu: [{text: Quit, quit: true}]
`,
		`
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {n: int}}}
menu: [{text: x, menu: [{text: "{{w.data.nope}}"}]}]
`,
	} {
		if _, err := tryEmit(t, doc); err == nil {
			t.Errorf("emitted Swift for an expression that cannot be lowered:\n%s", doc)
		}
	}
}
