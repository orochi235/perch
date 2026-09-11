package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Compiler turns Swift sources into one executable. It is a function so a test
// can stand in for swiftc.
type Compiler func(sources []string, out string) error

// SwiftC compiles with the Swift toolchain on PATH.
func SwiftC(sources []string, out string) error {
	args := append([]string{"-O", "-o", out}, sources...)
	cmd := exec.Command("swiftc", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("swiftc: %w", err)
	}
	return nil
}

// BundleOpts describes one .app build.
type BundleOpts struct {
	App     App
	Sources []string
	Dest    string
	Compile Compiler
	// Icons is a directory whose .png files are copied into Resources. Absent
	// or empty is ordinary: a spec using only SF Symbols needs no artwork.
	Icons string
}

// BuildBundle stages a complete .app in a temporary directory and only then
// replaces Dest. Two failures this avoids: a failed compile damaging a working
// install, and a resource dropped from the build lingering in the old bundle.
func BuildBundle(o BundleOpts) error {
	if o.Compile == nil {
		o.Compile = SwiftC
	}
	// Staged beside Dest rather than in the system temp dir: the final step is a
	// rename, and a rename across filesystems fails.
	if err := os.MkdirAll(filepath.Dir(o.Dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(o.Dest), ".perch-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	staged := filepath.Join(tmp, filepath.Base(o.Dest))
	macos := filepath.Join(staged, "Contents", "MacOS")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		return err
	}
	if err := o.Compile(o.Sources, filepath.Join(macos, o.App.Executable)); err != nil {
		return err
	}
	if err := copyIcons(o.Icons, filepath.Join(staged, "Contents", "Resources")); err != nil {
		return err
	}
	info := filepath.Join(staged, "Contents", "Info.plist")
	if err := os.WriteFile(info, []byte(InfoPlist(o.App)), 0o644); err != nil {
		return err
	}

	if err := os.RemoveAll(o.Dest); err != nil {
		return err
	}
	if err := os.Rename(staged, o.Dest); err != nil {
		return fmt.Errorf("installing %s: %w", o.Dest, err)
	}
	return nil
}

// copyIcons stages every .png beside the spec. Nothing is filtered against what
// the spec references: an icon chosen by a status rule is still referenced, and
// working that out here would duplicate the check Load already makes.
func copyIcons(src, dest string) error {
	if src == "" {
		return nil
	}
	matches, err := filepath.Glob(filepath.Join(src, "*.png"))
	if err != nil || len(matches) == 0 {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, m := range matches {
		body, err := os.ReadFile(m)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dest, filepath.Base(m)), body, 0o644); err != nil {
			return err
		}
	}
	return nil
}
