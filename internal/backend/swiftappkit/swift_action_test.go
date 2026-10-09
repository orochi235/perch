package swiftappkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orochi235/perch/v2/internal/spec"
)

const swiftActionDoc = `
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
menu:
  - {text: Prefs, swift: Dash.showPreferences}
  - {text: Quit, quit: true}
`

func TestSwiftActionLowersToAMethodReference(t *testing.T) {
	files := emit(t, swiftActionDoc)
	if !strings.Contains(files["Render.swift"], ".swift(Dash.showPreferences)") {
		t.Errorf("Render.swift does not reference the method:\n%s", files["Render.swift"])
	}
}

// The point of the dotted path: perch cannot typecheck the target, swiftc can.
// This compiles the emitted call beside the file a consumer would write.
func TestSwiftActionCompilesAgainstHandWrittenSources(t *testing.T) {
	compileWithHand(t, swiftActionDoc, `import AppKit

enum Dash {
    static func showPreferences() {
        NSLog("hand-written")
    }
}
`)
}

const swiftFieldDoc = `
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
menu:
  - {field: Find, swift: Dash.find}
`

func TestSwiftActionInAFieldTakesTheText(t *testing.T) {
	if r := emit(t, swiftFieldDoc)["Render.swift"]; !strings.Contains(r, ".swift({ Dash.find(query1) })") {
		t.Errorf("Render.swift does not pass the text:\n%s", r)
	}
	compileWithHand(t, swiftFieldDoc, `import AppKit

enum Dash {
    static func find(_ query: String) {
        NSLog("%@", query)
    }
}
`)
}

// compileWithHand typechecks what doc emits beside hand, as Dash.swift.
func compileWithHand(t *testing.T, doc, hand string) {
	t.Helper()
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	files, err := New().Emit(s)
	if err != nil {
		t.Fatalf("Emit: %v", err)
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
	path := filepath.Join(dir, "Dash.swift")
	if err := os.WriteFile(path, []byte(hand), 0o644); err != nil {
		t.Fatal(err)
	}
	paths = append(paths, path)
	out, err := exec.Command(swiftc, append([]string{"-typecheck"}, paths...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("emitted swift: action does not compile against a hand-written target: %v\n%s", err, out)
	}
}
