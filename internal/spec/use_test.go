package spec

import (
	"slices"
	"strings"
	"testing"
)

// fakeTemplates stands in for a repo's menubar/templates/ and perch's own.
type fakeTemplates map[string]string

func (f fakeTemplates) Template(name string) ([]byte, string, bool, error) {
	src, ok := f[name]
	return []byte(src), "templates/" + name + ".yaml", ok, nil
}

func (f fakeTemplates) Names() []string {
	var out []string
	for name := range f {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

const useHead = `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
`

const agentTemplate = `
params:
  label: ~
  noun: Service
watch:
  agent:
    launchagent: ${label}
state:
  - stopped: "!self.agent.loaded"
  - running:
`

func parseWith(t *testing.T, doc string, src fakeTemplates) *Spec {
	t.Helper()
	s, err := ParseWith([]byte(doc), src)
	if err != nil {
		t.Fatalf("ParseWith: %v", err)
	}
	return s
}

func TestAUseGetsItsOwnWatchesAndStates(t *testing.T) {
	s := parseWith(t, useHead+`
use:
  daemon:
    svc: {label: dev.example.daemon}
menu: [{text: Quit, quit: true}]
`, fakeTemplates{"svc": agentTemplate})
	if len(s.Uses) != 1 {
		t.Fatalf("Uses = %+v, want one", s.Uses)
	}
	u := s.Uses[0]
	if u.Name != "daemon" || u.Template != "svc" || u.File != "templates/svc.yaml" {
		t.Errorf("use = %+v", u)
	}
	if len(u.Watches) != 1 || u.Watches[0].Label != "dev.example.daemon" || u.Watches[0].Scope != "daemon" {
		t.Errorf("watches = %+v, want agent on dev.example.daemon scoped to daemon", u.Watches)
	}
	if len(u.States) != 2 || u.States[0].Cond != "!self.agent.loaded" {
		t.Errorf("states = %+v", u.States)
	}
	if len(s.Watches) != 0 {
		t.Errorf("a use's watches leaked into the file's: %+v", s.Watches)
	}
}

func TestParametersFillEveryStringInATemplate(t *testing.T) {
	src := fakeTemplates{"echo": `
params:
  msg: ~
  decode: "false"
watch:
  w:
    run: [echo, "${msg}", "cost $$5"]
    json: ${decode}
`}
	s := parseWith(t, useHead+`
use:
  e:
    echo: {msg: "a: b", decode: "true"}
menu: [{text: Quit, quit: true}]
`, src)
	w := s.Uses[0].Watches[0]
	if !slices.Equal(w.Run, []string{"echo", "a: b", "cost $5"}) {
		t.Errorf("run = %q; a value holding \": \" has to stay one string, and $$ is a literal $", w.Run)
	}
	if !w.JSON {
		t.Error("json: ${decode} did not read back as a bool")
	}
}

func TestADefaultParameterIsUsedWhenAUsePassesNone(t *testing.T) {
	src := fakeTemplates{"echo": "params:\n  msg: hello\nwatch:\n  w: {run: [echo, \"${msg}\"]}\n"}
	s := parseWith(t, useHead+"use:\n  e:\n    echo:\nmenu: [{text: Quit, quit: true}]\n", src)
	if got := s.Uses[0].Watches[0].Run[1]; got != "hello" {
		t.Errorf("run[1] = %q, want the default", got)
	}
}

func TestUseRefusals(t *testing.T) {
	src := fakeTemplates{
		"svc":      agentTemplate,
		"nested":   "app: {name: x}\n",
		"dollar":   "watch:\n  w: {run: [echo, $1]}\n",
		"unknown":  "watch:\n  w: {run: [echo, \"${nope}\"]}\n",
		"badwatch": "watch:\n  w: {run: []}\n",
		"badstate": "watch:\n  w: {exists: /tmp}\nstate:\n  - a:\n  - b: \"w.ok\"\n",
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"an unknown template": {
			"use:\n  d:\n    nope: {}\n",
			`no template named "nope"; the templates are badstate, badwatch, dollar, nested, svc, unknown`,
		},
		"a missing required parameter": {
			"use:\n  d:\n    svc: {}\n",
			"requires label",
		},
		"an argument the template does not take": {
			"use:\n  d:\n    svc: {label: dev.example.d, color: red}\n",
			`"color" is not a parameter of this template; it takes label, noun`,
		},
		"a template that nests": {
			"use:\n  d:\n    nested:\n",
			"templates do not nest",
		},
		"a bare $ in a template": {
			"use:\n  d:\n    dollar:\n",
			"write $$ for a literal $",
		},
		"an unknown ${x}": {
			"use:\n  d:\n    unknown:\n",
			"${nope} is not a parameter",
		},
		"a use named self": {
			"use:\n  self:\n    svc: {label: dev.example.d}\n",
			"bound by perch itself",
		},
		"a use naming two templates": {
			"use:\n  d: {svc: {label: dev.example.d}, dollar: {}}\n",
			"exactly one template",
		},
		"a use named like a watch": {
			"use:\n  d:\n    svc: {label: dev.example.d}\nwatch:\n  d: {exists: /tmp}\n",
			"already a watch",
		},
		"a state named like a use": {
			"use:\n  d:\n    svc: {label: dev.example.d}\nstate:\n  - d: \"true\"\n  - b:\n",
			"already a use",
		},
		"a template's own watch is invalid": {
			"use:\n  d:\n    badwatch:\n",
			"use.d (templates/badwatch.yaml): watch.w.run: empty",
		},
		"a template's state list is out of order": {
			"use:\n  d:\n    badstate:\n",
			"use.d (templates/badstate.yaml): state[0]",
		},
		"a watch named self": {
			"watch:\n  self: {exists: /tmp}\n",
			"bound by perch itself",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseWith([]byte(useHead+tc.doc+"menu: [{text: Quit, quit: true}]\n"), src)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
