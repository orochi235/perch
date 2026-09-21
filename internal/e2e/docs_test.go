// Package e2e drives the built perch binary and the artifacts it produces.
// Everything here goes through a real process, a real compiler or a real
// launchd, which is where the in-process tests stop.
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/orochi235/perch/v2/internal/backend/swiftappkit"
	"github.com/orochi235/perch/v2/internal/spec"
	"gopkg.in/yaml.v3"
)

// docs are the files whose YAML a reader will copy. A doc example that no
// longer builds is worse than a stale sentence: it is followed.
var docs = []string{
	"../../README.md",
	"../../docs/schema.md",
	"../../docs/superpowers/specs/2026-09-05-perch-design.md",
}

// docDirs are published whole, so a page added to one is covered by being
// there rather than by being listed.
var docDirs = []string{"../../docs/guide", "../../docs/recipes"}

func docFiles(t *testing.T) []string {
	t.Helper()
	files := append([]string(nil), docs...)
	for _, dir := range docDirs {
		found, err := filepath.Glob(filepath.Join(dir, "*.md"))
		if err != nil {
			t.Fatal(err)
		}
		if len(found) == 0 {
			t.Fatalf("%s holds no pages; the site publishes it", dir)
		}
		files = append(files, found...)
	}
	return files
}

type block struct {
	file string
	line int
	body string
}

