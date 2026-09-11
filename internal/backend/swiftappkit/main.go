package swiftappkit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/orochi235/perch/internal/celswift"
	"github.com/orochi235/perch/internal/spec"
)

// emitMain writes main.swift: the app itself. It polls, hands what came back to
// Render.swift to decide, and hands that to the runtime to draw.
func emitMain(s *spec.Spec, n structNames) string {
	b := &buf{}

	b.line("%s", header)
	b.line("import AppKit")
	b.line("import Foundation")
	b.line("")

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
	// Artwork's fade is worked out at draw time, so it has to be redrawn when
	// the thing it is worked out from moves. The poll interval would get there
	// eventually and look like a lag.
	b.line("NSWorkspace.shared.notificationCenter.addObserver(")
	b.in()
	b.line("forName: NSWorkspace.didActivateApplicationNotification, object: nil, queue: .main")
	b.line(") { [weak self] _ in self?.refresh() }")
	b.out()
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
	emitDraw(b)

	b.out()
	b.line("}")
	b.line("")
	b.line("let app = NSApplication.shared")
	b.line("app.setActivationPolicy(.accessory)")
	b.line("let controller = Controller()")
	b.line("app.delegate = controller")
	b.line("app.run()")
	return b.String()
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
		b.line("let r = %s", watchCall(w, n))
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

// watchCall polls one watch and makes its result out of what came back.
func watchCall(w spec.Watch, n structNames) string {
	switch w.Kind {
	case spec.WatchHTTP:
		return fmt.Sprintf("%s(Watcher.http(%s))", n.resultTypeName(w), celswift.SwiftString(w.HTTP))
	case spec.WatchExists:
		return fmt.Sprintf("%s(exists: Watcher.exists(%s))", n.resultTypeName(w), celswift.SwiftString(w.Exists))
	default:
		return fmt.Sprintf("%s(Watcher.run(%s))", n.resultTypeName(w), swiftArray(w.Run))
	}
}

func emitDraw(b *buf) {
	b.line("func refresh() {")
	b.in()
	b.line("guard let button = statusItem.button else { return }")
	b.line("Draw.face(renderFace(results), on: button)")
	b.out()
	b.line("}")
	b.line("")
	b.line("func menuNeedsUpdate(_ menu: NSMenu) {")
	b.in()
	b.line("Draw.menu(renderMenu(results), into: menu) { [weak self] in self?.poll() }")
	b.out()
	b.line("}")
}

func swiftArray(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, a := range argv {
		parts = append(parts, celswift.SwiftString(a))
	}
	return "[" + strings.Join(parts, ", ") + "]"
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
