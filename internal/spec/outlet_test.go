package spec

import (
	"slices"
	"strings"
	"testing"
)

const fragTemplate = `
params:
  noun: thing
watch:
  w: {exists: /tmp}
status:
  default:
    - {when: "!self.w.ok", dim: true}
menu:
  default:
    - {text: "${noun} readout", menu: [{text: "${noun} detail"}]}
  controls:
    - {text: "${noun} control"}
`

const warnTemplate = `
watch:
  w: {exists: /tmp}
status:
  warn:
    - {when: "self.w.ok", badge: "\"warn\""}
  default:
    - {when: "!self.w.ok", dim: true}
`

var frags = fakeTemplates{"frag": fragTemplate, "warn": warnTemplate}

const twoUses = `
use:
  a:
    frag: {noun: A}
  b:
    frag: {noun: B}
`

func titles(items []Item) []string {
	var out []string
	for _, it := range items {
		if it.Separator {
			out = append(out, "-")
			continue
		}
		out = append(out, it.Text)
	}
	return out
}

func TestUseFragmentsLandAtTheirOutlets(t *testing.T) {
	s := parseWith(t, useHead+twoUses+`
menu:
  - {text: top}
  - outlet
  - separator
  - outlet: controls
  - {text: Quit, quit: true}
`, frags)
	want := []string{"top", "A readout", "B readout", "-", "A control", "B control", "Quit"}
	if got := titles(s.Menu); !slices.Equal(got, want) {
		t.Errorf("menu = %q, want %q", got, want)
	}
	if s.Menu[1].Scope != "a" || s.Menu[2].Scope != "b" || s.Menu[0].Scope != "" {
		t.Errorf("scopes = %q %q %q", s.Menu[0].Scope, s.Menu[1].Scope, s.Menu[2].Scope)
	}
	if sub := s.Menu[1].Menu; len(sub) != 1 || sub[0].Scope != "a" {
		t.Errorf("a submenu item did not keep its use: %+v", sub)
	}
}

// With no default declared, the default outlet is the end of menu: and the
// start of status: — the start, because a rule after the file's when:-less
// rule could never match.
func TestTheDefaultOutletHasAPlaceWhenTheFileDeclaresNone(t *testing.T) {
	s := parseWith(t, useHead+twoUses+`
status:
  - badge: "\"x\""
menu:
  - outlet: controls
  - {text: Quit, quit: true}
`, frags)
	want := []string{"A control", "B control", "Quit", "A readout", "B readout"}
	if got := titles(s.Menu); !slices.Equal(got, want) {
		t.Errorf("menu = %q, want %q", got, want)
	}
	if len(s.Status) != 3 || s.Status[0].Scope != "a" || s.Status[1].Scope != "b" ||
		s.Status[2].Scope != "" || s.Status[2].Badge != `"x"` {
		t.Errorf("status = %+v, want the two uses' rules ahead of the file's", s.Status)
	}
}

func TestAFragmentForAnUndeclaredOutletGoesToTheDefault(t *testing.T) {
	s := parseWith(t, useHead+"use:\n  a:\n    frag: {noun: A}\nmenu:\n  - outlet\n  - {text: Quit, quit: true}\n", frags)
	want := []string{"A readout", "A control", "Quit"}
	if got := titles(s.Menu); !slices.Equal(got, want) {
		t.Errorf("menu = %q, want %q", got, want)
	}
}

func TestTheDefaultOutletCanBeWrittenByName(t *testing.T) {
	for _, mark := range []string{"{outlet: ~}", "{outlet: default}"} {
		s := parseWith(t, useHead+"use:\n  a:\n    frag: {noun: A}\nmenu:\n  - {text: Quit, quit: true}\n  - "+mark+"\n", frags)
		want := []string{"Quit", "A readout", "A control"}
		if got := titles(s.Menu); !slices.Equal(got, want) {
			t.Errorf("%s: menu = %q, want %q", mark, got, want)
		}
	}
}

func TestAnOutletCanSitInASubmenu(t *testing.T) {
	s := parseWith(t, useHead+"use:\n  a:\n    frag: {noun: A}\nmenu:\n  - {text: More, menu: [outlet]}\n  - {text: Quit, quit: true}\n", frags)
	if got := titles(s.Menu[0].Menu); !slices.Equal(got, []string{"A readout", "A control"}) {
		t.Errorf("submenu = %q", got)
	}
}

func TestASubmenuOfOnlyEmptyOutletsIsDropped(t *testing.T) {
	s := parseWith(t, useHead+"menu:\n  - {text: More, menu: [outlet]}\n  - {text: Quit, quit: true}\n", frags)
	if got := titles(s.Menu); !slices.Equal(got, []string{"Quit"}) {
		t.Errorf("menu = %q, want only Quit", got)
	}
}

func TestStatusOutletsOrderUsesWithinEachOutlet(t *testing.T) {
	s := parseWith(t, useHead+`
use:
  a:
    warn:
  b:
    warn:
status:
  - outlet: warn
  - {when: "x", dim: true}
  - outlet
  - badge: "\"y\""
`, frags)
	type rule struct{ scope, when string }
	var got []rule
	for _, r := range s.Status {
		got = append(got, rule{r.Scope, r.When})
	}
	want := []rule{
		{"a", "self.w.ok"}, {"b", "self.w.ok"},
		{"", "x"},
		{"a", "!self.w.ok"}, {"b", "!self.w.ok"},
		{"", ""},
	}
	if !slices.Equal(got, want) {
		t.Errorf("status = %v, want %v", got, want)
	}
}

