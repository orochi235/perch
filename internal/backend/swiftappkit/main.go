package swiftappkit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/orochi235/perch/internal/celswift"
	"github.com/orochi235/perch/internal/spec"
)

func emitMain(s *spec.Spec, n structNames) (string, error) {
	b := &buf{}
	e := env(s)

	b.line("%s", header)
	b.line("import AppKit")
	b.line("import Foundation")
	b.line("")

	emitResultTypes(b, s, n)

	b.line("final class Controller: NSObject, NSApplicationDelegate, NSMenuDelegate {")
	b.in()
	b.line("private let statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)")
	b.line("private var results = Results()")
	b.line("private var timer: Timer?")
	b.line("")
	b.line("override init() {")
	b.in()
	b.line("super.init()")
	b.line("let menu = NSMenu()")
	b.line("menu.delegate = self")
	b.line("statusItem.menu = menu")
	b.line("refresh()")
	b.line("poll()")
	b.line("timer = Timer.scheduledTimer(withTimeInterval: %s, repeats: true) { [weak self] _ in", swiftDouble(s.App.Interval.Seconds()))
	b.in()
	b.line("self?.poll()")
	b.out()
	b.line("}")
	b.out()
	b.line("}")
	b.line("")

	emitPoll(b, s, n)
	if err := emitRefresh(b, s, e); err != nil {
		return "", err
	}
	if err := emitMenu(b, s, e); err != nil {
		return "", err
	}

	b.out()
	b.line("}")
	b.line("")
	b.line("let app = NSApplication.shared")
	b.line("app.setActivationPolicy(.accessory)")
	b.line("let controller = Controller()")
	b.line("app.delegate = controller")
	b.line("app.run()")
	return b.String(), nil
}

