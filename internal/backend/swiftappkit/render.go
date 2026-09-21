package swiftappkit

import (
	"fmt"
	"strings"

	"github.com/orochi235/perch/v2/internal/celswift"
	"github.com/orochi235/perch/v2/internal/spec"
)

// emitRender writes Render.swift: what one poll returned, and the two functions
// deciding from it what the status item and its menu show. Nothing here draws
// anything, which is what lets the docs site run it without a menu bar.
func emitRender(s *spec.Spec, n structNames) (string, error) {
	b := &buf{}
	e := renderEnv(s)
	scopes := scopeEnvs(s)

	b.line("%s", header)
	b.line("import Foundation")
	b.line("")

	emitResultTypes(b, s, n)
	emitResultInits(b, s, n)
	if err := emitUseStates(b, s, n); err != nil {
		return "", err
	}
	if err := emitStates(b, s); err != nil {
		return "", err
	}
	if err := emitFace(b, s, e, scopes); err != nil {
		return "", err
	}
	return emitMenu(b, s, e, scopes)
}

// renderEnv is the lowering environment for the file's own rules and items:
// watches, uses and states, reached through the results the functions take.
func renderEnv(s *spec.Spec) *celswift.Env {
	e := celswift.NewEnv(s.Watches).WithUses(s.Uses)
	for _, st := range s.States {
		e = e.WithState(st.Name, stateProp(st.Name))
	}
	return e.Prefixed("results.")
}

// scopeEnvs is what each use's rules and items lower against: self, reached
// through the results, and nothing of the file's.
func scopeEnvs(s *spec.Spec) map[string]*celswift.Env {
	out := map[string]*celswift.Env{}
	for _, u := range s.Uses {
		out[u.Name] = celswift.ForUse(u, u.Name).Prefixed("results.")
	}
	return out
}

