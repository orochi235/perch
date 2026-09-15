package swiftappkit

import (
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
