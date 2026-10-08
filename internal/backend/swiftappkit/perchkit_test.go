package swiftappkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/orochi235/perch/v2/internal/spec"
)

// PerchKit compiles runtime/Icon.swift and runtime/StatusItem.swift as their
// own module; this proves the public surface from outside it.
func TestPerchKitBuildsAsAModule(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not on PATH")
	}
	dir := t.TempDir()
	out, err := exec.Command(swiftc, "-emit-module", "-parse-as-library", "-module-name", "PerchKit",
		"-emit-module-path", filepath.Join(dir, "PerchKit.swiftmodule"),
		"runtime/Icon.swift", "runtime/StatusItem.swift").CombinedOutput()
	if err != nil {
		t.Fatalf("PerchKit does not build: %v\n%s", err, out)
	}
	if len(out) > 0 {
		t.Errorf("PerchKit builds with warnings:\n%s", out)
	}
	host := filepath.Join(dir, "host.swift")
	if err := os.WriteFile(host, []byte(`import AppKit
import PerchKit

@MainActor func tint(_ item: NSStatusItem) {
    item.perchTint(color: "#116C80", style: .chip, wrap: .all, menu: true, opacity: 0.5, badge: "3")
    item.perchTint(color: "teal", size: .large, corner: .topLeft)
    item.perchTint(color: nil)
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(swiftc, "-typecheck", "-I", dir, host).CombinedOutput(); err != nil {
		t.Fatalf("a host cannot call PerchKit: %v\n%s", err, out)
	}
}

// Every runtime file is either one of PerchKit's sources or excluded from it,
// so a new one is a decision rather than a SwiftPM warning.
func TestPackageSwiftCoversTheRuntime(t *testing.T) {
	src, err := os.ReadFile("../../../Package.swift")
	if err != nil {
		t.Fatal(err)
	}
	list := func(key string) []string {
		m := regexp.MustCompile(key + `: \[([^\]]*)\]`).FindSubmatch(src)
		if m == nil {
			t.Fatalf("Package.swift has no %s list", key)
		}
		var names []string
		for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(m[1], -1) {
			names = append(names, string(q[1]))
		}
		return names
	}
	sources, exclude := list("sources"), list("exclude")
	if want := []string{"Icon.swift", "StatusItem.swift"}; !slices.Equal(sources, want) {
		t.Errorf("PerchKit sources = %v, want %v", sources, want)
	}
	entries, err := os.ReadDir("runtime")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !slices.Contains(sources, e.Name()) && !slices.Contains(exclude, e.Name()) {
			t.Errorf("runtime/%s is neither a PerchKit source nor excluded in Package.swift", e.Name())
		}
	}
}

// perchTint's defaults are app.tint's, which the parser decides.
func TestPerchKitDefaultsMatchTheParser(t *testing.T) {
	icon, err := os.ReadFile("runtime/Icon.swift")
	if err != nil {
		t.Fatal(err)
	}
	item, err := os.ReadFile("runtime/StatusItem.swift")
	if err != nil {
		t.Fatal(err)
	}
	parse := func(tint string) spec.Tint {
		t.Helper()
		s, err := spec.Parse([]byte("app: {name: a, id: dev.a, icon: circle, interval: 1s, tint: " + tint + "}\nmenu: [{text: Q, quit: true}]\n"))
		if err != nil {
			t.Fatal(err)
		}
		return s.App.Tint
	}

	m := regexp.MustCompile(`static let chipAlpha: CGFloat = ([0-9.]+)`).FindSubmatch(icon)
	if m == nil {
		t.Fatal("Icon.swift declares no chipAlpha")
	}
	if got, _ := strconv.ParseFloat(string(m[1]), 64); got != parse("{color: teal, style: chip}").Alpha {
		t.Errorf("Icon.swift's chipAlpha is %s; the parser's chip alpha is %v", m[1], parse("{color: teal, style: chip}").Alpha)
	}

	d := parse("teal")
	sig := regexp.MustCompile(`style: Tint\.Style = \.(\w+), size: Tint\.Size = \.(\w+),\s*corner: Tint\.Corner = \.(\w+), wrap: Tint\.Wrap = \.(\w+), menu: Bool = (\w+)`).FindSubmatch(item)
	if sig == nil {
		t.Fatal("StatusItem.swift's perchTint signature has changed shape")
	}
	corner, side, _ := strings.Cut(string(d.Corner), "-")
	for _, c := range []struct{ name, swift, parser string }{
		{"style", string(sig[1]), string(d.Style)},
		{"size", string(sig[2]), string(d.Size)},
		{"corner", string(sig[3]), corner + strings.ToUpper(side[:1]) + side[1:]},
		{"wrap", string(sig[4]), string(d.Wrap)},
		{"menu", string(sig[5]), strconv.FormatBool(d.Menu)},
	} {
		if c.swift != c.parser {
			t.Errorf("perchTint's %s defaults to %s; app.tint's to %s", c.name, c.swift, c.parser)
		}
	}
}
