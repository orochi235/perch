package spec

import (
	"strings"
	"testing"
)

const agentHead = `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
`

func parseDoc(t *testing.T, doc string) *Spec {
	t.Helper()
	s, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}

// The plist path is the one thing about a LaunchAgent that follows from its
// label, so writing it out is a chance to write it differently.
func TestALaunchAgentWatchDefaultsItsPlistFromTheLabel(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu: [{text: Quit, quit: true}]
`)
	w := s.Watches[0]
	if w.Kind != WatchLaunchAgent {
		t.Fatalf("kind = %v, want launchagent", w.Kind)
	}
	if want := "~/Library/LaunchAgents/dev.example.worker.plist"; w.Plist != want {
		t.Errorf("plist = %q, want %q", w.Plist, want)
	}
}

func TestAnExplicitPlistWins(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker, plist: ~/opt/worker/agent.plist}
menu: [{text: Quit, quit: true}]
`)
	if want := "~/opt/worker/agent.plist"; s.Watches[0].Plist != want {
		t.Errorf("plist = %q, want %q", s.Watches[0].Plist, want)
	}
}

func TestAnAgentActionNamesAWatchAndAVerb(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - {text: Start, agent: worker.start}
  - {text: Stop, agent: worker.stop}
  - {text: Restart, agent: worker.restart}
`)
	for i, want := range []AgentVerb{AgentStart, AgentStop, AgentRestart} {
		got := s.Menu[i].Action
		if got.Kind != ActionAgent || got.Agent != "worker" || got.Verb != want {
			t.Errorf("menu[%d] = %+v, want agent worker.%s", i, got, want)
		}
	}
}

func TestLaunchAgentRefusals(t *testing.T) {
	for name, tc := range map[string]struct{ doc, want string }{
		"json on a launchagent watch": {agentHead + `
watch:
  worker: {launchagent: dev.example.worker, json: true}
menu: [{text: Quit, quit: true}]
`, "json has no meaning"},

		"a plist with no label": {agentHead + `
watch:
  worker: {plist: ~/Library/LaunchAgents/x.plist}
menu: [{text: Quit, quit: true}]
`, "needs launchagent:"},

		"a label that launchd would not take": {agentHead + `
watch:
  worker: {launchagent: "dev example/worker"}
menu: [{text: Quit, quit: true}]
`, "is not a launchd label"},

		"a launchagent alongside another kind": {agentHead + `
watch:
  worker: {launchagent: dev.example.worker, run: [echo, x]}
menu: [{text: Quit, quit: true}]
`, "exactly one of run, http, exists or launchagent"},

		"an agent action with no verb": {agentHead + `
watch:
  worker: {launchagent: dev.example.worker}
menu: [{text: Go, agent: worker}]
`, "names no verb"},

		"an agent action with a verb perch does not do": {agentHead + `
watch:
  worker: {launchagent: dev.example.worker}
menu: [{text: Go, agent: worker.reload}]
`, "not something perch can do"},

		"an agent action on a run watch": {agentHead + `
watch:
  worker: {run: [launchctl, print, x]}
menu: [{text: Go, agent: worker.start}]
`, "is a run watch"},

		"an agent action naming nothing": {agentHead + `
watch:
  worker: {launchagent: dev.example.worker}
menu: [{text: Go, agent: wroker.start}]
`, "the launchagent watches are worker"},

		"an agent action where no agent is declared": {agentHead + `
menu: [{text: Go, agent: worker.start}]
`, "declares no launchagent watch"},

		"an agent action beside another verb": {agentHead + `
watch:
  worker: {launchagent: dev.example.worker}
menu: [{text: Go, agent: worker.start, quit: true}]
`, "at most one of run, open, post, quit or agent"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.doc))
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
