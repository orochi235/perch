package swiftappkit

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// runtimeProbe drives Runtime.swift from a command line. The golden and
// typecheck tests both stay green while the runtime hangs or traps, which is
// what this covers: a wedged poll freezes the menu at the last good results,
// and a trap takes the whole status item down.
const runtimeProbe = `
import Foundation

// A child that fills the stderr pipe before it finishes writing stdout.
let noisy = Watcher.run(["sh", "-c", "head -c 300000 /dev/zero | tr '\\0' 'x' >&2; echo done"])
print("noisy ok=\(noisy.ok) out=\(noisy.out.trimmingCharacters(in: .whitespacesAndNewlines)) err=\(noisy.err.count)")

let failed = Watcher.run(["sh", "-c", "echo oops >&2; exit 3"])
print("failed ok=\(failed.ok) code=\(failed.code) err=\(failed.err.trimmingCharacters(in: .whitespacesAndNewlines))")

let empty = Watcher.run([])
print("empty ok=\(empty.ok) code=\(empty.code)")

// Whatever a watch prints reaches these, so neither may trap.
print("huge int=\(JSONValue.parse("1e30").asInt)")
print("huge string=\(JSONValue.parse("{\"v\": 1e30}")["v"].asString)")
print("round string=\(JSONValue.parse("[2.0]")[0].asString)")
print("missing=\(JSONValue.parse("{}")["nope"].asString)|\(JSONValue.parse("{}")["nope"].exists)")
print("garbage=\(JSONValue.parse("not json").exists)")
print("outofrange=\(JSONValue.parse("[1]")[7].exists)")

print("exists=\(Watcher.exists("~"))|\(Watcher.exists("/no/such/path"))")

// Every lowered expression bottoms out in one of these, so a coercion that is
// wrong shows as a widget quietly displaying the wrong thing. asInt rounds
// rather than truncating: 2.5 is 3.
let vals = ["null", "true", "false", "0", "7", "0.0", "2.5", "\"\"", "\"x\"", "[]", "[1]", "{}", "{\"a\":1}"]
for v in vals {
    let j = JSONValue.parse(v)
    print("\(v) bool=\(j.asBool) int=\(j.asInt) double=\(j.asDouble) string=\(j.asString) size=\(j.size) array=\(j.asArray.count)")
}
print("contains=\(JSONValue.parse("[1,2]").contains(JSONValue(2)))|\(JSONValue.parse("[1,2]").contains(JSONValue(3)))|\(JSONValue.parse("{}").contains(JSONValue(1)))")

// An action item carries its own closure; a menu built from one that never
// fires opens and does nothing.
var fired = 0
let item = ActionItem(title: "Go") { fired += 1 }
_ = item.target?.perform(item.action)
print("item title=\(item.title) fired=\(fired)")

// run dispatches and repolls, so acting on the menu visibly changes it.
var repolled = false
Act.run(["echo", "hi"]) { repolled = true }
let until = Date().addingTimeInterval(20)
while !repolled && Date() < until {
    RunLoop.main.run(mode: .default, before: Date().addingTimeInterval(0.05))
}
print("run repolled=\(repolled)")
`

func TestRuntimeBehavior(t *testing.T) {
	bin := buildProbe(t, runtimeProbe)

	// A deadlocked drain shows up as a test that never returns, so bound it.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("the runtime did not finish; a poll that never returns freezes the menu\n%s", out)
	}
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}

	want := []string{
		"noisy ok=true out=done err=300000",
		"failed ok=false code=3 err=oops",
		"empty ok=false code=-1",
		"huge int=0",
		"huge string=1e+30",
		"round string=2",
		"missing=|false",
		"garbage=false",
		"outofrange=false",
		"exists=true|false",
		"null bool=false int=0 double=0.0 string= size=0 array=0",
		"true bool=true int=1 double=1.0 string=true size=0 array=0",
		"false bool=false int=0 double=0.0 string=false size=0 array=0",
		"0 bool=false int=0 double=0.0 string=0 size=0 array=0",
		"7 bool=true int=7 double=7.0 string=7 size=0 array=0",
		"0.0 bool=false int=0 double=0.0 string=0 size=0 array=0",
		"2.5 bool=true int=3 double=2.5 string=2.5 size=0 array=0",
		`"" bool=false int=0 double=0.0 string= size=0 array=0`,
		`"x" bool=true int=0 double=0.0 string=x size=1 array=0`,
		"[] bool=false int=0 double=0.0 string= size=0 array=0",
		"[1] bool=true int=0 double=0.0 string= size=1 array=1",
		"{} bool=false int=0 double=0.0 string= size=0 array=0",
		`{"a":1} bool=true int=0 double=0.0 string= size=1 array=0`,
		"contains=true|false|false",
		"item title=Go fired=1",
		"run repolled=true",
	}
	got := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), out)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got[i], want[i])
		}
	}
}

// tidyProbe drives the separator pass. A menu is rebuilt from the last poll
// every time it opens, so which items are missing — and therefore which
// dividers have nothing beside them — is not knowable where they are written.
const tidyProbe = `
import Foundation

func show(_ nodes: [MenuNode]) -> String {
    nodes.map { node in
        switch node {
        case .separator: return "-"
        case .item(let t, _): return t
        case .submenu(let t, let items): return t + "(" + show(items) + ")"
        }
    }.joined(separator: ",")
}

print("empty=\(show(tidy([])))")
print("allseps=\(show(tidy([.separator, .separator])))")
print("leading=\(show(tidy([.separator, .item("a", nil)])))")
print("trailing=\(show(tidy([.item("a", nil), .separator])))")
print("run=\(show(tidy([.item("a", nil), .separator, .separator, .separator, .item("b", nil)])))")
print("kept=\(show(tidy([.item("a", nil), .separator, .item("b", nil)])))")
print("nested=\(show(tidy([.submenu("s", [.separator, .item("x", nil), .separator]), .separator])))")
`

func TestSeparatorsWithNothingBesideThemAreDropped(t *testing.T) {
	bin := buildProbe(t, tidyProbe)
	out, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}
	want := []string{
		"empty=",
		"allseps=",
		"leading=a",
		"trailing=a",
		"run=a,-,b",
		"kept=a,-,b",
		"nested=s(x)",
	}
	got := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), out)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got[i], want[i])
		}
	}
}
