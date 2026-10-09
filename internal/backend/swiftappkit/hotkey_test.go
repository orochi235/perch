package swiftappkit

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/orochi235/perch/v2/internal/backend"
	"github.com/orochi235/perch/v2/internal/spec"
)

func TestHotkeyRuntimeEmittedOnlyWhenDeclared(t *testing.T) {
	with := emit(t, fieldDoc)
	if _, ok := with["Hotkey.swift"]; !ok {
		t.Error("an app with app.hotkey got no Hotkey.swift")
	}
	if !strings.Contains(with["main.swift"], "Hotkey(key: kVK_Space, modifiers: cmdKey | shiftKey)") {
		t.Errorf("main.swift does not register the hotkey:\n%s", with["main.swift"])
	}
	if _, ok := emit(t, noWindowDoc)["Hotkey.swift"]; ok {
		t.Error("an app with no app.hotkey links Carbon anyway")
	}
}

// TestEveryHotkeyKeyIsACarbonKeyCode ties the keys the spec accepts to
// constants swiftc knows, so a key added to one list and not the other fails.
func TestEveryHotkeyKeyIsACarbonKeyCode(t *testing.T) {
	var codes []string
	for _, k := range spec.HotkeyKeys() {
		c, ok := carbonKeys[k]
		if !ok {
			t.Errorf("%q has no Carbon key code", k)
		}
		codes = append(codes, c)
	}
	if len(carbonKeys) != len(codes) {
		t.Errorf("carbonKeys names %d keys, the spec accepts %d", len(carbonKeys), len(codes))
	}
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	src := "import Carbon.HIToolbox\nlet codes: [Int] = [" + strings.Join(codes, ", ") + "]\nprint(Set(codes).count)\n"
	typecheck(t, swiftc, []backend.File{{Name: "main.swift", Body: []byte(src)}})
}

// hotkeyProbe registers a hotkey the way main.swift does and presses it from
// code: typing it would need Accessibility. Registering the same combination
// twice is what another app holding it looks like.
const hotkeyProbe = `
import AppKit
import Carbon.HIToolbox

let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let status = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
status.button?.title = "probe"
let menu = NSMenu()
Draw.menu([.field("Search", { q in .swift({ print("submitted=\(q)"); exit(0) }) })], into: menu, repoll: {})
status.menu = menu

let mods = cmdKey | optionKey | controlKey | shiftKey
let hotkey = Hotkey(key: kVK_F19, modifiers: mods) { status.button?.performClick(nil) }
print("registered=\(hotkey != nil)")
print("again=\(Hotkey(key: kVK_F19, modifiers: mods) {} != nil)")

let check = Timer(timeInterval: 0.5, repeats: false) { _ in
    let field = (menu.items[0] as! FieldItem).field
    let editor = field.currentEditor()
    print("focused=\(editor != nil && field.window?.firstResponder === editor)")
    editor?.insertText("q")
    field.sendAction(field.action, to: field.target)
}
RunLoop.main.add(check, forMode: .common)
let timeout = Timer(timeInterval: 10, repeats: false) { _ in print("timeout"); exit(1) }
RunLoop.main.add(timeout, forMode: .common)
DispatchQueue.main.async { hotkey?.press() }
app.run()
`

func TestHotkeyOpensTheMenuIntoTheField(t *testing.T) {
	requireScreen(t)
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	bin := compileProbe(t, swiftc, append(runtimeFiles(),
		backend.File{Name: "Hotkey.swift", Body: hotkeySwift},
		backend.File{Name: "main.swift", Body: []byte(hotkeyProbe)}))
	out, _ := exec.Command(bin).CombinedOutput()
	if want := "registered=true\nagain=false\nfocused=true\nsubmitted=q\n"; string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}
