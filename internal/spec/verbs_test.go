package spec

import "testing"

func TestAnAgentItemGetsALabelAndAGuard(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - agent: worker.start
  - agent: worker.stop
  - {text: Bounce, agent: worker.restart}
  - {text: More, menu: [{agent: worker.stop, when: "worker.running"}]}
`)
	for i, want := range []struct{ text, guard string }{
		{"Start", "worker.installed && !worker.loaded"},
		{"Stop", "worker.loaded"},
		{"Bounce", "worker.installed"},
	} {
		if got := s.Menu[i]; got.Text != want.text || got.Guard != want.guard || got.When != "" {
			t.Errorf("menu[%d] = %q guard %q when %q, want %q guard %q and no when", i, got.Text, got.Guard, got.When, want.text, want.guard)
		}
	}
	sub := s.Menu[3].Menu[0]
	if sub.Text != "Stop" || sub.When != "worker.running" || sub.Guard != "worker.loaded" {
		t.Errorf("submenu item = %q when %q guard %q; an author's when: is kept verbatim beside the verb's guard", sub.Text, sub.When, sub.Guard)
	}
}

func TestARestartItemGetsALabel(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - agent: worker.restart
`)
	if got := s.Menu[0]; got.Text != "Restart" || got.Guard != "worker.installed" || got.When != "" {
		t.Errorf("menu[0] = %q guard %q when %q, want Restart guarded by worker.installed with no when", got.Text, got.Guard, got.When)
	}
}

func TestAnAuthorsCommentDoesNotSwallowTheGuard(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - {agent: worker.stop, when: "worker.running // up"}
`)
	got := s.Menu[0]
	if got.When != "worker.running // up" || got.Guard != "worker.loaded" {
		t.Errorf("when = %q guard = %q, want the author's when: kept verbatim and the guard set separately", got.When, got.Guard)
	}
}

func TestATemplatesAgentItemIsGuardedThroughSelf(t *testing.T) {
	s := parseWith(t, useHead+"use:\n  daemon:\n    ctl: {label: dev.example.daemon}\nmenu: [outlet]\n", ctl)
	if got := s.Menu[0]; got.Guard != "self.agent.installed && !self.agent.loaded" || got.When != "" {
		t.Errorf("guard = %q when %q", got.Guard, got.When)
	}
}

func TestATemplatesAgentItemKeepsAnAuthorsWhenBesideSelfsGuard(t *testing.T) {
	src := fakeTemplates{"ctl": "params:\n  label: ~\nwatch:\n  agent:\n    launchagent: ${label}\n  other: {exists: /tmp}\nmenu:\n  default:\n    - {text: Start, agent: self.agent.start, when: self.other.ok}\n"}
	s := parseWith(t, useHead+"use:\n  daemon:\n    ctl: {label: dev.example.daemon}\nmenu: [outlet]\n", src)
	got := s.Menu[0]
	if got.When != "self.other.ok" || got.Guard != "self.agent.installed && !self.agent.loaded" {
		t.Errorf("when = %q guard = %q", got.When, got.Guard)
	}
}

func TestNonAgentItemsAreUntouched(t *testing.T) {
	s := parseDoc(t, `
app:
  name: w
  id: dev.example.w
  icon: circle
  interval: 10s
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - {text: Refresh, run: [ls]}
`)
	if got := s.Menu[0]; got.Text != "Refresh" || got.When != "" || got.Guard != "" {
		t.Errorf("non-agent item = %q when %q guard %q, want unchanged", got.Text, got.When, got.Guard)
	}
}
