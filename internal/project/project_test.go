package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/orochi235/perch/internal/backend"
)

func scratch(t *testing.T) *Project {
	t.Helper()
	root := t.TempDir()
	yaml := "app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 1s}\nmenu: [{text: Quit, quit: true}]\n"
	if err := os.WriteFile(filepath.Join(root, "menubar.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return p
}

func TestWriteGeneratedCreatesTheDirectory(t *testing.T) {
	p := scratch(t)
	if err := p.WriteGenerated([]backend.File{{Name: "main.swift", Body: []byte("x")}}); err != nil {
		t.Fatalf("WriteGenerated: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(p.GeneratedDir(), "main.swift"))
	if err != nil || string(b) != "x" {
		t.Errorf("main.swift not written: %q %v", b, err)
	}
}

// Nothing inside Generated/ is ever authored, so a file the spec no longer
// produces must not survive a rebuild.
func TestWriteGeneratedDropsFilesTheSpecNoLongerProduces(t *testing.T) {
	p := scratch(t)
	if err := p.WriteGenerated([]backend.File{
		{Name: "main.swift", Body: []byte("x")},
		{Name: "Shapes.swift", Body: []byte("y")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteGenerated([]backend.File{{Name: "main.swift", Body: []byte("x2")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.GeneratedDir(), "Shapes.swift")); err == nil {
		t.Error("Shapes.swift survived a build that no longer emits it")
	}
}

// Nothing outside Generated/ is ever overwritten. This is the whole of the
// drift story, so it gets a test rather than a comment.
func TestWriteGeneratedNeverTouchesSources(t *testing.T) {
	p := scratch(t)
	if err := os.MkdirAll(p.SourcesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	hand := filepath.Join(p.SourcesDir(), "Notifications.swift")
	if err := os.WriteFile(hand, []byte("hand-written"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteGenerated([]backend.File{{Name: "main.swift", Body: []byte("x")}}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(hand)
	if err != nil || string(b) != "hand-written" {
		t.Errorf("hand-written source was disturbed: %q %v", b, err)
	}
}

func TestSwiftSourcesCoversBothDirectories(t *testing.T) {
	p := scratch(t)
	if err := os.MkdirAll(p.SourcesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.SourcesDir(), "Extra.swift"), []byte("//"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteGenerated([]backend.File{{Name: "main.swift", Body: []byte("//")}}); err != nil {
		t.Fatal(err)
	}
	got, err := p.SwiftSources()
	if err != nil {
		t.Fatalf("SwiftSources: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sources, want 2: %v", len(got), got)
	}
}

func TestLoadReportsAMissingSpec(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("want an error when menubar.yaml is absent, got nil")
	}
}
