package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The rest of the launchctl tests run against a fake, which cannot notice that
// the real one takes its arguments in a different shape. These drive the real
// launchctl with a label nothing has ever registered, so nothing loads and
// nothing is left behind — but a wrong domain or argument order still shows up
// as a command that does not run at all.
func TestCLIDrivesTheRealLaunchctl(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchctl is macOS only")
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		t.Skip("launchctl not on PATH")
	}
	c := NewCLI()
	if c.UID != os.Getuid() {
		t.Errorf("UID = %d, want the caller's %d", c.UID, os.Getuid())
	}
	if want := "gui/" + strconv.Itoa(os.Getuid()); c.domain() != want {
		t.Errorf("domain = %q, want %q", c.domain(), want)
	}

	const absent = "dev.perch.test.never-registered"
	if err := c.Print(absent); err == nil {
		t.Error("Print reported a label launchd has never seen as loaded")
	}
	if err := c.Bootout(absent); err == nil {
		t.Error("Bootout of an unloaded label reported success")
	}

	// Bootstrap's error carries launchctl's own output, which is the only place
	// the reason a load failed is ever written down.
	err := c.Bootstrap(filepath.Join(t.TempDir(), "not-a-plist.plist"))
	if err == nil {
		t.Fatal("Bootstrap of a missing plist reported success")
	}
	if !strings.Contains(err.Error(), "launchctl bootstrap") {
		t.Errorf("error = %q, want it to name the command", err)
	}
}

// Reload polls with this, so a zero Tries would bootstrap into the gap on the
// first pass and a missing Sleep would spin.
func TestDefaultWaitPollsForAboutTwoSeconds(t *testing.T) {
	w := DefaultWait()
	if w.Tries < 10 {
		t.Errorf("Tries = %d, too few to outlast launchd letting go of a label", w.Tries)
	}
	if w.Sleep == nil {
		t.Fatal("Sleep is nil; Reload would spin")
	}
	start := time.Now()
	w.Sleep()
	if d := time.Since(start); d < time.Millisecond {
		t.Errorf("Sleep returned in %v; Reload would spin", d)
	}
	if total := time.Duration(w.Tries) * 50 * time.Millisecond; total < time.Second || total > 10*time.Second {
		t.Errorf("the whole poll is %v, which is either too short to help or long enough to look hung", total)
	}
}

// SwiftC is the compiler every install actually uses; the bundle tests stand in
// for it, so nothing else would notice its arguments going wrong.
func TestSwiftCCompilesAndReportsFailure(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	_ = swiftc
	dir := t.TempDir()
	src := filepath.Join(dir, "main.swift")
	if err := os.WriteFile(src, []byte(`print("hi")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "prog")
	if err := SwiftC([]string{src}, out); err != nil {
		t.Fatalf("SwiftC: %v", err)
	}
	got, err := exec.Command(out).Output()
	if err != nil {
		t.Fatalf("running the compiled binary: %v", err)
	}
	if strings.TrimSpace(string(got)) != "hi" {
		t.Errorf("compiled binary printed %q", got)
	}

	bad := filepath.Join(dir, "bad.swift")
	if err := os.WriteFile(bad, []byte("this is not swift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = SwiftC([]string{bad}, filepath.Join(dir, "nope"))
	if err == nil {
		t.Fatal("SwiftC reported success on source that does not compile")
	}
	if !strings.Contains(err.Error(), "swiftc") {
		t.Errorf("error = %q, want it to name swiftc", err)
	}
}

// BuildBundle stages beside Dest rather than in the system temp dir, because
// the last step is a rename and a rename across filesystems fails. A Dest whose
// parent cannot be created has to surface as an error, not a panic.
func TestBuildBundleReportsAnUnusableDestination(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err := BuildBundle(opts(filepath.Join(file, "onto.app"), stubCompiler("X")))
	if err == nil {
		t.Fatal("want an error for a destination under a file, got nil")
	}
}

// The default compiler is swiftc, so an install that forgets to set one still
// builds rather than dereferencing a nil function.
func TestBuildBundleDefaultsToSwiftC(t *testing.T) {
	if _, err := exec.LookPath("swiftc"); err != nil {
		t.Skip("swiftc not on PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.swift")
	if err := os.WriteFile(src, []byte(`print("hi")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "onto.app")
	o := opts(dest, nil)
	o.Sources = []string{src}
	if err := BuildBundle(o); err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "Contents", "MacOS", "onto")); err != nil {
		t.Errorf("no executable in the bundle: %v", err)
	}
}

// Every plist perch writes is read by launchd, which parses it as XML. A
// template that drifts out of shape shows up there as a silent refusal to load.
func TestPlistsAreWellFormed(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("plutil is macOS only")
	}
	if _, err := exec.LookPath("plutil"); err != nil {
		t.Skip("plutil not on PATH")
	}
	dir := t.TempDir()
	files := map[string]string{
		"Info.plist": InfoPlist(App{Name: `a&b "c" <d>`, ID: "dev.a-b_c.menubar", Executable: "a&b"}),
		"agent.plist": AgentPlist("dev.a&b.menubar",
			"/Users/x/Applications/a&b.app/Contents/MacOS/a&b", "/a<b>:/usr/bin"),
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("plutil", "-lint", path).CombinedOutput(); err != nil {
			t.Errorf("%s is not a valid plist: %v\n%s\n%s", name, err, out, body)
		}
	}
	// launchd reads the label back out, so escaping has to survive the round trip.
	out, err := exec.Command("plutil", "-extract", "Label", "raw", filepath.Join(dir, "agent.plist")).Output()
	if err != nil {
		t.Fatalf("reading the label back: %v", err)
	}
	if strings.TrimSpace(string(out)) != "dev.a&b.menubar" {
		t.Errorf("label came back as %q", out)
	}
}
