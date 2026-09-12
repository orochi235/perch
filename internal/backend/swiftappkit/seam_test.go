package swiftappkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

// delegateSeam is every NSApplicationDelegate lifecycle method reserved for
// menubar/Sources/. Controller declares the conformance and implements none of
// them, so a hand-written extension supplies the witness and AppKit calls it —
// which is how a repo keeps behavior the schema refuses (notification delivery,
// an embedded web view) without ejecting the whole app.
//
// Implementing one here would shadow the consumer's with no error and no
// warning: their file still compiles, their code just stops running.
var delegateSeam = []string{
	"applicationWillFinishLaunching",
	"applicationDidFinishLaunching",
	"applicationWillTerminate",
}

func TestEmitLeavesDelegateLifecycleToHandWrittenSources(t *testing.T) {
	for name, doc := range specs() {
		t.Run(name, func(t *testing.T) {
			files := emit(t, doc)
			for _, method := range delegateSeam {
				if strings.Contains(files["main.swift"], method) {
					t.Errorf("main.swift implements %s, which shadows a hand-written "+
						"menubar/Sources/ extension silently; see docs/schema.md", method)
				}
			}
		})
	}
}

// TestHandWrittenExtensionHooksTheDelegate compiles what perch emits beside the
// file a consumer would write. The absence check above says the method is not
// there; this says the seam it leaves open is one Swift will actually accept.
func TestHandWrittenExtensionHooksTheDelegate(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	s, err := spec.Parse([]byte(minimal))
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
	hand := filepath.Join(dir, "HandWritten.swift")
	if err := os.WriteFile(hand, []byte(`import AppKit

extension Controller {
    func applicationDidFinishLaunching(_ notification: Notification) {
        NSLog("hand-written")
    }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	paths = append(paths, hand)
	out, err := exec.Command(swiftc, append([]string{"-typecheck"}, paths...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("a hand-written delegate extension no longer compiles against the "+
			"emitted app: %v\n%s", err, out)
	}
	if len(out) > 0 {
		t.Errorf("a hand-written delegate extension compiles with warnings:\n%s", out)
	}
}

func specs() map[string]string {
	return map[string]string{
		"minimal":      minimal,
		"designDoc":    designDocExample,
		"everyFeature": everyFeature,
		"states":       states,
		"launchAgent":  launchAgent,
	}
}
