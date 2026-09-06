package spec

import "testing"

const watchDoc = `
app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 5s}
watch:
  fleet:
    run: [onto, top, --once, --json]
    json: true
  server:
    http: http://127.0.0.1:8080/health
  plist:
    exists: ~/Library/LaunchAgents/dev.onto.plist
menu:
  - {text: Quit, quit: true}
`

func TestParseWatchKinds(t *testing.T) {
	s, err := Parse([]byte(watchDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.Watches) != 3 {
		t.Fatalf("got %d watches, want 3", len(s.Watches))
	}
	w := s.Watches[0]
	if w.Name != "fleet" || w.Kind != WatchRun {
		t.Errorf("watch 0 = %q/%v, want fleet/run", w.Name, w.Kind)
	}
	if got, want := len(w.Run), 4; got != want {
		t.Fatalf("fleet.run has %d args, want %d", got, want)
	}
	if w.Run[0] != "onto" || w.Run[3] != "--json" {
		t.Errorf("fleet.run = %v", w.Run)
	}
	if !w.JSON {
		t.Error("fleet.json = false, want true")
	}
	if s.Watches[1].Kind != WatchHTTP || s.Watches[1].HTTP != "http://127.0.0.1:8080/health" {
		t.Errorf("watch 1 = %v/%q", s.Watches[1].Kind, s.Watches[1].HTTP)
	}
	if s.Watches[2].Kind != WatchExists || s.Watches[2].Exists == "" {
		t.Errorf("watch 2 = %v/%q", s.Watches[2].Kind, s.Watches[2].Exists)
	}
}

func TestParseWatchesKeepDocumentOrder(t *testing.T) {
	s, err := Parse([]byte(watchDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{"fleet", "server", "plist"}
	for i, name := range want {
		if s.Watches[i].Name != name {
			t.Errorf("watch %d = %q, want %q", i, s.Watches[i].Name, name)
		}
	}
}

func TestParseWatchRejectsTwoKinds(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: a, id: b, icon: c, interval: 1s}
watch:
  both:
    run: [ls]
    http: http://example.com
menu: [{text: Quit, quit: true}]
`))
	if err == nil {
		t.Fatal("want error for a watch with two kinds, got nil")
	}
}

func TestParseWatchRejectsNoKind(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: a, id: b, icon: c, interval: 1s}
watch:
  empty:
    json: true
menu: [{text: Quit, quit: true}]
`))
	if err == nil {
		t.Fatal("want error for a watch with no kind, got nil")
	}
}
