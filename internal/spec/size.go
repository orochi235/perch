package spec

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// parseSize keeps a size as written: a literal is checked here, and an
// expression is lowered as a number where the menu is built.
func parseSize(n *yaml.Node, path string) (string, error) {
	if n.Kind == 0 {
		return "", nil
	}
	if n.Kind != yaml.ScalarNode || strings.TrimSpace(n.Value) == "" {
		return "", fmt.Errorf("%s: want a font size in points, or a CEL expression giving one", path)
	}
	if v, err := strconv.ParseFloat(n.Value, 64); err == nil && (v < 6 || v > 32) {
		return "", fmt.Errorf("%s: %v is outside 6 to 32 points; the menu's own is 13", path, v)
	}
	return n.Value, nil
}
