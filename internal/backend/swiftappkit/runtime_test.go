package swiftappkit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

func timed(_ body: () -> RunOutcome) -> (RunOutcome, Double) {
    let t = Date()
    let o = body()
    return (o, Date().timeIntervalSince(t))
}

let (hung, hungFor) = timed { Watcher.run(["sleep", "30"], timeout: 0.5) }
print("hung ok=\(hung.ok) code=\(hung.code) prompt=\(hungFor < 5) said=\(hung.err.contains("stopped after 0.5s"))")

// Ignoring SIGTERM, which the shell's sleep inherits, leaves SIGKILL to end it.
let (stubborn, stubbornFor) = timed { Watcher.run(["sh", "-c", "trap '' TERM; sleep 30"], timeout: 0.5) }
print("stubborn code=\(stubborn.code) prompt=\(stubbornFor < 8)")

// The sleep outlives sh and holds both pipes; the watch must not wait for it.
let (orphan, orphanFor) = timed { Watcher.run(["sh", "-c", "sleep 30 & echo $!"]) }
let stray = pid_t(orphan.out.trimmingCharacters(in: .whitespacesAndNewlines)) ?? 0
print("orphan ok=\(orphan.ok) left=\(stray > 0 && kill(stray, 0) == 0) prompt=\(orphanFor < 5)")
if stray > 0 { kill(stray, SIGKILL) }

// A timeout stops the whole group, not only the process it spawned.
let tree = Watcher.run(["sh", "-c", "sleep 31 & echo $!; wait"], timeout: 0.5)
let child = pid_t(tree.out.trimmingCharacters(in: .whitespacesAndNewlines)) ?? 0
usleep(200_000)
print("tree code=\(tree.code) child gone=\(child > 0 && kill(child, 0) != 0)")

// The child exits on SIGTERM but its own child ignores it, so the group still
// needs the SIGKILL after the child is gone.
let survivor = Watcher.run(["sh", "-c", "trap 'exit 0' TERM; (trap '' TERM; exec sleep 32) & echo $!; wait"], timeout: 0.3)
let kept = pid_t(survivor.out.trimmingCharacters(in: .whitespacesAndNewlines)) ?? 0
usleep(200_000)
print("survivor code=\(survivor.code) gone=\(kept > 0 && kill(kept, 0) != 0)")
if kept > 0 { kill(kept, SIGKILL) }

let ordered = ["2", "2.5", "\"b\"", "true"].map { JSONValue.parse($0) }
print("order=\(ordered[0].order(ordered[1]) ?? 9)|\(ordered[1].order(ordered[0]) ?? 9)|\(ordered[2].order(ordered[2]) ?? 9)|\(ordered[0].order(ordered[2]) ?? 9)|\(ordered[3].order(ordered[3]) ?? 9)")
let keyed = JSONValue.parse("{\"k\": [10, 20]}")
print("dynamic=\(keyed[JSONValue("k")][JSONValue(1)].asInt)|\(keyed[JSONValue("k")][JSONValue(1.0)].asInt)|\(keyed[JSONValue(true)].exists)")
print("past end=\([String]().at(0))|\([1, 2].at(-1))|\([[3]].at(5).count)|\([1, 2].at(1))")

let flood = Watcher.run(["sh", "-c", "head -c 5000000 /dev/zero | tr '\\0' 'x'"])
print("flood ok=\(flood.ok) out=\(flood.out.count) said=\(flood.err.contains("stdout cut at"))")

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
Act.run(["echo", "hi"]) { _ in repolled = true }
let until = Date().addingTimeInterval(20)
while !repolled && Date() < until {
    RunLoop.main.run(mode: .default, before: Date().addingTimeInterval(0.05))
}
print("run repolled=\(repolled)")

// Every action completes exactly once with whether it worked, which is what
// lets a failed quit-button action cancel the quit.
var alerted: [String] = []
Act.alert = { command, _ in alerted.append(command) }
func outcome(_ action: MenuAction) -> String {
    var results: [Bool] = []
    Act.perform(action) { results.append($0) }
    let until = Date().addingTimeInterval(10)
    while results.isEmpty && Date() < until {
        RunLoop.main.run(until: Date().addingTimeInterval(0.05))
    }
    RunLoop.main.run(until: Date().addingTimeInterval(0.1))
    return results.map(String.init).joined(separator: ",")
}
print("done run=\(outcome(.run(["true"]))) fail=\(outcome(.run(["false"]))) open=\(outcome(.open("/no/such/perch/path"))) badurl=\(outcome(.post(url: "", body: "{}"))) alerts=\(alerted.count)")
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
		"hung ok=false code=-1 prompt=true said=true",
		"stubborn code=-1 prompt=true",
		"orphan ok=true left=true prompt=true",
		"tree code=-1 child gone=true",
		"survivor code=-1 gone=true",
		"order=-1|1|0|9|9",
		"dynamic=20|20|false",
		"past end=|0|0|2",
		"flood ok=true out=4194304 said=true",
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
		"done run=true fail=false open=false badurl=false alerts=3",
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
        case .item(let t, _, let look): return t + (look.badges.isEmpty ? "" : "{" + look.badges.map(\.text).joined() + "}")
        case .toggle(let t, let on, _, _): return (on ? "✓" : "") + t
        case .submenu(let t, let items, let look):
            let mark = if case .symbol(let name)? = look.icon { "[" + name + "]" } else { "" }
            return t + mark + "(" + show(items) + ")"
        case .field(let t, _): return "[" + t + "]"
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
print("icon=\(show(tidy([.submenu("s", [.item("x", nil)], look: Look(icon: .symbol("gear")))])))")
print("emptied=\(show(tidy([.submenu("s", [.separator]), .item("b", nil)])))")
print("header=\(show(tidy([.submenu("h", [.item("x", nil)], look: Look(header: true)), .item("a", .quit, look: Look(header: true)), .submenu("s", [.item("y", nil)])])))")
print("badged=\(show(tidy([.submenu("s", [.separator], look: Look(badges: [MenuBadge(text: "x")]))])))")
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
		"icon=s[gear](x)",
		"emptied=s,b",
		"header=h,a,s(y)",
		"badged=s{x}",
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

