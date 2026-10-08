// PerchKit's entry point: app.tint for a status item an app draws itself.
// Only the library compiles this file; generated apps draw through Render.swift.

import AppKit
import ObjectiveC

public extension NSStatusItem {
    /// Tints the status item as `app.tint` does in menubar.yaml, with the same
    /// options and defaults. Set the button's image first; call again to change
    /// the tint, or pass nil to clear it. `color` is #rrggbb, #rrggbbaa, or a
    /// system color name, and the user's `tint` default replaces it.
    @MainActor
    func perchTint(color: String?, style: Tint.Style = .dot, size: Tint.Size = .small,
                   corner: Tint.Corner = .bottomRight, wrap: Tint.Wrap = .icon, menu: Bool = false,
                   opacity: CGFloat? = nil) {
        let painter = TintPainter.of(self)
        painter.tint = color.map { color in
            let (base, alpha) = Tint.split(color)
            return Tint(color: base, alpha: opacity ?? alpha ?? (style == .chip ? Tint.chipAlpha : 1),
                        style: style, size: size, corner: corner, wrap: wrap, menu: menu)
        }
        painter.paint()
    }
}

extension Tint {
    /// A #rrggbbaa color's #rrggbb and alpha; any other color as given.
    static func split(_ color: String) -> (String, CGFloat?) {
        guard color.hasPrefix("#"), color.count == 9, let a = UInt8(color.suffix(2), radix: 16) else { return (color, nil) }
        return (String(color.prefix(7)), CGFloat(a) / 255)
    }
}

/// Keeps what the app drew so the tint can be redrawn over it, as the menu
/// opens and closes, without the app's knowing.
@MainActor
final class TintPainter {
    private static var key = 0
    private weak var item: NSStatusItem?
    var tint: Tint?
    private var image: NSImage?
    private var badge = ""
    private var drawn: (image: NSImage?, title: String)?
    private var menuOpen = false
    private var following: NSMenu?

    private init(_ item: NSStatusItem) { self.item = item }

    static func of(_ item: NSStatusItem) -> TintPainter {
        if let painter = objc_getAssociatedObject(item, &key) as? TintPainter { return painter }
        let painter = TintPainter(item)
        objc_setAssociatedObject(item, &key, painter, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)
        MenuGlass.follow { [weak painter] in MainActor.assumeIsolated { painter?.shownTint } }
        return painter
    }

    private var shownTint: Tint? {
        tint.map { var t = $0; t.name = Prefs.tint(or: t.name); return t }
    }

    func paint() {
        guard let item, let button = item.button else { return }
        // Whatever the button holds that this did not draw is the app's own.
        if button.image !== drawn?.image { image = button.image }
        if button.title != drawn?.title { badge = button.title.trimmingCharacters(in: .whitespaces) }
        if let menu = item.menu, menu !== following {
            following = menu
            StatusButton.follow(menu) { [weak self] open in
                MainActor.assumeIsolated {
                    self?.menuOpen = open
                    self?.paint()
                }
            }
        }
        StatusButton.paint(image, tint: shownTint, badge: badge, dim: button.appearsDisabled,
                           menuOpen: menuOpen, on: button)
        drawn = (button.image, button.title)
    }
}
