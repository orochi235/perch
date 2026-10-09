package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/orochi235/perch/v2/internal/project"
	"github.com/orochi235/perch/v2/internal/schema"
	"github.com/orochi235/perch/v2/internal/spec"
	"gopkg.in/yaml.v3"
)

const schemaFile = "menubar.schema.json"

const starter = `# yaml-language-server: $schema=./%s
#
# %s's menu bar widget. ` + "`perch run`" + ` puts it in the foreground to look at;
# ` + "`perch install`" + ` builds it and hands it to launchd. The guide and the schema
# reference are at https://michaelbaker.tech/perch/.

app:
  name: %s
  id: %s
  icon: circle
  interval: 30s

menu:
  - text: %s
`

var notInID = regexp.MustCompile(`[^a-z0-9.-]+`)

// defaultID turns a directory name into a bundle id spec.Parse accepts.
func defaultID(name string) string {
	part := strings.Trim(notInID.ReplaceAllString(strings.ToLower(name), "-"), "-.")
	if part == "" {
		part = "app"
	}
	return "dev." + part + ".menubar"
}

func runInit(args []string, e *env) error {
	fs, dir := flags("init", e)
	name := fs.String("name", "", "app.name (defaults to the directory's name)")
	id := fs.String("id", "", "app.id, the bundle id and LaunchAgent label (defaults to dev.<name>.menubar)")
	if err := parse(fs, args); err != nil {
		return err
	}
	root, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}
	if *name == "" {
		*name = filepath.Base(root)
	}
	if *id == "" {
		*id = defaultID(*name)
	}

	specPath := filepath.Join(root, project.SpecFile)
	if _, err := os.Stat(specPath); err == nil {
		return fmt.Errorf("%s already exists", specPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	body := fmt.Sprintf(starter, schemaFile, *name, yamlScalar(*name), yamlScalar(*id), yamlScalar(*name))
	if _, err := spec.Parse([]byte(body)); err != nil {
		return fmt.Errorf("%w; pick another with -name or -id", err)
	}
	if err := os.WriteFile(specPath, []byte(body), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, schemaFile), []byte(schema.JSON()), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(e.out, "  %s\n  %s\nperch run to see it\n", project.SpecFile, schemaFile)
	return nil
}

// yamlScalar quotes a value only when it would not read back as the same string.
func yamlScalar(s string) string {
	var back any
	if yaml.Unmarshal([]byte("v: "+s), &struct{ V *any }{&back}) == nil && back == s {
		return s
	}
	return fmt.Sprintf("%q", s)
}
