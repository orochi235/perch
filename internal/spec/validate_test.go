package spec

import (
	"strings"
	"testing"
)

func parseErr(t *testing.T, doc string) string {
	t.Helper()
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("want an error, got nil")
	}
	return err.Error()
}

func TestValidateRequiresAppName(t *testing.T) {
	got := parseErr(t, `
app: {id: b, icon: circle, interval: 1s}
menu: [{text: Quit, quit: true}]
`)
	if !strings.Contains(got, "app.name") {
		t.Errorf("error = %q, want it to name app.name", got)
	}
}

func TestValidateRequiresAppID(t *testing.T) {
	got := parseErr(t, `
app: {name: a, icon: circle, interval: 1s}
menu: [{text: Quit, quit: true}]
`)
	if !strings.Contains(got, "app.id") {
		t.Errorf("error = %q, want it to name app.id", got)
	}
}

func TestValidateRequiresPositiveInterval(t *testing.T) {
	got := parseErr(t, `
app: {name: a, id: b, icon: circle, interval: 0s}
menu: [{text: Quit, quit: true}]
`)
	if !strings.Contains(got, "app.interval") {
		t.Errorf("error = %q, want it to name app.interval", got)
	}
}

func TestValidateRejectsUnknownTopLevelKey(t *testing.T) {
	got := parseErr(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
menus: []
menu: [{text: Quit, quit: true}]
`)
	if !strings.Contains(got, "menus") {
		t.Errorf("error = %q, want it to quote the unknown key", got)
	}
}

func TestValidateRejectsShapeWithoutJSON(t *testing.T) {
	got := parseErr(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {run: [x], shape: {n: int}}}
menu: [{text: Quit, quit: true}]
`)
	if !strings.Contains(got, "shape") {
		t.Errorf("error = %q, want it to explain shape needs json", got)
	}
}

func TestValidateRejectsJSONOnExistsWatch(t *testing.T) {
	got := parseErr(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {w: {exists: /tmp/x, json: true}}
menu: [{text: Quit, quit: true}]
`)
	if !strings.Contains(got, "json") {
		t.Errorf("error = %q, want it to say an exists watch has no output", got)
	}
}

func TestValidateRejectsDuplicateWatchNames(t *testing.T) {
	got := parseErr(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
watch:
  w: {run: [x]}
  w: {run: [y]}
menu: [{text: Quit, quit: true}]
`)
	if !strings.Contains(got, "watch.w") || !strings.Contains(got, "twice") {
		t.Errorf("error = %q, want it to name watch.w as declared twice", got)
	}
}

func TestValidateAcceptsTheDesignDocExample(t *testing.T) {
	_, err := Parse([]byte(`
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
      jobs: [{id: string, node: string, cmd: string}]
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
`))
	if err != nil {
		t.Fatalf("the design doc's own example must parse: %v", err)
	}
}

func TestValidateRejectsUnguardedStatusRuleBeforeOthers(t *testing.T) {
	got := parseErr(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
status:
  - {badge: "1"}
  - {when: "true", icon: circle}
menu: [{text: Quit, quit: true}]
`)
	if !strings.Contains(got, "last") {
		t.Errorf("error = %q, want it to say an unguarded rule must be last", got)
	}
}

func TestValidateRejectsActionOnAnItemWithASubmenu(t *testing.T) {
	got := parseErr(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
menu:
  - text: More
    run: [ls]
    menu: [{text: Quit, quit: true}]
`)
	if !strings.Contains(got, "submenu") {
		t.Errorf("error = %q, want it to explain a submenu supersedes an action", got)
	}
}
