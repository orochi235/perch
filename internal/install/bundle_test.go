package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubCompiler(body string) Compiler {
	return func(sources []string, out string) error {
		return os.WriteFile(out, []byte(body), 0o755)
	}
}

func failingCompiler(sources []string, out string) error {
	return errors.New("swiftc: error: no such module")
}

func opts(dest string, c Compiler) BundleOpts {
	return BundleOpts{
		App:     App{Name: "onto", ID: "dev.onto.menubar", Executable: "onto"},
		Sources: []string{"main.swift"},
		Dest:    dest,
		Compile: c,
		Sign:    func(string, string) error { return nil },
	}
}

func TestBuildBundleLaysOutTheApp(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "onto.app")
	if err := BuildBundle(opts(dest, stubCompiler("BINARY"))); err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	bin := filepath.Join(dest, "Contents", "MacOS", "onto")
	if b, err := os.ReadFile(bin); err != nil || string(b) != "BINARY" {
		t.Errorf("executable not at %s: %v", bin, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "Contents", "Info.plist")); err != nil {
		t.Errorf("no Info.plist: %v", err)
	}
}

// A failed compile must not damage a working install — the fix the other app has
// and brainhouse never got.
func TestBuildBundleLeavesAWorkingInstallAloneWhenCompilationFails(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "onto.app")
	if err := BuildBundle(opts(dest, stubCompiler("GOOD"))); err != nil {
		t.Fatalf("first build: %v", err)
	}
	if err := BuildBundle(opts(dest, failingCompiler)); err == nil {
		t.Fatal("want the compile error to surface, got nil")
	}
	bin := filepath.Join(dest, "Contents", "MacOS", "onto")
	if b, err := os.ReadFile(bin); err != nil || string(b) != "GOOD" {
		t.Errorf("the working install was damaged: %q %v", b, err)
	}
}

// A resource dropped from the spec must not linger in the installed bundle.
func TestBuildBundleReplacesRatherThanMergesTheBundle(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "onto.app")
	if err := BuildBundle(opts(dest, stubCompiler("GOOD"))); err != nil {
		t.Fatalf("first build: %v", err)
	}
	stale := filepath.Join(dest, "Contents", "Resources", "gone.txt")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := BuildBundle(opts(dest, stubCompiler("GOOD2"))); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("a file dropped from the build survived the rebuild")
	}
}

// A signature seals what is in the bundle, so it has to be the last thing the
// staged build does — and it has to happen before the rename, or a bundle would
// briefly be installed unsigned.
func TestBuildBundleSignsTheStagedBundleLast(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "onto.app")
	var signed, identity string
	var sawInfo, sawIcon bool
	o := opts(dest, stubCompiler("BINARY"))
	o.App.Identity = "perch local signing"
	o.Icons = iconsDir(t)
	o.Sign = func(bundle, id string) error {
		signed, identity = bundle, id
		_, err := os.Stat(filepath.Join(bundle, "Contents", "Info.plist"))
		sawInfo = err == nil
		_, err = os.Stat(filepath.Join(bundle, "Contents", "Resources", "o-0.png"))
		sawIcon = err == nil
		return nil
	}
	if err := BuildBundle(o); err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	if identity != "perch local signing" {
		t.Errorf("identity = %q, want app.sign", identity)
	}
	if signed == dest {
		t.Error("signed the installed bundle; it must sign the staged one, before the rename")
	}
	if !sawInfo || !sawIcon {
		t.Errorf("signed before the bundle was complete: Info.plist=%v icon=%v", sawInfo, sawIcon)
	}
}

// An identity the keychain does not hold fails the install rather than falling
// back to ad-hoc: the fallback works, which is what would make losing a stable
// identity — and with it every permission granted to the app — go unnoticed.
func TestBuildBundleFailsWhenSigningFails(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "onto.app")
	o := opts(dest, stubCompiler("BINARY"))
	o.Sign = func(string, string) error { return errors.New("no identity found matching") }
	if err := BuildBundle(o); err == nil {
		t.Fatal("a failed signature was accepted")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("an unsigned bundle was installed")
	}
}

func iconsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "o-0.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBundleWritesAnAppIconWhenOneIsThere(t *testing.T) {
	requireIconTools(t)
	dir := t.TempDir()
	icon := filepath.Join(dir, "AppIcon.png")
	writeTestPNG(t, icon, 512)

	dest := filepath.Join(dir, "W.app")
	err := BuildBundle(BundleOpts{
		App:     App{Name: "W", ID: "dev.example.w", Executable: "W", IconFile: "AppIcon"},
		Dest:    dest,
		AppIcon: icon,
		Compile: func(sources []string, out string) error { return os.WriteFile(out, []byte("x"), 0o755) },
		Sign:    func(bundle, identity string) error { return nil },
	})
	if err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "Contents", "Resources", "AppIcon.icns")); err != nil {
		t.Errorf("no AppIcon.icns in the bundle: %v", err)
	}
	info, err := os.ReadFile(filepath.Join(dest, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(info), "<key>CFBundleIconFile</key>") {
		t.Error("Info.plist names no icon, so the Dock tile stays generic")
	}
}

func TestBundleWithoutAnAppIconNamesNone(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "W.app")
	err := BuildBundle(BundleOpts{
		App:     App{Name: "W", ID: "dev.example.w", Executable: "W"},
		Dest:    dest,
		Compile: func(sources []string, out string) error { return os.WriteFile(out, []byte("x"), 0o755) },
		Sign:    func(bundle, identity string) error { return nil },
	})
	if err != nil {
		t.Fatalf("BuildBundle: %v", err)
	}
	info, err := os.ReadFile(filepath.Join(dest, "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(info), "CFBundleIconFile") {
		t.Error("named an icon that is not there")
	}
}