// applicationShouldTerminate under a real logout cannot be tested without
// logging out, so this drives the discriminator it hangs on. Getting it wrong
// means either a helper that blocks someone's logout with a modal nobody is
// there to dismiss, or one that stops asking at all.
const quitReasonProbe = `
import AppKit

func withReason(_ code: OSType) -> NSAppleEventDescriptor {
    let event = NSAppleEventDescriptor.appleEvent(
        withEventClass: OSType(kCoreEventClass),
        eventID: OSType(kAEQuitApplication),
        targetDescriptor: nil,
        returnID: 0,
        transactionID: 0)
    event.setAttribute(NSAppleEventDescriptor(enumCode: code), forKeyword: AEKeyword(kAEQuitReason))
    return event
}

print("logout=\(Quit.isUserInitiated(withReason(OSType(kAELogOut))))")
print("shutdown=\(Quit.isUserInitiated(withReason(OSType(kAEShutDown))))")
print("restart=\(Quit.isUserInitiated(withReason(OSType(kAERestart))))")
print("quitall=\(Quit.isUserInitiated(withReason(OSType(kAEQuitAll))))")
print("noevent=\(Quit.isUserInitiated(nil))")

let bare = NSAppleEventDescriptor.appleEvent(
    withEventClass: OSType(kCoreEventClass),
    eventID: OSType(kAEQuitApplication),
    targetDescriptor: nil,
    returnID: 0,
    transactionID: 0)
print("noreason=\(Quit.isUserInitiated(bare))")
`

func TestQuitReasonSeparatesLogoutFromAQuit(t *testing.T) {
	bin := buildProbe(t, quitReasonProbe)
	out, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}
	want := []string{
		"logout=false",
		"shutdown=false",
		"restart=false",
		"quitall=false",
		// NSApp.terminate from our own menus and from the Dock tile carries no
		// reason at all, so an absent one has to read as a deliberate quit.
		"noevent=true",
		"noreason=true",
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

// drawProbe hands Draw.menu one of each node that can carry an icon, and says
// what AppKit was left holding.
const drawProbe = `
import AppKit

func describe(_ item: NSMenuItem) -> String {
    guard let image = item.image else { return item.title + "=none" }
    return item.title + "=" + (image.isTemplate ? "template" : "art")
}

let menu = NSMenu()
Draw.menu([
    .item("plain", nil),
    .item("label", nil, look: Look(icon: .symbol("gearshape"))),
    .item("action", .quit, look: Look(icon: .symbol("power"))),
    .submenu("sub", [.item("inner", nil, look: Look(icon: .symbol("star")))], look: Look(icon: .symbol("folder"))),
], into: menu, repoll: {})
for item in menu.items { print(describe(item)) }
if let inner = menu.items.last?.submenu?.items.first { print(describe(inner)) }
`

func TestDrawGivesAMenuItemItsIcon(t *testing.T) {
	bin := buildProbe(t, drawProbe)
	out, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}
	want := "plain=none\nlabel=template\naction=template\nsub=template\ninner=template\n"
	if string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

// assetProbe loads artwork by a name only known at run time. A CLI's main
// bundle is the directory it runs from, so ok.png is found and ../evil.png is
// one step outside it.
const assetProbe = `
import AppKit

_ = NSApplication.shared
for name in ["ok", "../evil", "ok/..", ""] {
    print(name + "=" + (MenuIcon.asset(name).menuImage() == nil ? "none" : "image"))
}
`

func TestAssetNameResolvedAtRunTimeStaysInTheBundle(t *testing.T) {
	bin := buildProbe(t, assetProbe)
	dir := filepath.Dir(bin)
	png, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "recipes", "icons", "ci-pass.png"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(dir, "ok.png"), filepath.Join(dir, "..", "evil.png")} {
		if err := os.WriteFile(p, png, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("the probe died: %v\n%s", err, out)
	}
	want := "ok=image\n../evil=none\nok/..=none\n=none\n"
	if string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}
