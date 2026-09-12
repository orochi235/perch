package swiftappkit

import (
	"fmt"
	"strings"

	"github.com/orochi235/perch/internal/celswift"
	"github.com/orochi235/perch/internal/spec"
)

// emitRender writes Render.swift: what one poll returned, and the two functions
// deciding from it what the status item and its menu show. Nothing here draws
// anything, which is what lets the docs site run it without a menu bar.
func emitRender(s *spec.Spec, n structNames) (string, error) {
	b := &buf{}
	e := renderEnv(s)

	b.line("%s", header)
	b.line("import Foundation")
	b.line("")

	emitResultTypes(b, s, n)
	emitResultInits(b, s, n)
	if err := emitStates(b, s); err != nil {
		return "", err
	}
	if err := emitFace(b, s, e); err != nil {
		return "", err
	}
	return emitMenu(b, s, e)
}

// renderEnv is the lowering environment for Render.swift: watches and states
// are reached through the results the two functions take.
func renderEnv(s *spec.Spec) *celswift.Env {
	e := celswift.NewEnv(s.Watches)
	for _, st := range s.States {
		e = e.WithState(st.Name, stateProp(st.Name))
	}
	return e.Prefixed("results.")
}

// emitStates writes each state as a property of Results. A condition is
// lowered against the watches alone: the ordering already excludes every
// earlier state, so naming one could only ever be a constant, and a guard that
// contributes nothing is worse than one refused. They are properties rather
// than locals because a state a given function never reads is then simply
// unread, instead of an unused binding the compiler warns about.
func emitStates(b *buf, s *spec.Spec) error {
	if len(s.States) == 0 {
		return nil
	}
	e := celswift.NewEnv(s.Watches)
	b.line("extension Results {")
	b.in()
	for i, st := range s.States {
		parts := make([]string, 0, i+1)
		for _, earlier := range s.States[:i] {
			parts = append(parts, "!"+stateProp(earlier.Name))
		}
		if st.Cond == "" {
			if len(parts) == 0 {
				parts = append(parts, "true")
			}
		} else {
			cond, err := e.LowerCondition(st.Cond)
			if err != nil {
				return fmt.Errorf("state[%d].%s: %w", i, st.Name, err)
			}
			// Parenthesized: a condition of a && b would otherwise come apart
			// under the leading negations, and still compile.
			parts = append(parts, "("+cond+")")
		}
		b.line("var %s: Bool { %s }", stateProp(st.Name), strings.Join(parts, " && "))
	}
	b.out()
	b.line("}")
	b.line("")
	return nil
}

// stateProp keeps a state's property out of the way of a watch's, and of the
// names the emitter mints for itself.
func stateProp(name string) string { return "state_" + name }

func emitResultTypes(b *buf, s *spec.Spec, n structNames) {
	for _, w := range s.Watches {
		b.line("struct %s {", n.resultTypeName(w))
		b.in()
		switch w.Kind {
		case spec.WatchRun:
			b.line("var ok = false")
			b.line("var code = -1")
			b.line(`var out = ""`)
			b.line(`var err = ""`)
		case spec.WatchHTTP:
			b.line("var ok = false")
			b.line("var status = 0")
			b.line(`var out = ""`)
		case spec.WatchLaunchAgent:
			// No ok: loaded and running are both answers to it and they differ.
			b.line("var installed = false")
			b.line("var loaded = false")
			b.line("var running = false")
			b.line("var pid = 0")
			b.line(`var label = ""`)
			b.line(`var plist = ""`)
			b.line(`var domain = ""`)
			b.line(`var target = ""`)
		default:
			b.line("var ok = false")
		}
		if t, zero, ok := n.dataTypeAndZero(w); ok {
			b.line("var data: %s = %s", t, zero)
		}
		b.out()
		b.line("}")
		b.line("")
	}

	b.line("struct Results {")
	b.in()
	for _, w := range s.Watches {
		b.line("var %s = %s()", decl(w.Name), n.resultTypeName(w))
	}
	if len(s.Watches) == 0 {
		b.line("// no watches declared")
	}
	b.out()
	b.line("}")
	b.line("")
}

