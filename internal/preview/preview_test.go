package preview

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

// jobs is the docs' own example: a run watch with a declared shape, rules that
// branch on it, and an each: with a submenu.
const jobs = `
app: {name: onto, id: dev.onto.menubar, icon: rectangle.3.group, interval: 5s}
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
      - {text: Logs, run: [onto, logs, "{{it.id}}"]}
  - separator
  - {text: Quit, quit: true}
`

// net covers the kinds jobs does not: an http watch, an exists watch, artwork
// for an icon, and the open and post actions.
const net = `
app: {name: net, id: dev.net.menubar, icon: circle, interval: 10s}
watch:
  server:
    http: http://127.0.0.1:8765/health
    json: true
  plist:
    exists: ~/Library/LaunchAgents/dev.net.plist
status:
  - when: "!plist.ok"
    icon: circle.dashed
  - when: "!server.ok"
    icon: {asset: alarm}
    dim: true
  - badge: "string(server.data.sessions.size())"
menu:
  - {text: Open, open: "http://127.0.0.1:8765"}
  - {text: Ping, post: {url: "http://127.0.0.1:8765/ping", body: {source: menubar}}}
  - {text: Quit, quit: true}
`

func parse(t *testing.T, doc string) *spec.Spec {
	t.Helper()
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	return s
}

func state(t *testing.T, s *spec.Spec, name, src string) State {
	t.Helper()
	st, err := ParseState(name, []byte(src), s)
	if err != nil {
		t.Fatalf("ParseState(%s): %v", name, err)
	}
	return st
}

func render(t *testing.T, s *spec.Spec, states []State) []Frame {
	t.Helper()
	if _, err := exec.LookPath("swiftc"); err != nil {
		t.Skip("swiftc not on PATH")
	}
	frames, err := Renderer{}.Render(s, states)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(frames) != len(states) {
		t.Fatalf("got %d frames for %d states", len(frames), len(states))
	}
	return frames
}

// A state naming something the file does not declare is the mistake worth
// catching: it renders a menu that silently ignores what the page stated.
func TestParseStateRefusesAWatchTheFileDoesNotDeclare(t *testing.T) {
	_, err := ParseState("healthy", []byte("fleat: {out: '{}'}\n"), parse(t, jobs))
	if err == nil {
		t.Fatal("a state naming no declared watch was accepted")
	}
	if got := err.Error(); !strings.Contains(got, "fleat") || !strings.Contains(got, "fleet") {
		t.Errorf("the error names neither the typo nor what is declared: %s", got)
	}
}

func TestParseStateRefusesAKeyTheKindDoesNotBind(t *testing.T) {
	_, err := ParseState("healthy", []byte("fleet: {status: 503}\n"), parse(t, jobs))
	if err == nil {
		t.Fatal("status was accepted on a run watch")
	}
}

func TestParseStateRefusesAMappingForAnExistsWatch(t *testing.T) {
	_, err := ParseState("gone", []byte("plist: {code: 1}\n"), parse(t, net))
	if err == nil {
		t.Fatal("a mapping was accepted for an exists watch")
	}
}

// out: is what the command printed, and a JSON-printing command is far easier
// to state as the document itself than as a quoted string.
func TestParseStateWritesStructuredOutAsJSON(t *testing.T) {
	st := state(t, parse(t, jobs), "healthy", "fleet:\n  out:\n    jobs: [{id: j1}]\n")
	got := st.Watches["fleet"].Out
	if got == nil {
		t.Fatal("no out recorded")
	}
	if *got != `{"jobs":[{"id":"j1"}]}` {
		t.Errorf("out = %s", *got)
	}
}

func TestRenderShowsWhatTheStatusItemAndMenuSay(t *testing.T) {
	s := parse(t, jobs)
	out := `{"nodes": [{"name": "studio", "up": true}], "jobs": [{"id": "j1", "node": "studio", "cmd": "render"}, {"id": "j2", "node": "studio", "cmd": "encode"}]}`
	frames := render(t, s, []State{
		state(t, s, "busy", "fleet:\n  out: '"+out+"'\n"),
		state(t, s, "idle", "fleet:\n  out: '{\"nodes\": [], \"jobs\": []}'\n"),
		state(t, s, "unreachable", "fleet:\n  code: 3\n  err: no such command\n"),
		state(t, s, "silent", ""),
	})

	busy := frames[0]
	if busy.Face.Icon.Symbol != "rectangle.3.group" || busy.Face.Badge != "2" || busy.Face.Dim {
		t.Errorf("busy shows %+v", busy.Face)
	}
	if busy.Menu[0].Title != "1 nodes · 2 running" {
		t.Errorf("busy's first item says %q", busy.Menu[0].Title)
	}
	if got := busy.Menu[2].Title; got != "studio — render" {
		t.Errorf("busy's first job says %q", got)
	}
	logs := busy.Menu[2].Items[0]
	if logs.Action == nil || len(logs.Action.Run) != 3 || logs.Action.Run[2] != "j1" {
		t.Errorf("Logs runs %+v", logs.Action)
	}
	if last := busy.Menu[len(busy.Menu)-1]; last.Action == nil || !last.Action.Quit {
		t.Errorf("the last item is not Quit: %+v", last)
	}

	idle := frames[1]
	if !idle.Face.Dim || idle.Face.Badge != "" {
		t.Errorf("an idle fleet shows %+v", idle.Face)
	}

	// A watch that failed and a watch that never answered read the same, and
	// the menu still opens on whatever the zero value says.
	for _, f := range frames[2:] {
		if f.Face.Icon.Symbol != "exclamationmark.triangle" {
			t.Errorf("%s shows %+v", f.State, f.Face)
		}
		if f.Menu[0].Title != "0 nodes · 0 running" {
			t.Errorf("%s's first item says %q", f.State, f.Menu[0].Title)
		}
	}
}

func TestRenderCoversTheOtherWatchKindsAndActions(t *testing.T) {
	s := parse(t, net)
	frames := render(t, s, []State{
		state(t, s, "up", "server:\n  out:\n    sessions: [{id: a}, {id: b}]\nplist: true\n"),
		state(t, s, "down", "server:\n  status: 503\nplist: true\n"),
		state(t, s, "uninstalled", "plist: false\n"),
	})

	up := frames[0]
	if up.Face.Badge != "2" || up.Face.Icon.Symbol != "circle" {
		t.Errorf("up shows %+v", up.Face)
	}
	if got := up.Menu[0].Action; got == nil || got.Open != "http://127.0.0.1:8765" {
		t.Errorf("Open opens %+v", got)
	}
	ping := up.Menu[1].Action
	if ping == nil || ping.Post == nil || ping.Post.Body != `{"source":"menubar"}` {
		t.Errorf("Ping posts %+v", ping)
	}

	if down := frames[1].Face; down.Icon.Asset != "alarm" || !down.Dim {
		t.Errorf("a 503 shows %+v", down)
	}
	if gone := frames[2].Face; gone.Icon.Symbol != "circle.dashed" {
		t.Errorf("a missing plist shows %+v", gone)
	}
}
