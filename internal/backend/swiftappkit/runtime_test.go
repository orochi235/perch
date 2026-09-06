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
`

func TestRuntimeBehavior(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Runtime.swift"), runtimeSwift, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.swift"), []byte(runtimeProbe), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "probe")
	build := exec.Command(swiftc, "-o", bin, filepath.Join(dir, "Runtime.swift"), filepath.Join(dir, "main.swift"))
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compiling the probe: %v\n%s", err, out)
	}

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
