package spec

import (
	"strings"
	"testing"
)

func TestSizeTakesANumberOrAnExpression(t *testing.T) {
	s := parse(t, badgeHead+"menu:\n  - {text: a, size: 15}\n  - {text: b, size: 'it.header ? 15 : 0'}\n  - {text: Quit, quit: true}\n")
	if s.Menu[0].Look.Size != "15" || s.Menu[1].Look.Size != "it.header ? 15 : 0" {
		t.Errorf("sizes = %q, %q", s.Menu[0].Look.Size, s.Menu[1].Look.Size)
	}
}

func TestSizeRefusesWhatCannotBeAFont(t *testing.T) {
	for name, c := range map[string]struct{ item, want string }{
		"tiny":  {"{text: a, size: 2}", "outside 6 to 32"},
		"huge":  {"{text: a, size: 90}", "outside 6 to 32"},
		"list":  {"{text: a, size: [15]}", "want a font size"},
		"field": {"{field: Search, open: 'https://x/?q={{query}}', size: 15}", "takes no size:"},
	} {
		_, err := Parse([]byte(badgeHead + "menu: [" + c.item + ", {text: Quit, quit: true}]\n"))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

func TestLookTakesBareBooleans(t *testing.T) {
	s := parse(t, badgeHead+"menu:\n  - {text: h, header: true}\n  - {text: e, enabled: false, open: 'https://x'}\n  - {text: Quit, quit: true}\n")
	if s.Menu[0].Look.Header != "true" || s.Menu[1].Look.Enabled != "false" {
		t.Errorf("look = %+v / %+v", s.Menu[0].Look, s.Menu[1].Look)
	}
}

func TestLookRefusesWhatAnItemCannotWear(t *testing.T) {
	agent := "watch: {svc: {launchagent: dev.example.svc}}\n"
	for name, c := range map[string]struct{ app, extra, item, want string }{
		"weight":       {"", "", "{text: a, weight: chunky}", "not one of"},
		"indent":       {"", "", "{text: a, indent: 20}", "outside 0 to 15"},
		"key, no act":  {"", "", "{text: a, key: cmd+r}", "no action for a shortcut"},
		"key, each":    {"", "", "{text: a, each: x, key: cmd+r, open: 'https://x'}", "every row the same shortcut"},
		"key, bad":     {"", "", "{text: a, key: cmd+banana, open: 'https://x'}", "key"},
		"header acts":  {"", "", "{text: a, header: true, open: 'https://x'}", "takes no action"},
		"header menu":  {"", "", "{text: a, header: true, menu: [{text: b}]}", "takes no submenu"},
		"gutter, leaf": {"", "", "{text: a, gutter: false}", "it has no menu:"},
		"toggle, app":  {", gutter: false", agent, "{text: a, toggle: svc.enabled}", "app.gutter: false"},
		"toggle, sub":  {"", agent, "{text: s, gutter: false, menu: [{text: a, toggle: svc.enabled}]}", "menu[0].gutter: false"},
		"field weight": {"", "", "{field: Find, open: 'https://x/?q={{query}}', weight: bold}", "takes no size:"},
	} {
		doc := "app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s" + c.app + "}\n" + c.extra +
			"menu: [" + c.item + ", {text: Quit, quit: true}]\n"
		_, err := Parse([]byte(doc))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

func TestLookTakesEveryPart(t *testing.T) {
	s := parse(t, "app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s, gutter: false}\nmenu:\n"+
		"  - {text: a, size: 15, weight: '{{w}}', indent: 'it.depth', enabled: 'x.ok', subtitle: '{{s}}', key: cmd+shift+r, open: 'https://x'}\n"+
		"  - {text: s, gutter: false, menu: [{text: b}]}\n  - {text: Quit, quit: true}\n")
	want := Look{Size: "15", Weight: "{{w}}", Indent: "it.depth", Enabled: "x.ok", Subtitle: "{{s}}", Key: Hotkey{Cmd: true, Shift: true, Key: "r"}}
	if got := s.Menu[0].Look; got != want {
		t.Errorf("look = %+v", got)
	}
	if !s.App.NoGutter || !s.Menu[1].Look.NoGutter {
		t.Error("gutter: false was not kept")
	}
}

func TestDividerIsACondition(t *testing.T) {
	s := parse(t, badgeHead+"menu:\n  - {text: a, divider: it.header, each: x}\n  - {text: Quit, quit: true}\n")
	if s.Menu[0].Divider != "it.header" {
		t.Errorf("divider = %q", s.Menu[0].Divider)
	}
}
