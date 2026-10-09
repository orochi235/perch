package swiftappkit

import (
	"strings"

	"github.com/orochi235/perch/v2/internal/celswift"
	"github.com/orochi235/perch/v2/internal/spec"
)

// carbonKeys names each of spec.HotkeyKeys as Carbon's virtual key code.
var carbonKeys = func() map[string]string {
	keys := map[string]string{
		"space": "kVK_Space", "return": "kVK_Return", "tab": "kVK_Tab",
		"escape": "kVK_Escape", "delete": "kVK_Delete", "forwarddelete": "kVK_ForwardDelete",
		"left": "kVK_LeftArrow", "right": "kVK_RightArrow", "up": "kVK_UpArrow", "down": "kVK_DownArrow",
		"home": "kVK_Home", "end": "kVK_End", "pageup": "kVK_PageUp", "pagedown": "kVK_PageDown",
		"-": "kVK_ANSI_Minus", "=": "kVK_ANSI_Equal", "[": "kVK_ANSI_LeftBracket",
		"]": "kVK_ANSI_RightBracket", ";": "kVK_ANSI_Semicolon", "'": "kVK_ANSI_Quote",
		",": "kVK_ANSI_Comma", ".": "kVK_ANSI_Period", "/": "kVK_ANSI_Slash",
		"\\": "kVK_ANSI_Backslash", "`": "kVK_ANSI_Grave",
	}
	for _, k := range spec.HotkeyKeys() {
		if _, ok := keys[k]; ok {
			continue
		}
		if len(k) == 1 {
			keys[k] = "kVK_ANSI_" + strings.ToUpper(k)
		} else {
			keys[k] = "kVK_F" + k[1:]
		}
	}
	return keys
}()

func carbonModifiers(h spec.Hotkey) string {
	var mods []string
	for _, m := range []struct {
		on   bool
		name string
	}{{h.Cmd, "cmdKey"}, {h.Opt, "optionKey"}, {h.Ctrl, "controlKey"}, {h.Shift, "shiftKey"}} {
		if m.on {
			mods = append(mods, m.name)
		}
	}
	if len(mods) == 0 {
		return "0"
	}
	return strings.Join(mods, " | ")
}

// emitHotkey registers app.hotkey to open the menu, which hands a field: the
// keyboard as a click would.
func emitHotkey(b *buf, s *spec.Spec) {
	h := s.App.Hotkey
	if !h.IsSet() {
		return
	}
	b.line("hotkey = Hotkey(key: %s, modifiers: %s) { [weak self] in", carbonKeys[h.Key], carbonModifiers(h))
	b.in()
	b.line("self?.statusItem.button?.performClick(nil)")
	b.out()
	b.line("}")
	b.line("if hotkey == nil {")
	b.in()
	b.line("NSLog(\"%%@\", %s)", celswift.SwiftString("perch: the system refused the hotkey "+h.String()+"; another app may hold it"))
	b.out()
	b.line("}")
}
