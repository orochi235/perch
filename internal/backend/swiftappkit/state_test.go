package swiftappkit

import (
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

func emitErr(t *testing.T, doc string) string {
	t.Helper()
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	if _, err := New().Emit(s); err != nil {
		return err.Error()
	}
	t.Fatal("want an error from Emit, got nil")
	return ""
}

const stateRefHead = `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
watch:
  agent: {run: [launchctl, print, gui/501/dev.example.w]}
`

// A state's condition sees the watches and nothing else. The ordering has
// already excluded every earlier state, so naming one could only ever be a
// constant; naming a later one has no meaning at all. Both arrive as the same
// refusal, which is the point of declaring them one at a time.
func TestAStateConditionCannotNameAnotherState(t *testing.T) {
	for name, doc := range map[string]string{
		"an earlier state": stateRefHead + `
state:
  - stopped: "!agent.ok"
  - odd: "stopped"
  - running:
menu: [{text: Quit, quit: true}]
`,
		"a later state": stateRefHead + `
state:
  - odd: "running"
  - running:
menu: [{text: Quit, quit: true}]
`,
	} {
		t.Run(name, func(t *testing.T) {
			got := emitErr(t, doc)
			if !strings.Contains(got, "unknown name") {
				t.Errorf("error = %q, want it to refuse the name", got)
			}
		})
	}
}

// The states are properties of Results rather than locals in the two render
// functions, so a state only one of them reads is simply unread. As locals,
// the other function would not compile without a warning, and the typecheck
// test treats a warning as a failure.
func TestStatesAreEmittedAsPropertiesOfResults(t *testing.T) {
	files := emit(t, states)
	render := files["Render.swift"]
	if !strings.Contains(render, "extension Results {") {
		t.Fatalf("Render.swift does not extend Results:\n%s", render)
	}
	for _, want := range []string{
		"var state_uninstalled: Bool",
		"var state_stopped: Bool { !state_uninstalled &&",
		"var state_running: Bool { !state_uninstalled && !state_stopped }",
	} {
		if !strings.Contains(render, want) {
			t.Errorf("Render.swift is missing %q", want)
		}
	}
}

// A condition is wrapped before the leading negations are prepended. Without
// that, a state whose condition is a disjunction comes apart under them and
// still compiles, which is the whole failure this schema avoids. The lowering
// happens to bracket its own output too; this does not lean on that.
func TestAStateConditionIsParenthesized(t *testing.T) {
	files := emit(t, stateRefHead+`
state:
  - first: "agent.ok"
  - loose: "agent.code == 1 || agent.code == 2"
  - rest:
menu: [{text: Quit, quit: true}]
`)
	want := "var state_loose: Bool { !state_first && (((agent.code == 1) || (agent.code == 2))) }"
	if got := files["Render.swift"]; !strings.Contains(got, want) {
		t.Errorf("Render.swift is missing %q", want)
	}
}

// A launchagent watch is the only one with no .ok, because the two answers it
// could mean differ — which is the whole reason the kind exists. Listing the
// fields it does have would leave an author to guess which of loaded and
// running they meant.
func TestALaunchAgentWatchHasNoOkAndSaysWhy(t *testing.T) {
	got := emitErr(t, `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
watch:
  worker: {launchagent: dev.example.worker}
menu: [{text: Up, when: "worker.ok"}]
`)
	for _, want := range []string{`has no field "ok"`, "loaded", "running"} {
		if !strings.Contains(got, want) {
			t.Errorf("error = %q, want it to mention %q", got, want)
		}
	}
}
