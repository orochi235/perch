package celswift

import (
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

func env(t *testing.T, doc string) *Env {
	t.Helper()
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	return NewEnv(s.Watches)
}

// runWatch is an untyped run watch: .data is dynamic.
func runWatch(t *testing.T) *Env {
	return env(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {fleet: {run: [x], json: true}}
menu: [{text: Quit, quit: true}]
`)
}

func TestLowerConditionWatchOK(t *testing.T) {
	got, err := runWatch(t).LowerCondition("fleet.ok")
	if err != nil {
		t.Fatalf("LowerCondition: %v", err)
	}
	if got != "fleet.ok" {
		t.Errorf("got %q, want %q", got, "fleet.ok")
	}
}

func TestLowerConditionNegation(t *testing.T) {
	got, err := runWatch(t).LowerCondition("!fleet.ok")
	if err != nil {
		t.Fatalf("LowerCondition: %v", err)
	}
	if got != "!(fleet.ok)" {
		t.Errorf("got %q, want %q", got, "!(fleet.ok)")
	}
}

func TestLowerConditionRejectsUnknownWatch(t *testing.T) {
	_, err := runWatch(t).LowerCondition("server.ok")
	if err == nil {
		t.Fatal("want an error naming the unknown watch, got nil")
	}
}

func TestLowerConditionRejectsUnknownWatchField(t *testing.T) {
	_, err := runWatch(t).LowerCondition("fleet.okay")
	if err == nil {
		t.Fatal("want an error for a field a watch does not bind, got nil")
	}
}

func TestLowerConditionRejectsNonBool(t *testing.T) {
	_, err := runWatch(t).LowerCondition("fleet.code")
	if err == nil {
		t.Fatal("want an error: a condition must be a bool, got nil")
	}
}
