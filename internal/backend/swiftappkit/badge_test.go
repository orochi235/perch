package swiftappkit

import (
	"strings"
	"testing"

	"github.com/orochi235/perch/v2/internal/spec"
)

// badgesDoc writes badges every way a spec can: a literal list with holes,
// an expression over a shape that leaves fg and at out, and one over data with
// no shape at all.
const badgesDoc = `
app: {name: pills, id: dev.example.pills.menubar, icon: circle, interval: 3s}
watch:
  m:
    run: [echo, "{}"]
    json: true
    shape:
      rows: [{text: string, state: string, port: string, badges: [{text: string, symbol: string, bg: string}]}]
  loose:
    run: [echo, "{}"]
    json: true
menu:
  - each: m.data.rows
    text: "{{it.text}}"
    badges:
      - {text: "{{it.state}}", symbol: "{{it.state == 'up' ? 'bolt' : ''}}", bg: green}
      - {text: ":{{it.port}}", fg: "#ffffff", bg: "{{it.state == 'up' ? 'blue' : 'gray'}}", at: start}
  - each: m.data.rows
    text: "{{it.text}}"
    size: "it.state == 'up' ? 15 : 0"
    badges: it.badges
    menu:
      - {text: Open, open: "https://example.com"}
  - {text: Loose, size: 14, badges: loose.data.badges}
  - {text: Quit, quit: true}
`

func TestBadgesLowerToMenuBadges(t *testing.T) {
	s, err := spec.Parse([]byte(badgesDoc))
	if err != nil {
		t.Fatal(err)
	}
	files, err := New().Emit(s)
	if err != nil {
		t.Fatal(err)
	}
	var render string
	for _, f := range files {
		if f.Name == "Render.swift" {
			render = string(f.Body)
		}
	}
	for _, want := range []string{
		`badges: [MenuBadge(text: "\(`,
		`at: "start")]`,
		`.badges.map { badge`,
		`in MenuBadge(text: `,
		`symbol: badge`,
		`symbol: "\(`,
		`.asArray.map { badge`,
		`size: Double(14)`,
		`size: Double(((it2.state == "up")`,
	} {
		if !strings.Contains(render, want) {
			t.Errorf("Render.swift lacks %q\n%s", want, render)
		}
	}
}

func TestBadgesRefuseAListOfTheWrongThing(t *testing.T) {
	for name, doc := range map[string]string{
		"not a list": "app: {name: p, id: a.b, icon: circle, interval: 3s}\nwatch: {w: {run: [echo], json: true, shape: {n: string}}}\nmenu: [{text: a, badges: w.data.n}]\n",
		"no text":    "app: {name: p, id: a.b, icon: circle, interval: 3s}\nwatch: {w: {run: [echo], json: true, shape: {bs: [{bg: string}]}}}\nmenu: [{text: a, badges: w.data.bs}]\n",
		"strings":    "app: {name: p, id: a.b, icon: circle, interval: 3s}\nwatch: {w: {run: [echo], json: true, shape: {bs: [string]}}}\nmenu: [{text: a, badges: w.data.bs}]\n",
	} {
		s, err := spec.Parse([]byte(doc))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := New().Emit(s); err == nil || !strings.Contains(err.Error(), "badges") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
