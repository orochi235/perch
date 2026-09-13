# Window, Swift action, and quit policy — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teach perch to put a WebKit window behind a menu item, call hand-written Swift from a menu item, and ask before quitting — then retire reviewplex's 904 hand-written lines onto it.

**Architecture:** Three schema additions, each following the existing pipeline: parse in `internal/spec`, validate in `internal/spec/validate.go`, lower to Swift in `internal/backend/swiftappkit/render.go`, execute in the embedded `runtime/*.swift`. Two installer bugs are fixed first because a window is what makes them visible. The consumer port lands last and is the acceptance test for all of it.

**Tech Stack:** Go 1.23+ generator, Swift/AppKit/WebKit output, `gopkg.in/yaml.v3`, CEL lowered at build time.

**Spec:** [`docs/superpowers/specs/2026-09-12-window-and-quit-design.md`](../specs/2026-09-12-window-and-quit-design.md)

## Global Constraints

- **macOS only.** `swiftc` (Xcode Command Line Tools) required for most tests.
- **Nothing evaluates CEL at runtime.** Every `when:`, `badge:` and `{{ }}` is lowered to a plain Swift expression at build time. An expression perch cannot lower is a build error.
- **Unknown keys are rejected everywhere.** New structs go through `decodeStrict`; every new field needs a `yaml:"..."` tag or it becomes an "unknown key" error.
- **The delegate seam is three methods.** `applicationWillFinishLaunching`, `applicationDidFinishLaunching`, `applicationWillTerminate` must never appear in emitted `main.swift` — `internal/backend/swiftappkit/seam_test.go` enforces it. `applicationShouldTerminate` and `applicationShouldHandleReopen` are perch's and are deliberately not added to that list.
- **Generated Swift is committed** in consuming repos, so a repo builds without perch installed.
- **Emitted files live only in `menubar/Generated/`.** `menubar/Sources/` is never read or written by perch.
- **`internal/schema/schema.go` mirrors `spec.Parse`.** Every schema change needs the JSON Schema updated in the same task, and `internal/schema/drift_test.go` checks the two agree.
- **Trap:** `~/.local/bin/perch` shadows `~/go/bin/perch`, and `~/go/bin` is not on PATH. `go install` alone changes nothing — copy the binary across after building.

---

### Task 1: Quit stays quit

`internal/install/plist.go` writes an unconditional `KeepAlive`, so any installed widget that exits 0 from its own Quit item is restarted by launchd at once. Every example in the docs ends with `{text: Quit, quit: true}` and none of them work once installed. Independent of everything else in this plan.

