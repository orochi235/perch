package spec

import (
	"strings"
	"testing"
)

// A menubar.yaml is a file a person edits, so every malformed state on the way
// to a working one reaches Parse. Each has to come back as an error naming a
// place in the document; a panic is perch crashing on a half-typed file.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"",
		"app: {name: a, id: b, icon: c, interval: 1s}\nmenu: [{text: Quit, quit: true}]\n",
		"app:\n  name: a\n",
		"watch: {w: {run: [x], json: true, shape: {a: [{b: int}]}}}\n",
		"status: [{when: x, badge: y}]\n",
		"menu: [separator]\n",
		"menu: [{each: xs, text: a, menu: [{text: b}]}]\n",
		"menu: [{text: a, post: {url: u, body: {k: v}}}]\n",
		"[]", "null", "- a", "a: *x", "&a b", "!!binary x",
		"a: &x {b: *x}", "app: {interval: 99999999999999999999s}",
		"\x00", "app: {name: \"../evil\"}", "watch: {it: {run: [x]}}",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, doc string) {
		s, err := Parse([]byte(doc))
		if err != nil {
			// A diagnostic with no path in it leaves the author nowhere to look.
			if msg := err.Error(); strings.TrimSpace(msg) == "" {
				t.Fatalf("%q was refused with an empty message", doc)
			}
			return
		}
		// Anything Parse accepts, a backend then emits from, so the invariants the
		// emitter relies on have to hold on every accepted document.
		if s.App.Name == "" || s.App.ID == "" || s.App.Icon.IsZero() || s.App.Interval <= 0 {
			t.Fatalf("%q parsed into an incomplete app: %+v", doc, s.App)
		}
		if strings.ContainsAny(s.App.Name, `/\`) {
			t.Fatalf("%q accepted an app.name that walks out of ~/Applications: %q", doc, s.App.Name)
		}
		seen := map[string]bool{}
		for _, w := range s.Watches {
			if !identifier.MatchString(w.Name) || celReserved[w.Name] || w.Name == "it" {
				t.Fatalf("%q accepted an unusable watch name %q", doc, w.Name)
			}
			if seen[w.Name] {
				t.Fatalf("%q accepted a watch declared twice: %q", doc, w.Name)
			}
			seen[w.Name] = true
			if w.Shape != nil {
				checkShape(t, doc, w.Shape)
			}
		}
		checkItems(t, doc, s.Menu)
	})
}

func checkShape(t *testing.T, doc string, ty *Type) {
	t.Helper()
	switch ty.Kind {
	case TypeList:
		if ty.Elem == nil {
			t.Fatalf("%q accepted a list type with no element", doc)
		}
		checkShape(t, doc, ty.Elem)
	case TypeObject:
		for _, f := range ty.Fields {
			if !identifier.MatchString(f.Name) {
				t.Fatalf("%q accepted an unusable field name %q", doc, f.Name)
			}
			if f.Type == nil {
				t.Fatalf("%q accepted field %q with no type", doc, f.Name)
			}
			checkShape(t, doc, f.Type)
		}
	}
}

func checkItems(t *testing.T, doc string, items []Item) {
	t.Helper()
	for _, it := range items {
		if len(it.Menu) > 0 && it.Action.Kind != ActionNone {
			t.Fatalf("%q accepted a submenu item whose %v action can never run", doc, it.Action.Kind)
		}
		if it.Action.Kind == ActionRun && len(it.Action.Run) == 0 {
			t.Fatalf("%q accepted an empty run", doc)
		}
		if it.Action.Kind == ActionPost && it.Action.PostURL == "" {
			t.Fatalf("%q accepted a post with no url", doc)
		}
		checkItems(t, doc, it.Menu)
	}
}
