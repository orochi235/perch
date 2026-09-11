// Package project is the layout of a consuming repo: an authored menubar.yaml,
// an emitted menubar/Generated/, and a hand-written menubar/Sources/ that perch
// never reads or writes.
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/orochi235/perch/internal/backend"
	"github.com/orochi235/perch/internal/spec"
)

const SpecFile = "menubar.yaml"

type Project struct {
	Root string
	Spec *spec.Spec
}

// Load reads and validates root/menubar.yaml.
func Load(root string) (*Project, error) {
	p := &Project{Root: root}
	src, err := os.ReadFile(p.SpecPath())
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", p.SpecPath(), err)
	}
	s, err := spec.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.SpecPath(), err)
	}
	p.Spec = s
	if err := p.checkIcons(); err != nil {
		return nil, fmt.Errorf("%s: %w", p.SpecPath(), err)
	}
	return p, nil
}

// SpecPath is the file every build error is reported against.
func (p *Project) SpecPath() string { return filepath.Join(p.Root, SpecFile) }

func (p *Project) GeneratedDir() string { return filepath.Join(p.Root, "menubar", "Generated") }
func (p *Project) SourcesDir() string   { return filepath.Join(p.Root, "menubar", "Sources") }

// IconsDir holds artwork an icon: {asset: name} refers to. Copied into the
// bundle's Resources whole, so a file nothing references costs only its bytes.
func (p *Project) IconsDir() string { return filepath.Join(p.Root, "menubar", "Icons") }

// checkIcons refuses a spec naming artwork that is not there. The app would
// otherwise build, install and run with no status item image at all, which
// looks like the poller failing rather than a missing file.
func (p *Project) checkIcons() error {
	for path, icon := range p.Spec.Icons() {
		if icon.Asset == "" {
			continue
		}
		file := filepath.Join(p.IconsDir(), icon.Asset+".png")
		if _, err := os.Stat(file); err != nil {
			return fmt.Errorf("%s: no such file: %s", path, file)
		}
	}
	return nil
}

// WriteGenerated replaces the generated directory outright, so a file the spec
// no longer produces cannot linger. It touches nothing else.
func (p *Project) WriteGenerated(files []backend.File) error {
	dir := p.GeneratedDir()
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.Name), f.Body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// SwiftSources is every file the compiler needs: emitted and hand-written.
func (p *Project) SwiftSources() ([]string, error) {
	var out []string
	for _, dir := range []string{p.GeneratedDir(), p.SourcesDir()} {
		matches, err := filepath.Glob(filepath.Join(dir, "*.swift"))
		if err != nil {
			return nil, err
		}
		out = append(out, matches...)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no Swift sources; run perch build first")
	}
	return out, nil
}

// BundlePath is where install puts the .app.
func (p *Project) BundlePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Applications", p.Spec.App.Name+".app"), nil
}

// PlistPath is the LaunchAgent that keeps it running.
func (p *Project) PlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", p.Spec.App.ID+".plist"), nil
}
