package swiftappkit

import (
	"fmt"
	"strings"

	"github.com/orochi235/perch/v2/internal/celswift"
	"github.com/orochi235/perch/v2/internal/spec"
)

// itemLook is the trailing look: argument, omitted where the item sets no
// icon, badges or size, so such an item emits what it always did. Each part is
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
	if it.Size != "" {
		size, err := e.LowerNumber(it.Size)
		if err != nil {
			return "", fmt.Errorf("%s.size: %w", it.Path(), err)
		}
		parts = append(parts, "size: "+size)
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
