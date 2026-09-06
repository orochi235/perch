package spec

import "testing"

func TestParseStatusRules(t *testing.T) {
	s, err := Parse([]byte(`
app: {name: a, id: b, icon: circle, interval: 1s}
status:
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - when: "fleet.data.jobs.size() == 0"
    dim: true
  - badge: "fleet.data.jobs.size()"
menu: [{text: Quit, quit: true}]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.Status) != 3 {
		t.Fatalf("got %d status rules, want 3", len(s.Status))
	}
	if s.Status[0].When != "!fleet.ok" || s.Status[0].Icon != "exclamationmark.triangle" {
		t.Errorf("rule 0 = %+v", s.Status[0])
	}
	if !s.Status[1].Dim {
		t.Error("rule 1 Dim = false, want true")
	}
	if s.Status[2].When != "" {
		t.Errorf("rule 2 When = %q, want empty (an unguarded rule always matches)", s.Status[2].When)
	}
	if s.Status[2].Badge != "fleet.data.jobs.size()" {
		t.Errorf("rule 2 Badge = %q", s.Status[2].Badge)
	}
}

func TestParseRejectsUnknownStatusKey(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: a, id: b, icon: circle, interval: 1s}
status:
  - {when: "x", icno: circle}
menu: [{text: Quit, quit: true}]
`))
	if err == nil {
		t.Fatal("want error for an unknown status rule key, got nil")
	}
}
