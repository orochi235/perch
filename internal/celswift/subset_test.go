package celswift

import "testing"

// shaped binds a typed watch, so field names and element types are checked.
func shaped(t *testing.T) *Env {
	return env(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
watch:
  fleet:
    run: [x]
    json: true
    shape:
      count: int
      label: string
      jobs: [{id: string, node: string}]
      tags: [string]
  plist: {exists: /tmp/p}
menu: [{text: Quit, quit: true}]
`)
}

func TestLowerSupportedSubset(t *testing.T) {
	e := shaped(t)
	cases := []struct{ cel, want string }{
		// literals and comparison
		{"fleet.data.count == 0", "(fleet.data.count == 0)"},
		{"fleet.data.count != 3", "(fleet.data.count != 3)"},
		{"fleet.data.count > 1", "(fleet.data.count > 1)"},
		{"fleet.data.count >= 1", "(fleet.data.count >= 1)"},
		{"fleet.data.count < 1", "(fleet.data.count < 1)"},
		{"fleet.data.count <= 1", "(fleet.data.count <= 1)"},
		{`fleet.data.label == "idle"`, `(fleet.data.label == "idle")`},
		// boolean operators
		{"fleet.ok && plist.ok", "(fleet.ok && plist.ok)"},
		{"fleet.ok || plist.ok", "(fleet.ok || plist.ok)"},
		{"!fleet.ok", "!(fleet.ok)"},
		// ternary
		{"fleet.ok ? 1 : 0", "(fleet.ok ? 1 : 0)"},
		// size, both spellings
		{"fleet.data.jobs.size() == 0", "(fleet.data.jobs.count == 0)"},
		{"size(fleet.data.jobs) == 0", "(fleet.data.jobs.count == 0)"},
		{"fleet.data.label.size() > 0", "(fleet.data.label.count > 0)"},
		// indexing
		{`fleet.data.jobs[0].id == "x"`, `(fleet.data.jobs[0].id == "x")`},
		// string functions
		{`fleet.data.label.startsWith("id")`, `fleet.data.label.hasPrefix("id")`},
		{`fleet.data.label.contains("dl")`, `fleet.data.label.contains("dl")`},
		{`string(fleet.data.count) == "3"`, `(String(fleet.data.count) == "3")`},
		// in
		{`"a" in fleet.data.tags`, `fleet.data.tags.contains("a")`},
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

func TestLowerUntypedDataSubscripts(t *testing.T) {
	got, err := runWatch(t).LowerExpr("fleet.data.jobs.size() == 0")
	if err != nil {
		t.Fatalf("LowerExpr: %v", err)
	}
	if got != `(fleet.data["jobs"].size == 0)` {
		t.Errorf("got %q", got)
	}
}

func TestLowerRejectsUnsupportedConstructs(t *testing.T) {
	e := shaped(t)
	for _, src := range []string{
		"fleet.data.jobs.map(x, x.id)",
		"fleet.data.jobs.filter(x, x.id == 'a')",
		"fleet.data.jobs.all(x, x.id != '')",
		"timestamp(0)",
		"fleet.data.count + 1",
		"fleet.data.count - 1",
		"[1, 2, 3]",
		"{'a': 1}",
		"fleet.data.label.matches('x')",
	} {
		got, err := e.LowerExpr(src)
		if err == nil {
			t.Errorf("%s: want a refusal, lowered to %q", src, got)
		}
	}
}

func TestLowerRefusalQuotesTheExpression(t *testing.T) {
	_, err := shaped(t).LowerExpr("fleet.data.jobs.map(x, x.id)")
	if err == nil {
		t.Fatal("want an error")
	}
	if !contains(err.Error(), "fleet.data.jobs.map(x, x.id)") {
		t.Errorf("error = %q, want it to quote the offending expression", err)
	}
}

func TestLowerRejectsMisspelledShapeField(t *testing.T) {
	_, err := shaped(t).LowerExpr("fleet.data.jbos.size() == 0")
	if err == nil {
		t.Fatal("a declared shape must turn a misspelled field into an error")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
