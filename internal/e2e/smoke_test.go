package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The generated app is the deliverable, and nothing else here runs it. A golden
// test, a typecheck and a compile all stay green while the app traps on launch
// — which reaches the user as a menu bar with nothing in it.
//
// This installs a real LaunchAgent under a temporary HOME, with a label nothing
// else uses, and removes it again. Its status item is in the menu bar for the
// few seconds the test takes.
func TestInstalledAppStaysRunning(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd is macOS only")
	}
	if _, err := exec.LookPath("swiftc"); err != nil {
		t.Skip("swiftc not on PATH")
	}
	if out, err := exec.Command("launchctl", "managername").Output(); err != nil ||
		strings.TrimSpace(string(out)) != "Aqua" {
		t.Skipf("not a GUI login session (%s); a status item has nowhere to appear", out)
	}

	suffix := strconv.Itoa(os.Getpid())
	name, label := "perchSmoke"+suffix, "dev.perch.smoke."+suffix
	home := t.TempDir()
	dir := project(t, `
app: {name: `+name+`, id: `+label+`, icon: circle.dashed, interval: 2s}
watch:
  fleet:
    run: [echo, '{"jobs": [{"id": "j1"}]}']
    json: true
    shape: {jobs: [{id: string}]}
  here:
    exists: /usr/bin/true
status:
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - badge: "fleet.data.jobs.size()"
menu:
  - text: "{{fleet.data.jobs.size()}} running"
  - separator
  - each: fleet.data.jobs
    text: "job {{it.id}}"
    menu: [{text: Echo, run: [echo, "{{it.id}}"]}]
  - {text: Quit, quit: true}
`)

	// Registered before anything is installed, so a failure part way through
	// still leaves nothing loaded.
	t.Cleanup(func() {
		_ = exec.Command("launchctl", "bootout", domain()+"/"+label).Run()
		_ = os.RemoveAll(filepath.Join(home, "Applications", name+".app"))
		_ = os.RemoveAll(filepath.Join(home, "Library", "LaunchAgents", label+".plist"))
	})

	r := runWithHome(t, dir, home, "install")
	if r.status != 0 {
		t.Fatalf("install: status %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, name+" is running") {
		t.Errorf("stdout = %q", r.stdout)
	}

	pid := waitForPID(t, label, 15*time.Second)
	if pid == 0 {
		t.Fatalf("%s never reported a running process; the app did not start", label)
	}

	// KeepAlive restarts a crashing app, so a trap on launch shows as a new pid
	// rather than as nothing running at all.
	time.Sleep(6 * time.Second)
	again := currentPID(t, label)
	if again == 0 {
		t.Fatalf("%s stopped after starting; the app exited on its own", label)
	}
	if again != pid {
		t.Fatalf("%s was restarted (pid %d then %d); the app is crash-looping", label, pid, again)
	}

	r = runWithHome(t, dir, home, "uninstall")
	if r.status != 0 {
		t.Fatalf("uninstall: status %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	// bootout returns before launchd has let go of the label, which is the same
	// gap install.Reload polls across.
	if !waitForGone(t, label, 10*time.Second) {
		t.Error("launchd still knows the label well after uninstall")
	}
	for _, path := range []string{
		filepath.Join(home, "Applications", name+".app"),
		filepath.Join(home, "Library", "LaunchAgents", label+".plist"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s survived uninstall", path)
		}
	}
}

func domain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

var pidLine = regexp.MustCompile(`(?m)^\s*pid = (\d+)$`)

// currentPID is the process launchd is running for label, or 0 while there is
// none — which is both "not loaded" and "between restarts".
func currentPID(t *testing.T, label string) int {
	t.Helper()
	out, err := exec.Command("launchctl", "print", domain()+"/"+label).Output()
	if err != nil {
		return 0
	}
	m := pidLine.FindSubmatch(out)
	if m == nil {
		return 0
	}
	pid, _ := strconv.Atoi(string(m[1]))
	return pid
}

func waitForPID(t *testing.T, label string, within time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if pid := currentPID(t, label); pid != 0 {
			return pid
		}
		time.Sleep(200 * time.Millisecond)
	}
	return 0
}

// waitForGone reports whether launchd has let go of the label.
func waitForGone(t *testing.T, label string, within time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if err := exec.Command("launchctl", "print", domain()+"/"+label).Run(); err != nil {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func runWithHome(t *testing.T, dir, home string, args ...string) result {
	t.Helper()
	c := exec.Command(perch(t), args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "HOME="+home)
	var out, errOut strings.Builder
	c.Stdout, c.Stderr = &out, &errOut
	err := c.Run()
	r := result{stdout: out.String(), stderr: errOut.String()}
	if ee, ok := err.(*exec.ExitError); ok {
		r.status = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running perch %v: %v", args, err)
	}
	return r
}
