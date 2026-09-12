package spec

import (
	"fmt"
	"regexp"
	"strings"
)

// identifier is what a watch name or a shape field has to be: expressions
// select it and the backend declares it, and neither survives a hyphen.
var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// celReserved are words CEL's own grammar refuses as identifiers. Rejecting
// them here names the watch; letting one through only fails later, quoting an
// expression the author did not write.
var celReserved = map[string]bool{
	"as": true, "break": true, "const": true, "continue": true, "else": true,
	"false": true, "for": true, "function": true, "if": true, "import": true,
	"in": true, "let": true, "loop": true, "namespace": true, "null": true,
	"package": true, "return": true, "true": true, "var": true, "void": true,
	"while": true,
}

func checkName(kind, path, name string) error {
	switch {
	case name == "":
		return fmt.Errorf("%s: a %s name cannot be empty", path, kind)
	case name == "_":
		return fmt.Errorf("%s: _ is not a usable %s name", path, kind)
	case !identifier.MatchString(name):
		return fmt.Errorf("%s: %q is not a usable %s name; expressions select it by name, so it must be letters, digits and underscores, starting with a letter or underscore", path, name, kind)
	case celReserved[name]:
		return fmt.Errorf("%s: %q is reserved in CEL, so no expression could name it", path, name)
	}
	return nil
}

// bundleID is what launchd will take as a label and what install will use as a
// plist filename.
var bundleID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// checkAppName keeps app.name a single path component. install joins it onto
// ~/Applications and uninstall calls RemoveAll on the result, so a name that
// walks out of that directory deletes somewhere else entirely.
func checkAppName(name string) error {
	switch {
	case strings.ContainsAny(name, `/\`):
		return fmt.Errorf("app.name: %q cannot contain a path separator; it names a bundle in ~/Applications", name)
	case name == "." || name == "..":
		return fmt.Errorf("app.name: %q is not a name", name)
	case strings.HasPrefix(name, "."):
		return fmt.Errorf("app.name: %q cannot start with a dot", name)
	}
	return nil
}

func checkAppID(id string) error {
	if !bundleID.MatchString(id) {
		return fmt.Errorf("app.id: %q is not a bundle identifier; launchd takes it as a label and install as a plist filename, so it must be letters, digits, dots, dashes and underscores, e.g. dev.example.menubar", id)
	}
	return nil
}

// checkLabel is what launchd will take as a service label. It is the same
// grammar app.id follows, checked separately because the two name different
// things: one is the widget, the other is what the widget watches.
func checkLabel(path, label string) error {
	if !bundleID.MatchString(label) {
		return fmt.Errorf("%s.launchagent: %q is not a launchd label; a label is letters, digits, dots, dashes and underscores, e.g. dev.example.worker", path, label)
	}
	return nil
}
