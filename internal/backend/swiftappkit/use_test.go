package swiftappkit

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

const templatesDoc = `
app: {name: wall, id: dev.example.wall.menubar, icon: rectangle.stack, interval: 5s}
use:
  daemon:
    service: {label: dev.example.wall.daemon}
  client:
    service: {label: dev.example.wall.client, noun: Client}
watch:
  health: {http: "http://127.0.0.1:8787/api/health"}
state:
  - down: "!daemon.running"
  - wedged: "!health.ok"
  - up:
status:
  - outlet
  - {when: down, dim: true}
menu:
  - outlet
  - separator
  - {text: "pid {{daemon.agent.pid}}", when: up}
  - outlet: controls
  - separator
  - {text: Quit, quit: true}
`

func TestAUseIsAStructInsideResults(t *testing.T) {
	files := emit(t, templatesDoc)
	for file, wants := range map[string][]string{
		"Render.swift": {
			"struct DaemonUse {",
			"var agent = DaemonAgentResult()",
			"var daemon = DaemonUse()",
			"var client = ClientUse()",
			"extension DaemonUse {",
			"var uninstalled: Bool { (!(self.agent.installed)) }",
			"var stopped: Bool { !self.uninstalled && (!(self.agent.loaded)) }",
			"var state_down: Bool { (!(self.daemon.running)) }",
			"results.daemon.agent.pid",
			`label: "dev.example.wall.client"`,
		},
		"main.swift": {
			"next.daemon.agent = r",
			`DaemonAgentResult(Watcher.launchAgent(label: "dev.example.wall.daemon"`,
		},
	} {
		for _, want := range wants {
			if !strings.Contains(files[file], want) {
				t.Errorf("%s is missing %q", file, want)
			}
		}
	}
}

// fakeTemplates stands in for a repo's own templates.
type fakeTemplates map[string]string

func (f fakeTemplates) Template(name string) ([]byte, string, bool, error) {
	src, ok := f[name]
	return []byte(src), "templates/" + name + ".yaml", ok, nil
}

func (f fakeTemplates) Names() []string {
	var out []string
	for name := range f {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func emitWith(t *testing.T, doc string, src spec.TemplateSource) error {
	t.Helper()
	s, err := spec.ParseWith([]byte(doc), src)
	if err != nil {
		t.Fatalf("spec.ParseWith: %v", err)
	}
	_, err = New().Emit(s)
	return err
}

// A template sees only self, so it cannot come to depend on a file it was not
// written for.
func TestATemplateItemCannotReachTheFilesWatches(t *testing.T) {
	src := fakeTemplates{"peek": "watch:\n  own: {exists: /tmp}\nmenu:\n  default:\n    - {text: peek, when: \"fileWatch.ok\"}\n"}
	err := emitWith(t, `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
use:
  p:
    peek:
watch:
  fileWatch: {exists: /tmp}
menu: [outlet]
`, src)
	if err == nil || !strings.Contains(err.Error(), `unknown name "fileWatch"`) {
		t.Errorf("err = %v, want the file's watch refused inside the template", err)
	}
}

// Placement merges a template's items into the file's list, so an error has to
// name where the author wrote the item, not its index in the merged menu.
func TestATemplateItemErrorNamesWhereItWasWritten(t *testing.T) {
	src := fakeTemplates{"peek": "watch:\n  agent: {launchagent: dev.example.p}\nmenu:\n  default:\n    - {text: \"{{self.agent.pidd}}\"}\n    - {text: two}\n"}
	head := `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
use:
  p:
    peek:
`
	err := emitWith(t, head+"menu: [outlet]\n", src)
	if want := "use.p (templates/peek.yaml): menu.default[0].text:"; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("err = %v, want it to start %q", err, want)
	}

	err = emitWith(t, head+"menu: [outlet, {text: \"{{nope}}\"}]\n", fakeTemplates{"peek": "menu:\n  default:\n    - {text: one}\n    - {text: two}\n"})
	if want := "menu[1].text:"; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("err = %v, want it to start %q", err, want)
	}
}

// kwTemplates names a template's watches and states with Swift keywords and
// nests an each: inside one of its items' submenus.
var kwTemplates = fakeTemplates{"kw": `
watch:
  default: {launchagent: dev.kw.x}
  list: {run: [x], json: true, shape: [{name: string}]}
state:
  - case: "!self.default.installed"
  - repeat: "!self.default.loaded"
  - fine:
status:
  default:
    - {when: self.case, icon: exclamationmark.triangle}
menu:
  default:
    - text: "pid {{self.default.pid}}"
      when: "self.fine || self.repeat"
      menu:
        - {text: "{{it.name}}", each: self.list.data, when: "it.name != ''"}
    - {agent: self.default.start, when: "self.fine || self.repeat"}
    - {text: "{{it.name}}", each: self.list.data}
`}

