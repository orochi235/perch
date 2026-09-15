// Package templates holds the templates perch ships. A menubar.yaml reaches one
// by name under use:, through spec.TemplatesIn.
package templates

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed *.yaml
var files embed.FS

// Source returns a shipped template's YAML, or false when perch ships none by
// that name.
func Source(name string) ([]byte, bool) {
	b, err := files.ReadFile(name + ".yaml")
	if err != nil {
		return nil, false
	}
	return b, true
}

// Names lists every shipped template, in file order.
func Names() []string {
	entries, _ := fs.ReadDir(files, ".")
	var out []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".yaml"); ok {
			out = append(out, name)
		}
	}
	return out
}
