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
	for i, want := range []struct{ text, when string }{
		{"Start", "worker.installed && !worker.loaded"},
		{"Stop", "worker.loaded"},
		{"Bounce", "worker.installed"},
	} {
		if got := s.Menu[i]; got.Text != want.text || got.When != want.when {
			t.Errorf("menu[%d] = %q when %q, want %q when %q", i, got.Text, got.When, want.text, want.when)
		}
	}
	sub := s.Menu[3].Menu[0]
	if sub.Text != "Stop" || sub.When != "(worker.running) && (worker.loaded)" {
		t.Errorf("submenu item = %q when %q; an author's when: combines with the verb's", sub.Text, sub.When)
	}
}

func TestATemplatesAgentItemIsGuardedThroughSelf(t *testing.T) {
	s := parseWith(t, useHead+"use:\n  daemon:\n    ctl: {label: dev.example.daemon}\nmenu: [outlet]\n", ctl)
	if got := s.Menu[0].When; got != "self.agent.installed && !self.agent.loaded" {
		t.Errorf("when = %q", got)
	}
}
