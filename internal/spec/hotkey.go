package spec

import (
	"fmt"
	"sort"
	"strings"
)

// Hotkey is a system-wide shortcut that opens the menu. The zero value is none.
type Hotkey struct {
	Cmd, Opt, Ctrl, Shift bool
	// Key is one of HotkeyKeys: a key by its position on a US keyboard.
	Key string
}

// IsSet reports whether a hotkey was given.
func (h Hotkey) IsSet() bool { return h.Key != "" }

// String writes it back in the form it was parsed from, modifiers in a fixed order.
func (h Hotkey) String() string {
	var parts []string
	for _, m := range []struct {
		on   bool
		name string
	}{{h.Ctrl, "ctrl"}, {h.Opt, "opt"}, {h.Shift, "shift"}, {h.Cmd, "cmd"}} {
		if m.on {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, h.Key), "+")
}

var hotkeyKeys = func() map[string]bool {
	keys := map[string]bool{}
	for c := 'a'; c <= 'z'; c++ {
		keys[string(c)] = true
	}
	for c := '0'; c <= '9'; c++ {
		keys[string(c)] = true
	}
	for i := 1; i <= 20; i++ {
		keys[fmt.Sprintf("f%d", i)] = true
	}
	for _, k := range []string{
		"space", "return", "tab", "escape", "delete", "forwarddelete",
		"left", "right", "up", "down", "home", "end", "pageup", "pagedown",
		"-", "=", "[", "]", ";", "'", ",", ".", "/", "\\", "`",
	} {
		keys[k] = true
	}
	return keys
}()

// HotkeyKeys is every key a hotkey may name, sorted.
func HotkeyKeys() []string {
	out := make([]string, 0, len(hotkeyKeys))
	for k := range hotkeyKeys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// modifierHints answers the spellings people reach for that perch does not take.
var modifierHints = map[string]string{
	"command": "cmd", "⌘": "cmd",
	"option": "opt", "alt": "opt", "⌥": "opt",
	"control": "ctrl", "⌃": "ctrl",
	"⇧": "shift",
}

// keyHints does the same for key names.
var keyHints = map[string]string{
	"enter": "return", "esc": "escape", "backspace": "delete", "del": "delete",
	"spacebar": "space", "pgup": "pageup", "pgdn": "pagedown",
}

func parseHotkey(src, path string) (Hotkey, error) {
	var h Hotkey
	if src == "" {
		return h, nil
	}
	if lower := strings.ToLower(src); lower != src {
		return h, fmt.Errorf("%s: write %q in lowercase, as %s", path, src, lower)
	}
	parts := strings.Split(src, "+")
	for _, p := range parts {
		if p == "" {
			return h, fmt.Errorf("%s: %q has an empty part; write modifiers and a key joined by +, like cmd+shift+space", path, src)
		}
	}
	key := parts[len(parts)-1]
	for _, m := range parts[:len(parts)-1] {
		var flag *bool
		switch m {
		case "cmd":
			flag = &h.Cmd
		case "opt":
			flag = &h.Opt
		case "ctrl":
			flag = &h.Ctrl
		case "shift":
			flag = &h.Shift
		default:
			if hint, ok := modifierHints[m]; ok {
				return h, fmt.Errorf("%s: write %q as %s", path, m, hint)
			}
			return h, fmt.Errorf("%s: %q is not a modifier; the modifiers are cmd, opt, ctrl and shift", path, m)
		}
		if *flag {
			return h, fmt.Errorf("%s: %s is written twice in %q", path, m, src)
		}
		*flag = true
	}
	if !hotkeyKeys[key] {
		if _, ok := modifierHints[key]; ok || key == "cmd" || key == "opt" || key == "ctrl" || key == "shift" {
			return h, fmt.Errorf("%s: %q ends in a modifier; it needs a key last, like cmd+shift+space", path, src)
		}
		if hint, ok := keyHints[key]; ok {
			return h, fmt.Errorf("%s: write %q as %s", path, key, hint)
		}
		return h, fmt.Errorf("%s: %q is not a key perch knows; see the hotkey section of the schema for the list", path, key)
	}
	h.Key = key
	// Shift alone still swallows a character from every app.
	fn := len(key) > 1 && key[0] == 'f' && key[1] >= '0' && key[1] <= '9'
	if !h.Cmd && !h.Opt && !h.Ctrl && !fn {
		return h, fmt.Errorf("%s: %q would take that key from every app; add cmd, opt or ctrl", path, src)
	}
	return h, nil
}
