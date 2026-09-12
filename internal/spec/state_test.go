package spec

import (
	"strings"
	"testing"
)

const stateHead = `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
watch:
  agent: {run: [launchctl, print, gui/501/dev.example.w]}
`

func TestStatesParse(t *testing.T) {
	s, err := Parse([]byte(stateHead + `
state:
  - stopped: "!agent.ok"
  - running:
menu: [{text: Quit, quit: true}]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.States) != 2 {
		t.Fatalf("States = %v, want 2", s.States)
	}
	if s.States[0].Name != "stopped" || s.States[0].Cond != "!agent.ok" {
		t.Errorf("States[0] = %+v", s.States[0])
	}
	if s.States[1].Name != "running" || s.States[1].Cond != "" {
		t.Errorf("States[1] = %+v, want the fallback to carry no condition", s.States[1])
	}
}

func TestStatesRefused(t *testing.T) {
	for name, tc := range map[string]struct{ doc, want string }{
		"the last state carries a condition": {
			doc:  "state:\n  - a: \"agent.ok\"\n",
			want: "fallback",
		},
		"a fallback is not last": {
			doc:  "state:\n  - a:\n  - b: \"agent.ok\"\n",
			want: "must be last",
		},
		"a name is declared twice": {
			doc:  "state:\n  - a: \"agent.ok\"\n  - a: \"agent.ok\"\n  - b:\n",
			want: "declared twice",
		},
		"a name is already a watch": {
			doc:  "state:\n  - agent: \"true\"\n  - b:\n",
			want: "already a watch",
		},
		"a name is it": {
			doc:  "state:\n  - it: \"agent.ok\"\n  - b:\n",
			want: "cannot take that name",
		},
		"a name is not an identifier": {
			doc:  "state:\n  - not-a-name: \"agent.ok\"\n  - b:\n",
			want: "not a usable state name",
		},
		"one entry names two states": {
			doc:  "state:\n  - {a: \"agent.ok\", b: \"agent.ok\"}\n  - c:\n",
			want: "each entry takes exactly one",
		},
		"the block is not a list": {
			doc:  "state:\n  a: \"agent.ok\"\n",
			want: "want a list of states",
		},
		"a condition is not a string": {
			doc:  "state:\n  - a: [1, 2]\n  - b:\n",
			want: "want a condition as a string",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := parseErr(t, stateHead+tc.doc+"menu: [{text: Quit, quit: true}]\n")
			if !strings.Contains(got, tc.want) {
				t.Errorf("error = %q, want it to mention %q", got, tc.want)
			}
		})
	}
}

// A single state is legal so long as it is the fallback, which is the degenerate
// machine: one state, always held.
func TestOneStateIsTheFallback(t *testing.T) {
	if _, err := Parse([]byte(stateHead + `
state:
  - always:
menu: [{text: Quit, quit: true}]
`)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
}
