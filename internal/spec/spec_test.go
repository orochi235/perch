package spec

import (
	"testing"
	"time"
)

func TestParseApp(t *testing.T) {
	s, err := Parse([]byte(`
app:
  name: onto
  id: dev.onto.menubar
  icon: rectangle.3.group
  interval: 5s
menu:
  - {text: Quit, quit: true}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.App.Name != "onto" {
		t.Errorf("Name = %q, want onto", s.App.Name)
	}
	if s.App.ID != "dev.onto.menubar" {
		t.Errorf("ID = %q, want dev.onto.menubar", s.App.ID)
	}
	if s.App.Icon.Symbol != "rectangle.3.group" {
		t.Errorf("Icon = %+v, want rectangle.3.group", s.App.Icon)
	}
	if s.App.Interval != 5*time.Second {
		t.Errorf("Interval = %v, want 5s", s.App.Interval)
	}
}
