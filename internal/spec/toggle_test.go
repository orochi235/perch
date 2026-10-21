package spec

import (
	"strings"
	"testing"
)

func TestAToggleReadsItsTargetAndGetsALabelAndAGuard(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - toggle: worker.enabled
  - toggle: "!worker.enabled"
  - {text: Boot with the Mac, toggle: worker.enabled, when: worker.running}
`)
	for i, want := range []struct {
		text    string
		negated bool
		when    string
	}{
		{"Start at Login", false, ""},
		{"Don't Start at Login", true, ""},
		{"Boot with the Mac", false, "worker.running"},
	} {
		got := s.Menu[i]
		if got.Text != want.text || got.Toggle != (Toggle{"worker", "enabled", want.negated}) ||
			got.Guard != "worker.installed" || got.When != want.when {
			t.Errorf("menu[%d] = %q %+v guard %q when %q", i, got.Text, got.Toggle, got.Guard, got.When)
		}
	}
}

func TestAToggleRefusesWhatAClickCannotFlip(t *testing.T) {
	for _, tc := range []struct{ item, want string }{
		{`{toggle: "worker.enabled && worker.running"}`, "is not something a click can flip"},
		{`{toggle: "worker.enabled == true"}`, "is not something a click can flip"},
		{`{toggle: worker}`, "is not something a click can flip"},
		{`{toggle: worker.running}`, `"running" is not something perch can flip`},
		{`{toggle: worker.enabled, run: [true]}`, "takes no run action"},
		{`{toggle: worker.enabled, agent: worker.stop}`, "takes no agent action"},
		{`{toggle: worker.enabled, menu: [{text: x}]}`, "takes no submenu"},
		{`{toggle: nope.enabled}`, "toggle:"},
		{`{toggle: ticker.enabled}`, "toggle:"},
	} {
		got := parseErr(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
  ticker: {run: [date]}
menu:
  - `+tc.item+"\n")
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: error = %q, want it to contain %q", tc.item, got, tc.want)
		}
	}
}

func TestATemplatesToggleIsGuardedThroughSelf(t *testing.T) {
	src := fakeTemplates{"ctl": "params:\n  label: ~\nwatch:\n  agent:\n    launchagent: ${label}\nmenu:\n  default:\n    - {text: At Login, toggle: self.agent.enabled}\n"}
	s := parseWith(t, useHead+"use:\n  daemon:\n    ctl: {label: dev.example.daemon}\nmenu: [outlet]\n", src)
	if got := s.Menu[0]; got.Guard != "self.agent.installed" || got.Toggle.Target != "self.agent" {
		t.Errorf("guard = %q toggle %+v", got.Guard, got.Toggle)
	}
}

func TestEnableAndDisableVerbsAreGuardedByTheCurrentState(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - agent: worker.enable
  - agent: worker.disable
`)
	if got := s.Menu[0]; got.Text != "Start at Login" || got.Guard != "worker.installed && !worker.enabled" {
		t.Errorf("enable = %q guard %q", got.Text, got.Guard)
	}
	if got := s.Menu[1]; got.Text != "Don't Start at Login" || got.Guard != "worker.installed && worker.enabled" {
		t.Errorf("disable = %q guard %q", got.Text, got.Guard)
	}
}
