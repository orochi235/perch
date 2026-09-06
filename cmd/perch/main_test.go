package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/install"
	"github.com/orochi235/perch/internal/project"
)

const exampleSpec = `
app: {name: onto, id: dev.onto.menubar, icon: rectangle.3.group, interval: 5s}
watch:
  fleet:
    run: [echo, "{}"]
    json: true
    shape: {jobs: [{id: string}]}
status:
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - badge: "fleet.data.jobs.size()"
menu:
  - text: "{{fleet.data.jobs.size()}} running"
  - separator
  - {text: Quit, quit: true}
`

// harness is one invocation of perch: a project directory, a HOME nothing else
// reaches, and stand-ins for the compiler and launchd.
type harness struct {
	t       *testing.T
	dir     string
	home    string
	env     *env
	out     strings.Builder
	errOut  strings.Builder
	loaded  []string
	booted  []string
	sources []string
}

func newHarness(t *testing.T, doc string) *harness {
	t.Helper()
	h := &harness{t: t, dir: t.TempDir(), home: t.TempDir()}
	t.Setenv("HOME", h.home)
	if doc != "" {
		h.write(project.SpecFile, doc)
	}
	h.env = &env{
		out: &h.out,
		err: &h.errOut,
		compile: func(sources []string, out string) error {
			h.sources = append([]string{}, sources...)
			return os.WriteFile(out, []byte("BINARY"), 0o755)
		},
		launchd: h,
		wait:    install.Wait{Tries: 3, Sleep: func() {}},
		start:   func(bin string) error { h.booted = append(h.booted, bin); return nil },
	}
	return h
}

// The fake launchctl reports the label as gone straight away, so Reload's poll
// is exercised without any waiting.
func (h *harness) Print(label string) error { return errors.New("could not find service") }
func (h *harness) Bootout(label string) error {
	h.booted = append(h.booted, "bootout:"+label)
	return nil
}
func (h *harness) Bootstrap(plist string) error {
	h.loaded = append(h.loaded, plist)
	return nil
}

func (h *harness) write(name, body string) {
	h.t.Helper()
	path := filepath.Join(h.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) run(args ...string) int {
	h.t.Helper()
	return run(append(args, "-C", h.dir), h.env)
}

func (h *harness) stdout() string { return h.out.String() }
func (h *harness) stderr() string { return h.errOut.String() }

func TestRunWithNoArgumentsPrintsUsageAndFails(t *testing.T) {
	var out, errOut strings.Builder
	if got := run(nil, &env{out: &out, err: &errOut}); got != 2 {
		t.Errorf("status = %d, want 2", got)
	}
	if !strings.Contains(errOut.String(), "perch generates a macOS menu bar app") {
		t.Errorf("stderr = %q, want the usage", errOut.String())
	}
	if out.String() != "" {
		t.Errorf("usage went to stdout on failure: %q", out.String())
	}
}

func TestRunUnknownCommandNamesIt(t *testing.T) {
	var out, errOut strings.Builder
	if got := run([]string{"buidl"}, &env{out: &out, err: &errOut}); got != 2 {
		t.Errorf("status = %d, want 2", got)
	}
	if !strings.Contains(errOut.String(), `unknown command "buidl"`) {
		t.Errorf("stderr = %q, want it to quote the command", errOut.String())
	}
}

// help is asked for, so it goes to stdout and succeeds; every other route to
// the same text is a failure and goes to stderr.
func TestRunHelpSucceedsOnStdout(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		var out, errOut strings.Builder
		if got := run([]string{arg}, &env{out: &out, err: &errOut}); got != 0 {
			t.Errorf("%s: status = %d, want 0", arg, got)
		}
		if !strings.Contains(out.String(), "perch build") {
			t.Errorf("%s: stdout = %q, want the usage", arg, out.String())
		}
		if errOut.String() != "" {
			t.Errorf("%s: stderr = %q, want it empty", arg, errOut.String())
		}
	}
}

