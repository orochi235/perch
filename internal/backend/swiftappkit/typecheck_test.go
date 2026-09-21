package swiftappkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/orochi235/perch/v2/internal/backend"
	"github.com/orochi235/perch/v2/internal/spec"
)

// designDocExample is the schema section of docs/superpowers/specs, verbatim.
const designDocExample = `
app:
  name: onto
  id: dev.onto.menubar
  icon: rectangle.3.group
  interval: 5s
watch:
  fleet:
    run: [onto, top, --once, --json]
    json: true
    shape:
      nodes: [{name: string, up: bool}]
      jobs:  [{id: string, node: string, cmd: string}]
status:
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - when: "fleet.data.jobs.size() == 0"
    dim: true
  - badge: "fleet.data.jobs.size()"
menu:
  - text: "{{fleet.data.nodes.size()}} nodes · {{fleet.data.jobs.size()}} running"
  - separator
  - each: fleet.data.jobs
    text: "{{it.node}} — {{it.cmd}}"
    menu:
      - {text: Logs,  run: [onto, logs, "{{it.id}}"]}
      - {text: Prune, run: [onto, prune, "{{it.id}}"]}
      - {text: Kill,  run: [onto, kill, "{{it.id}}"]}
  - separator
  - {text: Quit, quit: true}
`

// everyFeature exercises the parts the design doc example does not: an http and
// an exists watch, an untyped .data, open and post, a when: guard, and menu
// item icons.
const everyFeature = `
app: {name: brainhouse, id: dev.brainhouse.menubar, icon: brain, interval: 10s}
watch:
  server:
    http: http://127.0.0.1:8765/api/health
    json: true
  agent:
    run: [launchctl, print, gui/501/dev.brainhouse]
  plist:
    exists: ~/Library/LaunchAgents/dev.brainhouse.plist
status:
  - when: "!plist.ok"
    icon: circle.dashed
  - when: "!server.ok"
    icon: {asset: alarm}
    dim: true
  - badge: "string(server.data.sessions.size())"
menu:
  - text: "{{server.data.sessions.size()}} sessions"
    when: "server.ok"
  - text: "not running"
    icon: exclamationmark.triangle
    when: "!server.ok"
  - separator
  - {text: Open, icon: safari, open: "http://127.0.0.1:8765"}
  - {text: Restart, run: [launchctl, kickstart, -k, gui/501/dev.brainhouse], when: "agent.ok"}
  - text: Server
    icon: {asset: alarm}
    menu:
      - {text: Ping, post: {url: "http://127.0.0.1:8765/api/ping", body: {source: menubar}}}
  - separator
  - {text: Quit, quit: true}
`

// states reaches a state from both rendered functions, from inside a larger
// expression, and from inside a string literal — where the name is text and
// has to stay text. `running` is read only by the menu, which is what would
// warn if states were emitted as locals rather than properties.
const states = `
app: {name: worker, id: dev.example.worker.menubar, icon: gearshape, interval: 10s}
watch:
  agent:
    run: [launchctl, print, gui/501/dev.example.worker]
  plist:
    exists: ~/Library/LaunchAgents/dev.example.worker.plist
state:
  - uninstalled: "!plist.ok"
  - stopped: "!agent.ok"
  - running:
status:
  - when: uninstalled
    icon: exclamationmark.triangle
  - when: stopped
    dim: true
menu:
  - {text: "the agent is running", when: running}
  - {text: "needs attention", when: "uninstalled || stopped"}
  - {text: "uninstalled is a word here, not a state", when: "agent.out == \"uninstalled\""}
  - {text: Quit, quit: true}
`

// watchInAction reaches a watch from inside an action, which the two examples
// above never do: they only ever interpolate the each: element.
const watchInAction = `
app: {name: reach, id: dev.reach.menubar, icon: circle, interval: 3s}
watch:
  w:
    run: [echo, x]
    json: true
    shape: {id: string, host: string}
menu:
  - {text: Logs, run: [echo, "{{w.data.id}}"]}
  - {text: Site, open: "https://{{w.data.host}}"}
  - {text: Ping, post: {url: "https://{{w.data.host}}/ping", body: {id: "{{w.data.id}}"}}}
  - {text: Quit, quit: true}
`

// launchAgent covers the fourth watch kind and the agent: action: the fields
// it binds instead of .ok, a state block reading three of them, the escape
// hatch of reaching .target from inside argv, and all three verbs.
const launchAgent = `
app: {name: worker, id: dev.example.worker.menubar, icon: gearshape, interval: 10s}
watch:
  worker:
    launchagent: dev.example.worker
state:
  - uninstalled: "!worker.installed"
  - stopped: "!worker.loaded"
  - idle: "!worker.running"
  - running:
status:
  - {when: uninstalled, icon: exclamationmark.triangle}
  - {when: "stopped || idle", dim: true}
menu:
  - {text: "Running · pid {{worker.pid}}", when: running}
  - {text: "Loaded, not running", when: idle}
  - {text: "Not loaded", when: stopped}
  - {text: "Not installed", when: uninstalled}
  - separator
  - {text: Start, when: stopped, agent: worker.start}
  - {text: Stop, when: "running || idle", agent: worker.stop}
  - {text: Restart, when: "!uninstalled", agent: worker.restart}
  - {text: Blame, when: "worker.loaded", run: [launchctl, blame, "{{worker.target}}"]}
  - {text: Quit, quit: true}
`