func emitResultTypes(b *buf, s *spec.Spec, n structNames) {
	for _, w := range s.AllWatches() {
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

	for _, u := range s.Uses {
		b.line("struct %s {", n.useTypeName(u))
		b.in()
		for _, w := range u.Watches {
			b.line("var %s = %s()", decl(w.Name), n.resultTypeName(w))
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
	for _, u := range s.Uses {
		b.line("var %s = %s()", decl(u.Name), n.useTypeName(u))
	}
	if len(s.Watches) == 0 && len(s.Uses) == 0 {
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
	for _, w := range s.AllWatches() {
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
func emitFace(b *buf, s *spec.Spec, e *celswift.Env, scopes map[string]*celswift.Env) error {
	b.line("func renderFace(_ results: Results) -> Face {")
	b.in()
	mutated := anyRule(s, func(r spec.StatusRule) bool {
		return !r.Icon.IsZero() || r.Dim || r.Badge != ""
	})
	b.line("%s face = Face(icon: %s)", bind(mutated), swiftIcon(s.App.Icon))

	for i, rule := range s.Status {
		re := e
		if rule.Scope != "" {
			re = scopes[rule.Scope]
		}
		switch {
		case rule.When == "" && i == 0:
			b.line("if true {")
		case rule.When == "":
			b.line("} else {")
		default:
			cond, err := re.LowerCondition(rule.When)
			if err != nil {
				return fmt.Errorf("%s.when: %w", rule.Path(), err)
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
			badge, err := re.LowerText(rule.Badge)
			if err != nil {
				return fmt.Errorf("%s.badge: %w", rule.Path(), err)
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
	b      *buf
	n      int
	spec   *spec.Spec
	file   *celswift.Env
	scopes map[string]*celswift.Env
}

func (g *menuGen) envFor(scope string) *celswift.Env {
	if scope == "" {
		return g.file
	}
	return g.scopes[scope]
}

func (g *menuGen) name(prefix string) string {
	g.n++
	return fmt.Sprintf("%s%d", prefix, g.n)
}

func emitMenu(b *buf, s *spec.Spec, e *celswift.Env, scopes map[string]*celswift.Env) (string, error) {
	b.line("func renderMenu(_ results: Results) -> [MenuNode] {")
	b.in()
	if len(s.Menu) == 0 {
		b.line("return []")
		b.out()
		b.line("}")
		return b.String(), nil
	}
	b.line("var menu: [MenuNode] = []")
	g := &menuGen{b: b, spec: s, file: e, scopes: scopes}
	if err := g.items(s.Menu, "menu", e, ""); err != nil {
		return "", err
	}
	// tidy, not the author: which items a poll leaves out decides which
	// separators are left with nothing beside them.
	b.line("return tidy(menu)")
	b.out()
	b.line("}")
	if err := emitQuit(b, s, e, g); err != nil {
		return "", err
	}
	return b.String(), nil
}

// emitQuit writes renderQuit: the first rule whose condition holds, lowered in
// order so the fallback cannot outrank a guarded rule.
func emitQuit(b *buf, s *spec.Spec, e *celswift.Env, g *menuGen) error {
	if len(s.App.Quit) == 0 {
		return nil
	}
	b.line("")
	b.line("func renderQuit(_ results: Results) -> QuitPrompt? {")
	b.in()
	for i, r := range s.App.Quit {
		path := fmt.Sprintf("app.quit[%d]", i)
		guarded := r.When != ""
		if guarded {
			cond, err := e.LowerCondition(r.When)
			if err != nil {
				return fmt.Errorf("%s.when: %w", path, err)
			}
			b.line("if %s {", cond)
			b.in()
		}
		b.line("return QuitPrompt(")
		b.in()
		b.line("message: %s,", celswift.SwiftString(r.Confirm))
		b.line("detail: %s,", celswift.SwiftString(r.Detail))
		if len(r.Buttons) == 0 {
			b.line("buttons: [])")
		} else {
			b.line("buttons: [")
			b.in()
			for j, btn := range r.Buttons {
				action := "nil"
				if btn.Action.Kind != spec.ActionNone {
					lowered, err := g.action(btn.Action, e, fmt.Sprintf("%s.buttons[%d]", path, j), "")
					if err != nil {
						return err
					}
					action = lowered
				}
				b.line("QuitChoice(text: %s, action: %s),", celswift.SwiftString(btn.Text), action)
			}
			b.out()
			b.line("])")
		}
		b.out()
		if guarded {
			b.out()
			b.line("}")
		}
	}
	// A bare last rule always returns, so this is reachable only when every
	// rule is guarded and none of them held.
	if s.App.Quit[len(s.App.Quit)-1].When != "" {
		b.line("return nil")
	}
	b.out()
	b.line("}")
	return nil
}

// items lowers each item against the environment of the scope it came from:
// crossing into a template's items leaves the file's names, it included.
func (g *menuGen) items(items []spec.Item, into string, e *celswift.Env, scope string) error {
	for _, it := range items {
		ie := e
		if it.Scope != scope {
			ie = g.envFor(it.Scope)
		}
		if err := g.item(it, into, ie); err != nil {
			return err
		}
	}
	return nil
}

func (g *menuGen) item(it spec.Item, into string, e *celswift.Env) error {
	path := it.Path()
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

	var conds []string
	if it.When != "" {
		cond, err := inner.LowerCondition(it.When)
		if err != nil {
			return fmt.Errorf("%s.when: %w", path, err)
		}
		conds = append(conds, cond)
	}
	if it.Guard != "" {
		cond, err := inner.LowerCondition(it.Guard)
		if err != nil {
			return fmt.Errorf("%s.agent: the verb's guard: %w", path, err)
		}
		conds = append(conds, cond)
	}
	if len(conds) > 0 {
		g.b.line("if %s {", strings.Join(conds, " && "))
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
	icon, err := itemIcon(it.Icon, e)
	if err != nil {
		return fmt.Errorf("%s.icon: %w", path, err)
	}

	if len(it.Menu) > 0 {
		sub := g.name("sub")
		g.b.line("var %s: [MenuNode] = []", sub)
		if err := g.items(it.Menu, sub, e, it.Scope); err != nil {
			return err
		}
		g.b.line("%s.append(.submenu(%s, %s%s))", into, title, sub, icon)
		return nil
	}

	if it.Action.Kind == spec.ActionNone {
		g.b.line("%s.append(.item(%s, nil%s))", into, title, icon)
		return nil
	}

	action, err := g.action(it.Action, e, path, it.Scope)
	if err != nil {
		return err
	}
	g.b.line("%s.append(.item(%s, %s%s))", into, title, action, icon)
	return nil
}

// itemIcon is the trailing icon: argument, omitted where the item sets none so
// an item without one emits what it always did. The name is lowered where the
// item is built, so an each: item can take its icon from its element.
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
	return ", icon: MenuIcon." + kind + "(" + lowered + ")", nil
}

// action writes the verb as data. It is lowered where the menu is built rather
// than in a closure fired later, so an item runs the arguments it showed.
func (g *menuGen) action(a spec.Action, e *celswift.Env, path, scope string) (string, error) {
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
		w, err := g.spec.AgentWatch(scope, a.Agent)
		if err != nil {
			return "", fmt.Errorf("%s.agent: %w", path, err)
		}
		return fmt.Sprintf(".agent(label: %s, plist: %s, verb: .%s)",
			celswift.SwiftString(w.Label), celswift.SwiftString(w.Plist), a.Verb), nil

	case spec.ActionSwift:
		// Emitted verbatim as a method reference, which is a () -> Void. perch
		// cannot check the target exists; swiftc does, in the same module.
		return ".swift(" + a.Swift + ")", nil

	case spec.ActionWindow:
		return ".window(." + string(a.Window) + ")", nil

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
