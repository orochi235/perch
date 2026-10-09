package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orochi235/perch/v2/internal/project"
)

func TestInitWritesASpecAndItsSchema(t *testing.T) {
	h := newHarness(t, "")
	if got := h.run("init"); got != 0 {
		t.Fatalf("status = %d\n%s", got, h.stderr())
	}
	p, err := project.Load(h.dir)
	if err != nil {
		t.Fatalf("the starter does not load: %v", err)
	}
	if name := filepath.Base(h.dir); p.Spec.App.Name != name || p.Spec.App.ID != defaultID(name) {
		t.Errorf("app = %q %q, want the directory's name", p.Spec.App.Name, p.Spec.App.ID)
	}
	if _, err := os.Stat(filepath.Join(h.dir, schemaFile)); err != nil {
		t.Errorf("no schema written: %v", err)
	}
}

func TestInitTakesANameAndAnID(t *testing.T) {
	h := newHarness(t, "")
	if got := h.run("init", "-name", "true", "-id", "tech.example.w"); got != 0 {
		t.Fatalf("status = %d\n%s", got, h.stderr())
	}
	p, err := project.Load(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.App.Name != "true" || p.Spec.App.ID != "tech.example.w" {
		t.Errorf("app = %q %q", p.Spec.App.Name, p.Spec.App.ID)
	}
}

func TestInitRefusesToOverwriteASpec(t *testing.T) {
	h := newHarness(t, exampleSpec)
	if got := h.run("init"); got != 1 || !strings.Contains(h.stderr(), "already exists") {
		t.Errorf("status = %d, stderr = %q", got, h.stderr())
	}
	if body, _ := os.ReadFile(filepath.Join(h.dir, project.SpecFile)); string(body) != exampleSpec {
		t.Error("init changed an existing menubar.yaml")
	}
}

func TestInitRefusesANameTheSpecRefuses(t *testing.T) {
	h := newHarness(t, "")
	if got := h.run("init", "-name", ".hidden"); got != 1 || !strings.Contains(h.stderr(), "-name or -id") {
		t.Errorf("status = %d, stderr = %q", got, h.stderr())
	}
	if _, err := os.Stat(filepath.Join(h.dir, project.SpecFile)); err == nil {
		t.Error("init wrote a spec it refused")
	}
}

func TestDefaultIDIsABundleID(t *testing.T) {
	for name, want := range map[string]string{
		"brick-icons": "dev.brick-icons.menubar",
		"My Widget":   "dev.my-widget.menubar",
		"__":          "dev.app.menubar",
	} {
		if got := defaultID(name); got != want {
			t.Errorf("%q: got %q, want %q", name, got, want)
		}
	}
}