// brainhouseOnService is brainhouse's menubar.yaml rewritten on service, as
// evidence the template covers a file that exists. brainhouse is not changed.
const brainhouseOnService = `
app: {name: brainhouse, id: com.brainhouse.menubar, icon: brain, interval: 5s}
use:
  server:
    service: {label: com.brainhouse}
watch:
  summary:
    http: http://localhost:8765/api/summary
    json: true
    shape: {live: int, awaiting_input: int}
state:
  - down: "!server.agent.loaded"
  - wedged: "!summary.ok"
  - up:
status:
  - {when: wedged, icon: exclamationmark.triangle}
  - {when: down, dim: true}
  - badge: "summary.data.awaiting_input > 0 ? string(summary.data.awaiting_input) : ''"
menu:
  - text: "Server: running on :8765 — {{summary.data.awaiting_input}} awaiting input"
    when: "up && summary.data.awaiting_input > 0"
  - {text: "Server: not responding on :8765", when: wedged}
  - outlet
  - separator
  - {text: Open Dashboard, open: "http://localhost:8765/"}
  - separator
  - outlet: controls
  - {text: Open Logs, open: ~/Library/Logs/brainhouse}
  - separator
  - {text: Quit, quit: true}
`

// ontoOnService is the same for onto's local agent.
const ontoOnService = `
app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 10s}
use:
  local:
    service: {label: dev.onto.agent, noun: This agent}
watch:
  fleet: {run: [onto, top, --json], json: true}
menu:
  - {when: "!fleet.ok", text: "onto did not answer"}
  - separator
  - outlet
  - outlet: controls
  - separator
  - {text: Quit, quit: true}
`

// keywordUse names a use with a word Swift takes only after a dot, and reaches
// it from a file state, which is lowered inside an extension of Results.
const keywordUse = `
app: {name: kw, id: dev.kw.menubar, icon: circle, interval: 5s}
use:
  default:
    service: {label: dev.kw.agent}
state:
  - down: "!default.running"
  - up:
menu:
  - {text: "pid {{default.agent.pid}}", when: up}
  - outlet
  - {text: Quit, quit: true}
`

// TestEmittedSwiftTypechecks is the check a golden test alone cannot make: a
// golden stays green while emitting Swift that does not compile.
func TestEmittedSwiftTypechecks(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	for name, doc := range map[string]string{
		"minimal":             minimal,
		"designDoc":           designDocExample,
		"everyFeature":        everyFeature,
		"collidingNames":      collidingNames,
		"watchInAction":       watchInAction,
		"swiftKeywords":       swiftKeywordShape,
		"states":              states,
		"launchAgent":         launchAgent,
		"templates":           templatesDoc,
		"brainhouseOnService": brainhouseOnService,
		"ontoOnService":       ontoOnService,
		"keywordUse":          keywordUse,
	} {
		t.Run(name, func(t *testing.T) {
			s, err := spec.Parse([]byte(doc))
			if err != nil {
				t.Fatalf("spec.Parse: %v", err)
			}
			files, err := New().Emit(s)
			if err != nil {
				t.Fatalf("Emit: %v", err)
			}
			typecheck(t, swiftc, files)
		})
	}
}

// TestTemplatedSwiftTypechecks carries uses through swiftc, the preview
// driver's output as well as the app's, including templates perch does not
// ship.
func TestTemplatedSwiftTypechecks(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	for name, c := range map[string]struct {
		doc  string
		tmpl fakeTemplates // nil for the shipped templates
		app  bool          // false when TestEmittedSwiftTypechecks already covers the app
	}{
		"templates":  {doc: templatesDoc},
		"keywordUse": {doc: keywordUse},
		"kwTemplate": {doc: kwTemplateDoc, tmpl: kwTemplates, app: true},
		"usesOnly":   {doc: usesOnlyDoc, tmpl: usesOnlyTemplates, app: true},
	} {
		t.Run(name, func(t *testing.T) {
			var s *spec.Spec
			var err error
			if c.tmpl == nil {
				s, err = spec.Parse([]byte(c.doc))
			} else {
				s, err = spec.ParseWith([]byte(c.doc), c.tmpl)
			}
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			emits := map[string]func(*spec.Spec) ([]backend.File, error){"preview": New().EmitPreview}
			if c.app {
				emits["app"] = New().Emit
			}
			for kind, emit := range emits {
				t.Run(kind, func(t *testing.T) {
					files, err := emit(s)
					if err != nil {
						t.Fatalf("emit: %v", err)
					}
					typecheck(t, swiftc, files)
				})
			}
		})
	}
}

// typecheck fails t unless swiftc takes files with no error and no warning.
func typecheck(t *testing.T, swiftc string, files []backend.File) {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for _, f := range files {
		p := filepath.Join(dir, f.Name)
		if err := os.WriteFile(p, f.Body, 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	out, err := exec.Command(swiftc, append([]string{"-typecheck"}, paths...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("emitted Swift does not typecheck: %v\n%s", err, out)
	}
	if len(out) > 0 {
		t.Errorf("emitted Swift typechecks with warnings:\n%s", out)
	}
}
