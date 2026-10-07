package site

import (
	"testing"

	"github.com/orochi235/perch/v2/internal/spec"
)

func TestEverySystemColorHasACSSColor(t *testing.T) {
	for _, name := range spec.SystemColors {
		if _, ok := systemCSS[name]; !ok {
			t.Errorf("no CSS color for %q", name)
		}
	}
}
