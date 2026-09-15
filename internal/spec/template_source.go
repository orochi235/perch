package spec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/orochi235/perch/internal/templates"
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
	shipped, isShipped := templates.Source(name)
	if r.dir != "" {
		path := filepath.Join(r.dir, name+".yaml")
		src, err := os.ReadFile(path)
		switch {
		case err == nil && isShipped:
			return nil, path, false, fmt.Errorf("%s has the name of a template perch ships; a reader could not tell which one runs, so give it another name", path)
		case err == nil:
			return src, path, true, nil
		case !errors.Is(err, fs.ErrNotExist):
			return nil, path, false, err
		}
	}
	if isShipped {
		return shipped, "perch's " + name + ".yaml", true, nil
	}
	return nil, "", false, nil
}

func (r repoTemplates) Names() []string {
	names := templates.Names()
	if r.dir != "" {
		matches, _ := filepath.Glob(filepath.Join(r.dir, "*.yaml"))
		for _, m := range matches {
			names = append(names, strings.TrimSuffix(filepath.Base(m), ".yaml"))
		}
	}
	sort.Strings(names)
	return names
}
