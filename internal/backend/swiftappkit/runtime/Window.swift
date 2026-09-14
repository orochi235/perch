// Emitted only when menubar.yaml declares a window:, so an app without one
// links no WebKit.
import AppKit
import WebKit

struct ZoomConfig {
    let min: CGFloat
    let max: CGFloat
    let step: CGFloat
}

/// One long-lived window whose WebView is hidden rather than destroyed, so
/// reopening keeps scroll position and whatever the user had expanded.
final class WebWindow: NSObject, NSWindowDelegate, WKNavigationDelegate {
    private let url: URL
    private let windowTitle: String
    private let width: CGFloat
    private let height: CGFloat
    private let zoom: ZoomConfig?
    private let autosaveName: String

    private var window: NSWindow?
    private var webView: WKWebView?
    private var loadFailed = false

    /// Raised and lowered as the window appears and disappears, so the app only
    /// occupies the Dock while there is something to click on.
    var onVisibilityChange: ((Bool) -> Void)?

    init(url: String, title: String, width: Int, height: Int, zoom: ZoomConfig?, autosaveName: String) {
        self.url = URL(string: url) ?? URL(fileURLWithPath: "/")
        self.windowTitle = title
        self.width = CGFloat(width)
        self.height = CGFloat(height)
        self.zoom = zoom
        self.autosaveName = autosaveName
    }

    /// A miniaturized window still counts as present: isVisible is false while
    /// it sits in the Dock, but its tile is the way back to it, so the menu
    /// items that act on it have to stay live.
    var isPresented: Bool {
        guard let window else { return false }
        return window.isVisible || window.isMiniaturized
    }

    var canZoom: Bool { zoom != nil }

    func show() {
        if let window {
            // Held open across a server restart the page is dead HTML. Only
            // reload in that case, so the usual reopen keeps its state.
            if loadFailed { load() }
            present(window)
            return
        }
        let webView = WKWebView(frame: .zero)
        webView.navigationDelegate = self
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: width, height: height),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false)
        window.title = windowTitle
        window.contentView = webView
        window.delegate = self
        // Closing hides rather than destroys; without this AppKit frees the
        // window on close and the next open rebuilds the WebView.
        window.isReleasedWhenClosed = false
        window.center()
        // After center(), so a remembered size and position wins.
        window.setFrameAutosaveName(autosaveName)

        self.window = window
        self.webView = webView
        load()
        present(window)
    }

    /// Always a fresh load rather than reload(), which does nothing when the
    /// first navigation failed and left no back-forward entry to reload.
    func load() {
        loadFailed = false
        webView?.load(URLRequest(url: url))
    }

    func hide() {
        guard let window, isPresented else { return }
        window.orderOut(nil)
        onVisibilityChange?(false)
    }

    func zoomIn() { setZoom(currentZoom + (zoom?.step ?? 0)) }
    func zoomOut() { setZoom(currentZoom - (zoom?.step ?? 0)) }
    func actualSize() { setZoom(1) }

    private var currentZoom: CGFloat { webView?.pageZoom ?? 1 }

    private func setZoom(_ value: CGFloat) {
        guard let zoom else { return }
        webView?.pageZoom = Swift.min(Swift.max(value, zoom.min), zoom.max)
    }

    private func present(_ window: NSWindow) {
        // Announced before activating: the app has to be .regular already, or
        // the activation lands on an app with no Dock tile.
        onVisibilityChange?(true)
        NSApp.activate(ignoringOtherApps: true)
        // orderOut does not clear the miniaturized flag, so a window minimized
        // when it was hidden arrives here still miniaturized with no tile to
        // restore it from.
        if window.isMiniaturized { window.deminiaturize(nil) }
        window.makeKeyAndOrderFront(nil)
    }

    // MARK: - NSWindowDelegate

    func windowShouldClose(_ sender: NSWindow) -> Bool {
        hide()
        return false
    }

    // MARK: - WKNavigationDelegate

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        loadFailed = false
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        loadFailed = true
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        loadFailed = true
    }
}

/// Rebuilds the mirrored menu from the last poll every time it opens, so the
/// menu bar and the status item cannot disagree about what is possible.
final class MenuMirror: NSObject, NSMenuDelegate {
    private let nodes: () -> [MenuNode]
    private let repoll: () -> Void

    init(nodes: @escaping () -> [MenuNode], repoll: @escaping () -> Void) {
        self.nodes = nodes
        self.repoll = repoll
    }

    func menuNeedsUpdate(_ menu: NSMenu) {
        Draw.menu(nodes(), into: menu, repoll: repoll)
    }
}

/// Everything the main menu can ask the app to do. @objc so NSMenuItem can
/// drive it through target/action, and a protocol so the menu cannot reach past
/// it into the generated Controller.
@objc protocol WindowActions: AnyObject {
    func closeWindow()
    func reloadWindow()
    func zoomIn()
    func zoomOut()
    func actualSize()
    func requestQuit()
}