// kwTemplateDoc places kw's items through an outlet inside a file submenu,
// beside a file item and a file each: of its own.
const kwTemplateDoc = `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
use:
  default:
    kw:
  switch:
    kw:
watch:
  defaultDefault: {launchagent: dev.kw.y}
  rows: {run: [y], json: true, shape: [{id: string}]}
state:
  - down: "!default.default.running && !switch.fine"
  - up:
status:
  - outlet
  - {when: down, dim: true}
menu:
  - text: Sub
    menu:
      - outlet
      - {text: "{{it.id}}", each: rows.data}
  - text: Rows
    each: rows.data
    menu:
      - {text: "{{it.id}}"}
  - {agent: defaultDefault.stop, when: "up || down"}
  - {agent: default.default.stop}
  - {text: Quit, quit: true}
`

// usesOnlyTemplates has a use with states and no watches, and one named like
// the local the render functions declare.
var usesOnlyTemplates = fakeTemplates{
	"bare": "state:\n  - a: \"true\"\n  - b:\nmenu:\n  default:\n    - {text: a, when: self.a}\n",
	"svc":  "watch:\n  agent: {launchagent: dev.x}\nmenu:\n  default:\n    - {text: \"{{self.agent.pid}}\"}\n",
}

const usesOnlyDoc = `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
use:
  results:
    bare:
  menu:
    svc:
menu: [outlet]
`

func TestItemsCrossBetweenATemplatesScopeAndTheFiles(t *testing.T) {
	s, err := spec.ParseWith([]byte(kwTemplateDoc), kwTemplates)
	if err != nil {
		t.Fatalf("spec.ParseWith: %v", err)
	}
	files, err := New().Emit(s)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	render := ""
	for _, f := range files {
		if f.Name == "Render.swift" {
			render = string(f.Body)
		}
	}
	for _, want := range []string{
		"var `case`: Bool { (!(self.default.installed)) }",
		"var `repeat`: Bool { !self.case && (!(self.default.loaded)) }",
		// The template's pid item, reached through its use, is still a submenu.
		`.submenu("pid \(String(results.default.default.pid))"`,
	} {
		if !strings.Contains(render, want) {
			t.Errorf("Render.swift is missing %q:\n%s", want, render)
		}
	}

	// The template's item, reached through its use: it, bound inside the
	// template's submenu, is still it there.
	assertLoopVarUsed(t, render, `for (it\d+) in results\.default\.list\.data \{`, "name")
	// The file's each: after the outlet is back among the file's own names.
	// results.rows.data is walked twice (a plain append and a submenu), so
	// every loop the menu-wide counter names must use its own variable.
	assertLoopVarUsed(t, render, `for (it\d+) in results\.rows\.data \{`, "id")
}

// assertLoopVarUsed finds each `for <var> in <collection> {` loop matching
// loopPattern and checks <var> is what gets interpolated inside that specific
// loop's own body (its text up to the matching closing brace), rather than
// asserting the counter's exact numbering or matching anywhere in the file.
func assertLoopVarUsed(t *testing.T, render, loopPattern, field string) {
	t.Helper()
	locs := regexp.MustCompile(loopPattern).FindAllStringSubmatchIndex(render, -1)
	if len(locs) == 0 {
		t.Fatalf("Render.swift has no loop matching %q:\n%s", loopPattern, render)
	}
	for _, loc := range locs {
		v := render[loc[2]:loc[3]]
		body := loopBody(render, loc[1]-1)
		want := fmt.Sprintf(`.append(.item("\(%s.%s)"`, v, field)
		if !strings.Contains(body, want) {
			t.Errorf("Render.swift's loop over %s does not use %s.%s in its own body:\n%s", v, v, field, body)
		}
	}
}

// loopBody returns the text strictly between the brace at openBrace and its
// matching close, so a match can't be satisfied by a sibling loop's line.
func loopBody(render string, openBrace int) string {
	depth := 0
	for i := openBrace; i < len(render); i++ {
		switch render[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return render[openBrace+1 : i]
			}
		}
	}
	return render[openBrace+1:]
}

func TestThePreviewDriverFillsAUsesWatches(t *testing.T) {
	for doc, want := range map[string]string{
		templatesDoc: `if let o = state.watches["daemon.agent"] {`,
		keywordUse:   "results.`default`.agent = ",
	} {
		s, err := spec.Parse([]byte(doc))
		if err != nil {
			t.Fatalf("spec.Parse: %v", err)
		}
		files, err := New().EmitPreview(s)
		if err != nil {
			t.Fatalf("EmitPreview: %v", err)
		}
		found := false
		for _, f := range files {
			if f.Name == "main.swift" {
				found = true
				if !strings.Contains(string(f.Body), want) {
					t.Errorf("the preview driver is missing %q:\n%s", want, f.Body)
				}
			}
		}
		if !found {
			t.Errorf("EmitPreview did not emit main.swift")
		}
	}
}
