package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/backend"
)

func specAt(t *testing.T, root, doc string) *Project {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, SpecFile), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return p
}

// uninstall calls RemoveAll on both of these, so where they land is the
// difference between removing an app and removing somewhere else entirely.
func TestBundleAndPlistPathsSitUnderTheHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := scratch(t)

	bundle, err := p.BundlePath()
	if err != nil {
		t.Fatalf("BundlePath: %v", err)
	}
	if want := filepath.Join(home, "Applications", "onto.app"); bundle != want {
		t.Errorf("BundlePath = %q, want %q", bundle, want)
	}

	plist, err := p.PlistPath()
	if err != nil {
		t.Fatalf("PlistPath: %v", err)
	}
	if want := filepath.Join(home, "Library", "LaunchAgents", "dev.onto.menubar.plist"); plist != want {
		t.Errorf("PlistPath = %q, want %q", plist, want)
	}
}

// app.name is refused before it ever reaches a path, because these two are what
// uninstall removes. This is the check standing between a typo and rm -rf of
// the wrong directory.
func TestPathsCannotEscapeTheirDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, name := range []string{"../evil", "a/b", `a\b`, ".", "..", ".hidden"} {
		root := t.TempDir()
		doc := "app: {name: " + quote(name) + ", id: dev.a.menubar, icon: circle, interval: 1s}\n" +
			"menu: [{text: Quit, quit: true}]\n"
		if err := os.WriteFile(filepath.Join(root, SpecFile), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(root); err == nil {
			t.Errorf("app.name %q was accepted; it names a path uninstall removes", name)
		}
	}
	for _, id := range []string{"../evil", "a/b", "dev a"} {
		root := t.TempDir()
		doc := "app: {name: onto, id: " + quote(id) + ", icon: circle, interval: 1s}\n" +
			"menu: [{text: Quit, quit: true}]\n"
		if err := os.WriteFile(filepath.Join(root, SpecFile), []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(root); err == nil {
			t.Errorf("app.id %q was accepted; it names the plist uninstall removes", id)
		}
	}
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }

// Every diagnostic a build produces has to name the file, because perch is run
// from a repo root where menubar.yaml is not the only YAML.
func TestLoadNamesTheSpecFileInEveryError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, SpecFile), []byte("app: {name: a}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil {
		t.Fatal("want an error for an incomplete spec")
	}
	if !strings.Contains(err.Error(), SpecFile) {
		t.Errorf("error = %q, want it to name %s", err, SpecFile)
	}
	if _, err := Load(filepath.Join(root, "nope")); err == nil {
		t.Fatal("want an error for a missing directory")
	} else if !strings.Contains(err.Error(), SpecFile) {
		t.Errorf("error = %q, want it to name %s", err, SpecFile)
	}
}

// perch compiles whatever it hands swiftc in this order, so two runs of the
// same project have to produce the same argument list.
func TestSwiftSourcesIsSorted(t *testing.T) {
	p := scratch(t)
	if err := os.MkdirAll(p.SourcesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Zebra.swift", "Alpha.swift", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(p.SourcesDir(), name), []byte("//"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.WriteGenerated([]backend.File{
		{Name: "main.swift", Body: []byte("//")},
		{Name: "Shapes.swift", Body: []byte("//")},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := p.SwiftSources()
	if err != nil {
		t.Fatalf("SwiftSources: %v", err)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("sources are not sorted: %v", got)
		}
	}
	for _, path := range got {
		if filepath.Ext(path) != ".swift" {
			t.Errorf("SwiftSources returned %s, which is not Swift", path)
		}
	}
	if len(got) != 4 {
		t.Errorf("got %d sources, want 4: %v", len(got), got)
	}
}

// perch run and perch install both call this before compiling; an empty list
// would otherwise reach swiftc as no arguments at all.
func TestSwiftSourcesSaysToBuildFirst(t *testing.T) {
	p := scratch(t)
	_, err := p.SwiftSources()
	if err == nil {
		t.Fatal("want an error when nothing has been emitted")
	}
	if !strings.Contains(err.Error(), "perch build") {
		t.Errorf("error = %q, want it to say what to run", err)
	}
}
