package swiftappkit

import (
	"fmt"
	"strings"

	"github.com/orochi235/perch/v2/internal/celswift"
	"github.com/orochi235/perch/v2/internal/spec"
)

// itemLook is the trailing look: argument, omitted where the item sets none of
// its parts, so such an item emits what it always did. Each part is
// lowered where the item is built, so an each: item can take it from its element.
func itemLook(it spec.Item, e *celswift.Env, name func(string) string) (string, error) {
	var parts []string
	icon, err := itemIcon(it.Icon, e)
	if err != nil {
		return "", fmt.Errorf("%s.icon: %w", it.Path(), err)
	}
	if icon != "" {
		parts = append(parts, "icon: "+icon)
	}
	badges, err := itemBadges(it.Badges, e, name)
	if err != nil {
		return "", fmt.Errorf("%s.badges: %w", it.Path(), err)
	}
	if badges != "" {
		parts = append(parts, "badges: "+badges)
	}
	l := it.Look
	for _, f := range []struct {
		key, src string
		lower    func(string) (string, error)
	}{
		{"size", l.Size, e.LowerNumber},
		{"weight", l.Weight, e.LowerTemplate},
		{"indent", l.Indent, e.LowerNumber},
		{"enabled", l.Enabled, e.LowerCondition},
		{"subtitle", l.Subtitle, e.LowerTemplate},
		{"header", l.Header, e.LowerCondition},
	} {
		if f.src == "" {
			continue
		}
		lowered, err := f.lower(f.src)
		if err != nil {
			return "", fmt.Errorf("%s.%s: %w", it.Path(), f.key, err)
		}
		parts = append(parts, f.key+": "+lowered)
	}
	if l.Key.IsSet() {
		parts = append(parts, "key: "+keyCombo(l.Key))
	}
	if l.NoGutter {
		parts = append(parts, "gutter: false")
	}
	if len(parts) == 0 {
		return "", nil
	}
	return ", look: Look(" + strings.Join(parts, ", ") + ")", nil
}

// itemIcon is the icon as a MenuIcon, or "" where the item sets none.
func itemIcon(i spec.Icon, e *celswift.Env) (string, error) {
	if i.IsZero() {
		return "", nil
	}
	kind, name := "symbol", i.Symbol
	if i.Asset != "" {
		kind, name = "asset", i.Asset
	}
	lowered, err := e.LowerTemplate(name)
	if err != nil {
		return "", err
	}
	if i.Colored() {
		fg, err := e.LowerTemplate(i.FG)
		if err != nil {
			return "", fmt.Errorf("fg: %w", err)
		}
		bg, err := e.LowerTemplate(i.BG)
		if err != nil {
			return "", fmt.Errorf("bg: %w", err)
		}
		return "MenuIcon.colored(" + lowered + ", fg: " + fg + ", bg: " + bg + ")", nil
	}
	return "MenuIcon." + kind + "(" + lowered + ")", nil
}

// menuKeys is each named key as a menu item's keyEquivalent, in Swift. Letters,
// digits and punctuation are themselves; arrows, paging and f1 to f20 are
// AppKit's function keys, named here and looked up by KeyCombo.function.
var menuKeys = map[string]string{
	"space": `" "`, "return": `"\r"`, "tab": `"\t"`, "escape": `"\u{1b}"`,
	"delete": `"\u{8}"`, "forwarddelete": `"\u{7f}"`,
}

func keyCombo(h spec.Hotkey) string {
	key, ok := menuKeys[h.Key]
	if !ok {
		key = celswift.SwiftString(h.Key)
		if len(h.Key) > 1 {
			key = "KeyCombo.function(" + key + ")"
		}
	}
	var mods []string
	for _, m := range []struct {
		on   bool
		name string
	}{{h.Cmd, ".command"}, {h.Opt, ".option"}, {h.Ctrl, ".control"}, {h.Shift, ".shift"}} {
		if m.on {
			mods = append(mods, m.name)
		}
	}
	return "KeyCombo(key: " + key + ", modifiers: [" + strings.Join(mods, ", ") + "])"
}
