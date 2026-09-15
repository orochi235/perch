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

var frags = fakeTemplates{"frag": fragTemplate}

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
	if len(s.Status) != 3 || s.Status[0].Scope != "a" || s.Status[1].Scope != "b" || s.Status[2].Badge == "" {
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

func TestAnOutletCanSitInASubmenu(t *testing.T) {
	s := parseWith(t, useHead+"use:\n  a:\n    frag: {noun: A}\nmenu:\n  - {text: More, menu: [outlet]}\n  - {text: Quit, quit: true}\n", frags)
	if got := titles(s.Menu[0].Menu); !slices.Equal(got, []string{"A readout", "A control"}) {
		t.Errorf("submenu = %q", got)
	}
}

func TestOutletRefusals(t *testing.T) {
	src := fakeTemplates{
		"frag":     fragTemplate,
		"marks":    "menu:\n  default:\n    - outlet\n",
		"catchall": "status:\n  default:\n    - {dim: true}\n",
		"listmenu": "menu:\n  - {text: x}\n",
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"an outlet no use fills": {
			"use:\n  a:\n    frag:\nmenu:\n  - outlet: nope\n",
			`no use fills the outlet "nope"`,
		},
		"an outlet declared twice": {
			"use:\n  a:\n    frag:\nmenu:\n  - outlet\n  - outlet\n",
			"the default outlet is declared twice",
		},
		"a template that declares an outlet": {
			"use:\n  a:\n    marks:\nmenu:\n  - {text: Quit, quit: true}\n",
			"a template fills outlets; it cannot declare one",
		},
		"a template status rule with no when": {
			"use:\n  a:\n    catchall:\nmenu:\n  - {text: Quit, quit: true}\n",
			"shadow",
		},
		"a template menu written as a list": {
			"use:\n  a:\n    listmenu:\nmenu:\n  - {text: Quit, quit: true}\n",
			"maps outlet names to lists",
		},
		"an outlet with another key": {
			"menu:\n  - {outlet: x, text: y}\n",
			"takes nothing but its name",
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