func TestBuildEmitsSwiftAndNamesEveryFile(t *testing.T) {
	h := newHarness(t, exampleSpec)
	if got := h.run("build"); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	for _, want := range []string{"menubar/Generated/main.swift", "menubar/Generated/Shapes.swift", "built onto"} {
		if !strings.Contains(h.stdout(), want) {
			t.Errorf("stdout = %q, want it to mention %q", h.stdout(), want)
		}
	}
	entries, err := os.ReadDir(filepath.Join(h.dir, "menubar", "Generated"))
	if err != nil {
		t.Fatalf("nothing was emitted: %v", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	for _, want := range []string{"main.swift", "Runtime.swift", "Shapes.swift"} {
		if !contains(names, want) {
			t.Errorf("emitted %v, want it to include %s", names, want)
		}
	}
}

func TestBuildReportsAMissingSpecAgainstTheFile(t *testing.T) {
	h := newHarness(t, "")
	if got := h.run("build"); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	if !strings.Contains(h.stderr(), project.SpecFile) {
		t.Errorf("stderr = %q, want it to name menubar.yaml", h.stderr())
	}
	if !strings.HasPrefix(h.stderr(), "perch: ") {
		t.Errorf("stderr = %q, want it prefixed with the program name", h.stderr())
	}
}

// A build error is the whole point of lowering CEL ahead of time, so the
// expression has to reach the message.
func TestBuildReportsABadExpressionAgainstTheSpec(t *testing.T) {
	h := newHarness(t, `
app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 5s}
watch: {fleet: {run: [echo], json: true, shape: {jobs: [{id: string}]}}}
menu: [{text: "{{fleet.data.jbos.size()}}"}, {text: Quit, quit: true}]
`)
	if got := h.run("build"); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	if !strings.Contains(h.stderr(), "jbos") {
		t.Errorf("stderr = %q, want it to name the misspelled field", h.stderr())
	}
	if !strings.Contains(h.stderr(), project.SpecFile) {
		t.Errorf("stderr = %q, want it to name the file", h.stderr())
	}
}

// A failed build must not leave a half-written Generated/ that the next
// compile would pick up.
func TestBuildLeavesNothingBehindWhenItFails(t *testing.T) {
	h := newHarness(t, exampleSpec)
	if got := h.run("build"); got != 0 {
		t.Fatalf("first build: %d\n%s", got, h.stderr())
	}
	h.write(project.SpecFile, strings.Replace(exampleSpec, "fleet.data.jobs", "fleet.data.jbos", 1))
	if got := h.run("build"); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	body, err := os.ReadFile(filepath.Join(h.dir, "menubar", "Generated", "main.swift"))
	if err != nil {
		t.Fatalf("the previous build's output was removed: %v", err)
	}
	if strings.Contains(string(body), "jbos") {
		t.Error("a failed build wrote its partial output over a working one")
	}
}

func TestInstallBuildsBundlesWritesThePlistAndLoadsIt(t *testing.T) {
	h := newHarness(t, exampleSpec)
	if got := h.run("install"); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	bundle := filepath.Join(h.home, "Applications", "onto.app")
	if _, err := os.Stat(filepath.Join(bundle, "Contents", "MacOS", "onto")); err != nil {
		t.Errorf("no executable in the bundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "Contents", "Info.plist")); err != nil {
		t.Errorf("no Info.plist: %v", err)
	}
	plist := filepath.Join(h.home, "Library", "LaunchAgents", "dev.onto.menubar.plist")
	body, err := os.ReadFile(plist)
	if err != nil {
		t.Fatalf("no LaunchAgent: %v", err)
	}
	if !strings.Contains(string(body), filepath.Join(bundle, "Contents", "MacOS", "onto")) {
		t.Errorf("the plist does not point at the installed binary:\n%s", body)
	}
	if len(h.loaded) != 1 || h.loaded[0] != plist {
		t.Errorf("bootstrapped %v, want the plist it just wrote", h.loaded)
	}
	if !strings.Contains(h.stdout(), "onto is running") {
		t.Errorf("stdout = %q", h.stdout())
	}
	// A watch reaching the network fails silently until Local Network is
	// granted, which is the one thing the author has to be told.
	if !strings.Contains(h.stdout(), "Local Network") {
		t.Errorf("stdout = %q, want the Local Network note for a spec with watches", h.stdout())
	}
	// Everything the compiler was handed has to be Swift the project owns.
	if len(h.sources) == 0 {
		t.Fatal("nothing was handed to the compiler")
	}
	for _, src := range h.sources {
		if !strings.HasPrefix(src, h.dir) || filepath.Ext(src) != ".swift" {
			t.Errorf("compiler was handed %q", src)
		}
	}
}

// The note is about watches, so a spec with none must not print it.
func TestInstallOmitsTheNetworkNoteWithoutWatches(t *testing.T) {
	h := newHarness(t, `
app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 5s}
menu: [{text: Quit, quit: true}]
`)
	if got := h.run("install"); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	if strings.Contains(h.stdout(), "Local Network") {
		t.Errorf("stdout = %q, want no note for a spec with no watches", h.stdout())
	}
}

// A failed compile must not leave a plist pointing at a bundle that is not
// there, and must not load anything.
func TestInstallStopsAtAFailedCompile(t *testing.T) {
	h := newHarness(t, exampleSpec)
	h.env.compile = func([]string, string) error { return errors.New("swiftc: error: no such module") }
	if got := h.run("install"); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	if _, err := os.Stat(filepath.Join(h.home, "Library", "LaunchAgents", "dev.onto.menubar.plist")); err == nil {
		t.Error("a plist was written for a build that never compiled")
	}
	if len(h.loaded) != 0 {
		t.Errorf("bootstrapped %v after a failed compile", h.loaded)
	}
}

func TestUninstallRemovesBothArtifactsAndBootsOut(t *testing.T) {
	h := newHarness(t, exampleSpec)
	if got := h.run("install"); got != 0 {
		t.Fatalf("install: %d\n%s", got, h.stderr())
	}
	h.out.Reset()
	if got := h.run("uninstall"); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	for _, path := range []string{
		filepath.Join(h.home, "Applications", "onto.app"),
		filepath.Join(h.home, "Library", "LaunchAgents", "dev.onto.menubar.plist"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s survived uninstall", path)
		}
	}
	if !contains(h.booted, "bootout:dev.onto.menubar") {
		t.Errorf("launchctl calls = %v, want a bootout of the label", h.booted)
	}
	if !strings.Contains(h.stdout(), "onto is uninstalled") {
		t.Errorf("stdout = %q", h.stdout())
	}
}

// Uninstalling something that was never loaded is the ordinary case after a
// machine restart, so it succeeds and says so.
func TestUninstallSucceedsWhenNothingWasLoaded(t *testing.T) {
	h := newHarness(t, exampleSpec)
	h.env.launchd = notLoaded{}
	if got := h.run("uninstall"); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	if !strings.Contains(h.stdout(), "was not loaded") {
		t.Errorf("stdout = %q, want it to say the label was not loaded", h.stdout())
	}
}

type notLoaded struct{}

func (notLoaded) Print(string) error     { return errors.New("could not find service") }
func (notLoaded) Bootout(string) error   { return errors.New("no such process") }
func (notLoaded) Bootstrap(string) error { return nil }

// uninstall reads the spec for the two paths it removes, so a spec it cannot
// read has to stop it rather than have it guess.
func TestUninstallRefusesWithoutASpec(t *testing.T) {
	h := newHarness(t, "")
	if got := h.run("uninstall"); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	if !strings.Contains(h.stderr(), project.SpecFile) {
		t.Errorf("stderr = %q, want it to name the file", h.stderr())
	}
}

func TestRunCompilesAndStartsTheBinary(t *testing.T) {
	h := newHarness(t, exampleSpec)
	if got := h.run("run"); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	if len(h.booted) != 1 {
		t.Fatalf("started %v, want one binary", h.booted)
	}
	if filepath.Base(h.booted[0]) != "onto" {
		t.Errorf("started %q, want the app's executable", h.booted[0])
	}
	// perch run must not install anything.
	if _, err := os.Stat(filepath.Join(h.home, "Applications", "onto.app")); err == nil {
		t.Error("perch run wrote into ~/Applications")
	}
	if _, err := os.Stat(filepath.Join(h.home, "Library", "LaunchAgents")); err == nil {
		t.Error("perch run wrote a LaunchAgent")
	}
}

func TestSchemaWritesToStdoutOrAFile(t *testing.T) {
	h := newHarness(t, "")
	if got := run([]string{"schema"}, h.env); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	if !strings.HasPrefix(h.stdout(), "{") || !strings.Contains(h.stdout(), `"menubar.yaml"`) {
		t.Errorf("stdout = %.60q, want the schema", h.stdout())
	}

	h.out.Reset()
	out := filepath.Join(h.dir, "menubar.schema.json")
	if got := run([]string{"schema", "-o", out}, h.env); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no schema written: %v", err)
	}
	if len(body) == 0 || h.stdout() != "" {
		t.Errorf("with -o the schema still went to stdout: %.60q", h.stdout())
	}
}

func TestShapeReadsSamplesAndRunsCommands(t *testing.T) {
	h := newHarness(t, "")
	sample := filepath.Join(h.dir, "one.json")
	if err := os.WriteFile(sample, []byte(`{"jobs": [{"id": "j1"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"shape", "-sample", sample}, h.env); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	want := "shape:\n  jobs: [{id: string}]\n"
	if h.stdout() != want {
		t.Errorf("\n got %q\nwant %q", h.stdout(), want)
	}

	// Two sources union, which is the reason both flags repeat.
	h.out.Reset()
	second := filepath.Join(h.dir, "two.json")
	if err := os.WriteFile(second, []byte(`{"pruned": 3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run([]string{"shape", "-sample", sample, "-sample", second, "-from", `echo {"extra":true}`}, h.env); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	for _, field := range []string{"jobs:", "pruned: int", "extra: bool"} {
		if !strings.Contains(h.stdout(), field) {
			t.Errorf("stdout = %q, want it to include %q", h.stdout(), field)
		}
	}
}

// The block is meant to be pasted under a watch, so it comes out indented by
// the two spaces that puts it at.
func TestShapeIndentsForPasting(t *testing.T) {
	h := newHarness(t, "")
	if got := run([]string{"shape", "-from", `echo {"a":{"b":1}}`}, h.env); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	want := "shape:\n  a: {b: int}\n"
	if h.stdout() != want {
		t.Errorf("\n got %q\nwant %q", h.stdout(), want)
	}

	// A block that does not fit on one line is indented under its own key, so
	// every line past the first still lands under a watch at two spaces.
	h.out.Reset()
	if got := run([]string{"shape", "-from", `echo {"a":{"b":{"c":1}},"d":2}`}, h.env); got != 0 {
		t.Fatalf("status = %d, want 0\n%s", got, h.stderr())
	}
	want = "shape:\n  a:\n    b: {c: int}\n  d: int\n"
	if h.stdout() != want {
		t.Errorf("\n got %q\nwant %q", h.stdout(), want)
	}
}

func TestShapeNeedsASource(t *testing.T) {
	h := newHarness(t, "")
	if got := run([]string{"shape"}, h.env); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	if !strings.Contains(h.stderr(), "--from") || !strings.Contains(h.stderr(), "--sample") {
		t.Errorf("stderr = %q, want it to name both flags", h.stderr())
	}
}

// A command that fails writes its complaint to stderr, which is the only place
// the reason there is nothing to read is ever stated.
func TestShapeCarriesTheCommandsComplaint(t *testing.T) {
	h := newHarness(t, "")
	if got := run([]string{"shape", "-from", "sh -c exit1"}, h.env); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	h.errOut.Reset()
	if got := run([]string{"shape", "-from", `sh -c echo_this_is_not_a_command`}, h.env); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	if !strings.Contains(h.stderr(), "not found") && !strings.Contains(h.stderr(), "echo_this_is_not_a_command") {
		t.Errorf("stderr = %q, want the shell's own complaint", h.stderr())
	}
}

func TestShapeReportsAMissingSample(t *testing.T) {
	h := newHarness(t, "")
	if got := run([]string{"shape", "-sample", filepath.Join(h.dir, "nope.json")}, h.env); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	if !strings.Contains(h.stderr(), "nope.json") {
		t.Errorf("stderr = %q, want it to name the missing file", h.stderr())
	}
}

func TestShapeRejectsAnEmptyCommand(t *testing.T) {
	h := newHarness(t, "")
	if got := run([]string{"shape", "-from", "   "}, h.env); got != 1 {
		t.Fatalf("status = %d, want 1", got)
	}
	if !strings.Contains(h.stderr(), "argv") {
		t.Errorf("stderr = %q, want it to say what --from takes", h.stderr())
	}
}

// A bad flag reports rather than exiting the process, so the status is the
// usage one and nothing is emitted.
func TestBadFlagsFailWithoutRunning(t *testing.T) {
	for _, args := range [][]string{
		{"build", "-nope"},
		{"schema", "-nope"},
		{"shape", "-nope"},
		{"install", "-nope"},
	} {
		h := newHarness(t, exampleSpec)
		if got := run(append(args, "-C", h.dir), h.env); got != 2 {
			t.Errorf("%v: status = %d, want 2", args, got)
		}
		if h.stdout() != "" {
			t.Errorf("%v: stdout = %q, want nothing to have run", args, h.stdout())
		}
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// Everything above replaces these, so nothing else would notice a default left
// nil — which reaches the user as a crash on the one path tests never take.
func TestUnsetSeamsFallBackToTheRealThing(t *testing.T) {
	e := &env{out: &strings.Builder{}, err: &strings.Builder{}}
	if _, ok := e.launchctl().(*install.CLI); !ok {
		t.Errorf("launchctl() = %T, want the real launchctl", e.launchctl())
	}
	if w := e.waiter(); w.Tries == 0 || w.Sleep == nil {
		t.Errorf("waiter() = %+v, want DefaultWait", w)
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "prog")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := e.run(bin); err != nil {
		t.Errorf("run(%s): %v", bin, err)
	}
	if err := e.run(filepath.Join(dir, "not-there")); err == nil {
		t.Error("running a binary that is not there reported success")
	}
}
