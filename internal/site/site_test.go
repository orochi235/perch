package site

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// root is the repo, since the site is built out of its docs/ rather than out of
// anything a test could invent.
const root = "../.."

func TestExtractFencesGroupsAFileWithTheStatesBelowIt(t *testing.T) {
	md := "text\n\n```yaml\napp: {}\n```\n\n```state healthy\nw: true\n```\n\n```state broken\nw: false\n```\n\nmore\n"
	out, blocks := extractFences(md)
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want the file and its states as one", len(blocks))
	}
	if got := blocks[0].yaml; got != "app: {}" {
		t.Errorf("file is %q", got)
	}
	if len(blocks[0].states) != 2 || blocks[0].states[1].name != "broken" {
		t.Errorf("states are %+v", blocks[0].states)
	}
	if !strings.Contains(out, blocks[0].placeholder()) || strings.Contains(out, "```") {
		t.Errorf("the markdown still holds fences:\n%s", out)
	}
}

func TestExtractFencesLeavesAFileWithNoStatesAsCode(t *testing.T) {
	_, blocks := extractFences("```yaml\napp: {}\n```\n\ntext\n")
	if len(blocks) != 1 || len(blocks[0].states) != 0 {
		t.Fatalf("blocks are %+v", blocks)
	}
}

func TestHighlightMarksKeysStringsAndComments(t *testing.T) {
	got := highlightYAML("# a note\napp:\n  name: \"fleet\"\n  interval: 5s\n")
	for _, want := range []string{
		`<span class="c"># a note</span>`,
		`<span class="k">app</span>`,
		`<span class="s">&#34;fleet&#34;</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
}

func TestHighlightEscapesWhatItDoesNotColor(t *testing.T) {
	if got := highlightYAML("text: a <b> & c"); strings.Contains(got, "<b>") {
		t.Errorf("unescaped markup: %s", got)
	}
}

func TestSchemaSectionIsItsOwnPageWithHeadingsPromoted(t *testing.T) {
	src := "# menubar.yaml\n\nintro\n\n## watch\n\nbody\n\n### shape\n\ndeeper\n\n## status\n\nother\n"
	section, ok := schemaSection(src, "watch")
	if !ok {
		t.Fatal("no watch section")
	}
	if !strings.HasPrefix(section, "# watch") {
		t.Errorf("the section does not start at its own heading:\n%s", section)
	}
	if !strings.Contains(section, "\n## shape") || strings.Contains(section, "status") {
		t.Errorf("section is:\n%s", section)
	}

	intro, ok := schemaSection(src, introSection)
	if !ok || strings.Contains(intro, "## watch") {
		t.Errorf("intro is:\n%s", intro)
	}
}

// A link written for a reader of schema.md has to land on whichever page the
// section it names ended up on.
func TestLinksFollowASectionToItsPage(t *testing.T) {
	s, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	from := s.pageFor("docs/schema.md")
	if from == nil {
		t.Fatal("schema.md publishes no page")
	}
	if got := s.rewrite(from, "#icon-assets"); got != "../menubar/icons/#icon-assets" {
		t.Errorf("an anchor to another page resolves to %q", got)
	}
	if got := s.rewrite(from, "superpowers/specs/2026-09-05-perch-design.md"); !strings.HasPrefix(got, repo+"/blob/main/") {
		t.Errorf("a file the site does not publish resolves to %q", got)
	}
}

func TestBuildWritesEveryPageWithItsPreviews(t *testing.T) {
	if _, err := exec.LookPath("swiftc"); err != nil {
		t.Skip("swiftc not on PATH")
	}
	out := t.TempDir()
	if err := Build(root, out); err != nil {
		t.Fatal(err)
	}

	s, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range s.pages {
		path := filepath.Join(out, filepath.FromSlash(p.URL), "index.html")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", p.Title, err)
			continue
		}
		if strings.Contains(string(body), "perch-block") {
			t.Errorf("%s has a block that never rendered", p.Title)
		}
	}

	// The overview's preview is the whole point of the site: it has to carry
	// what the emitted app decided, not just the file it decided from.
	home, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`class="preview"`, `class="statusitem`, "3 running", "unreachable"} {
		if !strings.Contains(string(home), want) {
			t.Errorf("the overview's preview is missing %q", want)
		}
	}
}
