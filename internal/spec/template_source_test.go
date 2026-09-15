package spec

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestShippedTemplatesAreFoundByName(t *testing.T) {
	src, file, ok, err := TemplatesIn("").Template("service")
	if err != nil || !ok {
		t.Fatalf("service: ok = %v, err = %v", ok, err)
	}
	if !strings.Contains(string(src), "launchagent:") {
		t.Errorf("service.yaml does not declare a launchagent watch:\n%s", src)
	}
	if !strings.Contains(file, "service.yaml") {
		t.Errorf("file = %q, want it to name service.yaml", file)
	}
}

func TestARepoTemplateIsFoundBesideTheShippedOnes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "health.yaml")
	if err := os.WriteFile(path, []byte("watch: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, file, ok, err := TemplatesIn(dir).Template("health")
	if err != nil || !ok || file != path {
		t.Fatalf("health: file = %q, ok = %v, err = %v", file, ok, err)
	}
	names := TemplatesIn(dir).Names()
	if !slices.Contains(names, "health") || !slices.Contains(names, "service") {
		t.Errorf("Names() = %v, want both the repo's and perch's", names)
	}
}

// A reader of menubar.yaml could not tell which service runs, and a fix to the
// shipped one would silently never arrive.
func TestARepoTemplateCannotTakeAShippedName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "service.yaml"), []byte("watch: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := TemplatesIn(dir).Template("service")
	if err == nil || !strings.Contains(err.Error(), "perch ships") {
		t.Errorf("err = %v, want a refusal naming the shipped template", err)
	}
	if err != nil && !strings.Contains(err.Error(), "perch's service.yaml") {
		t.Errorf("err = %v, want it to name the shipped file too", err)
	}
}

// Names() must not offer a name twice just because a repo file shadows a
// shipped one.
func TestAShadowingRepoTemplateListsOnce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "service.yaml"), []byte("watch: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := TemplatesIn(dir).Names()
	n := 0
	for _, name := range names {
		if name == "service" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Names() = %v, want \"service\" listed once, got %d times", names, n)
	}
}

func TestAMissingTemplateIsNotAnError(t *testing.T) {
	_, _, ok, err := TemplatesIn(t.TempDir()).Template("nope")
	if ok || err != nil {
		t.Errorf("ok = %v, err = %v; want not found and no error", ok, err)
	}
}

// A directory named health.yaml is a real error, not simply an absent
// template: the repo author has something there, just not what use: wants.
func TestATemplateThatIsADirectoryIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "health.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, ok, err := TemplatesIn(dir).Template("health")
	if err == nil || ok {
		t.Errorf("ok = %v, err = %v; want an error, not not-found", ok, err)
	}
}

func TestATemplateNameThatWalksOutOfTheDirIsRefused(t *testing.T) {
	_, _, ok, err := TemplatesIn(t.TempDir()).Template("../x")
	if err == nil || ok {
		t.Errorf("ok = %v, err = %v; want a refusal, not a lookup", ok, err)
	}
}