// emitResultInits writes the step from what a watcher returned to the watch's
// result. The app reaches it from poll(), and a docs page's preview reaches it
// with the sample outcomes that page states, which is what keeps the two
// showing the same menu.
func emitResultInits(b *buf, s *spec.Spec, n structNames) {
	for _, w := range s.Watches {
		b.line("extension %s {", n.resultTypeName(w))
		b.in()
		switch w.Kind {
		case spec.WatchHTTP:
			b.line("init(_ o: HTTPOutcome) {")
			b.in()
			b.line("self.init()")
			b.line("ok = o.ok")
			b.line("status = o.status")
			b.line("out = o.out")
			emitDecode(b, w, n, "o.out")
		case spec.WatchExists:
			b.line("init(exists: Bool) {")
			b.in()
			b.line("self.init()")
			b.line("ok = exists")
		case spec.WatchLaunchAgent:
			b.line("init(_ o: LaunchAgentOutcome) {")
			b.in()
			b.line("self.init()")
			b.line("installed = o.installed")
			b.line("loaded = o.loaded")
			b.line("running = o.running")
			b.line("pid = o.pid")
			b.line("label = o.label")
			b.line("plist = o.plist")
			b.line("domain = Launchd.domain")
			b.line("target = Launchd.target(o.label)")
		default:
			b.line("init(_ o: RunOutcome) {")
			b.in()
			b.line("self.init()")
			b.line("ok = o.ok")
			b.line("code = o.code")
			b.line("out = o.out")
			b.line("err = o.err")
			emitDecode(b, w, n, "o.out")
		}
		b.out()
		b.line("}")
		b.out()
		b.line("}")
		b.line("")
	}
}

func emitDecode(b *buf, w spec.Watch, n structNames, from string) {
	if !w.JSON {
		return
	}
	if w.Shape == nil {
		b.line("data = JSONValue.parse(%s)", from)
		return
	}
	t, zero, _ := n.dataTypeAndZero(w)
	b.line("data = (try? JSONDecoder().decode(%s.self, from: Data(%s.utf8))) ?? %s", t, from, zero)
}

// emitFace writes the first-match-wins status chain. An unguarded rule ends the
// chain, which is why the spec requires it to be last.
func emitFace(b *buf, s *spec.Spec, e *celswift.Env) error {
	b.line("func renderFace(_ results: Results) -> Face {")
	b.in()
	mutated := anyRule(s, func(r spec.StatusRule) bool {
		return !r.Icon.IsZero() || r.Dim || r.Badge != ""
	})
	b.line("%s face = Face(icon: %s)", bind(mutated), swiftIcon(s.App.Icon))

	for i, rule := range s.Status {
		switch {
		case rule.When == "" && i == 0:
			b.line("if true {")
		case rule.When == "":
			b.line("} else {")
		default:
			cond, err := e.LowerCondition(rule.When)
			if err != nil {
				return fmt.Errorf("status[%d].when: %w", i, err)
			}
			if i == 0 {
				b.line("if %s {", cond)
			} else {
				b.line("} else if %s {", cond)
			}
		}
		b.in()
		if !rule.Icon.IsZero() {
			b.line("face.icon = %s", swiftIcon(rule.Icon))
		}
		if rule.Dim {
			b.line("face.dim = true")
		}
		if rule.Badge != "" {
			badge, err := e.LowerText(rule.Badge)
			if err != nil {
				return fmt.Errorf("status[%d].badge: %w", i, err)
			}
			b.line("face.badge = %s", badge)
		}
		b.out()
	}
	if len(s.Status) > 0 {
		b.line("}")
	}

	b.line("return face")
	b.out()
	b.line("}")
	b.line("")
	return nil
}

// swiftIcon writes the MenuIcon the runtime switches on. A spec that names a
// bare symbol emits exactly what it always did.
func swiftIcon(i spec.Icon) string {
	if i.Asset != "" {
		return "MenuIcon.asset(" + celswift.SwiftString(i.Asset) + ")"
	}
	return "MenuIcon.symbol(" + celswift.SwiftString(i.Symbol) + ")"
}

// menuGen hands out unique local names so nested submenus and each: loops do
// not shadow one another.
type menuGen struct {
	b       *buf
	n       int
	watches []spec.Watch // for agent:, which acts on the watch it names
}

func (g *menuGen) name(prefix string) string {
	g.n++
	return fmt.Sprintf("%s%d", prefix, g.n)
}

func emitMenu(b *buf, s *spec.Spec, e *celswift.Env) (string, error) {
	b.line("func renderMenu(_ results: Results) -> [MenuNode] {")
	b.in()
	if len(s.Menu) == 0 {
		b.line("return []")
		b.out()
		b.line("}")
		return b.String(), nil
	}
	b.line("var menu: [MenuNode] = []")
	g := &menuGen{b: b, watches: s.Watches}
	if err := g.items(s.Menu, "menu", e, "menu"); err != nil {
		return "", err
	}
	b.line("return menu")
	b.out()
	b.line("}")
	return b.String(), nil
}

