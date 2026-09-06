package celswift

import (
	"strings"
	"testing"
)

// The three watch kinds bind different records. Selecting a field one kind does
// not produce has to be a build error, not a widget reading an empty string.
func TestWatchKindsBindDifferentFields(t *testing.T) {
	e := env(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
watch:
  cmd: {run: [x]}
  web: {http: "http://127.0.0.1/health"}
  file: {exists: /tmp/p}
menu: [{text: Quit, quit: true}]
`)
	bound := map[string][]string{
		"cmd":  {"ok", "code", "out", "err"},
		"web":  {"ok", "status", "out"},
		"file": {"ok"},
	}
	unbound := map[string][]string{
		"cmd":  {"status", "data"},
		"web":  {"code", "err", "data"},
		"file": {"code", "out", "err", "status", "data"},
	}
	for watch, fields := range bound {
		for _, f := range fields {
			if _, err := e.LowerExpr(watch + "." + f); err != nil {
				t.Errorf("%s.%s: %v", watch, f, err)
			}
		}
	}
	for watch, fields := range unbound {
		for _, f := range fields {
			if got, err := e.LowerExpr(watch + "." + f); err == nil {
				t.Errorf("%s.%s: want a refusal, lowered to %q", watch, f, got)
			}
		}
	}
}

// json: true is what binds .data; without it a watch has text output only.
func TestDataIsBoundOnlyByJSON(t *testing.T) {
	e := env(t, `
app: {name: a, id: b, icon: circle, interval: 1s}
watch:
  plain: {run: [x]}
  decoded: {run: [y], json: true}
  web: {http: "http://127.0.0.1/health", json: true}
menu: [{text: Quit, quit: true}]
`)
	if got, err := e.LowerExpr("plain.data"); err == nil {
		t.Errorf("want a refusal for .data without json: true, lowered to %q", got)
	}
	for _, src := range []string{"decoded.data.x", "web.data.x"} {
		if _, err := e.LowerExpr(src); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
}

// it shadows a watch of the same name, so an each: over a list can bind an
// element without the surrounding watches becoming unreachable by other names.
func TestEachBindingShadowsAndUnbindsCleanly(t *testing.T) {
	e := mixed(t)
	_, elem, err := e.LowerList("fleet.data.jobs")
	if err != nil {
		t.Fatalf("LowerList: %v", err)
	}
	inner := e.WithEach(elem, "job")
	got, err := inner.LowerExpr("it.id")
	if err != nil {
		t.Fatalf("LowerExpr: %v", err)
	}
	if got != "job.id" {
		t.Errorf("got %q, want the generated loop variable", got)
	}
	if _, err := inner.LowerExpr("fleet.ok"); err != nil {
		t.Errorf("watches must stay reachable inside an each:: %v", err)
	}
	// WithEach copies, so the outer env is unchanged.
	if _, err := e.LowerExpr("it.id"); err == nil {
		t.Error("it leaked out of the each: it was bound for")
	}
}

// Prefixed must not touch it: the loop variable is a local, not a field of the
// results record.
func TestPrefixedLeavesTheEachVariableAlone(t *testing.T) {
	e := mixed(t)
	_, elem, err := e.LowerList("fleet.data.jobs")
	if err != nil {
		t.Fatalf("LowerList: %v", err)
	}
	inner := e.WithEach(elem, "job").Prefixed("self.results.")
	got, err := inner.LowerExpr("it.id")
	if err != nil {
		t.Fatalf("LowerExpr: %v", err)
	}
	if got != "job.id" {
		t.Errorf("got %q, want the prefix left off the loop variable", got)
	}
	got, err = inner.LowerExpr("fleet.ok")
	if err != nil {
		t.Fatalf("LowerExpr: %v", err)
	}
	if !strings.HasPrefix(got, "self.results.") {
		t.Errorf("got %q, want the watch reached through the results record", got)
	}
}
