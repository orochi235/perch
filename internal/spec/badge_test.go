package spec

import (
	"reflect"
	"strings"
	"testing"
)

const badgeHead = "app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s}\n"

func TestBadgesTakeAListOrAnExpression(t *testing.T) {
	s := parse(t, badgeHead+"menu:\n"+
		"  - {text: a, badges: [{text: '{{x}}', bg: teal}, {text: beta, fg: '#000000', bg: '{{y}}', at: start}]}\n"+
		"  - {text: b, badges: it.badges}\n"+
		"  - {text: Quit, quit: true}\n")
	want := []Badge{{Text: "{{x}}", BG: "teal", At: BadgeEnd}, {Text: "beta", FG: "#000000", BG: "{{y}}", At: BadgeStart}}
	if got := s.Menu[0].Badges.List; !reflect.DeepEqual(got, want) {
		t.Errorf("list = %+v", got)
	}
	if got := s.Menu[1].Badges; got.Expr != "it.badges" || got.List != nil {
		t.Errorf("expr = %+v", got)
	}
}

func TestBadgesRefuseWhatTheyCannotDraw(t *testing.T) {
	for name, c := range map[string]struct{ item, want string }{
		"nothing":     {"{text: a, badges: [{bg: teal}]}", "menu[0].badges[0]: needs text:, symbol:, or both"},
		"bad color":   {"{text: a, badges: [{text: x, bg: chartreuse}]}", "menu[0].badges[0].bg"},
		"alpha":       {"{text: a, badges: [{text: x, bg: '#ff000080'}]}", "menu[0].badges[0].bg"},
		"bad side":    {"{text: a, badges: [{text: x, at: middle}]}", "neither start nor end"},
		"unknown key": {"{text: a, badges: [{text: x, color: red}]}", "color"},
		"template":    {"{text: a, badges: '{{it.badges}}'}", "takes no {{ }}"},
		"mapping":     {"{text: a, badges: {text: x}}", "want a list"},
		"field item":  {"{field: Search, open: 'https://x/?q={{query}}', badges: [{text: x}]}", "takes no badges"},
	} {
		_, err := Parse([]byte(badgeHead + "menu: [" + c.item + ", {text: Quit, quit: true}]\n"))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

func TestAnItemCanBeOnlyBadges(t *testing.T) {
	s := parse(t, badgeHead+"menu:\n  - {icon: folder, badges: [{text: perch, fg: '#000000', bg: teal}]}\n  - {text: Quit, quit: true}\n")
	if got := s.Menu[0]; got.Text != "" || len(got.Badges.List) != 1 {
		t.Errorf("item = %+v", got)
	}
}

func TestABadgeCanBeOnlyASymbol(t *testing.T) {
	s := parse(t, badgeHead+"menu:\n  - {text: a, badges: [{symbol: '{{x}}', bg: teal}]}\n  - {text: Quit, quit: true}\n")
	if got := s.Menu[0].Badges.List[0]; got.Symbol != "{{x}}" || got.Text != "" {
		t.Errorf("badge = %+v", got)
	}
}

func TestAPlainBadgeTakesNoBackground(t *testing.T) {
	s := parse(t, badgeHead+"menu:\n  - {text: a, badges: [{text: '74915', plain: true}]}\n  - {text: Quit, quit: true}\n")
	if b := s.Menu[0].Badges.List[0]; !b.Plain || b.At != BadgeEnd {
		t.Errorf("badge = %+v", b)
	}
	_, err := Parse([]byte(badgeHead + "menu: [{text: a, badges: [{text: x, plain: true, bg: teal}]}, {text: Quit, quit: true}]\n"))
	if err == nil || !strings.Contains(err.Error(), "no pill to fill") {
		t.Errorf("err = %v", err)
	}
}