func (g *menuGen) items(items []spec.Item, into string, e *celswift.Env, path string) error {
	for i, it := range items {
		if err := g.item(it, into, e, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func (g *menuGen) item(it spec.Item, into string, e *celswift.Env, path string) error {
	inner := e
	closes := 0

	if it.Each != "" {
		list, elem, err := e.LowerList(it.Each)
		if err != nil {
			return fmt.Errorf("%s.each: %w", path, err)
		}
		loopVar := g.name("it")
		g.b.line("for %s in %s {", loopVar, list)
		g.b.in()
		closes++
		inner = e.WithEach(elem, loopVar)
	}

	if it.When != "" {
		cond, err := inner.LowerCondition(it.When)
		if err != nil {
			return fmt.Errorf("%s.when: %w", path, err)
		}
		g.b.line("if %s {", cond)
		g.b.in()
		closes++
	}

	if err := g.body(it, into, inner, path); err != nil {
		return err
	}

	for ; closes > 0; closes-- {
		g.b.out()
		g.b.line("}")
	}
	return nil
}

func (g *menuGen) body(it spec.Item, into string, e *celswift.Env, path string) error {
	if it.Separator {
		g.b.line("%s.append(.separator)", into)
		return nil
	}

	title, err := e.LowerTemplate(it.Text)
	if err != nil {
		return fmt.Errorf("%s.text: %w", path, err)
	}

	if len(it.Menu) > 0 {
		sub := g.name("sub")
		g.b.line("var %s: [MenuNode] = []", sub)
		if err := g.items(it.Menu, sub, e, path+".menu"); err != nil {
			return err
		}
		g.b.line("%s.append(.submenu(%s, %s))", into, title, sub)
		return nil
	}

	if it.Action.Kind == spec.ActionNone {
		g.b.line("%s.append(.item(%s, nil))", into, title)
		return nil
	}

	action, err := g.action(it.Action, e, path)
	if err != nil {
		return err
	}
	g.b.line("%s.append(.item(%s, %s))", into, title, action)
	return nil
}

// action writes the verb as data. It is lowered where the menu is built rather
// than in a closure fired later, so an item runs the arguments it showed.
func (g *menuGen) action(a spec.Action, e *celswift.Env, path string) (string, error) {
	switch a.Kind {
	case spec.ActionQuit:
		return ".quit", nil

	case spec.ActionRun:
		argv, err := templateArray(e, a.Run)
		if err != nil {
			return "", fmt.Errorf("%s.run: %w", path, err)
		}
		return ".run(" + argv + ")", nil

	case spec.ActionOpen:
		target, err := e.LowerTemplate(a.Open)
		if err != nil {
			return "", fmt.Errorf("%s.open: %w", path, err)
		}
		return ".open(" + target + ")", nil

	case spec.ActionAgent:
		// spec.validate has already refused an agent: naming anything else.
		for _, w := range g.watches {
			if w.Name == a.Agent && w.Kind == spec.WatchLaunchAgent {
				return fmt.Sprintf(".agent(label: %s, plist: %s, verb: .%s)",
					celswift.SwiftString(w.Label), celswift.SwiftString(w.Plist), a.Verb), nil
			}
		}
		return "", fmt.Errorf("%s.agent: no launchagent watch named %q", path, a.Agent)

	case spec.ActionPost:
		url, err := e.LowerTemplate(a.PostURL)
		if err != nil {
			return "", fmt.Errorf("%s.post.url: %w", path, err)
		}
		body, err := e.LowerTemplate(a.PostBody)
		if err != nil {
			return "", fmt.Errorf("%s.post.body: %w", path, err)
		}
		return ".post(url: " + url + ", body: " + body + ")", nil
	}
	return "", fmt.Errorf("%s: unknown action %v", path, a.Kind)
}

func templateArray(e *celswift.Env, argv []string) (string, error) {
	parts := make([]string, 0, len(argv))
	for _, a := range argv {
		s, err := e.LowerTemplate(a)
		if err != nil {
			return "", err
		}
		parts = append(parts, s)
	}
	return "[" + strings.Join(parts, ", ") + "]", nil
}

func anyRule(s *spec.Spec, pred func(spec.StatusRule) bool) bool {
	for _, r := range s.Status {
		if pred(r) {
			return true
		}
	}
	return false
}

func bind(mutated bool) string {
	if mutated {
		return "var"
	}
	return "let"
}
