package spec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/orochi235/perch/v2/internal/templates"
)

// TemplateSource finds a template's YAML by the name use: gives it.
type TemplateSource interface {
	// Template returns the file's contents and the name to report it by. ok is
	// false, with no error, when there is no template by that name.
	Template(name string) (src []byte, file string, ok bool, err error)
	// Names lists every template the source can find, for the error naming one
	// that is not there.
	Names() []string
}

// TemplatesIn is what a repo can use: its own templates in dir, and the ones
// perch ships. dir is "" for the shipped ones alone. Nothing outside the repo
// is read, so a build answers the same on a laptop and on CI.
func TemplatesIn(dir string) TemplateSource { return repoTemplates{dir: dir} }

type repoTemplates struct{ dir string }

func (r repoTemplates) Template(name string) ([]byte, string, bool, error) {
	if err := checkTemplateName("use", name); err != nil {
		return nil, "", false, err
	}
	shipped, isShipped := templates.Source(name)
	if r.dir != "" {
		entries, err := os.ReadDir(r.dir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, r.dir, false, err
		}
		for _, e := range entries {
			stem, ok := strings.CutSuffix(e.Name(), ".yaml")
			if !ok || !strings.EqualFold(stem, name) {
				continue
			}
			path := filepath.Join(r.dir, e.Name())
			// Compared ignoring case: on macOS use: service would read Service.yaml.
			if s := shippedName(stem); s != "" {
				return nil, path, false, fmt.Errorf("%s has the name of a template perch ships (perch's %s.yaml); a reader could not tell which one runs, so give it another name", path, s)
			}
			if stem != name {
				continue
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil, path, false, err
			}
			return src, path, true, nil
		}
	}
	if isShipped {
		return shipped, "perch's " + name + ".yaml", true, nil
	}
	return nil, "", false, nil
}

// shippedName is the shipped template that name matches ignoring case, or "".
func shippedName(name string) string {
	for _, s := range templates.Names() {
		if strings.EqualFold(s, name) {
			return s
		}
	}
	return ""
}

// Names lists every usable template: perch's shipped ones plus the repo's own,
// sorted. A repo file that is not a valid template name, a directory, or one
// shadowing a shipped name is skipped rather than offered as something use:
// could ask for.
func (r repoTemplates) Names() []string {
	names := templates.Names()
	if entries, err := os.ReadDir(r.dir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name, ok := strings.CutSuffix(e.Name(), ".yaml")
			if !ok || checkTemplateName("", name) != nil || shippedName(name) != "" {
				continue
			}
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
