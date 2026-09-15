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
	if sub.Text != "Stop" || sub.When != "(worker.running\n) && (worker.loaded)" {
		t.Errorf("submenu item = %q when %q; an author's when: combines with the verb's", sub.Text, sub.When)
	}
}

func TestARestartItemGetsALabel(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - agent: worker.restart
`)
	if got := s.Menu[0]; got.Text != "Restart" || got.When != "worker.installed" {
		t.Errorf("menu[0] = %q when %q, want Restart when worker.installed", got.Text, got.When)
	}
}

func TestAnAuthorsCommentDoesNotSwallowTheGuard(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - {agent: worker.stop, when: "worker.running // up"}
`)
	want := "(worker.running // up\n) && (worker.loaded)"
	if got := s.Menu[0].When; got != want {
		t.Errorf("when = %q, want %q", got, want)
	}
}

func TestATemplatesAgentItemIsGuardedThroughSelf(t *testing.T) {
	s := parseWith(t, useHead+"use:\n  daemon:\n    ctl: {label: dev.example.daemon}\nmenu: [outlet]\n", ctl)
	if got := s.Menu[0].When; got != "self.agent.installed && !self.agent.loaded" {
		t.Errorf("when = %q", got)
	}
}

func TestATemplatesAgentItemCombinesAnAuthorsWhenWithSelf(t *testing.T) {
	src := fakeTemplates{"ctl": "params:\n  label: ~\nwatch:\n  agent:\n    launchagent: ${label}\n  other: {exists: /tmp}\nmenu:\n  default:\n    - {text: Start, agent: self.agent.start, when: self.other.ok}\n"}
	s := parseWith(t, useHead+"use:\n  daemon:\n    ctl: {label: dev.example.daemon}\nmenu: [outlet]\n", src)
	want := "(self.other.ok\n) && (self.agent.installed && !self.agent.loaded)"
	if got := s.Menu[0].When; got != want {
		t.Errorf("when = %q, want %q", got, want)
	}
}

func TestNonAgentItemsAndQuitButtonsAreUntouched(t *testing.T) {
	s := parseDoc(t, `
app:
  name: w
  id: dev.example.w
  icon: circle
  interval: 10s
  quit:
    - confirm: "Quit?"
      buttons: [{text: Quit}]
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - {text: Refresh, run: [ls]}
  - {agent: worker.stop, when: "worker.running"}
`)
	if got := s.Menu[0]; got.Text != "Refresh" || got.When != "" {
		t.Errorf("non-agent item = %q when %q, want unchanged", got.Text, got.When)
	}
	if len(s.App.Quit) == 0 || len(s.App.Quit[0].Buttons) == 0 {
		t.Fatal("no quit button parsed")
	}
	if b := s.App.Quit[0].Buttons[0]; b.Text != "Quit" {
		t.Errorf("quit button text = %q, want unchanged Quit", b.Text)
	}
}
