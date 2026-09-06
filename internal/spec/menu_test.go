package spec

import "testing"

func menuOf(t *testing.T, menu string) []Item {
	t.Helper()
	s, err := Parse([]byte(`
app: {name: a, id: b, icon: c, interval: 1s}
menu:
` + menu + `
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return s.Menu
}

func TestParseSeparatorScalar(t *testing.T) {
	items := menuOf(t, "  - separator\n  - {text: Quit, quit: true}")
	if !items[0].Separator {
		t.Error("item 0 Separator = false, want true")
	}
	if items[1].Separator {
		t.Error("item 1 Separator = true, want false")
	}
}

func TestParseTextAndWhen(t *testing.T) {
	items := menuOf(t, `  - text: "{{fleet.data.jobs.size()}} running"
    when: "fleet.ok"`)
	if items[0].Text != "{{fleet.data.jobs.size()}} running" {
		t.Errorf("Text = %q", items[0].Text)
	}
	if items[0].When != "fleet.ok" {
		t.Errorf("When = %q, want fleet.ok", items[0].When)
	}
}

func TestParseEachWithSubmenu(t *testing.T) {
	items := menuOf(t, `  - each: fleet.data.jobs
    text: "{{it.node}} — {{it.cmd}}"
    menu:
      - {text: Logs, run: [onto, logs, "{{it.id}}"]}
      - {text: Kill, run: [onto, kill, "{{it.id}}"]}`)
	it := items[0]
	if it.Each != "fleet.data.jobs" {
		t.Errorf("Each = %q", it.Each)
	}
	if len(it.Menu) != 2 {
		t.Fatalf("submenu has %d items, want 2", len(it.Menu))
	}
	if it.Menu[0].Action.Kind != ActionRun {
		t.Fatalf("submenu 0 action = %v, want run", it.Menu[0].Action.Kind)
	}
	if got := it.Menu[0].Action.Run; len(got) != 3 || got[2] != "{{it.id}}" {
		t.Errorf("submenu 0 run = %v", got)
	}
}

func TestParseActionVerbs(t *testing.T) {
	items := menuOf(t, `  - {text: R, run: [ls, -l]}
  - {text: O, open: "https://example.com"}
  - {text: P, post: {url: "http://x/y", body: {action: restart}}}
  - {text: Q, quit: true}`)
	if items[0].Action.Kind != ActionRun || items[0].Action.Run[1] != "-l" {
		t.Errorf("run action = %+v", items[0].Action)
	}
	if items[1].Action.Kind != ActionOpen || items[1].Action.Open != "https://example.com" {
		t.Errorf("open action = %+v", items[1].Action)
	}
	if items[2].Action.Kind != ActionPost || items[2].Action.PostURL != "http://x/y" {
		t.Errorf("post action = %+v", items[2].Action)
	}
	if items[2].Action.PostBody != `{"action":"restart"}` {
		t.Errorf("post body = %q, want compact JSON", items[2].Action.PostBody)
	}
	if items[3].Action.Kind != ActionQuit {
		t.Errorf("quit action = %+v", items[3].Action)
	}
}

func TestParseItemWithNoActionIsInert(t *testing.T) {
	items := menuOf(t, `  - {text: Just a label}`)
	if items[0].Action.Kind != ActionNone {
		t.Errorf("action = %v, want none", items[0].Action.Kind)
	}
}

func TestParseRejectsTwoActionsOnOneItem(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: a, id: b, icon: c, interval: 1s}
menu:
  - {text: X, run: [ls], quit: true}
`))
	if err == nil {
		t.Fatal("want error for an item with two actions, got nil")
	}
}

func TestParseRejectsUnknownItemKey(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: a, id: b, icon: c, interval: 1s}
menu:
  - {text: X, tetx: typo}
`))
	if err == nil {
		t.Fatal("want error for an unknown menu item key, got nil")
	}
}
