package swiftappkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

const quitDoc = `
app:
  name: w
  id: dev.example.w
  icon: gear
  interval: 5s
  quit:
    - when: svc.loaded
      confirm: "Quit w?"
      detail: "The server is running as a launchd service."
      buttons:
        - {text: Quit Both, agent: svc.stop}
        - {text: Quit Helper Only}
    - confirm: "Quit w?"
      detail: "The server keeps running."
watch:
  svc:
    launchagent: dev.example.svc
menu:
  - {text: Quit, quit: true}
`

func TestQuitRulesLowerInOrder(t *testing.T) {
	render := emit(t, quitDoc)["Render.swift"]
	if !strings.Contains(render, "func renderQuit(") {
		t.Fatalf("no renderQuit:\n%s", render)
	}
	both := strings.Index(render, `"Quit Both"`)
	fallback := strings.Index(render, `"The server keeps running."`)
	if both < 0 || fallback < 0 {
		t.Fatalf("missing a rule:\n%s", render)
	}
	if both > fallback {
		t.Error("the fallback is emitted before the guarded rule, so it always wins")
	}
}

func TestQuitAppImplementsShouldTerminate(t *testing.T) {
	main := emit(t, quitDoc)["main.swift"]
	if !strings.Contains(main, "func applicationShouldTerminate(") {
		t.Errorf("main.swift does not intercept termination:\n%s", main)
	}
}

// An app that never asks should not carry the machinery for asking.
func TestNoQuitRulesEmitNoPrompt(t *testing.T) {
	main := emit(t, noWindowDoc)["main.swift"]
	if strings.Contains(main, "applicationShouldTerminate") {
		t.Error("an app with no quit: rules intercepts termination anyway")
	}
	if strings.Contains(emit(t, noWindowDoc)["Render.swift"], "renderQuit") {
		t.Error("an app with no quit: rules emits renderQuit anyway")
	}
}

func TestQuitAppCompiles(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	sp, err := spec.Parse([]byte(quitDoc))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	files, err := New().Emit(sp)
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
	out, err := exec.Command(swiftc, append([]string{"-typecheck"}, paths...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("the emitted quit app does not compile: %v\n%s", err, out)
	}
	if len(out) > 0 {
		t.Errorf("the emitted quit app compiles with warnings:\n%s", out)
	}
}
