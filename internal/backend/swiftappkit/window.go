package swiftappkit

import (
	"fmt"

	"github.com/orochi235/perch/v2/internal/celswift"
	"github.com/orochi235/perch/v2/internal/spec"
)

// emitWindowMembers writes the stored properties Controller needs for a window.
// Nothing is written when the spec declares none, so an app without one names
// no WebKit type at all.
func emitWindowMembers(b *buf, s *spec.Spec) {
	if s.Window == nil {
		return
	}
	w := s.Window
	zoom := "nil"
	if w.Zoom != nil {
		zoom = fmt.Sprintf("ZoomConfig(min: %s, max: %s, step: %s)",
			swiftDouble(w.Zoom.Min), swiftDouble(w.Zoom.Max), swiftDouble(w.Zoom.Step))
	}
	b.line("private let window = WebWindow(")
	b.in()
	b.line("url: %s,", celswift.SwiftString(w.URL))
	b.line("title: %s,", celswift.SwiftString(w.Title))
	b.line("width: %d, height: %d,", w.Width, w.Height)
	b.line("zoom: %s,", zoom)
	b.line("autosaveName: %s)", celswift.SwiftString(exported(s.App.Name)+"Window"))
	b.out()
	b.line("private var mirror: MenuMirror?")
	b.line("")
}

// emitWindowSetup writes the launch-time wiring, called at the end of init.
func emitWindowSetup(b *buf, s *spec.Spec) {
	if s.Window == nil {
		return
	}
	b.line("private func setUpWindow() {")
	b.in()
	b.line("window.onVisibilityChange = { [weak self] visible in self?.setDockPresence(visible) }")
	b.line("Windows.handler = { [weak self] verb in self?.act(verb) }")
	b.line("let mirror = MenuMirror(")
	b.in()
	b.line("nodes: { [weak self] in self.map { renderMenu($0.results) } ?? [] },")
	b.line("repoll: { [weak self] in self?.poll() })")
	b.out()
	b.line("self.mirror = mirror")
	b.line("MainMenu.install(appName: %s, canZoom: window.canZoom, mirror: mirror, target: self)",
		celswift.SwiftString(s.App.Name))
	b.out()
	b.line("}")
	b.line("")
}

// emitWindowActions writes the WindowActions conformance and the Dock policy.
func emitWindowActions(b *buf, s *spec.Spec) {
	if s.Window == nil {
		return
	}
	b.line("private func act(_ verb: WindowVerb) {")
	b.in()
	b.line("switch verb {")
	b.line("case .open: window.show()")
	b.line("case .close: window.hide()")
	b.line("case .reload: window.load()")
	b.line("}")
	b.out()
	b.line("}")
	b.line("")
	// Ordering matters on the way down: demoting to .accessory while the app is
	// frontmost with a visible window can strand the menu bar.
	b.line("private func setDockPresence(_ visible: Bool) {")
	b.in()
	b.line("if visible {")
	b.in()
	b.line("NSApp.setActivationPolicy(.regular)")
	b.out()
	b.line("} else {")
	b.in()
	b.line("NSApp.setActivationPolicy(.accessory)")
	b.line("NSApp.deactivate()")
	b.out()
	b.line("}")
	b.out()
	b.line("}")
	b.line("")
	b.line("func closeWindow() { window.hide() }")
	b.line("func reloadWindow() { window.load() }")
	b.line("func zoomIn() { window.zoomIn() }")
	b.line("func zoomOut() { window.zoomOut() }")
	b.line("func actualSize() { window.actualSize() }")
	b.line("func requestQuit() { NSApp.terminate(nil) }")
	b.line("")
	// Clicking the Dock icon should bring the window back, which is the whole
	// point of having one.
	b.line("func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows: Bool) -> Bool {")
	b.in()
	b.line("window.show()")
	b.line("return true")
	b.out()
	b.line("}")
	b.line("")
	// The window items go dim when there is no window to act on.
	b.line("func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {")
	b.in()
	b.line("switch menuItem.action {")
	b.line("case #selector(closeWindow), #selector(reloadWindow),")
	b.in()
	b.line(" #selector(zoomIn), #selector(zoomOut), #selector(actualSize):")
	b.line("return window.isPresented")
	b.out()
	b.line("default:")
	b.in()
	b.line("return true")
	b.out()
	b.line("}")
	b.out()
	b.line("}")
	b.line("")
}

// emitQuitDelegate writes applicationShouldTerminate. Nothing is written when
// the spec declares no rules, so an app that never asks does not carry the
// machinery for asking.
func emitQuitDelegate(b *buf, s *spec.Spec) {
	if len(s.App.Quit) == 0 {
		return
	}
	b.line("func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {")
	b.in()
	b.line("Quit.should(")
	b.in()
	b.line("renderQuit(results),")
	b.line("event: NSAppleEventManager.shared().currentAppleEvent,")
	b.line("then: { ok in NSApp.reply(toApplicationShouldTerminate: ok) })")
	b.out()
	b.out()
	b.line("}")
	b.line("")
}
