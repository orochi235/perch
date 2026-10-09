package spec

import (
	"strings"
	"testing"
)

const quitItemApp = "app: {name: a, id: b, icon: c, interval: 1s}\nwatch: {w: {exists: /tmp}}\n"

func addsQuit(t *testing.T, doc string, ts TemplateSource) bool {
	t.Helper()
	if !strings.HasPrefix(doc, "app:") {
		doc = quitItemApp + doc
	}
	s, err := ParseWith([]byte(doc), ts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	n := len(s.Menu)
	if n == 0 || (n > 1 && !s.Menu[n-2].Separator) {
		return false
	}
	last := s.Menu[n-1]
	return last.Text == "Quit" && last.Action.Kind == ActionQuit && last.Path() == "menu (added by perch)"
}

func TestDefaultQuit(t *testing.T) {
	quitter := fakeTemplates{"q": "menu:\n  default:\n    - {text: Bye, quit: true}\n"}
	for name, tc := range map[string]struct {
		doc  string
		want bool
	}{
		"no quit item":       {"menu: [{text: a}]\n", true},
		"empty menu":         {"menu: []\n", true},
		"no menu key":        {"status: []\n", true},
		"top-level quit":     {"menu: [{text: Bye, quit: true}]\n", false},
		"quit in a submenu":  {"menu: [{text: More, menu: [{text: Bye, quit: true}]}]\n", false},
		"guarded quit":       {"menu: [{text: Bye, quit: true, when: w.ok}]\n", false},
		"quit in a template": {"use: {u: {q: {}}}\nmenu: [outlet]\n", false},
		"quit only in a dialog button": {
			"app:\n  name: a\n  id: b\n  icon: c\n  interval: 1s\n  quit:\n    - confirm: Stop?\n      buttons: [{text: Stop, run: [x]}]\nmenu: [{text: a}]\n", true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := addsQuit(t, tc.doc, quitter); got != tc.want {
				t.Errorf("added Quit = %v, want %v", got, tc.want)
			}
		})
	}
}
