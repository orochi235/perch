package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	perchBin  string
	buildErr  error
)

// perch builds the command once per run and returns its path. Everything here
// goes through the real process: argument handling, exit statuses and the
// streams a shell would see.
func perch(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "perch-e2e-")
		if err != nil {
			buildErr = err
			return
		}
		perchBin = filepath.Join(dir, "perch")
		out, err := exec.Command("go", "build", "-o", perchBin, "github.com/orochi235/perch/cmd/perch").CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return perchBin
}

type result struct {
	status int
	stdout string
	stderr string
}

func runPerch(t *testing.T, dir string, args ...string) result {
	t.Helper()
	c := exec.Command(perch(t), args...)
	c.Dir = dir
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

const e2eSpec = `# yaml-language-server: $schema=./menubar.schema.json
app:
  name: perchE2E
  id: dev.perch.e2e
  icon: circle.dashed
  interval: 2s
watch:
  fleet:
    run: [echo, '{"jobs": [{"id": "j1", "cmd": "build"}], "healthy": true}']
    json: true
    shape:
      jobs: [{id: string, cmd: string}]
      healthy: bool
  here:
    exists: /usr/bin/true
status:
  - when: "!fleet.ok"
    icon: exclamationmark.triangle
  - when: "fleet.data.jobs.size() == 0"
    dim: true
  - badge: "fleet.data.jobs.size()"
menu:
  - text: "{{fleet.data.jobs.size()}} running"
  - separator
  - each: fleet.data.jobs
    text: "{{it.id}} — {{it.cmd}}"
    menu:
      - {text: Logs, run: [echo, "{{it.id}}"]}
  - separator
  - {text: Quit, quit: true}
`

func project(t *testing.T, doc string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "menubar.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// perch is run from a repo root, so -C and the working directory both have to
// find the same project.
func TestBuildFindsTheProjectByDirectoryAndByFlag(t *testing.T) {
	dir := project(t, e2eSpec)
	if r := runPerch(t, dir, "build"); r.status != 0 {
		t.Fatalf("status = %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	elsewhere := t.TempDir()
	r := runPerch(t, elsewhere, "build", "-C", dir)
	if r.status != 0 {
		t.Fatalf("status = %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, "built perchE2E") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want it empty on success", r.stderr)
	}
}

// The whole point of lowering CEL at build time is that a mistake stops the
// build. A shell sees that as a non-zero status and a message on stderr.
func TestBuildFailsLoudlyOnABadSpec(t *testing.T) {
	dir := project(t, strings.Replace(e2eSpec, "it.cmd", "it.cmdd", 1))
	r := runPerch(t, dir, "build")
	if r.status != 1 {
		t.Fatalf("status = %d, want 1\n%s%s", r.status, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stderr, "cmdd") {
		t.Errorf("stderr = %q, want it to name the bad field", r.stderr)
	}
	if r.stdout != "" && strings.Contains(r.stdout, "built") {
		t.Errorf("stdout claims a build: %q", r.stdout)
	}
}

// The emitted Swift is the deliverable, and a golden test alone stays green
// while it stops compiling.
func TestBuiltProjectCompiles(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	dir := project(t, e2eSpec)
	if r := runPerch(t, dir, "build"); r.status != 0 {
		t.Fatalf("build: %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	sources, err := filepath.Glob(filepath.Join(dir, "menubar", "Generated", "*.swift"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("nothing emitted: %v %v", sources, err)
	}
	out, err := exec.Command(swiftc, append([]string{"-typecheck"}, sources...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("the emitted project does not compile: %v\n%s", err, out)
	}
	if len(out) > 0 {
		t.Errorf("it compiles with warnings:\n%s", out)
	}
}

// perch never reads or writes menubar/Sources/, so a hand-written file
// compiles alongside the emitted ones and survives a rebuild.
func TestHandWrittenSourcesCompileAlongsideTheGeneratedOnes(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	dir := project(t, e2eSpec)
	sources := filepath.Join(dir, "menubar", "Sources")
	if err := os.MkdirAll(sources, 0o755); err != nil {
		t.Fatal(err)
	}
	hand := filepath.Join(sources, "Extra.swift")
	if err := os.WriteFile(hand, []byte("enum PerchExtra { static let tag = \"kept\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := runPerch(t, dir, "build"); r.status != 0 {
		t.Fatalf("build: %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	if b, err := os.ReadFile(hand); err != nil || !strings.Contains(string(b), "kept") {
		t.Fatalf("the hand-written source was disturbed: %q %v", b, err)
	}
	all, _ := filepath.Glob(filepath.Join(dir, "menubar", "*", "*.swift"))
	out, err := exec.Command(swiftc, append([]string{"-typecheck"}, all...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("emitted and hand-written sources do not compile together: %v\n%s", err, out)
	}
}

// perch shape prints a block to paste under a watch, so what it prints has to
// be something perch build then accepts.
func TestShapeOutputPastesIntoASpecThatBuilds(t *testing.T) {
	dir := t.TempDir()
	r := runPerch(t, dir, "shape", "-from", `echo {"nodes":[{"name":"studio","up":true}],"jobs":[]}`)
	if r.status != 0 {
		t.Fatalf("shape: %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	if !strings.HasPrefix(r.stdout, "shape:\n") {
		t.Fatalf("stdout = %q, want it to start with the key it goes under", r.stdout)
	}
	// Two spaces puts shape: under a watch; the block is already indented for it.
	var indented []string
	for _, line := range strings.Split(strings.TrimRight(r.stdout, "\n"), "\n") {
		indented = append(indented, "    "+line)
	}
	doc := "app: {name: shaped, id: dev.perch.shaped, icon: circle, interval: 5s}\n" +
		"watch:\n  w:\n    run: [echo, x]\n    json: true\n" +
		strings.Join(indented, "\n") + "\n" +
		"menu: [{text: \"{{w.data.nodes.size()}}\"}, {text: Quit, quit: true}]\n"
	built := project(t, doc)
	if r := runPerch(t, built, "build"); r.status != 0 {
		t.Fatalf("the pasted shape does not build:\n%s\n%s%s", doc, r.stdout, r.stderr)
	}
}

// The header in menubar.yaml points an editor at this file, so the two have to
// be written by the same command that reads them.
func TestSchemaWritesTheFileTheSpecHeaderNames(t *testing.T) {
	dir := project(t, e2eSpec)
	if r := runPerch(t, dir, "schema", "-o", filepath.Join(dir, "menubar.schema.json")); r.status != 0 {
		t.Fatalf("schema: %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	body, err := os.ReadFile(filepath.Join(dir, "menubar.schema.json"))
	if err != nil {
		t.Fatalf("no schema written: %v", err)
	}
	if !strings.Contains(string(body), `"menubar.yaml"`) {
		t.Errorf("the schema does not describe menubar.yaml:\n%.200s", body)
	}
	// The yaml-language-server header points an editor at a path beside the
	// spec, so the name -o is given and the name in the header have to match.
	spec, err := os.ReadFile(filepath.Join(dir, "menubar.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(spec), "$schema=./menubar.schema.json") {
		t.Errorf("the spec's header does not name the file schema -o wrote:\n%.120s", spec)
	}
}

// Rebuilding must be the same as building once: the second run replaces the
// directory rather than merging into it.
func TestBuildIsIdempotent(t *testing.T) {
	dir := project(t, e2eSpec)
	if r := runPerch(t, dir, "build"); r.status != 0 {
		t.Fatalf("build: %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	first := snapshot(t, filepath.Join(dir, "menubar", "Generated"))
	if r := runPerch(t, dir, "build"); r.status != 0 {
		t.Fatalf("rebuild: %d\n%s%s", r.status, r.stdout, r.stderr)
	}
	second := snapshot(t, filepath.Join(dir, "menubar", "Generated"))
	if len(first) != len(second) {
		t.Fatalf("a rebuild changed the file list:\n%v\n%v", keys(first), keys(second))
	}
	for name, body := range first {
		if second[name] != body {
			t.Errorf("%s differs between two builds of the same spec", name)
		}
	}
}

func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = string(b)
	}
	return out
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