// Placement moves items and rules, so an error has to name where the author
// wrote one, not where it landed.
func TestErrorsNameWhereTheAuthorWroteIt(t *testing.T) {
	src := fakeTemplates{
		"frag":   fragTemplate,
		"badrun": "menu:\n  default:\n    - {text: bad, run: []}\n",
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"a file rule with no when: ahead of another": {
			"use:\n  a:\n    frag:\nstatus: [{badge: x}, {when: y}]\n",
			"status[0]: a rule with no when: always matches",
		},
		"a file item after an outlet": {
			"use:\n  a:\n    frag:\nmenu: [outlet, {text: bad, run: []}]\n",
			"menu[1].run: empty",
		},
		"a template item": {
			"use:\n  a:\n    badrun:\nmenu: [outlet]\n",
			"use.a (templates/badrun.yaml): menu.default[0].run: empty",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseWith([]byte(useHead+tc.doc), src)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.HasPrefix(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to start %q", err, tc.want)
			}
		})
	}
}

func TestOutletRefusals(t *testing.T) {
	src := fakeTemplates{
		"frag":      fragTemplate,
		"warn":      warnTemplate,
		"marks":     "menu:\n  default:\n    - outlet\n",
		"submarks":  "menu:\n  default:\n    - {text: x, menu: [outlet]}\n",
		"catchall":  "status:\n  default:\n    - {dim: true}\n",
		"listmenu":  "menu:\n  - {text: x}\n",
		"badrule":   "status:\n  default:\n    - {when: x, icno: y}\n",
		"badoutlet": "menu:\n  a-b:\n    - {text: x}\n",
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"an outlet no use fills": {
			"use:\n  a:\n    frag:\nmenu:\n  - {text: Quit, quit: true}\n  - outlet: nope\n",
			`menu[1]: no use fills the outlet "nope"; the uses fill default, controls`,
		},
		"an outlet in a file with no use: block": {
			"menu:\n  - outlet: nope\n",
			`menu[0]: no use fills the outlet "nope"; this file has no use: block`,
		},
		"an action whose submenu holds only an empty outlet": {
			"menu:\n  - {text: More, run: [x], menu: [outlet]}\n",
			"menu[0]: has a submenu and a run action",
		},
		"an outlet declared twice": {
			"use:\n  a:\n    frag:\nmenu:\n  - outlet\n  - outlet\n",
			"menu[1]: the default outlet is declared twice",
		},
		"the default outlet declared twice, once by name": {
			"use:\n  a:\n    frag:\nmenu:\n  - outlet\n  - outlet: default\n",
			"menu[1]: the default outlet is declared twice",
		},
		"an outlet declared twice across a submenu": {
			"use:\n  a:\n    frag:\nmenu:\n  - outlet\n  - {text: More, menu: [outlet]}\n",
			"menu[1].menu[0]: the default outlet is declared twice",
		},
		"a status outlet declared twice": {
			"use:\n  a:\n    warn:\nstatus:\n  - outlet: warn\n  - outlet: warn\n",
			`status[1]: the outlet "warn" is declared twice`,
		},
		"a status outlet no use fills": {
			"use:\n  a:\n    frag:\nstatus:\n  - outlet: warn\n",
			`status[0]: no use fills the outlet "warn"; the uses fill default`,
		},
		"a status outlet after a rule with no when": {
			"use:\n  a:\n    warn:\nstatus:\n  - {dim: true}\n  - outlet: warn\n",
			"status[1]: an outlet after a rule with no when: could never apply",
		},
		"a template that declares an outlet": {
			"use:\n  a:\n    marks:\nmenu:\n  - {text: Quit, quit: true}\n",
			"use.a (templates/marks.yaml): menu.default[0]: a template fills outlets; it cannot declare one",
		},
		"a template submenu that declares an outlet": {
			"use:\n  a:\n    submarks:\nmenu:\n  - {text: Quit, quit: true}\n",
			"use.a (templates/submarks.yaml): menu.default[0].menu[0]: a template fills outlets; it cannot declare one",
		},
		"a template status rule with no when": {
			"use:\n  a:\n    catchall:\nmenu:\n  - {text: Quit, quit: true}\n",
			"shadow",
		},
		"a template status rule with an unknown key": {
			"use:\n  a:\n    badrule:\nmenu:\n  - {text: Quit, quit: true}\n",
			`use.a (templates/badrule.yaml): status.default[0]: unknown key "icno"`,
		},
		"a template menu written as a list": {
			"use:\n  a:\n    listmenu:\nmenu:\n  - {text: Quit, quit: true}\n",
			"use.a (templates/listmenu.yaml): menu: a template's menu maps outlet names to lists",
		},
		"a template outlet with an invalid name": {
			"use:\n  a:\n    badoutlet:\nmenu:\n  - {text: Quit, quit: true}\n",
			`use.a (templates/badoutlet.yaml): menu.a-b: "a-b" is not an outlet name`,
		},
		"a file outlet with an invalid name": {
			"menu:\n  - outlet: a-b\n",
			`menu[0]: "a-b" is not an outlet name`,
		},
		"an outlet with another key": {
			"menu:\n  - {outlet: x, text: y}\n",
			"menu[0]: an outlet takes nothing but its name",
		},
		"an outlet key after another key": {
			"menu:\n  - {text: y, outlet: x}\n",
			"menu[0]: an outlet takes nothing but its name",
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
