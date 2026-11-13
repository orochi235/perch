package swiftappkit

import (
	"fmt"
	"strings"

	"github.com/orochi235/perch/v2/internal/celswift"
	"github.com/orochi235/perch/v2/internal/spec"
)

// itemBadges is the badges as a Swift [MenuBadge], or "" where the item sets none.
func itemBadges(b spec.Badges, e *celswift.Env, name func(string) string) (string, error) {
	if b.Expr != "" {
		return badgesFromExpr(b.Expr, e, name)
	}
	if len(b.List) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(b.List))
	for i, badge := range b.List {
		var fields []string
		for _, f := range []struct{ key, src string }{{"text", badge.Text}, {"symbol", badge.Symbol}, {"fg", badge.FG}, {"bg", badge.BG}} {
			lowered, err := e.LowerTemplate(f.src)
			if err != nil {
				return "", fmt.Errorf("[%d].%s: %w", i, f.key, err)
			}
			fields = append(fields, f.key+": "+lowered)
		}
		fields = append(fields, "at: "+celswift.SwiftString(string(badge.At)))
		parts = append(parts, "MenuBadge("+strings.Join(fields, ", ")+")")
	}
	return "[" + strings.Join(parts, ", ") + "]", nil
}

// badgesFromExpr maps a list's elements to badges. Each element is bound as it,
// so its fields are read the way any expression reads them; a declared shape
// may leave out any field but text.
func badgesFromExpr(src string, e *celswift.Env, name func(string) string) (string, error) {
	list, elem, err := e.LowerList(src)
	if err != nil {
		return "", fmt.Errorf("%w; badges: takes a list of {text, symbol, fg, bg, at}", err)
	}
	if elem.Kind != spec.TypeObject && elem.Kind != spec.TypeAny {
		return "", fmt.Errorf("%q: badges: needs a list of {text, symbol, fg, bg, at}, this is a list of %v", src, elem.Kind)
	}
	v := name("badge")
	inner := e.WithEach(elem, v)
	fields := make([]string, 0, 5)
	for _, key := range []string{"text", "symbol", "fg", "bg", "at"} {
		if key != "text" && elem.Kind == spec.TypeObject && !hasField(elem, key) {
			continue
		}
		lowered, err := inner.LowerText("it." + key)
		if err != nil {
			return "", fmt.Errorf("%q: each badge's %s: %w", src, key, err)
		}
		fields = append(fields, key+": "+lowered)
	}
	return fmt.Sprintf("%s.map { %s in MenuBadge(%s) }", list, v, strings.Join(fields, ", ")), nil
}

func hasField(t *spec.Type, name string) bool {
	for _, f := range t.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}
