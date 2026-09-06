package celswift

import (
	"strings"
	"testing"
)

// Every refusal here would otherwise emit Swift that does not compile, which
// reports the author's mistake against generated source they did not write.
func TestLowerRefusesIllTypedOperands(t *testing.T) {
	e := mixed(t)
	cases := []struct{ cel, want string }{
		{"!fleet.data.count", "want a bool"},
		{"fleet.data.count && fleet.ok", "want a bool"},
		{"fleet.ok && fleet.data.count", "want a bool"},
		{"fleet.data.count || fleet.ok", "want a bool"},
		{"fleet.data.count ? 1 : 0", "want a bool"},
		{`fleet.ok ? 1 : "x"`, "same type"},
		{"fleet.data.jobs[fleet.data.label]", "index must be an int"},
		{"fleet.data.label[0]", "cannot index"},
		{"fleet.ok[0]", "cannot index"},
		{"fleet.ok.size()", "size() needs a list or a string"},
		{"string(fleet.data.jobs)", "no text form"},
		{"fleet.data.label.startsWith(fleet.data.jobs)", "no text form"},
		{"fleet.ok.field", "cannot select"},
	}
	for _, c := range cases {
		got, err := e.LowerExpr(c.cel)
		if err == nil {
			t.Errorf("%s: want a refusal, lowered to %q", c.cel, got)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %q, want it to mention %q", c.cel, err, c.want)
		}
	}
}

// CEL accepts these spellings; perch's lowering must reject them rather than
// index past the end of an argument list.
// asString stringifies any scalar, so a string method on a number lowers rather
// than failing — a divergence from CEL, which rejects it at type-check time.
func TestLowerStringMethodsAcceptANumericReceiver(t *testing.T) {
	got, err := mixed(t).LowerExpr(`fleet.data.count.startsWith("1")`)
	if err != nil {
		t.Fatalf("LowerExpr: %v", err)
	}
	if got != `String(fleet.data.count).hasPrefix("1")` {
		t.Errorf("got %q", got)
	}
}

func TestLowerRefusesWrongArity(t *testing.T) {
	e := mixed(t)
	for _, src := range []string{
		"size()",
		"size(fleet.data.jobs, 1)",
		"fleet.data.jobs.size(1)",
		"string()",
		"string(1, 2)",
		"startsWith(fleet.data.label, \"x\")",
		"contains(fleet.data.label, \"x\")",
		"fleet.data.label.startsWith()",
		`fleet.data.label.startsWith("a", "b")`,
	} {
		if got, err := e.LowerExpr(src); err == nil {
			t.Errorf("%s: want a refusal, lowered to %q", src, got)
		}
	}
}

// An unlowerable expression must name the construct the way the author wrote
// it, not the way CEL spells the operator internally.
func TestLowerRefusalNamesTheConstructNotCELsSpelling(t *testing.T) {
	_, err := mixed(t).LowerExpr("fleet.data.count + 1")
	if err == nil {
		t.Fatal("want a refusal")
	}
	if strings.Contains(err.Error(), "_+_") {
		t.Errorf("error leaks CEL's operator spelling: %q", err)
	}
	if !strings.Contains(err.Error(), "does not implement +") {
		t.Errorf("error = %q, want it to name +", err)
	}
}

func TestLowerNamesTheBoundWatchesWhenANameIsUnknown(t *testing.T) {
	_, err := mixed(t).LowerExpr("nope.ok")
	if err == nil {
		t.Fatal("want a refusal")
	}
	for _, name := range []string{"raw", "fleet"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error = %q, want it to list %q as bound", err, name)
		}
	}
}

// With nothing bound there is no list to offer, and saying "bound here:" with
// an empty list reads as a bug in perch.
func TestLowerSaysNoWatchesAreDeclaredWhenNoneAre(t *testing.T) {
	e := env(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
menu: [{text: Quit, quit: true}]
`)
	_, err := e.LowerExpr("fleet.ok")
	if err == nil {
		t.Fatal("want a refusal")
	}
	if !strings.Contains(err.Error(), "no watches are declared") {
		t.Errorf("error = %q, want it to say no watches are declared", err)
	}
}

func TestLowerRefusesSyntaxCELItselfRejects(t *testing.T) {
	e := mixed(t)
	for _, src := range []string{"", "fleet.", "((", `"unterminated`} {
		if got, err := e.LowerExpr(src); err == nil {
			t.Errorf("%q: want a refusal, lowered to %q", src, got)
		}
	}
}