**Files:**
- Modify: `internal/install/plist.go`
- Test: `internal/install/plist_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `install.AgentPlist(label, program, path string) string` — unchanged signature, changed output.

- [ ] **Step 1: Write the failing test**

In `internal/install/plist_test.go`:

```go
func TestAgentPlistKeepsAliveOnlyOnCrash(t *testing.T) {
	got := AgentPlist("dev.example.w", "/tmp/w", "/usr/bin")
	if !strings.Contains(got, "<key>SuccessfulExit</key>") {
		t.Error("KeepAlive is unconditional, so Quit exits 0 and launchd restarts the app immediately")
	}
	if strings.Contains(got, "<key>KeepAlive</key>\n\t<true/>") {
		t.Error("KeepAlive is still a bare <true/>")
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./internal/install/ -run TestAgentPlistKeepsAliveOnlyOnCrash -v`
Expected: FAIL on the first assertion — the template has no `SuccessfulExit`.

- [ ] **Step 3: Change the template**

In `internal/install/plist.go`, replace the two `KeepAlive` lines inside `agentTmpl`:

```
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
```

and update `AgentPlist`'s doc comment to say why:

```go
// AgentPlist renders the LaunchAgent that keeps the app running. It carries a
// PATH because launchd's default cannot resolve a bare command in run:, and a
// watch that cannot resolve its command just leaves the widget looking broken.
//
// KeepAlive is conditional on a non-zero exit: a widget quit from its own Quit
// item exits 0 and must stay quit, while a crash still brings it back.
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/install/ -v`
Expected: PASS, including the existing plist tests.

- [ ] **Step 5: Commit**

```bash
git add internal/install/plist.go internal/install/plist_test.go
git commit -m "keep a widget alive on a crash, not on its own Quit"
```

---

### Task 2: The `swift:` action

A menu item calls a static method in `menubar/Sources/`. The seam that already exists is launch-only; this is what lets a menu item invoke hand-written code.

**Files:**
- Modify: `internal/spec/menu.go`, `internal/spec/validate.go`, `internal/backend/swiftappkit/render.go`, `internal/backend/swiftappkit/runtime/Runtime.swift`, `internal/schema/schema.go`, `docs/schema.md`
- Test: `internal/spec/menu_test.go`, `internal/backend/swiftappkit/swift_action_test.go`

**Interfaces:**
- Consumes: `spec.Action`, `spec.ActionKind`, `menuGen.action` from the existing pipeline.
- Produces:
  - `spec.ActionSwift ActionKind` — new enum member, appended after `ActionAgent`.
  - `spec.Action.Swift string` — the dotted path, e.g. `"Dash.showPreferences"`.
  - `spec.parseSwiftAction(src, path string) (string, error)`.
  - Swift `MenuAction.swift(() -> Void)`.

- [ ] **Step 1: Write the failing parser tests**

In `internal/spec/menu_test.go`:

```go
func TestSwiftActionTakesADottedPath(t *testing.T) {
	s, err := Parse([]byte(`
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
menu:
  - {text: Prefs, swift: Dash.showPreferences}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := s.Menu[0].Action
	if got.Kind != ActionSwift {
		t.Fatalf("Kind = %v, want ActionSwift", got.Kind)
	}
	if got.Swift != "Dash.showPreferences" {
		t.Errorf("Swift = %q, want Dash.showPreferences", got.Swift)
	}
}

func TestSwiftActionRefusals(t *testing.T) {
	cases := map[string]string{
		"bare name":     `{text: P, swift: showPreferences}`,
		"leading dot":   `{text: P, swift: .showPreferences}`,
		"trailing dot":  `{text: P, swift: Dash.}`,
		"not an ident":  `{text: P, swift: "Dash.show preferences"}`,
		"two actions":   `{text: P, swift: Dash.show, quit: true}`,
	}
	for name, item := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(
				"app: {name: w, id: dev.example.w, icon: gear, interval: 5s}\nmenu:\n  - " + item + "\n"))
			if err == nil {
				t.Fatal("accepted; want a refusal")
			}
		})
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/spec/ -run 'TestSwiftAction' -v`
Expected: FAIL — `ActionSwift` undefined, and `swift` is an unknown key.

- [ ] **Step 3: Add the action to the spec**

In `internal/spec/menu.go`, append to the `ActionKind` block and its `String()`:

```go
	ActionAgent
	ActionSwift
)
```

```go
	case ActionAgent:
		return "agent"
	case ActionSwift:
		return "swift"
	}
```

Add the field to `Action`:

```go
	Agent    string // ActionAgent: the launchagent watch acted on
	Verb     AgentVerb
	Swift    string // ActionSwift: a dotted path, Type.method
}
```

Add the key to `itemFields`:

```go
	Agent string    `yaml:"agent"`
	Swift string    `yaml:"swift"`
}
```

Add the branch in `actionFrom`, before the arity check:

```go
	if f.Swift != "" {
		verbs = append(verbs, "swift")
		path, err := parseSwiftAction(f.Swift, path)
		if err != nil {
			return a, err
		}
		a.Kind, a.Swift = ActionSwift, path
	}
```

Update the arity message to name it:

```go
		return a, fmt.Errorf("%s: has %v; an item takes at most one of run, open, post, quit, agent or swift", path, verbs)
```

And add the parser beside `parseAgentAction`:

```go
// swiftPath is Type.method: two Swift identifiers and one dot. Dotted rather
// than bare so a hand-written name cannot collide with an emitted one —
// Controller, Results, Draw, Watcher, renderFace, renderMenu.
var swiftPath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\.[A-Za-z_][A-Za-z0-9_]*$`)

func parseSwiftAction(src, path string) (string, error) {
	if !swiftPath.MatchString(src) {
		return "", fmt.Errorf("%s.swift: %q is not <Type>.<method>; swift: names a static method in menubar/Sources/, e.g. Dash.showPreferences", path, src)
	}
	return src, nil
}
```

- [ ] **Step 4: Run the parser tests**

Run: `go test ./internal/spec/ -run 'TestSwiftAction' -v`
Expected: PASS.

- [ ] **Step 5: Write the failing emit test**

Create `internal/backend/swiftappkit/swift_action_test.go`:

```go
package swiftappkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
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
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	s, err := spec.Parse([]byte(swiftActionDoc))
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
	hand := filepath.Join(dir, "Dash.swift")
	if err := os.WriteFile(hand, []byte(`import AppKit

enum Dash {
    static func showPreferences() {
        NSLog("hand-written")
    }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	paths = append(paths, hand)
	out, err := exec.Command(swiftc, append([]string{"-typecheck"}, paths...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("emitted swift: action does not compile against a hand-written target: %v\n%s", err, out)
	}
}
```

- [ ] **Step 6: Run it to make sure it fails**

Run: `go test ./internal/backend/swiftappkit/ -run TestSwiftAction -v`
Expected: FAIL — `render.go` returns "unknown action".

- [ ] **Step 7: Lower it, and give the runtime a case**

In `internal/backend/swiftappkit/render.go`, add to `menuGen.action` before the trailing error:

```go
	case spec.ActionSwift:
		// Emitted verbatim as a method reference, which is a () -> Void. perch
		// cannot check the target exists; swiftc does, in the same module.
		return ".swift(" + a.Swift + ")", nil
```

In `internal/backend/swiftappkit/runtime/Runtime.swift`, add the case to `MenuAction`:

```swift
enum MenuAction {
    case run([String])
    case open(String)
    case post(url: String, body: String)
    case agent(label: String, plist: String, verb: LaunchAgentVerb)
    case swift(() -> Void)
    case quit
}
```

and to `Act.perform`:

```swift
        case .swift(let body): swift(body, then: repoll)
```

with the implementation beside `open`:

```swift
    /// Hand-written Swift from menubar/Sources/. It re-polls like every other
    /// action, and raises no alert: there is no exit status to inspect, so
    /// reporting a failure belongs to the hook.
    static func swift(_ body: () -> Void, then repoll: @escaping () -> Void) {
        body()
        repoll()
    }
```

- [ ] **Step 8: Run the emit tests**

Run: `go test ./internal/backend/swiftappkit/ -run TestSwiftAction -v`
Expected: PASS both.

- [ ] **Step 9: Update the JSON Schema and its drift test**

In `internal/schema/schema.go`, add to the menu item properties inside `definitions`:

```
          "swift": {"type": "string", "pattern": "^[A-Za-z_][A-Za-z0-9_]*\\.[A-Za-z_][A-Za-z0-9_]*$", "description": "A static method in menubar/Sources/, written Type.method."},
```

Run: `go test ./internal/schema/ -v`
Expected: PASS. If `drift_test.go` fails, it is naming a key the schema and the parser disagree on — fix the schema, not the parser.

- [ ] **Step 10: Document it**

In `docs/schema.md`, add the row to the action table in `## menu`:

```
| `swift` | A string, `<Type>.<method>`: a static method in `menubar/Sources/`. |
```

and a paragraph at the end of `### Actions`:

```
`swift:` calls hand-written Swift. It names a static method — dotted, so your
names cannot collide with the emitted ones — and perch emits the call without
checking it exists; `swiftc` does that when the app is built, in the same module
as `menubar/Sources/*.swift`. So `perch build` alone will not catch a misspelled
method, but `run` and `install` will. It re-polls afterward like every other
action, and raises no alert on failure: there is no exit status to inspect.

This is the menu-side half of [hooking the app from
Sources/](#hooking-the-app-from-sources), which is otherwise launch-only.
```

Add the cross-reference in the seam section, after the sentence about the status item staying perch's:

```
A menu item reaches hand-written code through [`swift:`](#actions).
```

- [ ] **Step 11: Run everything and commit**

Run: `go test ./...`
Expected: PASS.

```bash
git add internal/spec internal/backend internal/schema docs/schema.md
git commit -m "call hand-written Swift from a menu item"
```

---

### Task 3: An app icon in the bundle

`internal/install/bundle.go` writes no `.icns` and no `CFBundleIconFile`. Invisible while `LSUIElement` keeps the app out of the Dock; a `window:` app promotes to `.regular` and gets a tile with a blank generic icon.

**This fills a gap in the spec**, which says the pipeline is needed but not where the source PNG comes from. The answer: `menubar/AppIcon.png` beside the spec, a fixed path with no schema key — it is meaningful only when a window exists, and a key would be a second way to say what the file's presence already says. Update the spec's installer section to say so as part of this task.

**Files:**
- Create: `internal/install/icns.go`, `internal/install/icns_test.go`
- Modify: `internal/install/bundle.go`, `internal/install/plist.go`, `internal/project/project.go`, `cmd/perch/main.go`, `docs/schema.md`, `docs/superpowers/specs/2026-09-12-window-and-quit-design.md`
- Test: `internal/install/icns_test.go`, `internal/install/bundle_test.go`

**Interfaces:**
- Consumes: `install.BundleOpts`, `install.App` from Task 1's file.
- Produces:
  - `install.BuildICNS(srcPNG, destICNS string) error`
  - `install.BundleOpts.AppIcon string` — path to the source PNG, or empty.
  - `install.App.IconFile string` — basename written into `CFBundleIconFile`, or empty.
  - `(*project.Project).AppIconPath() string` — `<root>/menubar/AppIcon.png`.

- [ ] **Step 1: Write the failing icns test**

Create `internal/install/icns_test.go`:

```go
package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildICNSProducesEveryRendition(t *testing.T) {
	for _, tool := range []string{"sips", "iconutil"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "AppIcon.png")
	writeTestPNG(t, src, 1024)

	dest := filepath.Join(dir, "app.icns")
	if err := BuildICNS(src, dest); err != nil {
		t.Fatalf("BuildICNS: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("no icns written: %v", err)
	}
	if info.Size() == 0 {
		t.Error("icns is empty")
	}
}

func TestBuildICNSLeavesNoScratchDirectory(t *testing.T) {
	for _, tool := range []string{"sips", "iconutil"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "AppIcon.png")
	writeTestPNG(t, src, 512)
	if err := BuildICNS(src, filepath.Join(dir, "app.icns")); err != nil {
		t.Fatalf("BuildICNS: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("left a scratch directory behind: %s", e.Name())
		}
	}
}

// writeTestPNG writes a square PNG with sips, so the test needs no fixture.
func writeTestPNG(t *testing.T, path string, size int) {
	t.Helper()
	// A 1x1 opaque black PNG, scaled up by sips into a real square.
	seed := filepath.Join(t.TempDir(), "seed.png")
	if err := os.WriteFile(seed, onePixelPNG, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sips", "-z", itoa(size), itoa(size), seed, "--out", path).CombinedOutput()
	if err != nil {
		t.Fatalf("sips: %v\n%s", err, out)
	}
}
```

Add the fixture and helper at the bottom of the same file:

```go
func itoa(n int) string { return strconv.Itoa(n) }

// onePixelPNG is the smallest valid PNG: 1x1, 8-bit RGB, black.
var onePixelPNG = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
	0x00, 0x00, 0x00, 0x0c, 'I', 'D', 'A', 'T',
	0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x03, 0x01, 0x01, 0x00,
	0x18, 0xdd, 0x8d, 0xb0,
	0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
}
```

with `"strconv"` added to the imports.

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./internal/install/ -run TestBuildICNS -v`
Expected: FAIL — `BuildICNS` undefined.

- [ ] **Step 3: Write the icns builder**

Create `internal/install/icns.go`:

```go
package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// icnsSizes are the five point sizes macOS wants, each at 1x and 2x.
var icnsSizes = []int{16, 32, 128, 256, 512}

// BuildICNS renders srcPNG into the ten renditions macOS expects and packs them
// into destICNS. iconutil insists on a directory named *.iconset, so the
// scratch directory is made inside its own temp dir rather than beside dest.
func BuildICNS(srcPNG, destICNS string) error {
	tmp, err := os.MkdirTemp("", "perch-iconset-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	iconset := filepath.Join(tmp, "icon.iconset")
	if err := os.MkdirAll(iconset, 0o755); err != nil {
		return err
	}
	for _, size := range icnsSizes {
		for _, scale := range []int{1, 2} {
			px := size * scale
			name := fmt.Sprintf("icon_%dx%d.png", size, size)
			if scale == 2 {
				name = fmt.Sprintf("icon_%dx%d@2x.png", size, size)
			}
			out, err := exec.Command("sips",
				"-z", strconv.Itoa(px), strconv.Itoa(px), srcPNG,
				"--out", filepath.Join(iconset, name)).CombinedOutput()
			if err != nil {
				return fmt.Errorf("sips %dx%d: %w: %s", px, px, err, out)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(destICNS), 0o755); err != nil {
		return err
	}
	out, err := exec.Command("iconutil", "-c", "icns", iconset, "-o", destICNS).CombinedOutput()
	if err != nil {
		return fmt.Errorf("iconutil: %w: %s", err, out)
	}
	return nil
}
```

- [ ] **Step 4: Run the icns tests**

Run: `go test ./internal/install/ -run TestBuildICNS -v`
Expected: PASS.

- [ ] **Step 5: Write the failing bundle test**

Add to `internal/install/bundle_test.go`:

```go
func TestBundleWritesAnAppIconWhenOneIsThere(t *testing.T) {
	for _, tool := range []string{"sips", "iconutil"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	dir := t.TempDir()
	icon := filepath.Join(dir, "AppIcon.png")
	writeTestPNG(t, icon, 512)

	dest := filepath.Join(dir, "W.app")
	err := BuildBundle(BundleOpts{
		App:     App{Name: "W", ID: "dev.example.w", Executable: "W", IconFile: "AppIcon"},
		Sources: []string{},
		Dest:    dest,
		AppIcon: icon,
		Compile: func(sources []string, out string) error { return os.WriteFile(out, []byte("x"), 0o755) },
		Sign:    func(bundle, identity string) error { return nil },
	})
	if err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "Contents", "Resources", "AppIcon.icns")); err != nil {
		t.Errorf("no AppIcon.icns in the bundle: %v", err)
	}
	info, err := os.ReadFile(filepath.Join(dest, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(info), "<key>CFBundleIconFile</key>") {
		t.Error("Info.plist names no icon, so the Dock tile stays generic")
	}
}

func TestBundleWithoutAnAppIconNamesNone(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "W.app")
	err := BuildBundle(BundleOpts{
		App:     App{Name: "W", ID: "dev.example.w", Executable: "W"},
		Dest:    dest,
		Compile: func(sources []string, out string) error { return os.WriteFile(out, []byte("x"), 0o755) },
		Sign:    func(bundle, identity string) error { return nil },
	})
	if err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	info, err := os.ReadFile(filepath.Join(dest, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(info), "CFBundleIconFile") {
		t.Error("named an icon that is not there")
	}
}
```

- [ ] **Step 6: Run it to make sure it fails**

Run: `go test ./internal/install/ -run TestBundle -v`
Expected: FAIL — `BundleOpts` has no `AppIcon`, `App` has no `IconFile`.

- [ ] **Step 7: Wire the icon into the bundle**

In `internal/install/plist.go`, add the field to `App`:

```go
	// IconFile is the basename of the .icns in Resources, without its
	// extension. Empty leaves CFBundleIconFile out, which is right for a
	// status-bar app that never shows a Dock tile.
	IconFile string
}
```

and add the conditional key to `infoTmpl`, after `CFBundleExecutable`:

```
	{{if .IconFile}}<key>CFBundleIconFile</key>
	<string>{{.IconFile}}</string>
	{{end}}
```

and carry it through `InfoPlist`:

```go
	_ = infoTmpl.Execute(&sb, App{
		Name:       escape(a.Name),
		ID:         escape(a.ID),
		Executable: escape(a.Executable),
		IconFile:   escape(a.IconFile),
	})
```

In `internal/install/bundle.go`, add to `BundleOpts`:

```go
	// AppIcon is a source .png rendered into Resources/<IconFile>.icns. Empty
	// is ordinary: only an app that takes a Dock tile needs one.
	AppIcon string
```

and in `BuildBundle`, after `copyIcons` and before writing Info.plist:

```go
	if o.AppIcon != "" {
		icns := filepath.Join(staged, "Contents", "Resources", o.App.IconFile+".icns")
		if err := BuildICNS(o.AppIcon, icns); err != nil {
			return err
		}
	}
```

- [ ] **Step 8: Run the bundle tests**

Run: `go test ./internal/install/ -v`
Expected: PASS.

- [ ] **Step 9: Find the icon from the project, and touch the bundle**

In `internal/project/project.go`, beside `IconsDir`:

```go
// AppIconPath is artwork for the Dock tile, rendered into the bundle as .icns.
// Only an app that takes a tile needs one, so its absence is ordinary.
func (p *Project) AppIconPath() string {
	return filepath.Join(p.Root, "menubar", "AppIcon.png")
}
```

In `cmd/perch/main.go`, at both `BuildBundle` call sites (around the `install` and `run` paths), compute the icon and pass it:

```go
	appIcon := ""
	if _, err := os.Stat(p.AppIconPath()); err == nil {
		appIcon = p.AppIconPath()
		app.IconFile = "AppIcon"
	}
```

and add `AppIcon: appIcon` to the `BundleOpts` literal.

After the bundle is in place — in the `install` path only, after `BuildBundle` returns — touch it:

```go
	// LaunchServices caches icons per bundle, so a changed icon does not appear
	// until the bundle's mtime moves.
	now := time.Now()
	_ = os.Chtimes(dest, now, now)
```

with `"time"` added to the imports.

- [ ] **Step 10: Document it**

In `docs/schema.md`, at the end of `## Icon assets`:

```
### The Dock tile

`menubar/Icons` is the status item's artwork. A [`window:`](#window) app also
takes a Dock tile while its window is open, and that wants a different file:
`menubar/AppIcon.png`, one square PNG at 1024×1024. `perch install` renders it
into the ten sizes macOS asks for and names it in the bundle. There is no key
for it — the file being there is what turns it on.

Without one the tile is the generic blank application icon.
```

In `docs/superpowers/specs/2026-09-12-window-and-quit-design.md`, in the "Two installer bugs a window exposes" section, replace "the pipeline reviewplex has in bash: ten renditions from one source PNG" with:

```
the pipeline reviewplex has in bash: ten renditions from `menubar/AppIcon.png`
```

and add after that paragraph:

```
The source is a fixed path rather than a schema key. The file being there is
already the whole of the decision, and a key would be a second way to say it.
```

- [ ] **Step 11: Run everything and commit**

Run: `go test ./...`
Expected: PASS.

```bash
git add internal/install internal/project cmd/perch docs/schema.md docs/superpowers/specs
git commit -m "render menubar/AppIcon.png into the bundle as an icns"
```

---

### Task 4: `window:` parses and validates

**Files:**
- Create: `internal/spec/window.go`, `internal/spec/window_test.go`
- Modify: `internal/spec/spec.go`, `internal/spec/menu.go`, `internal/spec/validate.go`, `internal/schema/schema.go`

**Interfaces:**
- Consumes: `spec.Spec`, `spec.rawSpec`, `decodeStrict`, `Action`/`ActionKind` from Task 2.
- Produces:
  - `spec.Window{URL, Title string; Width, Height int; Zoom *Zoom}`
  - `spec.Zoom{Min, Max, Step float64}`
  - `spec.Spec.Window *Window` — nil when the block is absent.
  - `spec.ActionWindow ActionKind`, `spec.Action.Window WindowVerb`.
  - `spec.WindowVerb string` with `WindowOpen`/`WindowClose`/`WindowReload`, and `spec.WindowVerbs []WindowVerb`.

- [ ] **Step 1: Write the failing tests**

Create `internal/spec/window_test.go`:

```go
package spec

import "testing"

const windowDoc = `
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
window:
  url: http://localhost:4747/
  title: reviewplex
  size: [1280, 860]
  zoom: {min: 0.5, max: 2.0, step: 0.1}
menu:
  - {text: Open, window: open}
  - {text: Quit, quit: true}
`

func TestWindowParses(t *testing.T) {
	s, err := Parse([]byte(windowDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	w := s.Window
	if w == nil {
		t.Fatal("Window is nil")
	}
	if w.URL != "http://localhost:4747/" || w.Title != "reviewplex" {
		t.Errorf("url/title = %q/%q", w.URL, w.Title)
	}
	if w.Width != 1280 || w.Height != 860 {
		t.Errorf("size = %dx%d, want 1280x860", w.Width, w.Height)
	}
	if w.Zoom == nil || w.Zoom.Min != 0.5 || w.Zoom.Max != 2.0 || w.Zoom.Step != 0.1 {
		t.Errorf("zoom = %+v", w.Zoom)
	}
	if s.Menu[0].Action.Kind != ActionWindow || s.Menu[0].Action.Window != WindowOpen {
		t.Errorf("action = %v/%v", s.Menu[0].Action.Kind, s.Menu[0].Action.Window)
	}
}

func TestWindowDefaults(t *testing.T) {
	s, err := Parse([]byte(`
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
window: {url: "http://localhost:1/"}
menu:
  - {text: Open, window: open}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Window.Title != "w" {
		t.Errorf("title = %q, want the app name", s.Window.Title)
	}
	if s.Window.Width != 1024 || s.Window.Height != 768 {
		t.Errorf("size = %dx%d, want the 1024x768 default", s.Window.Width, s.Window.Height)
	}
	if s.Window.Zoom != nil {
		t.Error("zoom is on without being asked for")
	}
}

func TestWindowRefusals(t *testing.T) {
	cases := map[string]string{
		"no url":            "window: {title: w}",
		"not a url":         `window: {url: "not a url"}`,
		"size of one":       `window: {url: "http://x/", size: [800]}`,
		"size of three":     `window: {url: "http://x/", size: [800, 600, 400]}`,
		"zero size":         `window: {url: "http://x/", size: [0, 600]}`,
		"zoom min over max": `window: {url: "http://x/", zoom: {min: 2.0, max: 0.5, step: 0.1}}`,
		"zoom step zero":    `window: {url: "http://x/", zoom: {min: 0.5, max: 2.0, step: 0}}`,
		"unknown key":       `window: {url: "http://x/", widht: 10}`,
	}
	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(
				"app: {name: w, id: dev.example.w, icon: gear, interval: 5s}\n" + block + "\n"))
			if err == nil {
				t.Fatal("accepted; want a refusal")
			}
		})
	}
}

func TestWindowActionNeedsTheBlock(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
menu:
  - {text: Open, window: open}
`))
	if err == nil {
		t.Fatal("accepted a window: action with no window: block")
	}
}

func TestWindowVerbRefused(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
window: {url: "http://x/"}
menu:
  - {text: Open, window: maximize}
`))
	if err == nil {
		t.Fatal("accepted an unknown window verb")
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/spec/ -run TestWindow -v`
Expected: FAIL — `window` is an unknown key, `ActionWindow` undefined.

- [ ] **Step 3: Write the window parser**

Create `internal/spec/window.go`:

```go
package spec

import (
	"fmt"
	"net/url"

	"gopkg.in/yaml.v3"
)

// Window is the one WebKit window an app may declare. One per app, matching
// one status item per app, which is why nothing names it.
type Window struct {
	URL    string
	Title  string
	Width  int
	Height int
	// Zoom is nil when the author asked for no zoom controls, which is what
	// leaves Actual Size, Zoom In and Zoom Out out of the View menu.
	Zoom *Zoom
}

// Zoom bounds the page zoom. Clamped so a stray Cmd-+ cannot zoom a page into
// uselessness.
type Zoom struct {
	Min  float64
	Max  float64
	Step float64
}

const (
	defaultWindowWidth  = 1024
	defaultWindowHeight = 768
)

type rawWindow struct {
	URL   string    `yaml:"url"`
	Title string    `yaml:"title"`
	Size  []int     `yaml:"size"`
	Zoom  *rawZoom  `yaml:"zoom"`
}

type rawZoom struct {
	Min  float64 `yaml:"min"`
	Max  float64 `yaml:"max"`
	Step float64 `yaml:"step"`
}

// parseWindow reads the window: block. appName is the title's default, so an
// app that wants its own name on the window says nothing.
func parseWindow(n *yaml.Node, appName string) (*Window, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	var raw rawWindow
	if err := decodeStrict(n, &raw, "window"); err != nil {
		return nil, err
	}
	w := &Window{
		URL:    raw.URL,
		Title:  raw.Title,
		Width:  defaultWindowWidth,
		Height: defaultWindowHeight,
	}
	if w.Title == "" {
		w.Title = appName
	}
	switch len(raw.Size) {
	case 0:
	case 2:
		w.Width, w.Height = raw.Size[0], raw.Size[1]
	default:
		return nil, fmt.Errorf("window.size: want [width, height], got %d value(s)", len(raw.Size))
	}
	if raw.Zoom != nil {
		w.Zoom = &Zoom{Min: raw.Zoom.Min, Max: raw.Zoom.Max, Step: raw.Zoom.Step}
	}
	return w, nil
}

func (w *Window) validate() error {
	if w.URL == "" {
		return fmt.Errorf("window.url: required (the page the window loads)")
	}
	u, err := url.Parse(w.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("window.url: %q is not a URL with a scheme and a host", w.URL)
	}
	if w.Width <= 0 || w.Height <= 0 {
		return fmt.Errorf("window.size: both values must be positive, got %dx%d", w.Width, w.Height)
	}
	if z := w.Zoom; z != nil {
		if z.Min <= 0 || z.Max <= 0 || z.Step <= 0 {
			return fmt.Errorf("window.zoom: min, max and step must all be positive, got %v/%v/%v", z.Min, z.Max, z.Step)
		}
		if z.Min >= z.Max {
			return fmt.Errorf("window.zoom: min %v is not below max %v, so there is nothing to zoom between", z.Min, z.Max)
		}
	}
	return nil
}
```

- [ ] **Step 4: Add the window action**

In `internal/spec/menu.go`, add the verb type beside `AgentVerb`:

```go
// WindowVerb is what a window: action does to the declared window.
type WindowVerb string

const (
	WindowOpen   WindowVerb = "open"
	WindowClose  WindowVerb = "close"
	WindowReload WindowVerb = "reload"
)

// WindowVerbs is every verb window: accepts, in the order the docs list them.
var WindowVerbs = []WindowVerb{WindowOpen, WindowClose, WindowReload}
```

Append `ActionWindow` to the `ActionKind` block and its `String()` (returning `"window"`), add `Window WindowVerb` to `Action`, add `Window string` with tag `yaml:"window"` to `itemFields`, and add to `actionFrom`:

```go
	if f.Window != "" {
		verbs = append(verbs, "window")
		verb, err := parseWindowAction(f.Window, path)
		if err != nil {
			return a, err
		}
		a.Kind, a.Window = ActionWindow, verb
	}
```

extending the arity message to `"... quit, agent, swift or window"`, and add the parser:

```go
// parseWindowAction reads a bare verb. There is one window per app, so unlike
// agent: there is nothing to name.
func parseWindowAction(src, path string) (WindowVerb, error) {
	for _, v := range WindowVerbs {
		if src == string(v) {
			return v, nil
		}
	}
	var names []string
	for _, v := range WindowVerbs {
		names = append(names, string(v))
	}
	return "", fmt.Errorf("%s.window: %q is not something perch can do to a window; it does %s", path, src, strings.Join(names, ", "))
}
```

- [ ] **Step 5: Hang it off the Spec and validate it**

In `internal/spec/spec.go`, add `Window *Window` to `Spec`, add `Window yaml.Node` with tag `yaml:"window"` to `rawSpec`, and parse it in `Parse` after the app block is built (it needs `s.App.Name` for the title default):

```go
	win, err := parseWindow(&raw.Window, s.App.Name)
	if err != nil {
		return nil, err
	}
	s.Window = win
```

In `internal/spec/validate.go`, call it inside `validate()` after the app checks:

```go
	if s.Window != nil {
		if err := s.Window.validate(); err != nil {
			return err
		}
	}
```

and pass the window down to `validateItems`, changing its signature to
`validateItems(items []Item, path string, watches []Watch, window *Window) error`
(updating the recursive call and the call in `validate`), with the check in `Action.validate` — which also takes the window now:

```go
	case ActionWindow:
		if window == nil {
			return fmt.Errorf("%s.window: this file declares no window: block, so there is nothing to %s", path, a.Window)
		}
```

- [ ] **Step 6: Run the spec tests**

Run: `go test ./internal/spec/ -v`
Expected: PASS, including the existing tests.

- [ ] **Step 7: Update the JSON Schema**

In `internal/schema/schema.go`, add to the top-level `properties`:

```
    "window": {
      "type": "object",
      "description": "One WebKit window, opened from a menu item.",
      "required": ["url"],
      "additionalProperties": false,
      "properties": {
        "url": {"type": "string", "description": "The page the window loads."},
        "title": {"type": "string", "description": "Window title; defaults to app.name."},
        "size": {"type": "array", "minItems": 2, "maxItems": 2, "items": {"type": "integer", "minimum": 1}, "description": "[width, height] in points; defaults to [1024, 768]."},
        "zoom": {
          "type": "object",
          "description": "Page zoom bounds. Omit for no zoom controls.",
          "required": ["min", "max", "step"],
          "additionalProperties": false,
          "properties": {
            "min": {"type": "number", "exclusiveMinimum": 0},
            "max": {"type": "number", "exclusiveMinimum": 0},
            "step": {"type": "number", "exclusiveMinimum": 0}
          }
        }
      }
    },
```

and to the menu item properties:

```
          "window": {"enum": ["open", "close", "reload"], "description": "Act on the declared window."},
```

Run: `go test ./internal/schema/ -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/spec internal/schema
git commit -m "parse and validate a window: block"
```

---

### Task 5: The window at runtime

The Swift that draws it. Emitted only when `window:` is declared, so an app without one links no WebKit.

**Files:**
- Create: `internal/backend/swiftappkit/runtime/Window.swift`
- Modify: `internal/backend/swiftappkit/emit.go`
- Test: `internal/backend/swiftappkit/window_test.go`

**Interfaces:**
- Consumes: `MenuNode`, `Draw`, `Act` from `Runtime.swift`.
- Produces, for Task 6 to emit calls against:
  - `final class WebWindow` with `init(url:title:width:height:zoom:)`, `show()`, `hide()`, `load()`, `zoomIn()`, `zoomOut()`, `actualSize()`, `var isPresented: Bool`, `var onVisibilityChange: ((Bool) -> Void)?`
  - `struct ZoomConfig { let min: CGFloat; let max: CGFloat; let step: CGFloat }`
  - `enum MainMenu { static func install(appName:window:mirror:repoll:) }`
  - `final class MenuMirror: NSObject, NSMenuDelegate`

- [ ] **Step 1: Write the failing test**

Create `internal/backend/swiftappkit/window_test.go`:

```go
package swiftappkit

import (
	"strings"
	"testing"
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
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `go test ./internal/backend/swiftappkit/ -run TestWindowRuntime -v`
Expected: FAIL — no `Window.swift` is emitted.

- [ ] **Step 3: Write the runtime**

Create `internal/backend/swiftappkit/runtime/Window.swift`:

```swift
// Emitted only when menubar.yaml declares a window:, so an app without one
// links no WebKit.
import AppKit
import WebKit

struct ZoomConfig {
    let min: CGFloat
    let max: CGFloat
    let step: CGFloat
}

/// One long-lived window whose WebView is hidden rather than destroyed, so
/// reopening keeps scroll position and whatever the user had expanded.
final class WebWindow: NSObject, NSWindowDelegate, WKNavigationDelegate {
    private let url: URL
    private let title: String
    private let width: CGFloat
    private let height: CGFloat
    private let zoom: ZoomConfig?
    private let autosaveName: String

    private var window: NSWindow?
    private var webView: WKWebView?
    private var loadFailed = false

    /// Raised and lowered as the window appears and disappears, so the app only
    /// occupies the Dock while there is something to click on.
    var onVisibilityChange: ((Bool) -> Void)?

    init(url: String, title: String, width: Int, height: Int, zoom: ZoomConfig?, autosaveName: String) {
        self.url = URL(string: url) ?? URL(fileURLWithPath: "/")
        self.title = title
        self.width = CGFloat(width)
        self.height = CGFloat(height)
        self.zoom = zoom
        self.autosaveName = autosaveName
    }

    /// A miniaturized window still counts as present: isVisible is false while
    /// it sits in the Dock, but its tile is the way back to it, so the menu
    /// items that act on it have to stay live.
    var isPresented: Bool {
        guard let window else { return false }
        return window.isVisible || window.isMiniaturized
    }

    func show() {
        if let window {
            // Held open across a server restart the page is dead HTML. Only
            // reload in that case, so the usual reopen keeps its state.
            if loadFailed { load() }
            present(window)
            return
        }
        let webView = WKWebView(frame: .zero)
        webView.navigationDelegate = self
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: width, height: height),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false)
        window.title = title
        window.contentView = webView
        window.delegate = self
        // Closing hides rather than destroys; without this AppKit frees the
        // window on close and the next open rebuilds the WebView.
        window.isReleasedWhenClosed = false
        window.center()
        // After center(), so a remembered size and position wins.
        window.setFrameAutosaveName(autosaveName)

        self.window = window
        self.webView = webView
        load()
        present(window)
    }

    /// Always a fresh load rather than reload(), which does nothing when the
    /// first navigation failed and left no back-forward entry to reload.
    func load() {
        loadFailed = false
        webView?.load(URLRequest(url: url))
    }

    func hide() {
        guard let window, isPresented else { return }
        window.orderOut(nil)
        onVisibilityChange?(false)
    }

    var canZoom: Bool { zoom != nil }

    func zoomIn() { setZoom(currentZoom + (zoom?.step ?? 0)) }
    func zoomOut() { setZoom(currentZoom - (zoom?.step ?? 0)) }
    func actualSize() { setZoom(1) }

    private var currentZoom: CGFloat { webView?.pageZoom ?? 1 }

    private func setZoom(_ value: CGFloat) {
        guard let zoom else { return }
        webView?.pageZoom = Swift.min(Swift.max(value, zoom.min), zoom.max)
    }

    private func present(_ window: NSWindow) {
        // Announced before activating: the app has to be .regular already, or
        // the activation lands on an app with no Dock tile.
        onVisibilityChange?(true)
        NSApp.activate(ignoringOtherApps: true)
        // orderOut does not clear the miniaturized flag, so a window minimized
        // when it was hidden arrives here still miniaturized with no tile to
        // restore it from.
        if window.isMiniaturized { window.deminiaturize(nil) }
        window.makeKeyAndOrderFront(nil)
    }

    // MARK: - NSWindowDelegate

    func windowShouldClose(_ sender: NSWindow) -> Bool {
        hide()
        return false
    }

    // MARK: - WKNavigationDelegate

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        loadFailed = false
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        loadFailed = true
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        loadFailed = true
    }
}

/// Rebuilds the mirrored menu from the last poll every time it opens, so the
/// menu bar and the status item cannot disagree about what is possible.
final class MenuMirror: NSObject, NSMenuDelegate {
    private let nodes: () -> [MenuNode]
    private let repoll: () -> Void

    init(nodes: @escaping () -> [MenuNode], repoll: @escaping () -> Void) {
        self.nodes = nodes
        self.repoll = repoll
    }

    func menuNeedsUpdate(_ menu: NSMenu) {
        Draw.menu(nodes(), into: menu, repoll: repoll)
    }
}

/// The app's main menu. It only appears while the window is up — an .accessory
/// app owns no menu bar — but it is also what makes Cmd-C work inside the
/// WebView: the Edit items reach WKWebView through the responder chain.
enum MainMenu {
    /// windowsMenu is assigned only after the menu is in place; AppKit ignores
    /// the assignment otherwise and the window list silently never populates.
    static func install(appName: String, window: WebWindow, mirror: MenuMirror, target: AnyObject) {
        let main = NSMenu()
        main.addItem(appMenuItem(appName, target))
        main.addItem(fileMenuItem(target))
        main.addItem(editMenuItem())
        if window.canZoom {
            main.addItem(viewMenuItem(target))
        } else {
            main.addItem(viewMenuItemReloadOnly(target))
        }
        main.addItem(mirroredMenuItem(appName, mirror))
        let windowItem = windowMenuItem()
        main.addItem(windowItem)
        NSApp.mainMenu = main
        NSApp.windowsMenu = windowItem.submenu
    }

    private static func appMenuItem(_ name: String, _ target: AnyObject) -> NSMenuItem {
        let menu = NSMenu(title: name)
        menu.addItem(item("About \(name)", #selector(NSApplication.orderFrontStandardAboutPanel(_:)), target: NSApp))
        menu.addItem(.separator())
        menu.addItem(item("Hide \(name)", #selector(NSApplication.hide(_:)), "h", target: NSApp))
        menu.addItem(item("Hide Others", #selector(NSApplication.hideOtherApplications(_:)), "h", [.command, .option], target: NSApp))
        menu.addItem(item("Show All", #selector(NSApplication.unhideAllApplications(_:)), target: NSApp))
        menu.addItem(.separator())
        // Off the standard Cmd-Q: quitting takes the status item and its
        // polling with it, which is not what Cmd-Q usually costs. Not on
        // Shift-Cmd-Q either — the Apple menu reserves that for Log Out and
        // swallows it before the app's item sees the key.
        menu.addItem(item("Quit \(name)", #selector(WindowActions.requestQuit), "q", [.command, .option], target: target))
        return holder(menu)
    }

    private static func fileMenuItem(_ target: AnyObject) -> NSMenuItem {
        let menu = NSMenu(title: "File")
        menu.addItem(item("Close", #selector(WindowActions.closeWindow), "w", target: target))
        // Cmd-Q is muscle memory. Binding it visibly to "close the window" is
        // safer than leaving it on Quit and kinder than swallowing it.
        menu.addItem(item("Close Window", #selector(WindowActions.closeWindow), "q", target: target))
        return holder(menu)
    }

    /// Standard selectors with no target: AppKit walks the responder chain to
    /// WKWebView, which implements all of them. Wiring this menu is the whole
    /// fix for copy and paste in the window.
    private static func editMenuItem() -> NSMenuItem {
        let menu = NSMenu(title: "Edit")
        menu.addItem(item("Undo", Selector(("undo:")), "z"))
        menu.addItem(item("Redo", Selector(("redo:")), "z", [.command, .shift]))
        menu.addItem(.separator())
        menu.addItem(item("Cut", #selector(NSText.cut(_:)), "x"))
        menu.addItem(item("Copy", #selector(NSText.copy(_:)), "c"))
        menu.addItem(item("Paste", #selector(NSText.paste(_:)), "v"))
        menu.addItem(.separator())
        menu.addItem(item("Select All", #selector(NSText.selectAll(_:)), "a"))
        return holder(menu)
    }

    private static func viewMenuItem(_ target: AnyObject) -> NSMenuItem {
        let menu = NSMenu(title: "View")
        menu.addItem(item("Reload", #selector(WindowActions.reloadWindow), "r", target: target))
        menu.addItem(.separator())
        menu.addItem(item("Actual Size", #selector(WindowActions.actualSize), "0", target: target))
        menu.addItem(item("Zoom In", #selector(WindowActions.zoomIn), "+", target: target))
        menu.addItem(item("Zoom Out", #selector(WindowActions.zoomOut), "-", target: target))
        return holder(menu)
    }

    private static func viewMenuItemReloadOnly(_ target: AnyObject) -> NSMenuItem {
        let menu = NSMenu(title: "View")
        menu.addItem(item("Reload", #selector(WindowActions.reloadWindow), "r", target: target))
        return holder(menu)
    }

    /// The status item's own menu, in the menu bar. One declaration drives
    /// both, so the two cannot disagree about what is currently possible.
    private static func mirroredMenuItem(_ name: String, _ mirror: MenuMirror) -> NSMenuItem {
        let menu = NSMenu(title: name)
        menu.delegate = mirror
        let holder = NSMenuItem()
        holder.title = name
        holder.submenu = menu
        return holder
    }

    private static func windowMenuItem() -> NSMenuItem {
        let menu = NSMenu(title: "Window")
        menu.addItem(item("Minimize", #selector(NSWindow.performMiniaturize(_:)), "m"))
        menu.addItem(item("Zoom", #selector(NSWindow.performZoom(_:))))
        menu.addItem(.separator())
        menu.addItem(item("Bring All to Front", #selector(NSApplication.arrangeInFront(_:)), target: NSApp))
        return holder(menu)
    }

    /// Top-level menus hang off a titleless item whose submenu carries the
    /// title — the shape AppKit expects of a main menu.
    private static func holder(_ menu: NSMenu) -> NSMenuItem {
        let item = NSMenuItem()
        item.submenu = menu
        return item
    }

    private static func item(_ title: String,
                             _ action: Selector?,
                             _ key: String = "",
                             _ mask: NSEvent.ModifierFlags = .command,
                             target: AnyObject? = nil) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: action, keyEquivalent: key)
        if !key.isEmpty { item.keyEquivalentModifierMask = mask }
        item.target = target
        return item
    }
}

/// Everything the main menu can ask the app to do. @objc so NSMenuItem can
/// drive it through target/action, and a protocol so the menu cannot reach
/// past it into the generated Controller.
@objc protocol WindowActions: AnyObject {
    func closeWindow()
    func reloadWindow()
    func zoomIn()
    func zoomOut()
    func actualSize()
    func requestQuit()
}
```

- [ ] **Step 4: Emit it conditionally**

In `internal/backend/swiftappkit/emit.go`, add the embed beside the existing one:

```go
//go:embed runtime/Window.swift
var windowSwift []byte
```

and change `Emit` to include it only when a window is declared:

```go
func (*Backend) Emit(s *spec.Spec) ([]backend.File, error) {
	fixed := []backend.File{{Name: "Runtime.swift", Body: runtimeSwift}}
	if s.Window != nil {
		fixed = append(fixed, backend.File{Name: "Window.swift", Body: windowSwift})
	}
	return emitFiles(s, fixed, emitMain)
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/backend/swiftappkit/ -run TestWindowRuntime -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/backend/swiftappkit
git commit -m "add the WebKit window and main menu runtime"
```

---

### Task 6: Emit the window, and wire the app to it

**Files:**
- Create: `internal/backend/swiftappkit/window.go`
- Modify: `internal/backend/swiftappkit/main.go`, `internal/backend/swiftappkit/render.go`, `docs/schema.md`, `docs/recipes/window.md` (create), `docs/superpowers/specs/2026-09-05-perch-design.md`, `internal/backend/swiftappkit/seam_test.go`
- Test: `internal/backend/swiftappkit/window_test.go`

**Interfaces:**
- Consumes: `WebWindow`, `MenuMirror`, `MainMenu`, `WindowActions`, `ZoomConfig` from Task 5; `spec.Window`, `spec.ActionWindow` from Task 4.
- Produces: `emitWindow(b *buf, s *spec.Spec)` writing the `Controller` extension, and `MenuAction.window(WindowVerb)` in the runtime.

- [ ] **Step 1: Write the failing tests**

Add to `internal/backend/swiftappkit/window_test.go`:

```go
func TestWindowActionLowers(t *testing.T) {
	files := emit(t, windowDoc)
	if !strings.Contains(files["Render.swift"], ".window(.open)") {
		t.Errorf("Render.swift does not lower the window action:\n%s", files["Render.swift"])
	}
}

func TestWindowAppBuildsTheWindowAndMainMenu(t *testing.T) {
	files := emit(t, windowDoc)
	main := files["main.swift"]
	for _, want := range []string{
		`WebWindow(url: "http://localhost:4747/"`,
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
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/backend/swiftappkit/ -run 'TestWindowAction|TestWindowApp|TestNoWindowApp' -v`
Expected: FAIL — the action is unknown and `main.swift` has no window.

- [ ] **Step 3: Lower the action**

In `internal/backend/swiftappkit/render.go`, add to `menuGen.action`:

```go
	case spec.ActionWindow:
		return ".window(." + string(a.Window) + ")", nil
```

In `internal/backend/swiftappkit/runtime/Runtime.swift`, add the verb enum beside `LaunchAgentVerb`:

```swift
enum WindowVerb {
    case open
    case close
    case reload
}
```

add the case to `MenuAction`:

```swift
    case window(WindowVerb)
```

and to `Act.perform`, routing through a weak global the app sets at launch:

```swift
        case .window(let verb): Windows.perform(verb, then: repoll)
```

with the holder, in `Window.swift` (it is the file that exists only when there is a window — but `Act.perform` lives in `Runtime.swift`, so the holder must be in `Runtime.swift` and typed loosely):

In `Runtime.swift`, beside `Act`:

```swift
/// The app's one window, if it declared one. Set at launch by the generated
/// Controller. A closure rather than a typed reference so Runtime.swift does
/// not depend on Window.swift, which is emitted only when there is a window.
enum Windows {
    static var handler: ((WindowVerb) -> Void)?

    static func perform(_ verb: WindowVerb, then repoll: @escaping () -> Void) {
        handler?(verb)
        repoll()
    }
}
```

- [ ] **Step 4: Emit the app's window wiring**

Create `internal/backend/swiftappkit/window.go`:

```go
package swiftappkit

import (
	"fmt"

	"github.com/orochi235/perch/internal/celswift"
	"github.com/orochi235/perch/internal/spec"
)

// emitWindowMembers writes the stored properties Controller needs for a window.
// Nothing is written when the spec declares none, so an app without one names
// no WebKit type at all.
func emitWindowMembers(b *buf, s *spec.Spec) {
	if s.Window == nil {
		return
	}
	w := s.Window
	zoom := "nil"
	if w.Zoom != nil {
		zoom = fmt.Sprintf("ZoomConfig(min: %s, max: %s, step: %s)",
			swiftDouble(w.Zoom.Min), swiftDouble(w.Zoom.Max), swiftDouble(w.Zoom.Step))
	}
	b.line("private let window = WebWindow(")
	b.in()
	b.line("url: %s,", celswift.SwiftString(w.URL))
	b.line("title: %s,", celswift.SwiftString(w.Title))
	b.line("width: %d, height: %d,", w.Width, w.Height)
	b.line("zoom: %s,", zoom)
	b.line("autosaveName: %s)", celswift.SwiftString(s.App.Name+"Window"))
	b.out()
	b.line("private var mirror: MenuMirror?")
	b.line("")
}

// emitWindowSetup writes the launch-time wiring, called from Controller's init.
func emitWindowSetup(b *buf, s *spec.Spec) {
	if s.Window == nil {
		return
	}
	b.line("private func setUpWindow() {")
	b.in()
	b.line("window.onVisibilityChange = { [weak self] visible in self?.setDockPresence(visible) }")
	b.line("Windows.handler = { [weak self] verb in self?.act(verb) }")
	b.line("let mirror = MenuMirror(")
	b.in()
	b.line("nodes: { [weak self] in self.map { renderMenu($0.results) } ?? [] },")
	b.line("repoll: { [weak self] in self?.poll() })")
	b.out()
	b.line("self.mirror = mirror")
	b.line("MainMenu.install(appName: %s, window: window, mirror: mirror, target: self)",
		celswift.SwiftString(s.App.Name))
	b.out()
	b.line("}")
	b.line("")
}

// emitWindowActions writes the WindowActions conformance and the Dock policy.
func emitWindowActions(b *buf, s *spec.Spec) {
	if s.Window == nil {
		return
	}
	b.line("private func act(_ verb: WindowVerb) {")
	b.in()
	b.line("switch verb {")
	b.line("case .open: window.show()")
	b.line("case .close: window.hide()")
	b.line("case .reload: window.load()")
	b.line("}")
	b.out()
	b.line("}")
	b.line("")
	// Ordering matters on the way down: demoting to .accessory while the app is
	// frontmost with a visible window can strand the menu bar.
	b.line("private func setDockPresence(_ visible: Bool) {")
	b.in()
	b.line("if visible {")
	b.in()
	b.line("NSApp.setActivationPolicy(.regular)")
	b.out()
	b.line("} else {")
	b.in()
	b.line("NSApp.setActivationPolicy(.accessory)")
	b.line("NSApp.deactivate()")
	b.out()
	b.line("}")
	b.out()
	b.line("}")
	b.line("")
	b.line("func closeWindow() { window.hide() }")
	b.line("func reloadWindow() { window.load() }")
	b.line("func zoomIn() { window.zoomIn() }")
	b.line("func zoomOut() { window.zoomOut() }")
	b.line("func actualSize() { window.actualSize() }")
	b.line("func requestQuit() { NSApp.terminate(nil) }")
	b.line("")
	// Clicking the Dock icon should bring the window back, which is the whole
	// point of having one.
	b.line("func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows: Bool) -> Bool {")
	b.in()
	b.line("window.show()")
	b.line("return true")
	b.out()
	b.line("}")
	b.line("")
	b.line("func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {")
	b.in()
	b.line("switch menuItem.action {")
	b.line("case #selector(closeWindow), #selector(reloadWindow),")
	b.in()
	b.line("#selector(zoomIn), #selector(zoomOut), #selector(actualSize):")
	b.line("return window.isPresented")
	b.out()
	b.line("default:")
	b.in()
	b.line("return true")
	b.out()
	b.line("}")
	b.out()
	b.line("}")
	b.line("")
}
```

- [ ] **Step 5: Call them from main.go**

In `internal/backend/swiftappkit/main.go`, change the class declaration to conform when there is a window:

```go
	conformance := "NSObject, NSApplicationDelegate, NSMenuDelegate"
	if s.Window != nil {
		conformance += ", WindowActions, NSMenuItemValidation"
	}
	b.line("final class Controller: %s {", conformance)
```

Change `results` from `private var` to `fileprivate var` so the mirror closure can read it:

```go
	b.line("fileprivate var results = Results()")
```

Add the members after the timer declaration:

```go
	emitWindowMembers(b, s)
```

Call the setup at the end of `init()`, after `poll()`:

```go
	if s.Window != nil {
		b.line("setUpWindow()")
	}
```

And emit the action block after `emitDraw(b)`:

```go
	emitWindowActions(b, s)
	emitWindowSetup(b, s)
```

Change `poll()` from `func poll()` to `fileprivate func poll()` — the mirror's repoll closure calls it.

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/backend/swiftappkit/ -v`
Expected: PASS, including `seam_test.go` — `applicationShouldHandleReopen` is not one of the three reserved methods.

- [ ] **Step 7: Add the window doc to `specs()` so every golden covers it**

In `internal/backend/swiftappkit/seam_test.go`, add the window document to the map so the seam check runs against it too:

```go
		"window":       windowDoc,
```

Run: `go test ./internal/backend/swiftappkit/ -v`
Expected: PASS.

- [ ] **Step 8: Document the block**

In `docs/schema.md`, add a `## window` section between `## status` and `## menu`:

````
## window

One WebKit window, opened from a menu item. One per app, matching one status
item per app — which is why nothing names it.

```yaml
window:
  url: http://localhost:4747/
  title: reviewplex          # defaults to app.name
  size: [1280, 860]          # width, height; defaults to [1024, 768]
  zoom: {min: 0.5, max: 2.0, step: 0.1}   # omit for no zoom controls
```

Only `url` is required.

Declaring a window also emits a main menu — App, File, Edit, View, Window, and
the status item's own menu mirrored under the app's name. Not optional: an
`.accessory` app owns no menu bar until it is `.regular`, and without the Edit
menu ⌘C does not work inside the window at all. ⌘Q closes the window; quitting
moves to ⌥⌘Q, because for a menu-bar-first app ⌘Q otherwise costs the status
item and its polling.

The mirrored menu is rebuilt from the last poll every time it opens, the same
way the status menu is, so the two cannot disagree about what is possible.

What the window does is fixed, because each one is a trap:

| Behavior | Why |
|---|---|
| Close hides, never destroys | Reopening keeps scroll position and whatever was expanded |
| A reopen reloads only if the last load failed | Held open across a server restart the page is dead HTML; reloading every time throws away the state the row above buys |
| A load is always fresh, never `reload()` | `reload()` does nothing when the first navigation failed and left no history entry |
| Presenting deminiaturizes first | `orderOut` does not clear the miniaturized flag, so a window hidden while minimized has no tile to come back from |
| The Dock icon follows the window | Raised before activating, or the activation lands on an app with no tile |

The frame is remembered across launches. Zoom is not — it resets to 1.0.

A Dock tile wants artwork: see [The Dock tile](#the-dock-tile).
````

Add the action row in `## menu`:

```
| `window` | `open`, `close` or `reload`. Needs a `window:` block. |
```

- [ ] **Step 9: Write the recipe**

Create `docs/recipes/window.md` following the shape of `docs/recipes/launchagent.md` — a complete `menubar.yaml` fronting a local server, with the states it can be in:

````markdown
# A local dashboard in a window

A server on localhost, its health in the menu bar, and its page in a window.

```yaml
app:
  name: dash
  id: dev.example.dash
  icon: tray
  interval: 5s

watch:
  health:
    http: http://localhost:8080/api/health

state:
  - down: "!health.ok && health.status == 0"
  - unhealthy: "!health.ok"
  - up:

window:
  url: http://localhost:8080/
  size: [1280, 860]
  zoom: {min: 0.5, max: 2.0, step: 0.1}

status:
  - {when: down, icon: tray, dim: true}
  - {when: unhealthy, icon: exclamationmark.triangle}
  - {icon: tray.full}

menu:
  - {text: "Server: not running", when: down}
  - {text: "Server: not responding", when: unhealthy}
  - {text: "Server: running", when: up}
  - separator
  - {text: Open Dashboard, window: open}
  - separator
  - {text: Quit, quit: true}
```

`health.status` is `0` when nothing answered at all, which is what separates a
server that is down from one that is up and unwell.

Put a 1024×1024 `menubar/AppIcon.png` beside the spec, or the Dock tile the
window brings with it is the generic blank icon.
````

- [ ] **Step 10: Retire the web-view exclusion**

In `docs/superpowers/specs/2026-09-05-perch-design.md`, in `## Out of scope`, replace the first paragraph with:

```
Notification delivery is hand-written Swift, not schema. A banner fires on a
transition rather than on a condition holding, and the cursor that makes it fire
once is state no `when:` can name.
```

In the `## Codegen, not a runtime interpreter` section, replace "an embedded WebKit window with a `Cmd-W`/`Cmd-R` event monitor" with "a notification cursor seeded from the first poll so a restart does not replay banners" and drop the now-duplicated second example, so the sentence reads:

```
Both existing apps grew things a schema would choke on — a notification cursor
seeded from the first poll so a restart does not replay banners. A runtime would
have to either express all of that or invent an escape hatch. Codegen's escape
hatch is "stop running the generator."
```

In `internal/backend/swiftappkit/seam_test.go`, update the `delegateSeam` comment: replace "(notification delivery, an embedded web view)" with "(notification delivery)".

- [ ] **Step 11: Run everything and commit**

Run: `go test ./...`
Expected: PASS.

```bash
git add internal docs
git commit -m "open a WebKit window from a menu item"
```

---

### Task 7: `app.quit:` parses and validates

**Files:**
- Create: `internal/spec/quit.go`, `internal/spec/quit_test.go`
- Modify: `internal/spec/spec.go`, `internal/spec/validate.go`, `internal/schema/schema.go`

**Interfaces:**
- Consumes: `Action`, `actionFrom`, `itemFields`, `decodeStrict`.
- Produces:
  - `spec.QuitRule{When, Confirm, Detail string; Buttons []QuitButton}`
  - `spec.QuitButton{Text string; Action Action}`
  - `spec.App.Quit []QuitRule`

- [ ] **Step 1: Write the failing tests**

Create `internal/spec/quit_test.go`:

```go
package spec

import "testing"

const quitDoc = `
app:
  name: w
  id: dev.example.w
  icon: gear
  interval: 5s
  quit:
    - when: svc.loaded
      confirm: "Quit w?"
      detail: "Stops the menu bar item and its polling."
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

func TestQuitRulesParse(t *testing.T) {
	s, err := Parse([]byte(quitDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.App.Quit) != 2 {
		t.Fatalf("got %d rules, want 2", len(s.App.Quit))
	}
	first := s.App.Quit[0]
	if first.When != "svc.loaded" || first.Confirm != "Quit w?" {
		t.Errorf("first rule = %+v", first)
	}
	if len(first.Buttons) != 2 {
		t.Fatalf("got %d buttons, want 2", len(first.Buttons))
	}
	if first.Buttons[0].Text != "Quit Both" || first.Buttons[0].Action.Kind != ActionAgent {
		t.Errorf("first button = %+v", first.Buttons[0])
	}
	if first.Buttons[1].Action.Kind != ActionNone {
		t.Errorf("second button carries an action: %+v", first.Buttons[1].Action)
	}
	if s.App.Quit[1].When != "" {
		t.Error("the last rule has a condition, so nothing is the fallback")
	}
}

func TestQuitRefusals(t *testing.T) {
	head := "app:\n  name: w\n  id: dev.example.w\n  icon: gear\n  interval: 5s\n  quit:\n"
	cases := map[string]string{
		"bare rule not last":  "    - {confirm: A}\n    - {when: x, confirm: B}\n",
		"no confirm":          "    - {detail: nothing to ask}\n",
		"button with no text": "    - {confirm: A, buttons: [{agent: svc.stop}]}\n",
		"button quits":        "    - {confirm: A, buttons: [{text: X, quit: true}]}\n",
		"button opens window": "    - {confirm: A, buttons: [{text: X, window: open}]}\n",
		"unknown key":         "    - {confirm: A, detial: x}\n",
	}
	for name, rules := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(head + rules + "menu:\n  - {text: Quit, quit: true}\n"))
			if err == nil {
				t.Fatal("accepted; want a refusal")
			}
		})
	}
}

func TestQuitIsOptional(t *testing.T) {
	s, err := Parse([]byte("app: {name: w, id: dev.example.w, icon: gear, interval: 5s}\nmenu:\n  - {text: Quit, quit: true}\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.App.Quit) != 0 {
		t.Errorf("got %d rules from a doc with no quit:", len(s.App.Quit))
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/spec/ -run TestQuit -v`
Expected: FAIL — `quit` is an unknown key under `app`.

- [ ] **Step 3: Write the quit parser**

Create `internal/spec/quit.go`:

```go
package spec

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// QuitRule decides what quitting asks first. The first rule whose When holds
// wins; a rule with no When always matches, so it must be last.
//
// It is app policy rather than an item's, because the Dock tile's own Quit
// calls terminate directly and skips anything wired to a menu item.
type QuitRule struct {
	When    string
	Confirm string
	Detail  string
	// Buttons are offered in order; the first is the default and Cancel is
	// implicit and last. Empty offers one button named for the app.
	Buttons []QuitButton
}

// QuitButton is one answer. Its action runs before the app terminates, and a
// failure cancels the quit.
type QuitButton struct {
	Text   string
	Action Action
}

type rawQuitRule struct {
	When    string      `yaml:"when"`
	Confirm string      `yaml:"confirm"`
	Detail  string      `yaml:"detail"`
	Buttons []yaml.Node `yaml:"buttons"`
}

func parseQuit(n *yaml.Node) ([]QuitRule, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("app.quit: want a list of rules, first match wins")
	}
	out := make([]QuitRule, 0, len(n.Content))
	for i, c := range n.Content {
		path := fmt.Sprintf("app.quit[%d]", i)
		var raw rawQuitRule
		if err := decodeStrict(c, &raw, path); err != nil {
			return nil, err
		}
		rule := QuitRule{When: raw.When, Confirm: raw.Confirm, Detail: raw.Detail}
		for j := range raw.Buttons {
			b, err := parseQuitButton(&raw.Buttons[j], fmt.Sprintf("%s.buttons[%d]", path, j))
			if err != nil {
				return nil, err
			}
			rule.Buttons = append(rule.Buttons, b)
		}
		out = append(out, rule)
	}
	return out, nil
}

func parseQuitButton(n *yaml.Node, path string) (QuitButton, error) {
	var f itemFields
	if err := decodeStrict(n, &f, path); err != nil {
		return QuitButton{}, err
	}
	if f.When != "" || f.Each != "" || f.Menu.Kind != 0 {
		return QuitButton{}, fmt.Errorf("%s: a button takes a label and at most one action; when, each and menu have no meaning on one", path)
	}
	act, err := actionFrom(f, path)
	if err != nil {
		return QuitButton{}, err
	}
	switch act.Kind {
	case ActionQuit:
		return QuitButton{}, fmt.Errorf("%s: every button quits, so quit: on one says nothing", path)
	case ActionWindow:
		return QuitButton{}, fmt.Errorf("%s: the app is already terminating, so there is no window to %s", path, act.Window)
	}
	return QuitButton{Text: f.Text, Action: act}, nil
}

// validateQuit checks the ordering and the parts an expression cannot supply.
func (s *Spec) validateQuit() error {
	for i, r := range s.App.Quit {
		path := fmt.Sprintf("app.quit[%d]", i)
		if r.Confirm == "" {
			return fmt.Errorf("%s.confirm: required; a rule with nothing to ask is a rule that quits, which is what leaving the rule out already does", path)
		}
		if r.When == "" && i != len(s.App.Quit)-1 {
			return fmt.Errorf("%s: a rule with no when: always matches, so it must be last; %d rule(s) after it can never apply", path, len(s.App.Quit)-1-i)
		}
		for j, b := range r.Buttons {
			bp := fmt.Sprintf("%s.buttons[%d]", path, j)
			if b.Text == "" {
				return fmt.Errorf("%s.text: required; a button with no label cannot be told from Cancel", bp)
			}
			if err := b.Action.validate(bp, s.Watches, s.Window); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Hang it off App and validate it**

In `internal/spec/spec.go`, add `Quit []QuitRule` to `App`, add `Quit yaml.Node` with tag `yaml:"quit"` to `rawApp`, and parse it in `Parse` after the app block is built:

```go
	quit, err := parseQuit(&raw.App.Quit)
	if err != nil {
		return nil, err
	}
	s.App.Quit = quit
```

In `internal/spec/validate.go`, call it from `validate()` after the window check:

```go
	if err := s.validateQuit(); err != nil {
		return err
	}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/spec/ -v`
Expected: PASS.

- [ ] **Step 6: Update the JSON Schema**

In `internal/schema/schema.go`, add to `app`'s properties:

```
        "quit": {
          "type": "array",
          "description": "What quitting asks first. First matching rule wins; the last takes no when:.",
          "items": {
            "type": "object",
            "required": ["confirm"],
            "additionalProperties": false,
            "properties": {
              "when": {"type": "string", "description": "CEL condition; omit on the last rule."},
              "confirm": {"type": "string", "description": "The question."},
              "detail": {"type": "string", "description": "The smaller text under it."},
              "buttons": {
                "type": "array",
                "description": "Offered in order; the first is the default, Cancel is implicit and last.",
                "items": {
                  "type": "object",
                  "required": ["text"],
                  "additionalProperties": false,
                  "properties": {
                    "text": {"type": "string"},
                    "run": {"type": "array", "minItems": 1, "items": {"type": "string"}},
                    "open": {"type": "string"},
                    "post": {"$ref": "#/definitions/post"},
                    "agent": {"type": "string"},
                    "swift": {"type": "string", "pattern": "^[A-Za-z_][A-Za-z0-9_]*\\.[A-Za-z_][A-Za-z0-9_]*$"}
                  }
                }
              }
            }
          }
        },
```

If `post` is currently spelled inline in the menu definition rather than as `#/definitions/post`, factor it out into `definitions` first so both sites share it.

Run: `go test ./internal/schema/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/spec internal/schema
git commit -m "parse and validate app.quit rules"
```

---

### Task 8: Quitting asks

**Files:**
- Modify: `internal/backend/swiftappkit/render.go`, `internal/backend/swiftappkit/main.go`, `internal/backend/swiftappkit/runtime/Runtime.swift`, `docs/schema.md`
- Create: `internal/backend/swiftappkit/quit_test.go`

**Interfaces:**
- Consumes: `spec.QuitRule`, `spec.QuitButton` from Task 7; `Act.perform`, `MenuAction` from the runtime.
- Produces:
  - Swift `struct QuitPrompt { let message: String; let detail: String; let buttons: [QuitChoice] }`, `struct QuitChoice { let text: String; let action: MenuAction? }`
  - `func renderQuit(_ results: Results) -> QuitPrompt?` in `Render.swift`
  - `enum Quit { static func should(_ prompt: QuitPrompt?, event: NSAppleEventDescriptor?) -> NSApplication.TerminateReply }`

- [ ] **Step 1: Write the failing tests**

Create `internal/backend/swiftappkit/quit_test.go`:

```go
package swiftappkit

import (
	"strings"
	"testing"
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

// Without any quit: rules nothing should be emitted — an app that never asks
// should not carry the machinery for asking.
func TestNoQuitRulesEmitNoPrompt(t *testing.T) {
	main := emit(t, noWindowDoc)["main.swift"]
	if strings.Contains(main, "applicationShouldTerminate") {
		t.Error("an app with no quit: rules intercepts termination anyway")
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/backend/swiftappkit/ -run TestQuit -v`
Expected: FAIL — no `renderQuit`.

- [ ] **Step 3: Add the runtime**

In `internal/backend/swiftappkit/runtime/Runtime.swift`, after `Act`:

```swift
// MARK: - Quitting

/// One answer in the quit prompt. A nil action just quits.
struct QuitChoice {
    let text: String
    let action: MenuAction?
}

/// What quitting asks. Cancel is implicit and always last; the first button is
/// the default.
struct QuitPrompt {
    let message: String
    let detail: String
    let buttons: [QuitChoice]
}

enum Quit {
    /// Quit reasons that mean the machine is going down or the user is walking
    /// away — nobody is asking this app a question, and nobody is waiting to
    /// answer one.
    private static let unattended: Set<OSType> = [
        OSType(kAELogOut), OSType(kAEReallyLogOut),
        OSType(kAEShutDown), OSType(kAERestart), OSType(kAEQuitAll),
    ]

    /// Whether this termination is aimed at this app specifically. An absent
    /// event or an absent reason means it is: NSApp.terminate from our own
    /// menus and from the Dock tile carries no quit reason at all.
    static func isUserInitiated(_ event: NSAppleEventDescriptor?) -> Bool {
        guard let reason = event?.attributeDescriptor(forKeyword: AEKeyword(kAEQuitReason)) else {
            return true
        }
        return !unattended.contains(reason.enumCodeValue)
    }

    /// Asks, and reports what to do. A helper that cancels someone's logout is
    /// worse than one that exits without asking, and a modal raised during
    /// shutdown is a dialog nobody is there to dismiss — so an unattended quit
    /// never reaches the alert.
    static func should(_ prompt: QuitPrompt?,
                       event: NSAppleEventDescriptor?,
                       then finish: @escaping (Bool) -> Void) -> NSApplication.TerminateReply {
        guard let prompt, isUserInitiated(event) else { return .terminateNow }

        let alert = NSAlert()
        alert.messageText = prompt.message
        alert.informativeText = prompt.detail
        let choices = prompt.buttons.isEmpty ? [QuitChoice(text: "Quit", action: nil)] : prompt.buttons
        for choice in choices {
            alert.addButton(withTitle: choice.text)
        }
        alert.addButton(withTitle: "Cancel").keyEquivalent = "\u{1b}"

        // The status item does not activate the app, so an unactivated modal
        // can sit behind another app with nothing to click while runModal blocks.
        NSApp.activate(ignoringOtherApps: true)

        let picked = alert.runModal().rawValue - NSApplication.ModalResponse.alertFirstButtonReturn.rawValue
        guard picked >= 0, picked < choices.count else { return .terminateCancel }

        guard let action = choices[picked].action else { return .terminateNow }
        // Captured strongly on purpose: every path out of this has to reply
        // exactly once, and a dropped reply hangs termination forever.
        Act.perform(action) { finish(true) }
        return .terminateLater
    }
}
```

`Act.perform`'s completion fires after the action's own alert, if any, so a failure is seen before the app goes. Change `Act.perform`'s `agent`, `run` and `post` cases to report success through the repoll closure — they already run it on both paths, so `finish(true)` is reached either way; a button whose action failed still shows the alert first.

- [ ] **Step 4: Emit `renderQuit`**

In `internal/backend/swiftappkit/render.go`, add after the `renderMenu` emitter:

```go
// emitQuit writes renderQuit: the first rule whose condition holds, lowered in
// order so the fallback cannot outrank a guarded rule.
func emitQuit(b *buf, s *spec.Spec, e *celswift.Env, g *menuGen) error {
	if len(s.App.Quit) == 0 {
		return nil
	}
	b.line("func renderQuit(_ results: Results) -> QuitPrompt? {")
	b.in()
	for i, r := range s.App.Quit {
		path := fmt.Sprintf("app.quit[%d]", i)
		open, close := "", ""
		if r.When != "" {
			cond, err := e.Lower(r.When)
			if err != nil {
				return fmt.Errorf("%s.when: %w", path, err)
			}
			open, close = "if "+cond+" {", "}"
			b.line("%s", open)
			b.in()
		}
		b.line("return QuitPrompt(")
		b.in()
		b.line("message: %s,", celswift.SwiftString(r.Confirm))
		b.line("detail: %s,", celswift.SwiftString(r.Detail))
		b.line("buttons: [")
		b.in()
		for j, btn := range r.Buttons {
			action := "nil"
			if btn.Action.Kind != spec.ActionNone {
				lowered, err := g.action(btn.Action, e, fmt.Sprintf("%s.buttons[%d]", path, j))
				if err != nil {
					return err
				}
				action = lowered
			}
			b.line("QuitChoice(text: %s, action: %s),", celswift.SwiftString(btn.Text), action)
		}
		b.out()
		b.line("])")
		b.out()
		if close != "" {
			b.out()
			b.line("%s", close)
		}
	}
	if s.App.Quit[len(s.App.Quit)-1].When != "" {
		b.line("return nil")
	}
	b.out()
	b.line("}")
	b.line("")
	return nil
}
```

Call it from `emitRender` after the menu is written, passing the same `Env` and `menuGen` the menu used.

- [ ] **Step 5: Intercept termination**

In `internal/backend/swiftappkit/main.go`, after `emitWindowActions`:

```go
	emitQuitDelegate(b, s)
```

and add it to `window.go` (it is the same family of delegate emission):

```go
// emitQuitDelegate writes applicationShouldTerminate. Nothing is written when
// the spec declares no rules, so an app that never asks does not carry the
// machinery for asking.
func emitQuitDelegate(b *buf, s *spec.Spec) {
	if len(s.App.Quit) == 0 {
		return
	}
	b.line("func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {")
	b.in()
	b.line("Quit.should(")
	b.in()
	b.line("renderQuit(results),")
	b.line("event: NSAppleEventManager.shared().currentAppleEvent,")
	b.line("then: { ok in NSApp.reply(toApplicationShouldTerminate: ok) })")
	b.out()
	b.out()
	b.line("}")
	b.line("")
}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/backend/swiftappkit/ -v`
Expected: PASS, including `seam_test.go` — `applicationShouldTerminate` is not one of the three reserved methods, and `applicationWillTerminate` still is.

- [ ] **Step 7: Add the quit-reason unit test**

`applicationShouldTerminate` under a real logout cannot be tested without logging out, so test the discriminator directly. Add to the Swift compiled in `internal/backend/swiftappkit/runtime_test.go` (following the pattern the existing runtime tests use to compile and run a snippet):

```go
func TestQuitReasonSeparatesLogoutFromAQuit(t *testing.T) {
	runRuntimeSnippet(t, `
let logout = NSAppleEventDescriptor.record()
logout.setDescriptor(NSAppleEventDescriptor(enumCode: OSType(kAELogOut)),
                     forKeyword: AEKeyword(kAEQuitReason))
assert(!Quit.isUserInitiated(logout), "a logout read as user-initiated")
assert(Quit.isUserInitiated(nil), "a quit with no event read as unattended")
let bare = NSAppleEventDescriptor.record()
assert(Quit.isUserInitiated(bare), "a quit with no reason read as unattended")
`)
}
```

If `runRuntimeSnippet` does not exist, add it beside the other runtime tests: write `Runtime.swift` plus the snippet wrapped in a `main.swift` into a temp dir, compile with `swiftc`, run, and fail on a non-zero exit.

Run: `go test ./internal/backend/swiftappkit/ -run TestQuitReason -v`
Expected: PASS.

- [ ] **Step 8: Document it**

In `docs/schema.md`, add a `### quit` subsection at the end of `## app`:

````
### quit

What quitting asks first. An ordered list, first match wins, the last rule bare
— the idiom `status:` uses.

```yaml
app:
  name: reviewplex
  id: com.reviewplex.menubar
  icon: tray
  interval: 5s
  quit:
    - when: svc.loaded
      confirm: "Quit reviewplex?"
      detail: "Stops the menu bar item and its polling. The server is running as a launchd service."
      buttons:
        - {text: Quit Both, agent: svc.stop}
        - {text: Quit Helper Only}
    - confirm: "Quit reviewplex?"
      detail: "The menu bar item goes away. The server itself keeps running."
```

| Key | Takes |
|---|---|
| `when` | A string: the condition. Omit it to match always, which means it must be last. |
| `confirm` | A string: the question. Required. |
| `detail` | A string: the smaller text under it. |
| `buttons` | A list of `{text: …}` mappings, each with at most one action — the same ones a menu item takes, less `quit:` and `window:`. |

Buttons are offered in order, the first is the default, and Cancel is implicit
and always last. A button's action runs before the app terminates; if it fails,
the alert names the command and the quit is canceled. With no `buttons:` the
prompt offers one button named Quit.

This is app policy rather than something on the Quit item, because the Dock
tile's own Quit calls `terminate` directly and would skip anything wired to a
menu. Every route in goes through it: the menu item, ⌥⌘Q, the Dock tile.

Logging out, restarting and shutting down skip the prompt and quit. A helper
that cancels someone's logout is worse than one that exits without asking.

Leave the block out and the app quits without asking.
````

- [ ] **Step 9: Run everything and commit**

Run: `go test ./...`
Expected: PASS.

```bash
git add internal docs
git commit -m "ask before quitting, and never during a logout"
```

---

### Task 9: Aqua-session integration tests

The four behaviors nothing else can tell the difference about. These run against real launchd and a real window, and skip outside an Aqua session the way `launchd_test.go` already does.

**Files:**
- Create: `internal/backend/swiftappkit/aqua_test.go`

**Interfaces:**
- Consumes: everything above. No new production code.

- [ ] **Step 1: Write the quit-stays-quit test**

Create `internal/backend/swiftappkit/aqua_test.go` with the skip guard copied from `launchd_test.go`, then:

```go
// TestInstalledWidgetStaysQuit is the regression for KeepAlive. A widget quit
// from its own menu exits 0; an unconditional KeepAlive brings it straight back.
func TestInstalledWidgetStaysQuit(t *testing.T) {
	requireAqua(t)
	dir := buildAndInstall(t, noWindowDoc)
	defer uninstall(t, dir)

	pid := waitForProcess(t, "dev.example.w")
	if pid == 0 {
		t.Fatal("the widget never started")
	}
	quitApp(t, pid)
	if again := waitForProcess(t, "dev.example.w"); again != 0 {
		t.Fatalf("launchd restarted the widget after a clean quit (pid %d)", again)
	}
}
```

- [ ] **Step 2: Write the Dock-presence test**

```go
// The Dock tile is what makes a window app different from a status-bar one,
// and getting the ordering wrong strands the menu bar rather than erroring.
func TestDockTileFollowsTheWindow(t *testing.T) {
	requireAqua(t)
	dir := buildAndRun(t, windowDoc)
	defer stop(t, dir)

	if policy := activationPolicy(t); policy != "accessory" {
		t.Fatalf("policy at launch = %q, want accessory", policy)
	}
	openWindow(t)
	if policy := activationPolicy(t); policy != "regular" {
		t.Errorf("policy with the window open = %q, want regular", policy)
	}
	closeWindow(t)
	if policy := activationPolicy(t); policy != "accessory" {
		t.Errorf("policy with the window closed = %q, want accessory", policy)
	}
}
```

- [ ] **Step 3: Write the reload-after-failure test**

```go
// The one piece of window state the design argued could not be declared. A
// reopen after the server went away must reload rather than re-show WebKit's
// error page.
func TestReopenAfterAFailedLoadReloads(t *testing.T) {
	requireAqua(t)
	server, url := startTestServer(t)
	dir := buildAndRun(t, windowDocFor(url))
	defer stop(t, dir)

	openWindow(t)
	waitForTitle(t, "ok")
	closeWindow(t)
	server.Close()
	openWindow(t)  // loads and fails
	closeWindow(t)
	server = restartTestServer(t, url)
	defer server.Close()
	openWindow(t)
	if title := currentTitle(t); title != "ok" {
		t.Errorf("title after the server came back = %q, want the live page", title)
	}
}
```

- [ ] **Step 4: Write the helpers**

Write `requireAqua`, `buildAndInstall`, `buildAndRun`, `uninstall`, `stop`, `waitForProcess`, `quitApp`, `activationPolicy`, `openWindow`, `closeWindow`, `waitForTitle`, `currentTitle`, `startTestServer`, `restartTestServer`, `windowDocFor` in the same file. Drive the running app through its own menu with `osascript` and System Events, and read the activation policy by asking the app through a debug-only `run:` watch, or by checking whether the process appears in `System Events`' list of application processes with `background only` false.

Keep every helper failing loudly on a timeout rather than returning a zero value that reads as a pass.

- [ ] **Step 5: Run them**

Run: `go test ./internal/backend/swiftappkit/ -run 'TestInstalledWidget|TestDockTile|TestReopen' -v`
Expected: PASS in an Aqua session; SKIP over SSH.

- [ ] **Step 6: Commit**

```bash
git add internal/backend/swiftappkit/aqua_test.go
git commit -m "test quit, the Dock tile and reload against a real session"
```

---

### Task 10: Port reviewplex

The acceptance test for everything above: 904 lines of hand-written Swift and two shell scripts become one `menubar.yaml`.

**Files (in `~/src/pw/reviewplex`):**
- Create: `menubar.yaml`, `menubar/AppIcon.png`, `menubar/Generated/*` (emitted)
- Delete: `menubar/Sources/` (all seven files), `menubar/Resources/Info.plist`, `scripts/install-menubar.sh`, `scripts/uninstall-menubar.sh`
- Move: `menubar/Resources/icon.png` → `menubar/AppIcon.png`
- Modify: `package.json`, `README.md`

- [ ] **Step 1: Write the spec**

Create `menubar.yaml` at the repo root:

```yaml
# yaml-language-server: $schema=./menubar.schema.json
app:
  name: reviewplex
  id: com.reviewplex.menubar
  icon: tray
  interval: 5s
  quit:
    - when: svc.loaded
      confirm: "Quit reviewplex?"
      detail: "Stops the menu bar item and its polling. The server is running as a launchd service."
      buttons:
        - {text: Quit Both, agent: svc.stop}
        - {text: Quit Helper Only}
    - confirm: "Quit reviewplex?"
      detail: "The menu bar item goes away and stops watching the server. The server itself keeps running."

watch:
  health:
    http: http://localhost:4747/api/health
  summary:
    http: http://localhost:4747/api/summary
    json: true
    shape: {needsReview: int}
  svc:
    launchagent: com.reviewplex

state:
  - stopped: "!health.ok && health.status == 0"
  - unhealthy: "!health.ok"
  - running:

window:
  url: http://localhost:4747/
  title: reviewplex
  size: [1280, 860]
  zoom: {min: 0.5, max: 2.0, step: 0.1}

status:
  - {when: stopped, icon: tray, dim: true}
  - {when: unhealthy, icon: exclamationmark.triangle}
  - icon: tray.full
    badge: "summary.data.needsReview > 0 ? string(summary.data.needsReview) : ''"

menu:
  - {text: "Server: not running", when: stopped}
  - {text: "Server: not responding on :4747", when: unhealthy}
  - {text: "Server: running on :4747 — {{summary.data.needsReview}} in inbox", when: "running && summary.data.needsReview > 0"}
  - {text: "Server: running on :4747", when: "running && summary.data.needsReview == 0"}
  - {text: "Service: not installed", when: "!svc.installed"}
  - {text: "Service: installed, not loaded", when: "svc.installed && !svc.loaded"}
  - {text: "Service: loaded", when: "svc.loaded"}
  - separator
  - {text: Open Dashboard, window: open}
  - {text: "Install with: npm run service:install", when: "!svc.installed"}
  - {text: Start Service, when: "svc.installed && !svc.loaded", agent: svc.start}
  - {text: Restart Service, when: "svc.loaded && running", agent: svc.restart}
  - {text: Start Service, when: "svc.loaded && !running", agent: svc.start}
  - {text: Stop Service, when: "svc.loaded", agent: svc.stop}
  - {text: Open Logs, open: "~/Library/Logs/reviewplex"}
  - separator
  - {text: Quit, quit: true}
```

- [ ] **Step 2: Check the states against reality**

The `state:` block must reproduce `ServerMonitor`'s three answers. Confirm `health.status` is `0` when the connection is refused — the `http` watch binds `.status` as an int and nothing answered means no status code. If a refused connection reports something other than `0`, fix the `stopped` condition here rather than in perch.

Run: `perch build -C .` then read `menubar/Generated/Render.swift` and check the three conditions lowered the way the Swift did.

- [ ] **Step 3: Move the artwork and build**

```bash
cd ~/src/pw/reviewplex
git mv menubar/Resources/icon.png menubar/AppIcon.png
perch schema -o menubar.schema.json
perch run -C .
```

Expected: the status item appears, the badge matches the dashboard's inbox count, and Open Dashboard opens the window with a Dock tile.

- [ ] **Step 4: Check every menu state by hand**

With the server up, the server down, the service unloaded, and the service not installed, open the menu and confirm the two status lines and the offered actions match what the hand-written menu showed. Confirm ⌘C works inside the window, ⌘W and ⌘Q close it, ⌥⌘Q asks, and Quit Both stops the service.

- [ ] **Step 5: Install and retire the scripts**

```bash
perch install -C .
git rm -r menubar/Sources menubar/Resources scripts/install-menubar.sh scripts/uninstall-menubar.sh
```

In `package.json`, replace the two menubar scripts:

```json
    "menubar:install": "perch install",
    "menubar:uninstall": "perch uninstall",
```

- [ ] **Step 6: Rewrite the README section**

Replace the "Menu bar helper (optional)" section's build instructions with perch's, keeping the behavior description — which is still accurate — and saying the widget is generated from `menubar.yaml`. Note that `perch` must be installed (`go install github.com/orochi235/perch/cmd/perch@latest`) rather than `swiftc` alone.

- [ ] **Step 7: Confirm it survives a login**

```bash
launchctl kickstart -k gui/$(id -u)/com.reviewplex.menubar
```

Confirm the item comes back, then quit it from its own menu and confirm it stays gone — the Task 1 regression, in the repo that will actually notice.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "generate the menu bar helper from menubar.yaml"
```

---

## Self-Review

**Spec coverage.** Every section of the design has a task: the window block (4, 5, 6), the fixed-policy table (5), the generated main menu and the mirrored menu (5, 6), `swift:` (2), `app.quit:` with ordered rules and buttons (7, 8), the unattended-quit bypass (8), both installer bugs (1, 3), the "Out of scope" rewrite (6, step 10), the four Aqua tests (9), and the port (10).

**One spec gap filled:** the design says an app icon pipeline is needed but not where the source PNG comes from. Task 3 fixes it at `menubar/AppIcon.png` — a fixed path, no schema key — and updates the spec in the same task.

**Ordering.** Task 1 is independent and ships alone. Task 3 must land before Task 6, or a `window:` app installs with a blank Dock tile. Task 7 must land before Task 8. Task 10 needs all of them.

**Types.** `WindowVerb` is spelled the same in Go (`spec.WindowVerb`, values `WindowOpen`/`WindowClose`/`WindowReload`) and in Swift (`enum WindowVerb { case open, close, reload }`), and `render.go` lowers `string(a.Window)` straight into `.window(.open)` — so the Go constant values must stay the lowercase verbs. `App.IconFile` (Task 3) is the basename without `.icns`; `BundleOpts.AppIcon` is the full source path. `Controller.results` and `Controller.poll()` become `fileprivate` in Task 6 because the `MenuMirror` closure reads both.

**Known risk in Task 8, step 3.** `Act.perform`'s completion closure currently means "re-poll", not "the action succeeded". Reusing it as `finish(true)` means a button whose action failed still terminates after showing its alert. That matches the hand-written behavior for every case except "Quit Both" with a service that would not stop, which cancels the quit. If the Aqua test in Task 9 shows this mattering, thread a success bool through `Act.perform` rather than working around it at the call site.
