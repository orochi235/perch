package swiftappkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

const windowDoc = `
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
window:
  url: http://localhost:4747/
  title: dash
  size: [1280, 860]
  zoom: {min: 0.5, max: 2.0, step: 0.1}
menu:
  - {text: Open, window: open}
  - {text: Quit, quit: true}
`

const noWindowDoc = `
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
menu:
  - {text: Quit, quit: true}
`

func TestWindowRuntimeEmittedOnlyWhenDeclared(t *testing.T) {
	with := emit(t, windowDoc)
	if _, ok := with["Window.swift"]; !ok {
		t.Error("a window: app got no Window.swift")
	}
	without := emit(t, noWindowDoc)
	if _, ok := without["Window.swift"]; ok {
		t.Error("an app with no window: links WebKit anyway")
	}
}

func TestWindowRuntimeImportsWebKit(t *testing.T) {
	files := emit(t, windowDoc)
	if !strings.Contains(files["Window.swift"], "import WebKit") {
		t.Error("Window.swift does not import WebKit")
	}
}

func TestWindowActionLowers(t *testing.T) {
	files := emit(t, windowDoc)
	if !strings.Contains(files["Render.swift"], ".window(.open)") {
		t.Errorf("Render.swift does not lower the window action:\n%s", files["Render.swift"])
	}
}

func TestWindowAppBuildsTheWindowAndMainMenu(t *testing.T) {
	main := emit(t, windowDoc)["main.swift"]
	for _, want := range []string{
		`WebWindow(`,
		`url: "http://localhost:4747/"`,
		`title: "dash"`,
		"width: 1280, height: 860",
		"ZoomConfig(min: 0.5, max: 2.0, step: 0.1)",
		"MainMenu.install",
		"applicationShouldHandleReopen",
		"setActivationPolicy(.regular)",
		"setActivationPolicy(.accessory)",
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main.swift is missing %q:\n%s", want, main)
		}
	}
}

func TestNoWindowAppMentionsNoWindow(t *testing.T) {
	main := emit(t, noWindowDoc)["main.swift"]
	for _, unwanted := range []string{"WebWindow", "MainMenu", "setActivationPolicy(.regular)"} {
		if strings.Contains(main, unwanted) {
			t.Errorf("an app with no window: emits %q", unwanted)
		}
	}
}

// The golden and typecheck tests cover Render.swift; this one says the whole
// window app — WebKit, the main menu, the Dock policy — actually compiles.
func TestWindowAppCompiles(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	sp, err := spec.Parse([]byte(windowDoc))
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
		t.Fatalf("the emitted window app does not compile: %v\n%s", err, out)
	}
	if len(out) > 0 {
		t.Errorf("the emitted window app compiles with warnings:\n%s", out)
	}
}