// fences pulls every fence whose info string is exactly info out of a markdown
// file.
func fences(t *testing.T, path, info string) []block {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var out []block
	lines := strings.Split(string(src), "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "```"+info {
			continue
		}
		start := i + 1
		for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
		}
		out = append(out, block{file: path, line: start + 1, body: strings.Join(lines[start:i], "\n")})
	}
	return out
}

// Every documented example has to parse, emit and compile: every menubar.yaml,
// and every template, used once by a stub file. The design doc's example is
// duplicated into typecheck_test.go by hand, so this is also what notices when
// the copy stops matching the source.
func TestDocumentedExamplesBuild(t *testing.T) {
	var found int
	for _, doc := range docFiles(t) {
		for _, b := range fences(t, doc, "yaml") {
			found++
			t.Run(filepath.Base(b.file)+":"+strconv.Itoa(b.line), func(t *testing.T) {
				s, err := spec.Parse([]byte(b.body))
				if err != nil {
					t.Fatalf("%s:%d does not parse:\n%s\n%v", b.file, b.line, b.body, err)
				}
				typecheck(t, b, s)
			})
		}
		for _, b := range fences(t, doc, "yaml template") {
			found++
			t.Run(filepath.Base(b.file)+":"+strconv.Itoa(b.line), func(t *testing.T) {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "example.yaml"), []byte(b.body), 0o644); err != nil {
					t.Fatal(err)
				}
				stub := templateStub(t, b)
				s, err := spec.ParseWith([]byte(stub), spec.TemplatesIn(dir))
				if err != nil {
					t.Fatalf("%s:%d does not parse as a template used by\n%s\n%v", b.file, b.line, stub, err)
				}
				typecheck(t, b, s)
			})
		}
	}
	if found == 0 {
		t.Fatal("no YAML examples found; the docs list or the fence marker changed")
	}
}

// templateStub is a menubar.yaml using the template once, passing a placeholder
// that is also a valid launchd label for every parameter it requires.
func templateStub(t *testing.T, b block) string {
	t.Helper()
	var tmpl struct {
		Params yaml.Node `yaml:"params"`
	}
	if err := yaml.Unmarshal([]byte(b.body), &tmpl); err != nil {
		t.Fatalf("%s:%d is not YAML: %v", b.file, b.line, err)
	}
	var args []string
	for i := 0; i+1 < len(tmpl.Params.Content); i += 2 {
		if v := tmpl.Params.Content[i+1]; v.Tag == "!!null" {
			args = append(args, tmpl.Params.Content[i].Value+": dev.example.x")
		}
	}
	return "app: {name: x, id: dev.example.x, icon: circle, interval: 5s}\n" +
		"use: {x: {example: {" + strings.Join(args, ", ") + "}}}\n"
}

func typecheck(t *testing.T, b block, s *spec.Spec) {
	t.Helper()
	files, err := swiftappkit.New().Emit(s)
	if err != nil {
		t.Fatalf("%s:%d does not emit: %v", b.file, b.line, err)
	}
	swiftc, _ := exec.LookPath("swiftc")
	if swiftc == "" {
		t.Skip("swiftc not on PATH")
	}
	dir := t.TempDir()
	var paths []string
	for _, f := range files {
		p := filepath.Join(dir, f.Name)
		if err := os.WriteFile(p, f.Body, 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	out, err := exec.Command(swiftc, append([]string{"-typecheck"}, paths...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s:%d emits Swift that does not compile: %v\n%s", b.file, b.line, err, out)
	}
}

// The README lists the commands perch answers to. One that has been renamed or
// dropped leaves the list quietly wrong.
func TestREADMEListsTheCommandsPerchAnswersTo(t *testing.T) {
	src, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{"build", "run", "install", "uninstall", "shape", "schema"} {
		if !strings.Contains(string(src), "perch "+cmd) {
			t.Errorf("the README does not mention perch %s", cmd)
		}
	}
}

// Every SF Symbol a documented example names has to resolve. perch does not
// check symbol names — the list is macOS's and grows with each release — and an
// app that names one macOS does not know draws nothing at all.
func TestDocumentedSymbolsResolve(t *testing.T) {
	where := map[string][]string{}
	note := func(b block, s *spec.Spec) {
		for path, icon := range s.Icons() {
			if icon.Symbol == "" {
				continue
			}
			at := b.file + ":" + strconv.Itoa(b.line) + " " + path
			where[icon.Symbol] = append(where[icon.Symbol], at)
		}
	}
	for _, doc := range docFiles(t) {
		// A fence that does not parse is TestDocumentedExamplesBuild's to report.
		for _, b := range fences(t, doc, "yaml") {
			if s, err := spec.Parse([]byte(b.body)); err == nil {
				note(b, s)
			}
		}
		for _, b := range fences(t, doc, "yaml template") {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "example.yaml"), []byte(b.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if s, err := spec.ParseWith([]byte(templateStub(t, b)), spec.TemplatesIn(dir)); err == nil {
				note(b, s)
			}
		}
	}
	// A templated icon is named only once a preview runs, and a preview cannot
	// draw one without its look-alike, so the look-alikes stand in for those.
	drawn, err := filepath.Glob(filepath.Join("..", "site", "symbols", "*.svg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range drawn {
		name := strings.TrimSuffix(filepath.Base(f), ".svg")
		where[name] = append(where[name], "internal/site/symbols/"+name+".svg")
	}
	names := make([]string, 0, len(where))
	for name := range where {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("no symbols found; the docs list or the fence marker changed")
	}
	for _, name := range missingSymbols(t, names) {
		t.Errorf("%q is not a symbol macOS knows, and draws as nothing; named at %s",
			name, strings.Join(where[name], ", "))
	}
}

// missingSymbols asks AppKit which of these names it cannot draw, since only
// the running macOS knows its own catalog.
func missingSymbols(t *testing.T, names []string) []string {
	t.Helper()
	swiftc, _ := exec.LookPath("swiftc")
	if swiftc == "" {
		t.Skip("swiftc not on PATH")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "symbols.swift")
	src := "import AppKit\n" +
		"for name in CommandLine.arguments.dropFirst()\n" +
		"where NSImage(systemSymbolName: name, accessibilityDescription: nil) == nil {\n" +
		"    print(name)\n" +
		"}\n"
	if err := os.WriteFile(source, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "symbols")
	if out, err := exec.Command(swiftc, "-o", bin, source).CombinedOutput(); err != nil {
		t.Fatalf("compiling the symbol check: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, names...).Output()
	if err != nil {
		t.Fatalf("running the symbol check: %v", err)
	}
	return strings.Fields(string(out))
}
