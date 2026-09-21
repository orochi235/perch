package spec

import (
	"strings"
	"testing"
)

func parse(t *testing.T, doc string) *Spec {
	t.Helper()
	s, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s
}

const tail = "menu: [{text: Quit, quit: true}]\n"

func TestIconTakesASymbolOrAnAsset(t *testing.T) {
	s := parse(t, "app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s}\n"+tail)
	if s.App.Icon.Symbol != "circle" || s.App.Icon.Asset != "" {
		t.Errorf("bare name = %+v, want a symbol", s.App.Icon)
	}

	s = parse(t, "app: {name: onto, id: dev.onto.menubar, icon: {asset: o-4}, interval: 1s}\n"+tail)
	if s.App.Icon.Asset != "o-4" || s.App.Icon.Symbol != "" {
		t.Errorf("mapping = %+v, want an asset", s.App.Icon)
	}
}

func TestStatusRuleIconTakesAnAsset(t *testing.T) {
	s := parse(t, "app: {name: onto, id: dev.onto.menubar, icon: {asset: o-0}, interval: 1s}\n"+
		"status: [{icon: {asset: o-8}}]\n"+tail)
	if got := s.Status[0].Icon.Asset; got != "o-8" {
		t.Errorf("status[0].icon.asset = %q, want o-8", got)
	}
}

// The name is interpolated into a path the bundler reads and the app loads, so
// anything that could climb out of menubar/Icons is refused at parse.
func TestAssetNameCannotEscapeItsDirectory(t *testing.T) {
	for _, name := range []string{"../evil", "a/b", `a\b`, ".", "..", "o 4", "o.png"} {
		doc := "app: {name: onto, id: dev.onto.menubar, icon: {asset: " + quoteYAML(name) + "}, interval: 1s}\n" + tail
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("asset %q was accepted", name)
		}
	}
}

func TestIconMappingWantsAnAssetKey(t *testing.T) {
	doc := "app: {name: onto, id: dev.onto.menubar, icon: {symbol: circle}, interval: 1s}\n" + tail
	if _, err := Parse([]byte(doc)); err == nil {
		t.Error("a mapping with no asset: was accepted")
	}
}

func quoteYAML(s string) string {
	out := `"`
	for _, r := range s {
		if r == '"' || r == '\\' {
			out += `\`
		}
		out += string(r)
	}
	return out + `"`
}

func TestMenuItemTakesAnIcon(t *testing.T) {
	s := parse(t, "app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s}\n"+
		"menu:\n"+
		"  - {text: Open, icon: safari, open: \"https://example.com\"}\n"+
		"  - text: More\n"+
		"    icon: {asset: o-4}\n"+
		"    menu: [{text: Quit, icon: power, quit: true}]\n")
	if got := s.Menu[0].Icon.Symbol; got != "safari" {
		t.Errorf("menu[0].icon = %q, want safari", got)
	}
	if got := s.Menu[1].Icon.Asset; got != "o-4" {
		t.Errorf("menu[1].icon.asset = %q, want o-4", got)
	}
	icons := s.Icons()
	for path, want := range map[string]Icon{
		"menu[0].icon":         {Symbol: "safari"},
		"menu[1].icon":         {Asset: "o-4"},
		"menu[1].menu[0].icon": {Symbol: "power"},
	} {
		if icons[path] != want {
			t.Errorf("Icons()[%s] = %+v, want %+v", path, icons[path], want)
		}
	}
}

func TestMenuItemIconAssetCannotEscapeItsDirectory(t *testing.T) {
	_, err := Parse([]byte("app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s}\n" +
		"menu: [{text: Open, icon: {asset: ../evil}}]\n"))
	if err == nil || !strings.Contains(err.Error(), "menu[0].icon.asset") {
		t.Errorf("err = %v, want one naming menu[0].icon.asset", err)
	}
}

func TestMenuItemIconTakesHoles(t *testing.T) {
	s := parse(t, "app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s}\n"+
		"menu:\n"+
		"  - {text: a, icon: \"{{it.icon}}\"}\n"+
		"  - {text: b, icon: {asset: \"logo-{{it.zone}}\"}}\n"+
		"  - {text: c, icon: gear}\n")
	if !s.Menu[0].Icon.Templated() || !s.Menu[1].Icon.Templated() || s.Menu[2].Icon.Templated() {
		t.Errorf("Templated() = %v %v %v, want true true false",
			s.Menu[0].Icon.Templated(), s.Menu[1].Icon.Templated(), s.Menu[2].Icon.Templated())
	}
	// Not known until the app polls, so there is nothing to check at build.
	icons := s.Icons()
	if _, ok := icons["menu[0].icon"]; ok {
		t.Errorf("Icons() holds the templated menu[0].icon")
	}
	if _, ok := icons["menu[2].icon"]; !ok {
		t.Errorf("Icons() lacks the fixed menu[2].icon")
	}
}

func TestIconHolesAreRefusedOffTheMenu(t *testing.T) {
	for _, doc := range []string{
		"app: {name: onto, id: dev.onto.menubar, icon: \"{{x}}\", interval: 1s}\n" + tail,
		"app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s}\nstatus: [{icon: {asset: \"o-{{x}}\"}}]\n" + tail,
	} {
		if _, err := Parse([]byte(doc)); err == nil || !strings.Contains(err.Error(), "holes are for a menu item's icon") {
			t.Errorf("Parse(%q) = %v, want holes refused", doc, err)
		}
	}
}

func TestTemplatedAssetStillChecksItsLiteralText(t *testing.T) {
	_, err := Parse([]byte("app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s}\n" +
		"menu: [{text: a, icon: {asset: \"../{{it.zone}}\"}}]\n"))
	if err == nil || !strings.Contains(err.Error(), "menu[0].icon.asset") {
		t.Errorf("err = %v, want menu[0].icon.asset refused", err)
	}
}
