package spec

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// yaml.v3 reports a type mismatch by naming a Go type and counting lines in a
// re-encoded copy, neither of which appears in the document the author wrote.
func TestParseTranslatesTypeErrorsIntoTheDocumentsVocabulary(t *testing.T) {
	cases := []struct{ doc, want string }{
		{`
app: {name: a, id: b, icon: c, interval: 1s}
menu: [{text: Quit, run: "echo hi"}]
`, "want a list of strings, got a string"},
		{`
app: {name: a, id: b, icon: c, interval: 1s}
menu: [{text: [a, b], quit: true}]
`, "want a string, got a list"},
		{`
app: {name: a, id: b, icon: c, interval: 1s}
menu: [{text: Quit, quit: yes please}]
`, "want true or false, got a string"},
		{`
app: {name: a, id: b, icon: c, interval: 1s}
menu: [{text: Quit, post: "http://x"}]
`, "want a mapping, got a string"},
	}
	for _, c := range cases {
		got := parseErr(t, c.doc)
		if !strings.Contains(got, c.want) {
			t.Errorf("error = %q, want it to contain %q", got, c.want)
		}
		if strings.Contains(got, "yaml:") || strings.Contains(got, "line ") {
			t.Errorf("error = %q, want it free of yaml.v3's own wording", got)
		}
	}
}

// A single typo names one key; several name all of them, so the author fixes
// them in one pass rather than one build at a time.
func TestParseNamesEveryUnknownKeyAtOnce(t *testing.T) {
	got := parseErr(t, `
app: {name: a, id: b, icon: c, interval: 1s}
menu: [{txet: Quit, quti: true}]
`)
	for _, key := range []string{"txet", "quti"} {
		if !strings.Contains(got, key) {
			t.Errorf("error = %q, want it to quote %q", got, key)
		}
	}
	if !strings.Contains(got, "unknown keys") {
		t.Errorf("error = %q, want the plural form", got)
	}
}

// Every diagnostic has to say where in the document to look, because a
// menubar.yaml has no other landmark than its own paths.
func TestParseErrorsNameTheirPath(t *testing.T) {
	cases := []struct{ doc, want string }{
		{`
app: {name: a, id: b, icon: c, interval: 1s}
menu: [{text: a}, {text: b, nope: 1}]
`, "menu[1]"},
		{`
app: {name: a, id: b, icon: c, interval: 1s}
menu: [{text: a, menu: [{text: b, nope: 1}]}]
`, "menu[0].menu[0]"},
		{`
app: {name: a, id: b, icon: c, interval: 1s}
watch: {w: {run: [x], nope: 1}}
menu: [{text: Quit, quit: true}]
`, "watch.w"},
		{`
app: {name: a, id: b, icon: c, interval: 1s}
watch: {w: {run: [x], json: true, shape: {a: {b: nope}}}}
menu: [{text: Quit, quit: true}]
`, "watch.w.shape.a.b"},
		{`
app: {name: a, id: b, icon: c, interval: 1s}
watch: {w: {run: [x], json: true, shape: {a: [nope]}}}
menu: [{text: Quit, quit: true}]
`, "watch.w.shape.a[]"},
		{`
app: {name: a, id: b, icon: c, interval: 1s}
status: [{when: "a", nope: 1}]
menu: [{text: Quit, quit: true}]
`, "status[0]"},
	}
	for _, c := range cases {
		if got := parseErr(t, c.doc); !strings.Contains(got, c.want) {
			t.Errorf("error = %q, want it to name %q", got, c.want)
		}
	}
}

// The kind names appear in diagnostics, so they have to read as the document's
// own vocabulary rather than as Go constant names.
func TestKindsPrintTheirDocumentSpelling(t *testing.T) {
	types := map[TypeKind]string{
		TypeString: "string", TypeInt: "int", TypeDouble: "double",
		TypeBool: "bool", TypeAny: "any", TypeObject: "object", TypeList: "list",
		TypeKind(99): "unknown",
	}
	for k, want := range types {
		if got := k.String(); got != want {
			t.Errorf("TypeKind(%d) = %q, want %q", k, got, want)
		}
	}
	watches := map[WatchKind]string{
		WatchRun: "run", WatchHTTP: "http", WatchExists: "exists", WatchKind(99): "unknown",
	}
	for k, want := range watches {
		if got := k.String(); got != want {
			t.Errorf("WatchKind(%d) = %q, want %q", k, got, want)
		}
	}
	actions := map[ActionKind]string{
		ActionNone: "none", ActionRun: "run", ActionOpen: "open",
		ActionPost: "post", ActionQuit: "quit", ActionKind(99): "unknown",
	}
	for k, want := range actions {
		if got := k.String(); got != want {
			t.Errorf("ActionKind(%d) = %q, want %q", k, got, want)
		}
	}
}

// A submenu item that also carries an action is the one mistake whose symptom
// is nothing at all: the submenu opens and the action never runs.
func TestValidateNamesTheActionASubmenuWouldSupersede(t *testing.T) {
	got := parseErr(t, `
app: {name: a, id: b, icon: c, interval: 1s}
menu: [{text: a, open: "http://x", menu: [{text: b, quit: true}]}]
`)
	if !strings.Contains(got, "open") {
		t.Errorf("error = %q, want it to name the action that would never run", got)
	}
}