/// The app's main menu. It only appears while the window is up — an .accessory
/// app owns no menu bar — but it is also what makes Cmd-C work inside the
/// WebView: the Edit items reach WKWebView through the responder chain.
enum MainMenu {
    /// windowsMenu is assigned only after the menu is in place; AppKit ignores
    /// the assignment otherwise and the window list silently never populates.
    static func install(appName: String, canZoom: Bool, mirror: MenuMirror, target: WindowActions) {
        let main = NSMenu()
        main.addItem(appMenuItem(appName, target))
        main.addItem(fileMenuItem(target))
        main.addItem(editMenuItem())
        main.addItem(viewMenuItem(canZoom: canZoom, target))
        main.addItem(mirroredMenuItem(mirror))
        let windowItem = windowMenuItem()
        main.addItem(windowItem)
        NSApp.mainMenu = main
        NSApp.windowsMenu = windowItem.submenu
    }

    private static func appMenuItem(_ name: String, _ target: WindowActions) -> NSMenuItem {
        let menu = NSMenu(title: name)
        menu.addItem(item("About \(name)", #selector(NSApplication.orderFrontStandardAboutPanel(_:)), target: NSApp))
        menu.addItem(.separator())
        menu.addItem(item("Hide \(name)", #selector(NSApplication.hide(_:)), "h", target: NSApp))
        menu.addItem(item("Hide Others", #selector(NSApplication.hideOtherApplications(_:)), "h", [.command, .option], target: NSApp))
        menu.addItem(item("Show All", #selector(NSApplication.unhideAllApplications(_:)), target: NSApp))
        menu.addItem(.separator())
        // Off the standard Cmd-Q: quitting takes the status item and its
        // polling with it, which is not what Cmd-Q usually costs. Not on
        // Shift-Cmd-Q either — the Apple menu reserves that for Log Out and
        // swallows it before the app's item ever sees the key.
        menu.addItem(item("Quit \(name)", #selector(WindowActions.requestQuit), "q", [.command, .option], target: target))
        return holder(menu)
    }

    private static func fileMenuItem(_ target: WindowActions) -> NSMenuItem {
        let menu = NSMenu(title: "File")
        menu.addItem(item("Close", #selector(WindowActions.closeWindow), "w", target: target))
        // Cmd-Q is muscle memory. Binding it visibly to "close the window" is
        // safer than leaving it on Quit and kinder than swallowing it.
        menu.addItem(item("Close Window", #selector(WindowActions.closeWindow), "q", target: target))
        return holder(menu)
    }

    /// Standard selectors with no target: AppKit walks the responder chain to
    /// WKWebView, which implements all of them. Wiring this menu is the whole
    /// fix for copy and paste in the window.
    private static func editMenuItem() -> NSMenuItem {
        let menu = NSMenu(title: "Edit")
        menu.addItem(item("Undo", Selector(("undo:")), "z"))
        menu.addItem(item("Redo", Selector(("redo:")), "z", [.command, .shift]))
        menu.addItem(.separator())
        menu.addItem(item("Cut", #selector(NSText.cut(_:)), "x"))
        menu.addItem(item("Copy", #selector(NSText.copy(_:)), "c"))
        menu.addItem(item("Paste", #selector(NSText.paste(_:)), "v"))
        menu.addItem(.separator())
        menu.addItem(item("Select All", #selector(NSText.selectAll(_:)), "a"))
        return holder(menu)
    }

    private static func viewMenuItem(canZoom: Bool, _ target: WindowActions) -> NSMenuItem {
        let menu = NSMenu(title: "View")
        menu.addItem(item("Reload", #selector(WindowActions.reloadWindow), "r", target: target))
        if canZoom {
            menu.addItem(.separator())
            menu.addItem(item("Actual Size", #selector(WindowActions.actualSize), "0", target: target))
            menu.addItem(item("Zoom In", #selector(WindowActions.zoomIn), "+", target: target))
            menu.addItem(item("Zoom Out", #selector(WindowActions.zoomOut), "-", target: target))
        }
        return holder(menu)
    }

    /// The status item's own menu, in the menu bar. One declaration drives
    /// both, so the two cannot disagree about what is currently possible.
    ///
    /// Titled Status rather than the app's name, which the App menu already
    /// carries: two menus with one name is a menu bar you cannot read.
    private static func mirroredMenuItem(_ mirror: MenuMirror) -> NSMenuItem {
        let menu = NSMenu(title: "Status")
        menu.delegate = mirror
        let item = NSMenuItem()
        item.title = "Status"
        item.submenu = menu
        return item
    }

    private static func windowMenuItem() -> NSMenuItem {
        let menu = NSMenu(title: "Window")
        menu.addItem(item("Minimize", #selector(NSWindow.performMiniaturize(_:)), "m"))
        menu.addItem(item("Zoom", #selector(NSWindow.performZoom(_:))))
        menu.addItem(.separator())
        menu.addItem(item("Bring All to Front", #selector(NSApplication.arrangeInFront(_:)), target: NSApp))
        return holder(menu)
    }

    /// Top-level menus hang off a titleless item whose submenu carries the
    /// title — the shape AppKit expects of a main menu.
    private static func holder(_ menu: NSMenu) -> NSMenuItem {
        let item = NSMenuItem()
        item.submenu = menu
        return item
    }

    private static func item(_ title: String,
                             _ action: Selector?,
                             _ key: String = "",
                             _ mask: NSEvent.ModifierFlags = .command,
                             target: AnyObject? = nil) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: action, keyEquivalent: key)
        if !key.isEmpty { item.keyEquivalentModifierMask = mask }
        item.target = target
        return item
    }
}