func emitResultTypes(b *buf, s *spec.Spec, n structNames) {
	for _, w := range s.Watches {
		b.line("struct %s {", n.resultTypeName(w))
		b.in()
		b.line("var ok = false")
		switch w.Kind {
		case spec.WatchRun:
			b.line("var code = -1")
			b.line(`var out = ""`)
			b.line(`var err = ""`)
		case spec.WatchHTTP:
			b.line("var status = 0")
			b.line(`var out = ""`)
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

func emitPoll(b *buf, s *spec.Spec, n structNames) {
	b.line("func poll() {")
	b.in()
	if len(s.Watches) == 0 {
		b.line("refresh()")
		b.out()
		b.line("}")
		b.line("")
		return
	}

	b.line("let group = DispatchGroup()")
	b.line("let sync = DispatchQueue(label: %s)", celswift.SwiftString(s.App.ID+".results"))
	b.line("var next = Results()")
	b.line("")
	for _, w := range s.Watches {
		b.line("group.enter()")
		b.line("DispatchQueue.global(qos: .utility).async {")
		b.in()
		b.line("var r = %s()", n.resultTypeName(w))
		emitWatchBody(b, w, n)
		b.line("sync.async {")
		b.in()
		b.line("next.%s = r", decl(w.Name))
		b.line("group.leave()")
		b.out()
		b.line("}")
		b.out()
		b.line("}")
		b.line("")
	}
	b.line("group.notify(queue: .main) { [weak self] in")
	b.in()
	b.line("guard let self else { return }")
	b.line("self.results = sync.sync { next }")
	b.line("self.refresh()")
	b.line("if let menu = self.statusItem.menu, menu.numberOfItems > 0 { self.menuNeedsUpdate(menu) }")
	b.out()
	b.line("}")
	b.out()
	b.line("}")
	b.line("")
}

func emitWatchBody(b *buf, w spec.Watch, n structNames) {
	switch w.Kind {
	case spec.WatchRun:
		b.line("let o = Watcher.run(%s)", swiftArray(w.Run))
		b.line("r.ok = o.ok")
		b.line("r.code = o.code")
		b.line("r.out = o.out")
		b.line("r.err = o.err")
		emitDecode(b, w, n, "o.out")
	case spec.WatchHTTP:
		b.line("let o = Watcher.http(%s)", celswift.SwiftString(w.HTTP))
		b.line("r.ok = o.ok")
		b.line("r.status = o.status")
		b.line("r.out = o.out")
		emitDecode(b, w, n, "o.out")
	case spec.WatchExists:
		b.line("r.ok = Watcher.exists(%s)", celswift.SwiftString(w.Exists))
	}
}

func emitDecode(b *buf, w spec.Watch, n structNames, from string) {
	if !w.JSON {
		return
	}
	if w.Shape == nil {
		b.line("r.data = JSONValue.parse(%s)", from)
		return
	}
	t, zero, _ := n.dataTypeAndZero(w)
	b.line("r.data = (try? JSONDecoder().decode(%s.self, from: Data(%s.utf8))) ?? %s", t, from, zero)
}

func swiftArray(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, a := range argv {
		parts = append(parts, celswift.SwiftString(a))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// emitRefresh writes the first-match-wins status chain. An unguarded rule ends
// the chain, which is why the spec requires it to be last.
func emitRefresh(b *buf, s *spec.Spec, e *celswift.Env) error {
	b.line("func refresh() {")
	b.in()
	b.line("guard let button = statusItem.button else { return }")
	b.line("%s icon = %s", bind(anyRule(s, func(r spec.StatusRule) bool { return r.Icon != "" })), celswift.SwiftString(s.App.Icon))
	b.line("%s dim = false", bind(anyRule(s, func(r spec.StatusRule) bool { return r.Dim })))
	b.line(`%s badge = ""`, bind(anyRule(s, func(r spec.StatusRule) bool { return r.Badge != "" })))

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
		if rule.Icon != "" {
			b.line("icon = %s", celswift.SwiftString(rule.Icon))
		}
		if rule.Dim {
			b.line("dim = true")
		}
		if rule.Badge != "" {
			badge, err := e.LowerText(rule.Badge)
			if err != nil {
				return fmt.Errorf("status[%d].badge: %w", i, err)
			}
			b.line("badge = %s", badge)
		}
		b.out()
	}
	if len(s.Status) > 0 {
		b.line("}")
	}

	b.line("let image = NSImage(systemSymbolName: icon, accessibilityDescription: nil)")
	b.line("image?.isTemplate = true")
	b.line("button.image = image")
	b.line("button.appearsDisabled = dim")
	b.line(`button.title = badge.isEmpty ? "" : " " + badge`)
	b.out()
	b.line("}")
	b.line("")
	return nil
}

// menuGen hands out unique local names so nested submenus and each: loops do
// not shadow one another.
type menuGen struct {
	b *buf
	n int
}

func (g *menuGen) name(prefix string) string {
	g.n++
	return fmt.Sprintf("%s%d", prefix, g.n)
}

func emitMenu(b *buf, s *spec.Spec, e *celswift.Env) error {
	b.line("func menuNeedsUpdate(_ menu: NSMenu) {")
	b.in()
	b.line("menu.removeAllItems()")
	g := &menuGen{b: b}
	if err := g.items(s.Menu, "menu", e, "menu"); err != nil {
		return err
	}
	b.out()
	b.line("}")
	return nil
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
		g.b.line("%s.addItem(NSMenuItem.separator())", into)
		return nil
	}

	title, err := e.LowerTemplate(it.Text)
	if err != nil {
		return fmt.Errorf("%s.text: %w", path, err)
	}

	if len(it.Menu) > 0 {
		item := g.name("item")
		sub := g.name("sub")
		g.b.line(`let %s = NSMenuItem(title: %s, action: nil, keyEquivalent: "")`, item, title)
		g.b.line("let %s = NSMenu()", sub)
		if err := g.items(it.Menu, sub, e, path+".menu"); err != nil {
			return err
		}
		g.b.line("%s.submenu = %s", item, sub)
		g.b.line("%s.addItem(%s)", into, item)
		return nil
	}

	if it.Action.Kind == spec.ActionNone {
		g.b.line(`%s.addItem(NSMenuItem(title: %s, action: nil, keyEquivalent: ""))`, into, title)
		return nil
	}

	// The action is lowered before its closure is opened: whether the closure
	// needs self depends on what the expressions reached for, not on the verb.
	// An argv or a URL can name a watch, and those lower to self.results.
	body := &buf{depth: g.b.depth + 1}
	if err := g.action(body, it.Action, e, path); err != nil {
		return err
	}
	rendered := body.String()

	if strings.Contains(rendered, "self.") {
		g.b.line("%s.addItem(ActionItem(title: %s) { [weak self] in", into, title)
		g.b.in()
		g.b.line("guard let self else { return }")
		g.b.out()
	} else {
		// A closure with no capture list has no `in` either.
		g.b.line("%s.addItem(ActionItem(title: %s) {", into, title)
	}
	g.b.raw(rendered)
	g.b.line("})")
	return nil
}

func (g *menuGen) action(b *buf, a spec.Action, e *celswift.Env, path string) error {
	switch a.Kind {
	case spec.ActionQuit:
		b.line("NSApp.terminate(nil)")

	case spec.ActionRun:
		argv, err := templateArray(e, a.Run)
		if err != nil {
			return fmt.Errorf("%s.run: %w", path, err)
		}
		b.line("Act.run(%s) { self.poll() }", argv)

	case spec.ActionOpen:
		target, err := e.LowerTemplate(a.Open)
		if err != nil {
			return fmt.Errorf("%s.open: %w", path, err)
		}
		b.line("Act.open(%s)", target)

	case spec.ActionPost:
		url, err := e.LowerTemplate(a.PostURL)
		if err != nil {
			return fmt.Errorf("%s.post.url: %w", path, err)
		}
		post, err := e.LowerTemplate(a.PostBody)
		if err != nil {
			return fmt.Errorf("%s.post.body: %w", path, err)
		}
		b.line("Act.post(%s, body: %s) { self.poll() }", url, post)
	}
	return nil
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

// swiftDouble always spells a decimal point, so the emitted literal reads as a
// TimeInterval rather than an Int.
func swiftDouble(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
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
