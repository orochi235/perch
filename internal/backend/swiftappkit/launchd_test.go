package swiftappkit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// launchdProbe reports what the launchagent watch would bind, for each
// label/plist pair on its command line.
const launchdProbe = `
import Foundation

var args = Array(CommandLine.arguments.dropFirst())
while args.count >= 2 {
    let o = Watcher.launchAgent(label: args[0], plist: args[1])
    print("\(args[0]) installed=\(o.installed) loaded=\(o.loaded) running=\(o.running) hasPid=\(o.pid > 0)")
    args = Array(args.dropFirst(2))
}
`

// agentPlist is a LaunchAgent the test owns. RunAtLoad decides the case it
// stands for: true is a job with a process, false is one launchd knows about
// and has never started.
func agentPlist(label string, argv []string, runAtLoad bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>%s</string>
	<key>ProgramArguments</key><array>`, label)
	for _, a := range argv {
		fmt.Fprintf(&b, "<string>%s</string>", a)
	}
	b.WriteString("</array>\n\t<key>RunAtLoad</key><")
	if runAtLoad {
		b.WriteString("true/>")
	} else {
		b.WriteString("false/>")
	}
	b.WriteString("\n</dict>\n</plist>\n")
	return b.String()
}

// A launchagent watch is the one watch whose answer cannot be read off an exit
// status: `launchctl print` succeeds for any label launchd holds, including a
// job that has never run or has run and exited. Nothing else here can catch
// that — a golden, a typecheck and a compile all stay green while `running`
// means `loaded`, and the widget then says Running about an idle job.
func TestLaunchAgentWatchSeparatesLoadedFromRunning(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd is macOS only")
	}
	out, err := exec.Command("launchctl", "managername").Output()
	if err != nil || strings.TrimSpace(string(out)) != "Aqua" {
		t.Skipf("not a GUI login session (%s); gui/<uid> has no domain to bootstrap into", out)
	}

	dir := t.TempDir()
	suffix := strconv.Itoa(os.Getpid())
	domain := "gui/" + strconv.Itoa(os.Getuid())

	agents := []struct {
		label     string
		argv      []string
		runAtLoad bool
		want      string
	}{
		{"dev.perch.test.running." + suffix, []string{"/bin/sleep", "120"}, true,
			"installed=true loaded=true running=true hasPid=true"},
		{"dev.perch.test.idle." + suffix, []string{"/usr/bin/true"}, false,
			"installed=true loaded=true running=false hasPid=false"},
	}

	var probeArgs []string
	for _, a := range agents {
		plist := filepath.Join(dir, a.label+".plist")
		if err := os.WriteFile(plist, []byte(agentPlist(a.label, a.argv, a.runAtLoad)), 0o644); err != nil {
			t.Fatal(err)
		}
		label := a.label
		// Registered before bootstrapping, so a failure part way through still
		// leaves nothing loaded.
		t.Cleanup(func() { _ = exec.Command("launchctl", "bootout", domain+"/"+label).Run() })
		if out, err := exec.Command("launchctl", "bootstrap", domain, plist).CombinedOutput(); err != nil {
			t.Fatalf("bootstrap %s: %v\n%s", a.label, err, out)
		}
		probeArgs = append(probeArgs, a.label, plist)
	}

	// A label nothing has bootstrapped, from a plist that is not there.
	absent := "dev.perch.test.absent." + suffix
	probeArgs = append(probeArgs, absent, filepath.Join(dir, absent+".plist"))

	bin := buildProbe(t, launchdProbe)
	var got []string
	// sleep is running the moment launchd has spawned it, which bootstrap does
	// not wait for.
	deadline := time.Now().Add(10 * time.Second)
	for {
		raw, err := exec.Command(bin, probeArgs...).CombinedOutput()
		if err != nil {
			t.Fatalf("probe: %v\n%s", err, raw)
		}
		got = strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		if strings.Contains(got[0], "running=true") || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	want := make([]string, 0, len(agents)+1)
	for _, a := range agents {
		want = append(want, a.label+" "+a.want)
	}
	want = append(want, absent+" installed=false loaded=false running=false hasPid=false")

	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got[i], want[i])
		}
	}
}

// restartProbe restarts a LaunchAgent twice: once while it is running, and once
// after it has been booted out. Both have to end with the job up.
const restartProbe = `
import Foundation

let label = CommandLine.arguments[1]
let plist = CommandLine.arguments[2]

func settled() -> LaunchAgentOutcome {
    var o = Watcher.launchAgent(label: label, plist: plist)
    let deadline = Date().addingTimeInterval(10)
    while !o.running && Date() < deadline {
        Thread.sleep(forTimeInterval: 0.1)
        o = Watcher.launchAgent(label: label, plist: plist)
    }
    return o
}

func restart(_ what: String) {
    let before = Watcher.launchAgent(label: label, plist: plist)
    var repolled = false
    Act.agent(label: label, plist: plist, verb: .restart) { repolled = true }
    let until = Date().addingTimeInterval(30)
    while !repolled && Date() < until {
        RunLoop.main.run(mode: .default, before: Date().addingTimeInterval(0.05))
    }
    let after = settled()
    print("\(what) repolled=\(repolled) running=\(after.running) newPid=\(after.pid > 0 && after.pid != before.pid)")
}

_ = settled()
restart("fromRunning")
_ = Watcher.run(["launchctl", "bootout", Launchd.target(label)])
print("stopped loaded=\(Watcher.launchAgent(label: label, plist: plist).loaded)")
restart("fromStopped")
`

// Restart is the verb the schema could not express before: bootout returns
// before launchd has released the label, so bootout-then-bootstrap as two menu
// items races, and `kickstart -k` — what the old recipe used instead — needs
// the job already loaded and so cannot start a stopped one.
func TestAgentRestartFromRunningAndFromStopped(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd is macOS only")
	}
	out, err := exec.Command("launchctl", "managername").Output()
	if err != nil || strings.TrimSpace(string(out)) != "Aqua" {
		t.Skipf("not a GUI login session (%s); gui/<uid> has no domain to bootstrap into", out)
	}

	dir := t.TempDir()
	label := "dev.perch.test.restart." + strconv.Itoa(os.Getpid())
	domain := "gui/" + strconv.Itoa(os.Getuid())
	plist := filepath.Join(dir, label+".plist")
	if err := os.WriteFile(plist, []byte(agentPlist(label, []string{"/bin/sleep", "120"}, true)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = exec.Command("launchctl", "bootout", domain+"/"+label).Run() })
	if out, err := exec.Command("launchctl", "bootstrap", domain, plist).CombinedOutput(); err != nil {
		t.Fatalf("bootstrap: %v\n%s", err, out)
	}

	bin := buildProbe(t, restartProbe)
	// A failed action raises an NSAlert, which in a command-line probe never
	// returns — so a wedge here is a real failure and has to be bounded.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, bin, label, plist).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("restart never finished:\n%s", raw)
	}
	if err != nil {
		t.Fatalf("probe: %v\n%s", err, raw)
	}

	want := []string{
		"fromRunning repolled=true running=true newPid=true",
		"stopped loaded=false",
		"fromStopped repolled=true running=true newPid=true",
	}
	got := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), raw)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, got[i], want[i])
		}
	}
}
