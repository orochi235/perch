package swiftappkit

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/orochi235/perch/v2/internal/backend"
)

const chipProbe = `
import AppKit

let button = NSStatusBarButton(frame: .zero)
func layout(_ tint: Tint?, badge: String, open: Bool) -> String {
    StatusButton.paint(MenuIcon.symbol("tray.full").image(), tint: tint, badge: badge, dim: false, menuOpen: open, on: button)
    return "image \(button.image?.size ?? .zero) title \"\(button.title)\" width \(button.intrinsicContentSize.width)"
}
for style in [Tint.Style.dot, .glyph, .accent, .chip] {
    for wrap in [Tint.Wrap.icon, .all] {
        for badge in ["", "12"] {
            let tint = Tint(color: "teal", alpha: style == .chip ? Tint.chipAlpha : 1, style: style,
                            size: .small, corner: .bottomRight, wrap: wrap)
            print("\(style) wrap:\(wrap) badge:\"\(badge)\"|\(layout(tint, badge: badge, open: false))|\(layout(tint, badge: badge, open: true))")
        }
    }
}
`

// Opening the menu must not move or resize the status item, whatever the tint
// and wherever its count sits: a chip's patch turns clear rather than going.
func TestStatusItemKeepsItsLayoutWhileTheMenuIsOpen(t *testing.T) {
	requireAqua(t)
	out, err := exec.Command(buildProbe(t, chipProbe)).CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 16 {
		t.Fatalf("probe printed %d cases, want 16:\n%s", len(lines), out)
	}
	for _, line := range lines {
		c := strings.Split(line, "|")
		if len(c) != 3 {
			t.Fatalf("probe line %q", line)
		}
		if c[1] != c[2] {
			t.Errorf("%s: closed %s\n\topen   %s", c[0], c[1], c[2])
		}
	}
}

const inkProbe = `
import AppKit

func ink(_ color: String, _ alpha: CGFloat, open: Bool = false) -> String {
    let ink = Tint(color: color, alpha: alpha, style: .chip, size: .small, corner: .bottomRight).shown(menuOpen: open).ink
    return ink == .white ? "white" : ink == .black ? "black" : "label"
}
print(ink("#a50a22", 0.85), ink("#ffe066", 0.85), ink("#a50a22", 0.35), ink("#a50a22", 0.85, open: true))
`

// A chip that hides the bar can't use the bar's text color: dark red at 85%
// under a light bar's black icon is unreadable.
func TestAStrongChipInksForItsOwnColor(t *testing.T) {
	requireAqua(t)
	out, err := exec.Command(buildProbe(t, inkProbe)).CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v\n%s", err, out)
	}
	if got, want := strings.TrimSpace(string(out)), "white black label label"; got != want {
		t.Errorf("ink: got %q, want %q", got, want)
	}
}

const menuInkProbe = `
import AppKit

func look(_ color: String, _ alpha: CGFloat, menu: Bool = true) -> String {
    let tint = Tint(color: color, alpha: alpha, style: .chip, size: .small, corner: .bottomRight, menu: menu)
    return MenuGlass.appearance(for: tint)?.name.rawValue ?? "system"
}
print(look("#116C80", 0.85), look("#ffe066", 0.85), look("#116C80", 0.35), look("#116C80", 0.85, menu: false))
`

// A tinted dropdown is the chip's color behind the menu's text, so it takes
// the chip's ink: black text on a light bar's teal menu was unreadable.
func TestAStronglyTintedMenuTakesTheChipsInk(t *testing.T) {
	requireAqua(t)
	out, err := exec.Command(buildProbe(t, menuInkProbe)).CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v\n%s", err, out)
	}
	want := "NSAppearanceNameDarkAqua NSAppearanceNameAqua system system"
	if got := strings.TrimSpace(string(out)); got != want {
		t.Errorf("menu appearance: got %q, want %q", got, want)
	}
}

const perchKitProbe = `
import AppKit

MainActor.assumeIsolated {
    let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    defer { NSStatusBar.system.removeStatusItem(item) }
    item.button?.image = NSImage(systemSymbolName: "tray.full", accessibilityDescription: nil)
    item.perchTint(color: "teal", style: .chip, wrap: .all, badge: "12")
    func corner() -> CGFloat {
        guard let image = item.button?.image, let tiff = image.tiffRepresentation,
              let rep = NSBitmapImageRep(data: tiff) else { return -1 }
        return rep.colorAt(x: 1, y: rep.pixelsHigh / 2)?.alphaComponent ?? -1
    }
    let wide = item.button?.image?.size.width ?? 0
    let menu = NSMenu()
    item.menu = menu // attached after perchTint
    NotificationCenter.default.post(name: NSMenu.didBeginTrackingNotification, object: menu)
    print("open patch \(corner())")
    NotificationCenter.default.post(name: NSMenu.didEndTrackingNotification, object: menu)
    print("closed patch \(corner())")
    item.perchTint(color: "teal", style: .chip, wrap: .all, badge: "")
    print("narrower \((item.button?.image?.size.width ?? 0) < wide)")
}
`

// PerchKit follows a menu attached after perchTint, and badge: "" clears a
// count the chip drew, which blanking the title could not.
func TestPerchKitFollowsALateMenuAndClearsABadge(t *testing.T) {
	requireScreen(t) // puts an item in the menu bar for a moment
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	files := []backend.File{
		{Name: "Icon.swift", Body: iconSwift},
		{Name: "StatusItem.swift", Body: statusItemSwift(t)},
		{Name: "main.swift", Body: []byte(perchKitProbe)},
	}
	out, err := exec.Command(compileProbe(t, swiftc, files)).CombinedOutput()
	if err != nil {
		t.Fatalf("probe: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"open patch 0.0\n", "narrower true\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("probe output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "closed patch 0.0\n") {
		t.Errorf("the patch stayed clear after the menu closed:\n%s", got)
	}
}

func statusItemSwift(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("runtime/StatusItem.swift")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
