package spec

import (
	"strings"
	"testing"
)

func TestSizeTakesANumberOrAnExpression(t *testing.T) {
	s := parse(t, badgeHead+"menu:\n  - {text: a, size: 15}\n  - {text: b, size: 'it.header ? 15 : 0'}\n  - {text: Quit, quit: true}\n")
	if s.Menu[0].Size != "15" || s.Menu[1].Size != "it.header ? 15 : 0" {
		t.Errorf("sizes = %q, %q", s.Menu[0].Size, s.Menu[1].Size)
	}
}

func TestSizeRefusesWhatCannotBeAFont(t *testing.T) {
	for name, c := range map[string]struct{ item, want string }{
		"tiny":  {"{text: a, size: 2}", "outside 6 to 32"},
		"huge":  {"{text: a, size: 90}", "outside 6 to 32"},
		"list":  {"{text: a, size: [15]}", "want a font size"},
		"field": {"{field: Search, open: 'https://x/?q={{query}}', size: 15}", "takes no size"},
	} {
		_, err := Parse([]byte(badgeHead + "menu: [" + c.item + ", {text: Quit, quit: true}]\n"))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}
