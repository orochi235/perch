// Package e2e drives the built perch binary and the artifacts it produces.
// Everything here goes through a real process, a real compiler or a real
// launchd, which is where the in-process tests stop.
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/backend/swiftappkit"
	"github.com/orochi235/perch/internal/spec"
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

// yamlBlocks pulls every ```yaml fence out of a markdown file.
func yamlBlocks(t *testing.T, path string) []block {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var out []block
	lines := strings.Split(string(src), "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "```yaml" {
			continue
		}
		start := i + 1
		for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
		}
		out = append(out, block{file: path, line: start + 1, body: strings.Join(lines[start:i], "\n")})
	}
	return out
}

// Every documented example has to parse, emit and compile. The design doc's
// example is duplicated into typecheck_test.go by hand, so this is also what
// notices when the copy stops matching the source.
func TestDocumentedExamplesBuild(t *testing.T) {
	swiftc, _ := exec.LookPath("swiftc")
	var found int
	for _, doc := range docFiles(t) {
		for _, b := range yamlBlocks(t, doc) {
			found++
			name := filepath.Base(b.file) + ":" + strconv.Itoa(b.line)
			t.Run(name, func(t *testing.T) {
				s, err := spec.Parse([]byte(b.body))
				if err != nil {
					t.Fatalf("%s:%d does not parse:\n%s\n%v", b.file, b.line, b.body, err)
				}
				files, err := swiftappkit.New().Emit(s)
				if err != nil {
					t.Fatalf("%s:%d does not emit: %v", b.file, b.line, err)
				}
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
			})
		}
	}
	if found == 0 {
		t.Fatal("no YAML examples found; the docs list or the fence marker changed")
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
