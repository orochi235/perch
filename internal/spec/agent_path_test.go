package spec

import (
	"strings"
	"testing"
)

const ctlTemplate = `
params:
  label: ~
watch:
  agent:
    launchagent: ${label}
  other: {exists: /tmp}
menu:
  default:
    - {text: Start, agent: self.agent.start}
`

var ctl = fakeTemplates{"ctl": ctlTemplate}

func TestAnAgentPathReachesAUsesWatch(t *testing.T) {
	s := parseWith(t, useHead+`
use:
  daemon:
    ctl: {label: dev.example.daemon}
menu:
  - outlet
  - {text: Stop, agent: daemon.agent.stop}
`, ctl)
	if got := s.Menu[0].Action; got.Agent != "self.agent" || got.Verb != AgentStart {
		t.Errorf("template item = %+v, want self.agent start", got)
	}
	if got := s.Menu[1].Action; got.Agent != "daemon.agent" || got.Verb != AgentStop {
		t.Errorf("file item = %+v, want daemon.agent stop", got)
	}
	w, err := s.AgentWatch("daemon", "self.agent")
	if err != nil || w.Label != "dev.example.daemon" {
		t.Errorf("AgentWatch(daemon, self.agent) = %+v, %v", w, err)
	}
	if w, err := s.AgentWatch("", "daemon.agent"); err != nil || w.Label != "dev.example.daemon" {
		t.Errorf("AgentWatch(\"\", daemon.agent) = %+v, %v", w, err)
	}
}

func TestAQuitButtonTakesAUsePath(t *testing.T) {
	s := parseWith(t, `
app:
  name: w
  id: dev.example.w
  icon: circle
  interval: 10s
  quit:
    - confirm: Quit?
      buttons: [{text: Stop too, agent: daemon.agent.stop}]
use:
  daemon:
    ctl: {label: dev.example.daemon}
menu: [outlet]
`, ctl)
	if got := s.App.Quit[0].Buttons[0].Action.Agent; got != "daemon.agent" {
		t.Errorf("button agent = %q", got)
	}
}

func TestAgentPathRefusals(t *testing.T) {
	src := fakeTemplates{
		"ctl":  ctlTemplate,
		"bare": "watch:\n  agent: {launchagent: dev.example.x}\nmenu:\n  default:\n    - {text: Go, agent: agent.start}\n",
		"peek": "watch:\n  agent: {launchagent: dev.example.x}\nmenu:\n  default:\n    - {text: Go, agent: daemon.agent.start}\n",
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"self outside a template": {
			"watch:\n  agent: {launchagent: dev.example.x}\nmenu:\n  - {text: Go, agent: self.agent.start}\n",
			"self is bound only inside a template",
		},
		"a bare watch name inside a template": {
			"use:\n  d:\n    bare:\nmenu: [outlet]\n",
			"a template sees only self",
		},
		"another use's name inside a template": {
			"use:\n  daemon:\n    peek:\nmenu: [outlet]\n",
			"a template sees only self",
		},
		"a use that is not there": {
			"menu:\n  - {text: Go, agent: nope.agent.start}\n",
			`no use named "nope"`,
		},
		"a use's watch that is not a launchagent": {
			"use:\n  d:\n    ctl: {label: dev.example.d}\nmenu:\n  - outlet\n  - {text: Go, agent: d.other.start}\n",
			"is a exists watch",
		},
		"too many segments": {
			"menu:\n  - {text: Go, agent: a.b.c.start}\n",
			"is not <watch>, <use>.<watch> or self.<watch>",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseWith([]byte(useHead+tc.doc), src)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
