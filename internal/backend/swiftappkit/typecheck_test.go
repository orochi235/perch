package swiftappkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/orochi235/perch/internal/spec"
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
// an exists watch, an untyped .data, open and post, and a when: guard.
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
    when: "!server.ok"
  - separator
  - {text: Open, open: "http://127.0.0.1:8765"}
  - {text: Restart, run: [launchctl, kickstart, -k, gui/501/dev.brainhouse], when: "agent.ok"}
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

// TestEmittedSwiftTypechecks is the check a golden test alone cannot make: a
// golden stays green while emitting Swift that does not compile.
func TestEmittedSwiftTypechecks(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	for name, doc := range map[string]string{
		"minimal":        minimal,
		"designDoc":      designDocExample,
		"everyFeature":   everyFeature,
		"collidingNames": collidingNames,
		"watchInAction":  watchInAction,
		"swiftKeywords":  swiftKeywordShape,
		"states":         states,
		"launchAgent":    launchAgent,
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
		})
	}
}
