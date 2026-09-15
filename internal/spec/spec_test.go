package spec

import (
	"strings"
	"testing"
	"time"
)

func TestParseApp(t *testing.T) {
	s, err := Parse([]byte(`
app:
  name: onto
  id: dev.onto.menubar
  icon: rectangle.3.group
  interval: 5s
menu:
  - {text: Quit, quit: true}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.App.Name != "onto" {
		t.Errorf("Name = %q, want onto", s.App.Name)
	}
	if s.App.ID != "dev.onto.menubar" {
		t.Errorf("ID = %q, want dev.onto.menubar", s.App.ID)
	}
	if s.App.Icon.Symbol != "rectangle.3.group" {
		t.Errorf("Icon = %+v, want rectangle.3.group", s.App.Icon)
	}
	if s.App.Interval != 5*time.Second {
		t.Errorf("Interval = %v, want 5s", s.App.Interval)
	}
}

// yaml.v3 keeps both entries of a repeated key in a node, so without this a
// repeated use emits Swift that does not compile and a repeated outlet or
// argument silently joins or takes the last.
func TestAKeyWrittenTwiceInOneMappingIsRefused(t *testing.T) {
	src := fakeTemplates{
		"svc":       agentTemplate,
		"dupparam":  "params:\n  a: x\n  a: y\n",
		"dupoutlet": "watch:\n  w: {exists: /tmp}\nmenu:\n  default:\n    - {text: x}\n  default:\n    - {text: y}\n",
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"a watch": {
			"watch:\n  w: {exists: /tmp}\n  w: {exists: /tmp}\n",
			`watch: "w" is written twice in one mapping, on lines 4 and 5`,
		},
		"a use": {
			"use:\n  d:\n    svc: {label: dev.example.a}\n  d:\n    svc: {label: dev.example.b}\n",
			`use: "d" is written twice in one mapping`,
		},
		"an argument": {
			"use:\n  d:\n    svc: {label: dev.example.a, label: dev.example.b}\n",
			`use.d.svc: "label" is written twice in one mapping`,
		},
		"a template's param": {
			"use:\n  d:\n    dupparam:\n",
			`use.d (templates/dupparam.yaml): params: "a" is written twice in one mapping, on lines 2 and 3`,
		},
		"a template's outlet": {
			"use:\n  d:\n    dupoutlet:\n",
			`use.d (templates/dupoutlet.yaml): menu: "default" is written twice in one mapping`,
		},
		"a top-level key": {
			"menu: []\n",
			`"menu" is written twice in one mapping`,
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
