package celswift

import (
	"strings"
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

func TestPrefixedSpellsWatchesThroughTheResultsRecord(t *testing.T) {
	got, err := runWatch(t).Prefixed("self.results.").LowerCondition("fleet.ok")
	if err != nil {
		t.Fatalf("LowerCondition: %v", err)
	}
	if got != "self.results.fleet.ok" {
		t.Errorf("got %q", got)
	}
}

// typedWatch declares a shape, so .data and its fields have concrete types.
func typedWatch(t *testing.T) *Env {
	return env(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
watch: {fleet: {run: [x], json: true, shape: {jobs: [{id: string}]}}}
menu: [{text: Quit, quit: true}]
`)
}

func TestLowerTextRefusesAnObject(t *testing.T) {
	_, err := typedWatch(t).LowerText("fleet")
	if err == nil {
		t.Fatal("want an error; String(<struct>) does not compile")
	}
	if !strings.Contains(err.Error(), "no text form") {
		t.Errorf("error = %q, want it to say an object has no text form", err)
	}
}

func TestLowerTextRefusesAList(t *testing.T) {
	_, err := typedWatch(t).LowerText("fleet.data.jobs")
	if err == nil {
		t.Fatal("want an error; String(<array>) does not compile")
	}
	if !strings.Contains(err.Error(), "no text form") {
		t.Errorf("error = %q, want it to say a list has no text form", err)
	}
}
