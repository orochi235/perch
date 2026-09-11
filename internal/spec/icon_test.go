package spec

import "testing"

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
