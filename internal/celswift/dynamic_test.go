package celswift

import (
	"strings"
	"testing"
)

// mixed binds one untyped watch and one with a declared shape, so an expression
// can put a dynamic operand and a concrete one on either side of the same
// comparison.
func mixed(t *testing.T) *Env {
	return env(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
watch:
  raw: {run: [x], json: true}
  fleet:
    run: [y]
    json: true
    shape:
      count: int
      label: string
      jobs: [{id: string}]
      tags: [string]
menu: [{text: Quit, quit: true}]
`)
}

// A dynamic operand takes the type of whatever it is compared against, because
// JSONValue has no == of its own against a Swift scalar.
func TestLowerCoercesDynamicOperandsToTheConcreteSide(t *testing.T) {
	e := mixed(t)
	cases := []struct{ cel, want string }{
		{`raw.data.n == 3`, `(raw.data["n"].asInt == 3)`},
		{`3 == raw.data.n`, `(3 == raw.data["n"].asInt)`},
		{`raw.data.f > 1.5`, `(raw.data["f"].asDouble > 1.5)`},
		{`raw.data.s == "x"`, `(raw.data["s"].asString == "x")`},
		{`raw.data.b == true`, `(raw.data["b"].asBool == true)`},
		{`raw.data.n == fleet.data.count`, `(raw.data["n"].asInt == fleet.data.count)`},
		// Two dynamic operands stay JSONValue on both sides.
		{`raw.data.a == raw.data.b`, `(raw.data["a"] == raw.data["b"])`},
	}
	for _, c := range cases {
		got, err := e.LowerExpr(c.cel)
		if err != nil {
			t.Errorf("%s: %v", c.cel, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s\n got %q\nwant %q", c.cel, got, c.want)
		}
	}
}

func TestLowerRefusesToCompareAgainstAListOrObject(t *testing.T) {
	e := mixed(t)
	for _, src := range []string{
		"raw.data.x == fleet.data.jobs",
		"fleet.data.jobs == raw.data.x",
		"fleet.data.jobs == fleet.data.jobs",
		"raw == fleet",
		"fleet.data.label == fleet.data.count",
	} {
		got, err := e.LowerExpr(src)
		if err == nil {
			t.Errorf("%s: want a refusal, lowered to %q", src, got)
			continue
		}
		if !strings.Contains(err.Error(), "compare") {
			t.Errorf("%s: error = %q, want it to say the comparison is the problem", src, err)
		}
	}
}

// in against a dynamic list has to wrap the concrete element, because
// JSONValue.contains takes a JSONValue.
func TestLowerInWrapsAConcreteElementForADynamicList(t *testing.T) {
	e := mixed(t)
	cases := []struct{ cel, want string }{
		{`"a" in raw.data.tags`, `raw.data["tags"].contains(JSONValue("a"))`},
		{`fleet.data.count in raw.data.tags`, `raw.data["tags"].contains(JSONValue(fleet.data.count))`},
		{`raw.data.x in raw.data.tags`, `raw.data["tags"].contains(raw.data["x"])`},
	}
	for _, c := range cases {
		got, err := e.LowerExpr(c.cel)
		if err != nil {
			t.Errorf("%s: %v", c.cel, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s\n got %q\nwant %q", c.cel, got, c.want)
		}
	}
}

func TestLowerInRefusesAMismatchedOrNonList(t *testing.T) {
	e := mixed(t)
	for _, src := range []string{
		`1 in fleet.data.tags`,    // int against a list of strings
		`"a" in fleet.data.label`, // a string is not a list
		`"a" in fleet.ok`,
	} {
		if got, err := e.LowerExpr(src); err == nil {
			t.Errorf("%s: want a refusal, lowered to %q", src, got)
		}
	}
}

func TestLowerListAcceptsADynamicListAndRefusesAScalar(t *testing.T) {
	e := mixed(t)
	got, elem, err := e.LowerList("raw.data.jobs")
	if err != nil {
		t.Fatalf("LowerList: %v", err)
	}
	if got != `raw.data["jobs"].asArray` {
		t.Errorf("got %q", got)
	}
	if elem.Kind.String() != "any" {
		t.Errorf("element type = %v, want any", elem.Kind)
	}
	if _, _, err := e.LowerList("fleet.ok"); err == nil {
		t.Error("want a refusal: each: over a bool")
	}
	if _, _, err := e.LowerList("fleet.data.label"); err == nil {
		t.Error("want a refusal: each: over a string")
	}
}

// has() is answered at build time against a declared shape and at run time
// against a dynamic one; neither reaches the CEL runtime.
func TestLowerHasIsConstantOnAShapeAndAnExistenceCheckOnDynamicData(t *testing.T) {
	e := mixed(t)
	got, err := e.LowerExpr("has(fleet.data.count)")
	if err != nil {
		t.Fatalf("LowerExpr: %v", err)
	}
	if got != "true" {
		t.Errorf("got %q, want true", got)
	}
	got, err = e.LowerExpr("has(raw.data.count)")
	if err != nil {
		t.Fatalf("LowerExpr: %v", err)
	}
	if got != `raw.data["count"].exists` {
		t.Errorf("got %q", got)
	}
	if _, err := e.LowerExpr("has(fleet.data.nope)"); err == nil {
		t.Error("has() of a field a declared shape lacks must still be an error")
	}
}

func TestLowerLiterals(t *testing.T) {
	e := mixed(t)
	cases := []struct{ cel, want string }{
		{"3", "3"},
		{"3u", "3"},
		{"1.5", "1.5"},
		{"true", "true"},
		{`"x"`, `"x"`},
	}
	for _, c := range cases {
		got, err := e.LowerExpr(c.cel)
		if err != nil {
			t.Errorf("%s: %v", c.cel, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.cel, got, c.want)
		}
	}
	for _, src := range []string{"null", `b"x"`} {
		if got, err := e.LowerExpr(src); err == nil {
			t.Errorf("%s: want a refusal, lowered to %q", src, got)
		}
	}
}
