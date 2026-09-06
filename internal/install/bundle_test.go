package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func stubCompiler(body string) Compiler {
	return func(sources []string, out string) error {
		return os.WriteFile(out, []byte(body), 0o755)
	}
}

func failingCompiler(sources []string, out string) error {
	return errors.New("swiftc: error: no such module")
}

func opts(dest string, c Compiler) BundleOpts {
	return BundleOpts{
		App:     App{Name: "onto", ID: "dev.onto.menubar", Executable: "onto"},
		Sources: []string{"main.swift"},
		Dest:    dest,
		Compile: c,
	}
}

func TestBuildBundleLaysOutTheApp(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "onto.app")
	if err := BuildBundle(opts(dest, stubCompiler("BINARY"))); err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	bin := filepath.Join(dest, "Contents", "MacOS", "onto")
	if b, err := os.ReadFile(bin); err != nil || string(b) != "BINARY" {
		t.Errorf("executable not at %s: %v", bin, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "Contents", "Info.plist")); err != nil {
		t.Errorf("no Info.plist: %v", err)
	}
}

// A failed compile must not damage a working install — the fix the other app has
// and brainhouse never got.
func TestBuildBundleLeavesAWorkingInstallAloneWhenCompilationFails(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "onto.app")
	if err := BuildBundle(opts(dest, stubCompiler("GOOD"))); err != nil {
		t.Fatalf("first build: %v", err)
	}
	if err := BuildBundle(opts(dest, failingCompiler)); err == nil {
		t.Fatal("want the compile error to surface, got nil")
	}
	bin := filepath.Join(dest, "Contents", "MacOS", "onto")
	if b, err := os.ReadFile(bin); err != nil || string(b) != "GOOD" {
		t.Errorf("the working install was damaged: %q %v", b, err)
	}
}

// A resource dropped from the spec must not linger in the installed bundle.
func TestBuildBundleReplacesRatherThanMergesTheBundle(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "onto.app")
	if err := BuildBundle(opts(dest, stubCompiler("GOOD"))); err != nil {
		t.Fatalf("first build: %v", err)
	}
	stale := filepath.Join(dest, "Contents", "Resources", "gone.txt")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := BuildBundle(opts(dest, stubCompiler("GOOD2"))); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("a file dropped from the build survived the rebuild")
	}
}