// A name perch cannot use has to say which of the reasons applies, because the
// four failures are fixed in four different ways.
func TestParseRefusesEachKindOfUnusableName(t *testing.T) {
	cases := []struct{ name, want string }{
		{"", "cannot be empty"},
		{"_", "not a usable watch name"},
		{"my-fleet", "letters, digits and underscores"},
		{"9fleet", "letters, digits and underscores"},
		{"package", "reserved in CEL"},
		{"it", "each: binds its element"},
	}
	for _, c := range cases {
		got := parseErr(t, `
app: {name: a, id: b, icon: c, interval: 1s}
watch: {"`+c.name+`": {run: [x]}}
menu: [{text: Quit, quit: true}]
`)
		if !strings.Contains(got, c.want) {
			t.Errorf("watch %q: error = %q, want it to say %q", c.name, got, c.want)
		}
	}
}

func TestParseRefusesUnusableShapeFieldNames(t *testing.T) {
	for _, name := range []string{"", "_", "a-b", "9a", "package"} {
		got := parseErr(t, `
app: {name: a, id: b, icon: c, interval: 1s}
watch: {w: {run: [x], json: true, shape: {"`+name+`": int}}}
menu: [{text: Quit, quit: true}]
`)
		if !strings.Contains(got, "watch.w.shape") {
			t.Errorf("field %q: error = %q, want it to name the shape", name, got)
		}
	}
}

// Every section says what it wants to be given, because the mistake is nearly
// always a mapping written where a list goes or the other way round.
func TestParseNamesTheShapeEachSectionTakes(t *testing.T) {
	cases := []struct{ doc, want string }{
		{"watch: [a]", "watch: want a mapping"},
		{"status: {a: b}", "status: want a list of rules"},
		{"menu: {a: b}", "menu: want a list of menu items"},
		{"menu: [{text: a, menu: {b: c}}]", "menu[0].menu: want a list"},
		{"menu: [[a]]", "want a menu item mapping"},
		{"menu: [nope]", `bare "nope" is not a menu item`},
		{"watch: {w: {run: [x], json: true, shape: [int, int]}}", "exactly one element"},
	}
	for _, c := range cases {
		got := parseErr(t, "app: {name: a, id: b, icon: c, interval: 1s}\n"+c.doc+"\n")
		if !strings.Contains(got, c.want) {
			t.Errorf("%s\n error = %q\n  want %q", c.doc, got, c.want)
		}
	}
}

// An empty document has no app: to report against, so it cannot come back as a
// nil dereference or as yaml.v3's own wording.
func TestParseRefusesAnEmptyDocument(t *testing.T) {
	for _, doc := range []string{"", "\n", "# just a comment\n", "---\n"} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%q: want an error", doc)
		}
	}
}

// An anchor shares one declaration between two places, which is what a document
// with two watches against the same command is written with. The parser
// re-encodes each subtree on its own to reject unknown keys, so an anchor
// defined in a sibling subtree has to be expanded before that happens.
func TestParseFollowsAliasesAcrossSections(t *testing.T) {
	s, err := Parse([]byte(`
app: {name: a, id: b, icon: c, interval: 1s}
watch:
  one: {run: [x], json: true, shape: &job {id: string, node: string}}
  two: {run: [y], json: true, shape: *job}
menu:
  - &quit {text: Quit, quit: true}
  - separator
  - *quit
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, w := range s.Watches {
		if w.Shape == nil || len(w.Shape.Fields) != 2 {
			t.Errorf("watch %s: shape = %+v, want the two fields the anchor declares", w.Name, w.Shape)
		}
	}
	if len(s.Menu) != 3 || s.Menu[2].Text != "Quit" || s.Menu[2].Action.Kind != ActionQuit {
		t.Errorf("menu = %+v, want the aliased item expanded", s.Menu)
	}
}

// A cycle and a doubling chain are both short to write and unbounded to expand,
// so each has to come back as a diagnostic rather than as a hang.
func TestParseRefusesAliasesThatDoNotTerminate(t *testing.T) {
	bomb := "app: {name: a, id: b, icon: c, interval: 1s}\nmenu: &a0 [{text: q, quit: true}]\n"
	for i := 1; i < 24; i++ {
		bomb += "x" + strconv.Itoa(i) + ": &a" + strconv.Itoa(i) +
			" [*a" + strconv.Itoa(i-1) + ", *a" + strconv.Itoa(i-1) + "]\n"
	}
	for _, doc := range []string{
		"app: &x {name: a, id: b, icon: c, interval: 1s, self: *x}\nmenu: [{text: q, quit: true}]\n",
		bomb,
	} {
		done := make(chan error, 1)
		go func() { _, err := Parse([]byte(doc)); done <- err }()
		select {
		case err := <-done:
			if err == nil {
				t.Error("want a refusal for an alias that does not terminate")
			}
		case <-time.After(20 * time.Second):
			t.Fatal("Parse did not return; an alias expansion ran away")
		}
	}
}
