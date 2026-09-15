# Templates, outlets and verb defaults — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status: not started.**

**Goal:** Let a `menubar.yaml` reuse a block of watches, states, status rules and menu items through `use:`, ship a `service` template for LaunchAgents, and give `agent:` items a default label and guard — as perch 2.0.

**Architecture:** Templates are expanded inside `spec.Parse`: each use's watches and states land on a new `Spec.Uses`, and its status rules and menu items are placed into the file's own lists at outlets, tagged with a `Scope`. Nothing downstream sees an outlet or a template. `celswift` binds a use's name, and `self` inside a template, to an object type; the backend emits each use as a Swift struct nested in `Results`, so `daemon.agent.pid` lowers to ordinary member access.

**Tech Stack:** Go (`gopkg.in/yaml.v3`, a small `${name}` expander, `embed`), CEL lowered to Swift at build time, `swiftc` for typecheck and previews.

**Spec:** [`docs/superpowers/specs/2026-09-14-templates-design.md`](../specs/2026-09-14-templates-design.md)

## Global Constraints

- **macOS only.** `swiftc` (Xcode Command Line Tools) is needed for the typecheck, golden-compile and preview tests.
- **Nothing evaluates CEL at runtime.** Every `when:`, `badge:` and `{{ }}` is lowered to Swift at build time; an expression perch cannot lower is a build error.
- **Unknown keys are rejected everywhere.** New structs go through `decodeStrict`; every field needs a `yaml:"..."` tag.
- **`internal/schema/schema.go` mirrors `spec.Parse`.** A new key in a `raw*` struct fails `internal/schema/drift_test.go` until the schema declares it, so the task adding the key updates the schema too.
- **Goldens.** `go test ./internal/backend/swiftappkit/ -run TestGolden -update` rewrites them. Read the diff before committing: an unexplained change in a golden is a bug, not noise.
- **Test scope.** While iterating, run only the package a task touches. The full suite runs once, in Task 11, as the pre-push gate — never while another session is running one (`ps aux | grep 'go test'`).
- **`${}` in a flow collection must be quoted.** `{launchagent: ${label}}` is a YAML syntax error, because `{` and `}` are flow indicators; write `{launchagent: "${label}"}`. Block style (`launchagent: ${label}`) needs no quotes.
- **Trap:** `~/.local/bin/perch` shadows `~/go/bin/perch`, and `~/go/bin` is not on PATH. `go install` alone changes nothing.

## File map

| File | Responsibility |
|---|---|
| `internal/templates/templates.go`, `service.yaml` | Create. The templates perch ships, embedded. |
| `internal/spec/template_source.go` | Create. `TemplateSource`, `TemplatesIn(dir)`: repo templates beside shipped ones. |
| `internal/spec/use.go` | Create. `Use`, `parseUses`, template expansion, parameters, fragments, outlet placement, `AllWatches`, `Watch.Key`. |
| `internal/spec/verbs.go` | Create. Verb defaults. |
| `internal/spec/spec.go` | Modify. `ParseWith`, `rawSpec.Use`, `Spec.Uses`, parse order. |
| `internal/spec/state.go`, `status.go`, `menu.go`, `validate.go`, `quit.go`, `watch.go` | Modify. `checkStates`; outlet marks; `Scope`; `agent:` paths; `AgentWatch`. |
| `internal/project/project.go` | Modify. Repo templates directory. |
| `internal/celswift/env.go`, `lower.go` | Modify. `WithUses`, `ForUse`, `ForUseStates`; the `self` diagnostic. |
| `internal/backend/swiftappkit/shapes.go`, `render.go`, `main.go`, `preview.go` | Modify. Use structs, scoped environments, scoped `agent:`. |
| `internal/preview/preview.go` | Modify. State fences for a use's watches. |
| `internal/schema/schema.go`, `drift_test.go`, `cmd/perch/main.go` | Modify. `use`, outlets, `agent:` paths, template schema, `perch schema -template`. |
| `internal/site/nav.go`, `fences.go`, `docs/schema.md`, `docs/recipes/launchagent.md` | Modify. Docs. |
| `go.mod`, every `.go` import, `README.md`, `docs/guide/*.md` | Modify. Module path `/v2`. |

---

### Task 1: Template lookup and the shipped `service` template

**Files:**
- Create: `internal/templates/templates.go`, `internal/templates/service.yaml`, `internal/spec/template_source.go`
- Modify: `internal/spec/spec.go`, `internal/project/project.go`
- Test: `internal/spec/template_source_test.go`

**Interfaces:**
- Produces: `templates.Source(name string) ([]byte, bool)`, `templates.Names() []string`; `spec.TemplateSource`; `spec.TemplatesIn(dir string) spec.TemplateSource`; `spec.ParseWith(src []byte, t spec.TemplateSource) (*spec.Spec, error)`; `(*project.Project).TemplatesDir() string`.

- [ ] **Step 1: Write the failing tests**

`internal/spec/template_source_test.go`:

```go
package spec

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestShippedTemplatesAreFoundByName(t *testing.T) {
	src, file, ok, err := TemplatesIn("").Template("service")
	if err != nil || !ok {
		t.Fatalf("service: ok = %v, err = %v", ok, err)
	}
	if !strings.Contains(string(src), "launchagent:") {
		t.Errorf("service.yaml does not declare a launchagent watch:\n%s", src)
	}
	if !strings.Contains(file, "service.yaml") {
		t.Errorf("file = %q, want it to name service.yaml", file)
	}
}

func TestARepoTemplateIsFoundBesideTheShippedOnes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "health.yaml")
	if err := os.WriteFile(path, []byte("watch: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, file, ok, err := TemplatesIn(dir).Template("health")
	if err != nil || !ok || file != path {
		t.Fatalf("health: file = %q, ok = %v, err = %v", file, ok, err)
	}
	names := TemplatesIn(dir).Names()
	if !slices.Contains(names, "health") || !slices.Contains(names, "service") {
		t.Errorf("Names() = %v, want both the repo's and perch's", names)
	}
}

// A reader of menubar.yaml could not tell which service runs, and a fix to the
// shipped one would silently never arrive.
func TestARepoTemplateCannotTakeAShippedName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "service.yaml"), []byte("watch: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := TemplatesIn(dir).Template("service")
	if err == nil || !strings.Contains(err.Error(), "perch ships") {
		t.Errorf("err = %v, want a refusal naming the shipped template", err)
	}
}

func TestAMissingTemplateIsNotAnError(t *testing.T) {
	_, _, ok, err := TemplatesIn(t.TempDir()).Template("nope")
	if ok || err != nil {
		t.Errorf("ok = %v, err = %v; want not found and no error", ok, err)
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/spec/ -run 'Template' -v`
Expected: FAIL to compile — `TemplatesIn` is undefined.

- [ ] **Step 3: Ship the template**

`internal/templates/service.yaml` — this is the spec's template verbatim. It does not parse as a use until Tasks 2–5 land; nothing reads it before then except the lookup test.

```yaml
# A LaunchAgent: whether it is installed, loaded and running, and the controls
# to run it. Copy it under another name to change its labels.
params:
  label: ~          # the launchd label, e.g. dev.example.worker
  noun: Service     # what the readout calls it

watch:
  agent:
    launchagent: ${label}

state:
  - uninstalled: "!self.agent.installed"
  - stopped: "!self.agent.loaded"
  - idle: "!self.agent.running"
  - running:

status:
  default:
    - {when: self.uninstalled, icon: exclamationmark.triangle}

menu:
  default:
    - {text: "${noun}: not installed", when: self.uninstalled}
    - {text: "${noun}: not loaded", when: self.stopped}
    - {text: "${noun}: loaded, not running", when: self.idle}
    - {text: "${noun}: running · pid {{self.agent.pid}}", when: self.running}
  controls:
    - agent: self.agent.start
    - agent: self.agent.restart
    - agent: self.agent.stop
```

`internal/templates/templates.go`:

```go
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
```

- [ ] **Step 4: Write the lookup**

`internal/spec/template_source.go`:

```go
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
```

In `internal/spec/spec.go`, replace the `Parse` signature line and its doc comment with:

```go
// Parse reads a menubar.yaml document into a Spec, with perch's shipped
// templates and no repo's.
func Parse(src []byte) (*Spec, error) { return ParseWith(src, TemplatesIn("")) }

// ParseWith reads a menubar.yaml document into a Spec, finding the templates its
// use: block names in templates.
func ParseWith(src []byte, templates TemplateSource) (*Spec, error) {
```

(The body that followed `func Parse(src []byte) (*Spec, error) {` is unchanged and now belongs to `ParseWith`. `templates` is unused until Task 2; a parameter unused is not a compile error.)

In `internal/project/project.go`, add below `SourcesDir`:

```go
// TemplatesDir holds a repo's own templates, which use: finds beside perch's.
func (p *Project) TemplatesDir() string { return filepath.Join(p.Root, "menubar", "templates") }
```

and in `Load` replace `s, err := spec.Parse(src)` with:

```go
	s, err := spec.ParseWith(src, spec.TemplatesIn(p.TemplatesDir()))
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/spec/ ./internal/project/ ./internal/templates/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/templates internal/spec/template_source.go internal/spec/template_source_test.go internal/spec/spec.go internal/project/project.go
git commit -m "look templates up in the repo and in perch, and ship service"
```

---

### Task 2: `use:` expands a template's watches and states

> **Landed differently:** parameters are braces-only, filled by a small expander in internal/spec/params.go rather than os.Expand; `$${` is the only escape. See the spec's Parameters section.

**Files:**
- Create: `internal/spec/use.go`
- Modify: `internal/spec/spec.go`, `internal/spec/state.go`, `internal/spec/validate.go`, `internal/spec/watch.go`, `internal/schema/schema.go`
- Test: `internal/spec/use_test.go`

**Interfaces:**
- Consumes: `TemplateSource` (Task 1).
- Produces: `spec.Use{Name, Template, File string; Watches []Watch; States []State}`; `Spec.Uses []Use`; `Watch.Scope string`; `checkStates(states []State, watches []Watch, uses []Use, where string) error`.

- [ ] **Step 1: Write the failing tests**

`internal/spec/use_test.go`:

```go
package spec

import (
	"slices"
	"strings"
	"testing"
)

// fakeTemplates stands in for a repo's menubar/templates/ and perch's own.
type fakeTemplates map[string]string

func (f fakeTemplates) Template(name string) ([]byte, string, bool, error) {
	src, ok := f[name]
	return []byte(src), "templates/" + name + ".yaml", ok, nil
}

func (f fakeTemplates) Names() []string {
	var out []string
	for name := range f {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

const useHead = `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
`

const agentTemplate = `
params:
  label: ~
  noun: Service
watch:
  agent:
    launchagent: ${label}
state:
  - stopped: "!self.agent.loaded"
  - running:
`

func parseWith(t *testing.T, doc string, src fakeTemplates) *Spec {
	t.Helper()
	s, err := ParseWith([]byte(doc), src)
	if err != nil {
		t.Fatalf("ParseWith: %v", err)
	}
	return s
}

func TestAUseGetsItsOwnWatchesAndStates(t *testing.T) {
	s := parseWith(t, useHead+`
use:
  daemon:
    svc: {label: dev.example.daemon}
menu: [{text: Quit, quit: true}]
`, fakeTemplates{"svc": agentTemplate})
	if len(s.Uses) != 1 {
		t.Fatalf("Uses = %+v, want one", s.Uses)
	}
	u := s.Uses[0]
	if u.Name != "daemon" || u.Template != "svc" || u.File != "templates/svc.yaml" {
		t.Errorf("use = %+v", u)
	}
	if len(u.Watches) != 1 || u.Watches[0].Label != "dev.example.daemon" || u.Watches[0].Scope != "daemon" {
		t.Errorf("watches = %+v, want agent on dev.example.daemon scoped to daemon", u.Watches)
	}
	if len(u.States) != 2 || u.States[0].Cond != "!self.agent.loaded" {
		t.Errorf("states = %+v", u.States)
	}
	if len(s.Watches) != 0 {
		t.Errorf("a use's watches leaked into the file's: %+v", s.Watches)
	}
}

func TestParametersFillEveryStringInATemplate(t *testing.T) {
	src := fakeTemplates{"echo": `
params:
  msg: ~
  decode: "false"
watch:
  w:
    run: [echo, "${msg}", "cost $$5"]
    json: ${decode}
`}
	s := parseWith(t, useHead+`
use:
  e:
    echo: {msg: "a: b", decode: "true"}
menu: [{text: Quit, quit: true}]
`, src)
	w := s.Uses[0].Watches[0]
	if !slices.Equal(w.Run, []string{"echo", "a: b", "cost $5"}) {
		t.Errorf("run = %q; a value holding \": \" has to stay one string, and $$ is a literal $", w.Run)
	}
	if !w.JSON {
		t.Error("json: ${decode} did not read back as a bool")
	}
}

func TestADefaultParameterIsUsedWhenAUsePassesNone(t *testing.T) {
	src := fakeTemplates{"echo": "params:\n  msg: hello\nwatch:\n  w: {run: [echo, \"${msg}\"]}\n"}
	s := parseWith(t, useHead+"use:\n  e:\n    echo:\nmenu: [{text: Quit, quit: true}]\n", src)
	if got := s.Uses[0].Watches[0].Run[1]; got != "hello" {
		t.Errorf("run[1] = %q, want the default", got)
	}
}

func TestUseRefusals(t *testing.T) {
	src := fakeTemplates{
		"svc":     agentTemplate,
		"nested":  "app: {name: x}\n",
		"dollar":  "watch:\n  w: {run: [echo, $1]}\n",
		"unknown": "watch:\n  w: {run: [echo, \"${nope}\"]}\n",
		"badwatch": "watch:\n  w: {run: []}\n",
		"badstate": "watch:\n  w: {exists: /tmp}\nstate:\n  - a:\n  - b: \"w.ok\"\n",
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"an unknown template": {
			"use:\n  d:\n    nope: {}\n",
			`no template named "nope"; the templates are badstate, badwatch, dollar, nested, svc, unknown`,
		},
		"a missing required parameter": {
			"use:\n  d:\n    svc: {}\n",
			"requires label",
		},
		"an argument the template does not take": {
			"use:\n  d:\n    svc: {label: dev.example.d, color: red}\n",
			`"color" is not a parameter of this template; it takes label, noun`,
		},
		"a template that nests": {
			"use:\n  d:\n    nested:\n",
			"templates do not nest",
		},
		"a bare $ in a template": {
			"use:\n  d:\n    dollar:\n",
			"write $$ for a literal $",
		},
		"an unknown ${x}": {
			"use:\n  d:\n    unknown:\n",
			"${nope} is not a parameter",
		},
		"a use named self": {
			"use:\n  self:\n    svc: {label: dev.example.d}\n",
			"bound by perch itself",
		},
		"a use naming two templates": {
			"use:\n  d: {svc: {label: dev.example.d}, dollar: {}}\n",
			"exactly one template",
		},
		"a use named like a watch": {
			"use:\n  d:\n    svc: {label: dev.example.d}\nwatch:\n  d: {exists: /tmp}\n",
			"already a watch",
		},
		"a state named like a use": {
			"use:\n  d:\n    svc: {label: dev.example.d}\nstate:\n  - d: \"true\"\n  - b:\n",
			"already a use",
		},
		"a template's own watch is invalid": {
			"use:\n  d:\n    badwatch:\n",
			"use.d (templates/badwatch.yaml): watch.w.run: empty",
		},
		"a template's state list is out of order": {
			"use:\n  d:\n    badstate:\n",
			"use.d (templates/badstate.yaml): state[0]",
		},
		"a watch named self": {
			"watch:\n  self: {exists: /tmp}\n",
			"bound by perch itself",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseWith([]byte(useHead+tc.doc+"menu: [{text: Quit, quit: true}]\n"), src)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/spec/ -run 'Use|Parameter' -v`
Expected: FAIL to compile — `Spec.Uses`, `Watch.Scope` are undefined.

- [ ] **Step 3: Add the types and the block**

In `internal/spec/watch.go`, add to `Watch` after `Shape *Type`:

```go
	// Scope is the use this watch belongs to, or "" for the file's own.
	Scope string
```

In `internal/spec/spec.go`, add a last field to `Spec` and to `rawSpec`:

```go
	Uses []Use // in Spec
```

```go
	Use yaml.Node `yaml:"use"` // in rawSpec
```

In `ParseWith`, directly after `decodeStrict(root, &raw, "")` succeeds and before `parseIcon`, insert:

```go
	uses, err := parseUses(&raw.Use, templates)
	if err != nil {
		return nil, err
	}
```

and after `s.Menu = menu`, before `s.validate()`:

```go
	s.Uses = uses
```

`internal/spec/use.go`:

```go
package spec

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Use is one use of a template: its own watches and its own ordered states,
// reached in expressions by Name, and as self from inside the template.
type Use struct {
	Name     string
	Template string
	File     string // where the template came from, for errors
	Watches  []Watch
	States   []State
}

type rawTemplate struct {
	Params yaml.Node `yaml:"params"`
	Watch  yaml.Node `yaml:"watch"`
	State  yaml.Node `yaml:"state"`
	Status yaml.Node `yaml:"status"`
	Menu   yaml.Node `yaml:"menu"`
}

// nested are the top-level keys a menubar.yaml takes and a template does not.
var nested = []string{"app", "window", "use"}

func parseUses(n *yaml.Node, src TemplateSource) ([]Use, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("use: want a mapping of names to templates, e.g. daemon: {service: {label: dev.example.daemon}}")
	}
	var out []Use
	for i := 0; i+1 < len(n.Content); i += 2 {
		name, body := n.Content[i].Value, n.Content[i+1]
		path := "use." + name
		if err := checkName("use", path, name); err != nil {
			return nil, err
		}
		if err := checkBound(path, name, "use"); err != nil {
			return nil, err
		}
		if body.Kind != yaml.MappingNode || len(body.Content) != 2 {
			return nil, fmt.Errorf("%s: want exactly one template and its arguments, e.g. service: {label: dev.example.daemon}", path)
		}
		u, err := expandUse(name, body.Content[0].Value, body.Content[1], src)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

// checkBound refuses the two names perch binds itself: it inside an each:, and
// self inside a template.
func checkBound(path, name, kind string) error {
	if name == "self" || name == "it" {
		return fmt.Errorf("%s: %q is bound by perch itself, so a %s cannot take that name", path, name, kind)
	}
	return nil
}

func expandUse(name, tmpl string, args *yaml.Node, src TemplateSource) (Use, error) {
	path := "use." + name
	if err := checkName("template", path, tmpl); err != nil {
		return Use{}, err
	}
	body, file, ok, err := src.Template(tmpl)
	if err != nil {
		return Use{}, fmt.Errorf("%s: %w", path, err)
	}
	if !ok {
		return Use{}, fmt.Errorf("%s: no template named %q; the templates are %s", path, tmpl, strings.Join(src.Names(), ", "))
	}
	where := fmt.Sprintf("%s (%s)", path, file)

	var doc yaml.Node
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	if len(doc.Content) == 0 {
		return Use{}, fmt.Errorf("%s: the template is empty", where)
	}
	root, err := resolveAliases(doc.Content[0])
	if err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	if err := refuseNesting(root, where); err != nil {
		return Use{}, err
	}
	var raw rawTemplate
	if err := decodeStrict(root, &raw, where); err != nil {
		return Use{}, err
	}
	values, err := bindParams(&raw.Params, args, path, where)
	if err != nil {
		return Use{}, err
	}
	for _, n := range []*yaml.Node{&raw.Watch, &raw.State, &raw.Status, &raw.Menu} {
		if err := substitute(n, values, where); err != nil {
			return Use{}, err
		}
	}

	u := Use{Name: name, Template: tmpl, File: file}
	if u.Watches, err = parseWatches(&raw.Watch); err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	for i := range u.Watches {
		u.Watches[i].Scope = name
	}
	if u.States, err = parseStates(&raw.State); err != nil {
		return Use{}, fmt.Errorf("%s: %w", where, err)
	}
	return u, nil
}

func refuseNesting(root *yaml.Node, where string) error {
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: want a mapping of params, watch, state, status and menu", where)
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if key := root.Content[i].Value; slices.Contains(nested, key) {
			return fmt.Errorf("%s: %s: belongs to a menubar.yaml; a template takes params, watch, state, status and menu, and templates do not nest", where, key)
		}
	}
	return nil
}

// bindParams pairs what a template declares with what a use passes. A param
// declared with ~ is required; anything else is its default.
func bindParams(declared, args *yaml.Node, path, where string) (map[string]string, error) {
	values := map[string]string{}
	required := map[string]bool{}
	if declared.Kind != 0 {
		if declared.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s: params: want a mapping of names to defaults, with ~ for a required one", where)
		}
		for i := 0; i+1 < len(declared.Content); i += 2 {
			name, def := declared.Content[i].Value, declared.Content[i+1]
			if err := checkName("param", where+": params", name); err != nil {
				return nil, err
			}
			switch {
			case def.Kind == yaml.ScalarNode && def.Tag == "!!null":
				required[name] = true
			case def.Kind == yaml.ScalarNode:
				values[name] = def.Value
			default:
				return nil, fmt.Errorf("%s: params.%s: want a default as a string, or ~ to make it required", where, name)
			}
		}
	}
	takes := make([]string, 0, len(values)+len(required))
	for name := range values {
		takes = append(takes, name)
	}
	for name := range required {
		takes = append(takes, name)
	}
	sort.Strings(takes)

	if args.Kind == yaml.ScalarNode && args.Tag == "!!null" {
		args = &yaml.Node{Kind: yaml.MappingNode}
	}
	if args.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: want the template's arguments as a mapping", path)
	}
	for i := 0; i+1 < len(args.Content); i += 2 {
		name, v := args.Content[i].Value, args.Content[i+1]
		if !slices.Contains(takes, name) {
			return nil, fmt.Errorf("%s: %q is not a parameter of this template; it takes %s", path, name, strings.Join(takes, ", "))
		}
		if v.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%s: %s: want a string", path, name)
		}
		values[name] = v.Value
		delete(required, name)
	}
	if len(required) > 0 {
		var missing []string
		for name := range required {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("%s: the template requires %s, which this use does not pass", path, strings.Join(missing, ", "))
	}
	return values, nil
}

// substitute fills ${name} into every string value of a parsed template, so a
// value holding ": " or a newline stays one string and cannot change the file's
// structure. Keys are left alone: they are names the template declares.
func substitute(n *yaml.Node, values map[string]string, where string) error {
	if n == nil || n.Kind == 0 {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if !strings.Contains(n.Value, "$") {
			return nil
		}
		var unknown []string
		out := os.Expand(n.Value, func(name string) string {
			if name == "$" {
				return "$"
			}
			v, ok := values[name]
			if !ok {
				unknown = append(unknown, name)
			}
			return v
		})
		if len(unknown) > 0 {
			return unknownParam(unknown[0], values, where)
		}
		n.Value = out
		// An unquoted scalar is resolved again, so json: ${decode} still reads
		// as a bool once it holds "true".
		if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) == 0 {
			n.Tag = ""
		}
	case yaml.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			if err := substitute(n.Content[i], values, where); err != nil {
				return err
			}
		}
	default:
		for _, c := range n.Content {
			if err := substitute(c, values, where); err != nil {
				return err
			}
		}
	}
	return nil
}

func unknownParam(name string, values map[string]string, where string) error {
	if !identifier.MatchString(name) {
		return fmt.Errorf("%s: $%s is not a parameter; write $$ for a literal $", where, name)
	}
	names := make([]string, 0, len(values))
	for k := range values {
		names = append(names, k)
	}
	sort.Strings(names)
	takes := "no parameters"
	if len(names) > 0 {
		takes = strings.Join(names, ", ")
	}
	return fmt.Errorf("%s: ${%s} is not a parameter of this template; it takes %s", where, name, takes)
}
```

- [ ] **Step 4: Validate uses, and share the state rules**

In `internal/spec/state.go`, replace `validateStates` whole with:

```go
// validateStates checks what can be checked without reading an expression. A
// condition naming a state declared after it needs no check of its own: the
// backend declares states in order, so the name is simply not bound yet.
func (s *Spec) validateStates() error { return checkStates(s.States, s.Watches, s.Uses, "") }

// checkStates holds one ordered list to its rules. where prefixes every error,
// and is empty for the file's own list.
func checkStates(states []State, watches []Watch, uses []Use, where string) error {
	if len(states) == 0 {
		return nil
	}
	taken := map[string]string{}
	for _, w := range watches {
		taken[w.Name] = "a watch"
	}
	for _, u := range uses {
		taken[u.Name] = "a use"
	}
	seen := map[string]bool{}
	for i, st := range states {
		path := fmt.Sprintf("%sstate[%d]", where, i)
		if err := checkName("state", path, st.Name); err != nil {
			return err
		}
		if err := checkBound(path, st.Name, "state"); err != nil {
			return err
		}
		if seen[st.Name] {
			return fmt.Errorf("%s: %q declared twice", path, st.Name)
		}
		seen[st.Name] = true
		if kind, ok := taken[st.Name]; ok {
			return fmt.Errorf("%s: %q is already %s; an expression names states, watches and uses the same way, so it could not tell them apart", path, st.Name, kind)
		}
		if last := i == len(states)-1; last {
			if st.Cond != "" {
				return fmt.Errorf("%s: %q is last, so it is the fallback and takes no condition; without one state that always holds, a poll can match none of them", path, st.Name)
			}
		} else if st.Cond == "" {
			return fmt.Errorf("%s: %q has no condition, so it always holds and must be last; %d state(s) after it can never be reached", path, st.Name, len(states)-1-i)
		}
	}
	return nil
}
```

The existing `TestStatesRefused` cases still pass: `"already a watch"` is still in the message, and `"cannot take that name"` becomes `"bound by perch itself"` — update that one case's `want` in `internal/spec/state_test.go` to `"bound by perch itself"`.

In `internal/spec/validate.go`, `Watch.validate`: replace the `if w.Name == "it" { ... }` block with:

```go
	if err := checkBound(path, w.Name, "watch"); err != nil {
		return err
	}
```

Search for other tests expecting the old `it` wording: `grep -rn 'cannot take that name' internal` — change each `want` to `"bound by perch itself"`.

In `(s *Spec) validate()`, replace the watch loop and the `validateStates` call with:

```go
	seen := map[string]bool{}
	for _, w := range s.Watches {
		if seen[w.Name] {
			return fmt.Errorf("watch.%s: declared twice", w.Name)
		}
		seen[w.Name] = true
		if err := w.validate(); err != nil {
			return err
		}
	}
	for _, u := range s.Uses {
		if seen[u.Name] {
			return fmt.Errorf("use.%s: %q is already a watch; an expression names watches and uses the same way, so it could not tell them apart", u.Name, u.Name)
		}
		where := fmt.Sprintf("use.%s (%s): ", u.Name, u.File)
		for _, w := range u.Watches {
			if err := w.validate(); err != nil {
				return fmt.Errorf("%s%w", where, err)
			}
		}
		if err := checkStates(u.States, u.Watches, nil, where); err != nil {
			return err
		}
	}
	if err := s.validateStates(); err != nil {
		return err
	}
```

- [ ] **Step 5: Declare `use` in the schema**

`internal/schema/drift_test.go` now fails: `rawSpec` has a `use` key the schema does not. In `internal/schema/schema.go`, add this property between `"state"` and `"status"`:

```json
    "use": {
      "type": "object",
      "description": "Named uses of templates. Each takes exactly one template, and its arguments.",
      "propertyNames": {"pattern": "^[A-Za-z_][A-Za-z0-9_]*$", "not": {"enum": ["it", "self", "as", "break", "const", "continue", "else", "false", "for", "function", "if", "import", "in", "let", "loop", "namespace", "null", "package", "return", "true", "var", "void", "while"]}},
      "additionalProperties": {
        "type": "object",
        "minProperties": 1,
        "maxProperties": 1,
        "additionalProperties": {"type": ["object", "null"], "additionalProperties": {"type": ["string", "number", "boolean"]}}
      }
    },
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/spec/ ./internal/schema/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/spec internal/schema/schema.go
git commit -m "expand a template's watches and states under use:"
```

---

### Task 3: Outlets place a template's status rules and menu items

**Files:**
- Modify: `internal/spec/use.go`, `internal/spec/menu.go`, `internal/spec/status.go`, `internal/spec/spec.go`
- Test: `internal/spec/outlet_test.go`

**Interfaces:**
- Consumes: `Use`, `expandUse` (Task 2).
- Produces: `Item.Scope string`, `StatusRule.Scope string`; outlet marks `- outlet` / `- outlet: <name>` in `menu:` and `status:`; `placeMenu`, `placeStatus`.

Templates in this task's tests carry no `agent:` items: `self.agent.start` does not parse until Task 4.

- [ ] **Step 1: Write the failing tests**

`internal/spec/outlet_test.go`:

```go
package spec

import (
	"slices"
	"strings"
	"testing"
)

const fragTemplate = `
params:
  noun: thing
watch:
  w: {exists: /tmp}
status:
  default:
    - {when: "!self.w.ok", dim: true}
menu:
  default:
    - {text: "${noun} readout", menu: [{text: "${noun} detail"}]}
  controls:
    - {text: "${noun} control"}
`

var frags = fakeTemplates{"frag": fragTemplate}

const twoUses = `
use:
  a:
    frag: {noun: A}
  b:
    frag: {noun: B}
`

func titles(items []Item) []string {
	var out []string
	for _, it := range items {
		if it.Separator {
			out = append(out, "-")
			continue
		}
		out = append(out, it.Text)
	}
	return out
}

func TestUseFragmentsLandAtTheirOutlets(t *testing.T) {
	s := parseWith(t, useHead+twoUses+`
menu:
  - {text: top}
  - outlet
  - separator
  - outlet: controls
  - {text: Quit, quit: true}
`, frags)
	want := []string{"top", "A readout", "B readout", "-", "A control", "B control", "Quit"}
	if got := titles(s.Menu); !slices.Equal(got, want) {
		t.Errorf("menu = %q, want %q", got, want)
	}
	if s.Menu[1].Scope != "a" || s.Menu[2].Scope != "b" || s.Menu[0].Scope != "" {
		t.Errorf("scopes = %q %q %q", s.Menu[0].Scope, s.Menu[1].Scope, s.Menu[2].Scope)
	}
	if sub := s.Menu[1].Menu; len(sub) != 1 || sub[0].Scope != "a" {
		t.Errorf("a submenu item did not keep its use: %+v", sub)
	}
}

// With no default declared, the default outlet is the end of menu: and the
// start of status: — the start, because a rule after the file's when:-less
// rule could never match.
func TestTheDefaultOutletHasAPlaceWhenTheFileDeclaresNone(t *testing.T) {
	s := parseWith(t, useHead+twoUses+`
status:
  - badge: "\"x\""
menu:
  - outlet: controls
  - {text: Quit, quit: true}
`, frags)
	want := []string{"A control", "B control", "Quit", "A readout", "B readout"}
	if got := titles(s.Menu); !slices.Equal(got, want) {
		t.Errorf("menu = %q, want %q", got, want)
	}
	if len(s.Status) != 3 || s.Status[0].Scope != "a" || s.Status[1].Scope != "b" || s.Status[2].Badge == "" {
		t.Errorf("status = %+v, want the two uses' rules ahead of the file's", s.Status)
	}
}

func TestAFragmentForAnUndeclaredOutletGoesToTheDefault(t *testing.T) {
	s := parseWith(t, useHead+"use:\n  a:\n    frag: {noun: A}\nmenu:\n  - outlet\n  - {text: Quit, quit: true}\n", frags)
	want := []string{"A readout", "A control", "Quit"}
	if got := titles(s.Menu); !slices.Equal(got, want) {
		t.Errorf("menu = %q, want %q", got, want)
	}
}

func TestAnOutletCanSitInASubmenu(t *testing.T) {
	s := parseWith(t, useHead+"use:\n  a:\n    frag: {noun: A}\nmenu:\n  - {text: More, menu: [outlet]}\n  - {text: Quit, quit: true}\n", frags)
	if got := titles(s.Menu[0].Menu); !slices.Equal(got, []string{"A readout", "A control"}) {
		t.Errorf("submenu = %q", got)
	}
}

func TestOutletRefusals(t *testing.T) {
	src := fakeTemplates{
		"frag":       fragTemplate,
		"marks":      "menu:\n  default:\n    - outlet\n",
		"catchall":   "status:\n  default:\n    - {dim: true}\n",
		"listmenu":   "menu:\n  - {text: x}\n",
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"an outlet no use fills": {
			"use:\n  a:\n    frag:\nmenu:\n  - outlet: nope\n",
			`no use fills the outlet "nope"`,
		},
		"an outlet declared twice": {
			"use:\n  a:\n    frag:\nmenu:\n  - outlet\n  - outlet\n",
			"the default outlet is declared twice",
		},
		"a template that declares an outlet": {
			"use:\n  a:\n    marks:\nmenu:\n  - {text: Quit, quit: true}\n",
			"a template fills outlets; it cannot declare one",
		},
		"a template status rule with no when": {
			"use:\n  a:\n    catchall:\nmenu:\n  - {text: Quit, quit: true}\n",
			"shadow",
		},
		"a template menu written as a list": {
			"use:\n  a:\n    listmenu:\nmenu:\n  - {text: Quit, quit: true}\n",
			"maps outlet names to lists",
		},
		"an outlet with another key": {
			"menu:\n  - {outlet: x, text: y}\n",
			"takes nothing but its name",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseWith([]byte(useHead+tc.doc), src)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/spec/ -run 'Outlet|Fragment' -v`
Expected: FAIL — `Item.Scope` is undefined.

- [ ] **Step 3: Outlet marks in the file's lists**

In `internal/spec/menu.go`, add to `Item`:

```go
	// Scope is the use this item came from, or "" for the file's own.
	Scope string

	outlet   string // set with isOutlet: the outlet's name, "" for the default
	isOutlet bool
```

At the top of `parseItem`, before `if n.Kind == yaml.ScalarNode {`:

```go
	if name, ok, err := outletMark(n, path); err != nil {
		return Item{}, err
	} else if ok {
		return Item{outlet: name, isOutlet: true}, nil
	}
```

and add to `menu.go`:

```go
// outletMark reads `- outlet`, the default outlet, or `- outlet: <name>`.
func outletMark(n *yaml.Node, path string) (string, bool, error) {
	if n.Kind == yaml.ScalarNode && n.Value == "outlet" {
		return "", true, nil
	}
	if n.Kind != yaml.MappingNode || len(n.Content) < 2 || n.Content[0].Value != "outlet" {
		return "", false, nil
	}
	if len(n.Content) != 2 {
		return "", false, fmt.Errorf("%s: an outlet takes nothing but its name", path)
	}
	v := n.Content[1]
	if v.Kind == yaml.ScalarNode && v.Tag == "!!null" {
		return "", true, nil
	}
	if v.Kind != yaml.ScalarNode {
		return "", false, fmt.Errorf("%s: an outlet's name is a string", path)
	}
	if err := checkName("outlet", path, v.Value); err != nil {
		return "", false, err
	}
	return v.Value, true, nil
}
```

The bare-scalar error in `parseItem` becomes: `"%s: bare %q is not a menu item; only 'separator' and 'outlet' are"`.

In `internal/spec/status.go`, add to `StatusRule`:

```go
	// Scope is the use this rule came from, or "" for the file's own.
	Scope string

	outlet   string
	isOutlet bool
```

and at the top of the loop body in `parseStatus`, after `path := ...`:

```go
		if name, ok, err := outletMark(c, path); err != nil {
			return nil, err
		} else if ok {
			out = append(out, StatusRule{outlet: name, isOutlet: true})
			continue
		}
```

- [ ] **Step 4: Fragments and placement**

In `internal/spec/use.go`, add to `Use`:

```go
	status []statusFragment
	menu   []menuFragment
```

In `expandUse`, before `return u, nil`:

```go
	if u.status, err = parseStatusFragments(&raw.Status, where); err != nil {
		return Use{}, err
	}
	if u.menu, err = parseMenuFragments(&raw.Menu, where); err != nil {
		return Use{}, err
	}
```

Append to `use.go`:

```go
// menuFragment is what one template puts at one outlet. "" is the default.
type menuFragment struct {
	outlet string
	items  []Item
}

type statusFragment struct {
	outlet string
	rules  []StatusRule
}

// outletKey is how a template names the default outlet.
func outletKey(key string) string {
	if key == "default" {
		return ""
	}
	return key
}

func outletLabel(name string) string {
	if name == "" {
		return "the default outlet"
	}
	return fmt.Sprintf("the outlet %q", name)
}

func parseMenuFragments(n *yaml.Node, where string) ([]menuFragment, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: menu: in a template, menu: maps outlet names to lists of items, e.g. default: [...]", where)
	}
	var out []menuFragment
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		path := "menu." + key
		if err := checkName("outlet", path, key); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		items, err := parseMenu(n.Content[i+1], path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		if err := refuseMenuMarks(items, path); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
		out = append(out, menuFragment{outlet: outletKey(key), items: items})
	}
	return out, nil
}

func refuseMenuMarks(items []Item, path string) error {
	for i, it := range items {
		if it.isOutlet {
			return fmt.Errorf("%s[%d]: a template fills outlets; it cannot declare one", path, i)
		}
		if err := refuseMenuMarks(it.Menu, fmt.Sprintf("%s[%d].menu", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func parseStatusFragments(n *yaml.Node, where string) ([]statusFragment, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: status: in a template, status: maps outlet names to lists of rules, e.g. default: [...]", where)
	}
	var out []statusFragment
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		if err := checkName("outlet", where+": status."+key, key); err != nil {
			return nil, err
		}
		rules, err := parseStatus(n.Content[i+1])
		if err != nil {
			return nil, fmt.Errorf("%s: status.%s: %w", where, key, err)
		}
		for j, r := range rules {
			switch {
			case r.isOutlet:
				return nil, fmt.Errorf("%s: status.%s[%d]: a template fills outlets; it cannot declare one", where, key, j)
			case r.When == "":
				return nil, fmt.Errorf("%s: status.%s[%d]: a rule with no when: would land ahead of the file's own rules and shadow every one of them", where, key, j)
			}
		}
		out = append(out, statusFragment{outlet: outletKey(key), rules: rules})
	}
	return out, nil
}

// landing is where a fragment goes: its own outlet if the file declares it,
// the default outlet otherwise.
func landing(outlet string, declared map[string]bool) string {
	if declared[outlet] {
		return outlet
	}
	return ""
}

func unfilled(declared map[string]bool, filled func(string) bool, list string) error {
	var names []string
	for name := range declared {
		if name != "" && !filled(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		return fmt.Errorf("%s: no use fills the outlet %q", list, names[0])
	}
	return nil
}

// placeMenu puts every use's menu fragments at the outlets the file declares.
func placeMenu(items []Item, uses []Use) ([]Item, error) {
	declared := map[string]bool{}
	if err := collectMenuOutlets(items, "menu", declared); err != nil {
		return nil, err
	}
	fill := func(outlet string) []Item {
		var out []Item
		for _, u := range uses {
			for _, f := range u.menu {
				if landing(f.outlet, declared) == outlet {
					out = append(out, scopeItems(f.items, u.Name)...)
				}
			}
		}
		return out
	}
	if err := unfilled(declared, func(n string) bool { return len(fill(n)) > 0 }, "menu"); err != nil {
		return nil, err
	}
	placed := expandMenuOutlets(items, fill)
	if !declared[""] {
		placed = append(placed, fill("")...)
	}
	return placed, nil
}

func collectMenuOutlets(items []Item, path string, declared map[string]bool) error {
	for i, it := range items {
		p := fmt.Sprintf("%s[%d]", path, i)
		if it.isOutlet {
			if declared[it.outlet] {
				return fmt.Errorf("%s: %s is declared twice", p, outletLabel(it.outlet))
			}
			declared[it.outlet] = true
			continue
		}
		if err := collectMenuOutlets(it.Menu, p+".menu", declared); err != nil {
			return err
		}
	}
	return nil
}

func expandMenuOutlets(items []Item, fill func(string) []Item) []Item {
	if items == nil {
		return nil
	}
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.isOutlet {
			out = append(out, fill(it.outlet)...)
			continue
		}
		it.Menu = expandMenuOutlets(it.Menu, fill)
		out = append(out, it)
	}
	return out
}

func scopeItems(items []Item, scope string) []Item {
	if items == nil {
		return nil
	}
	out := make([]Item, len(items))
	for i, it := range items {
		it.Scope = scope
		it.Menu = scopeItems(it.Menu, scope)
		out[i] = it
	}
	return out
}

// placeStatus is placeMenu for status rules. With no default declared, the
// default is the start of the list.
func placeStatus(rules []StatusRule, uses []Use) ([]StatusRule, error) {
	declared := map[string]bool{}
	for i, r := range rules {
		if !r.isOutlet {
			continue
		}
		if declared[r.outlet] {
			return nil, fmt.Errorf("status[%d]: %s is declared twice", i, outletLabel(r.outlet))
		}
		declared[r.outlet] = true
	}
	fill := func(outlet string) []StatusRule {
		var out []StatusRule
		for _, u := range uses {
			for _, f := range u.status {
				if landing(f.outlet, declared) != outlet {
					continue
				}
				for _, r := range f.rules {
					r.Scope = u.Name
					out = append(out, r)
				}
			}
		}
		return out
	}
	if err := unfilled(declared, func(n string) bool { return len(fill(n)) > 0 }, "status"); err != nil {
		return nil, err
	}
	var placed []StatusRule
	if !declared[""] {
		placed = append(placed, fill("")...)
	}
	for _, r := range rules {
		if r.isOutlet {
			placed = append(placed, fill(r.outlet)...)
			continue
		}
		placed = append(placed, r)
	}
	return placed, nil
}
```

In `internal/spec/spec.go`, `ParseWith`: replace `s.Status = st` with

```go
	if s.Status, err = placeStatus(st, uses); err != nil {
		return nil, err
	}
```

and `s.Menu = menu` with

```go
	if s.Menu, err = placeMenu(menu, uses); err != nil {
		return nil, err
	}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/spec/`
Expected: PASS. `TestUseRefusals` from Task 2 is unaffected.

- [ ] **Step 6: Commit**

```bash
git add internal/spec
git commit -m "place a template's status rules and menu items at outlets"
```

---

### Task 4: `agent:` names a watch by path

**Files:**
- Modify: `internal/spec/menu.go`, `internal/spec/validate.go`, `internal/spec/quit.go`, `internal/spec/window.go` (only if it calls `Action.validate`)
- Test: `internal/spec/agent_path_test.go`

**Interfaces:**
- Consumes: `Item.Scope` (Task 3), `Use` (Task 2).
- Produces: `Action.Agent` holds a path — `worker`, `daemon.agent` or `self.agent`; `(*Spec).AgentWatch(scope, target string) (Watch, error)`; `(*Spec).validateAction(a Action, path, scope string) error`.

- [ ] **Step 1: Write the failing tests**

`internal/spec/agent_path_test.go`:

```go
package spec

import (
	"strings"
	"testing"
)

const ctlTemplate = `
params:
  label: ~
watch:
  agent:
    launchagent: ${label}
  other: {exists: /tmp}
menu:
  default:
    - {text: Start, agent: self.agent.start}
`

var ctl = fakeTemplates{"ctl": ctlTemplate}

func TestAnAgentPathReachesAUsesWatch(t *testing.T) {
	s := parseWith(t, useHead+`
use:
  daemon:
    ctl: {label: dev.example.daemon}
menu:
  - outlet
  - {text: Stop, agent: daemon.agent.stop}
`, ctl)
	if got := s.Menu[0].Action; got.Agent != "self.agent" || got.Verb != AgentStart {
		t.Errorf("template item = %+v, want self.agent start", got)
	}
	if got := s.Menu[1].Action; got.Agent != "daemon.agent" || got.Verb != AgentStop {
		t.Errorf("file item = %+v, want daemon.agent stop", got)
	}
	w, err := s.AgentWatch("daemon", "self.agent")
	if err != nil || w.Label != "dev.example.daemon" {
		t.Errorf("AgentWatch(daemon, self.agent) = %+v, %v", w, err)
	}
	if w, err := s.AgentWatch("", "daemon.agent"); err != nil || w.Label != "dev.example.daemon" {
		t.Errorf("AgentWatch(\"\", daemon.agent) = %+v, %v", w, err)
	}
}

func TestAQuitButtonTakesAUsePath(t *testing.T) {
	s := parseWith(t, `
app:
  name: w
  id: dev.example.w
  icon: circle
  interval: 10s
  quit:
    - confirm: Quit?
      buttons: [{text: Stop too, agent: daemon.agent.stop}]
use:
  daemon:
    ctl: {label: dev.example.daemon}
menu: [outlet]
`, ctl)
	if got := s.App.Quit[0].Buttons[0].Action.Agent; got != "daemon.agent" {
		t.Errorf("button agent = %q", got)
	}
}

func TestAgentPathRefusals(t *testing.T) {
	src := fakeTemplates{
		"ctl":  ctlTemplate,
		"bare": "watch:\n  agent: {launchagent: dev.example.x}\nmenu:\n  default:\n    - {text: Go, agent: agent.start}\n",
		"peek": "watch:\n  agent: {launchagent: dev.example.x}\nmenu:\n  default:\n    - {text: Go, agent: daemon.agent.start}\n",
	}
	for name, tc := range map[string]struct{ doc, want string }{
		"self outside a template": {
			"watch:\n  agent: {launchagent: dev.example.x}\nmenu:\n  - {text: Go, agent: self.agent.start}\n",
			"self is bound only inside a template",
		},
		"a bare watch name inside a template": {
			"use:\n  d:\n    bare:\nmenu: [outlet]\n",
			"a template sees only self",
		},
		"another use's name inside a template": {
			"use:\n  daemon:\n    peek:\nmenu: [outlet]\n",
			"a template sees only self",
		},
		"a use that is not there": {
			"menu:\n  - {text: Go, agent: nope.agent.start}\n",
			`no use named "nope"`,
		},
		"a use's watch that is not a launchagent": {
			"use:\n  d:\n    ctl: {label: dev.example.d}\nmenu:\n  - outlet\n  - {text: Go, agent: d.other.start}\n",
			"is a exists watch",
		},
		"too many segments": {
			"menu:\n  - {text: Go, agent: a.b.c.start}\n",
			"is not <watch>, <use>.<watch> or self.<watch>",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseWith([]byte(useHead+tc.doc), src)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/spec/ -run 'AgentPath|QuitButtonTakes' -v`
Expected: FAIL to compile — `AgentWatch` is undefined.

- [ ] **Step 3: Split the verb off the end**

In `internal/spec/menu.go`, replace `parseAgentAction` with:

```go
// parseAgentAction reads `<target>.<verb>`, where the target is a watch, a
// use's watch, or self's inside a template. The verb is always the last
// segment, and no verb holds a dot.
func parseAgentAction(src, path string) (string, AgentVerb, error) {
	i := strings.LastIndex(src, ".")
	if i < 0 {
		return "", "", fmt.Errorf("%s.agent: %q names no verb; write <watch>.%s", path, src, strings.Join(verbList(), ", <watch>."))
	}
	target, rest := src[:i], src[i+1:]
	for _, v := range AgentVerbs {
		if rest == string(v) {
			return target, v, nil
		}
	}
	return "", "", fmt.Errorf("%s.agent: %q is not something perch can do to a LaunchAgent; it does %s", path, rest, strings.Join(verbList(), ", "))
}
```

Update the `Agent` field comment on `Action`: `// ActionAgent: <watch>, <use>.<watch> or self.<watch>`.

- [ ] **Step 4: Resolve a path, from a scope**

In `internal/spec/validate.go`, delete `validateItems`, `Action.validate` and `checkAgentTarget`, and add:

```go
func (s *Spec) validateItems(items []Item, path string) error {
	for i, it := range items {
		p := fmt.Sprintf("%s[%d]", path, i)
		if it.Scope != "" {
			p = fmt.Sprintf("%s (from use.%s)", p, it.Scope)
		}
		if len(it.Menu) > 0 && it.Action.Kind != ActionNone {
			return fmt.Errorf("%s: has a submenu and a %s action; opening a submenu supersedes the action, so it would never run", p, it.Action.Kind)
		}
		if err := s.validateAction(it.Action, p, it.Scope); err != nil {
			return err
		}
		if err := s.validateItems(it.Menu, fmt.Sprintf("%s[%d].menu", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Spec) validateAction(a Action, path, scope string) error {
	switch a.Kind {
	case ActionRun:
		if len(a.Run) == 0 {
			return fmt.Errorf("%s.run: empty; run takes the argv of a command", path)
		}
	case ActionPost:
		if a.PostURL == "" {
			return fmt.Errorf("%s.post: needs a url", path)
		}
	case ActionAgent:
		if _, err := s.AgentWatch(scope, a.Agent); err != nil {
			return fmt.Errorf("%s.agent: %w", path, err)
		}
	case ActionWindow:
		if s.Window == nil {
			return fmt.Errorf("%s.window: this file declares no window: block, so there is nothing to %s", path, a.Window)
		}
	}
	return nil
}

// AgentWatch finds the launchagent watch an agent: target names, as seen from
// an item in scope: "" for the file's own, or the use a template's item came
// from. A target is <watch>, <use>.<watch>, or self.<watch> inside a template.
func (s *Spec) AgentWatch(scope, target string) (Watch, error) {
	parts := strings.Split(target, ".")
	switch len(parts) {
	case 1:
		if scope != "" {
			return Watch{}, fmt.Errorf("%q: a template sees only self, so name its watch as self.%s", target, target)
		}
		return launchAgentIn(parts[0], s.Watches, "this file")
	case 2:
		useName := parts[0]
		switch {
		case useName == "self" && scope == "":
			return Watch{}, fmt.Errorf("%q: self is bound only inside a template, where it is that use", target)
		case useName == "self":
			useName = scope
		case scope != "":
			return Watch{}, fmt.Errorf("%q: a template sees only self, so write self.%s", target, parts[1])
		}
		for _, u := range s.Uses {
			if u.Name == useName {
				return launchAgentIn(parts[1], u.Watches, "use."+u.Name)
			}
		}
		return Watch{}, fmt.Errorf("%q: no use named %q", target, useName)
	}
	return Watch{}, fmt.Errorf("%q is not <watch>, <use>.<watch> or self.<watch>", target)
}

// launchAgentIn refuses anything but a launchagent watch: the label and the
// plist path both come from it, so there is nothing to act on without one.
func launchAgentIn(name string, watches []Watch, where string) (Watch, error) {
	var agents []string
	for _, w := range watches {
		if w.Kind == WatchLaunchAgent {
			agents = append(agents, w.Name)
		}
		if w.Name != name {
			continue
		}
		if w.Kind != WatchLaunchAgent {
			return Watch{}, fmt.Errorf("%q is a %s watch; agent: acts on a launchagent watch, which is where the label and the plist come from", name, w.Kind)
		}
		return w, nil
	}
	if len(agents) == 0 {
		return Watch{}, fmt.Errorf("no watch named %q, and %s declares no launchagent watch", name, where)
	}
	return Watch{}, fmt.Errorf("no watch named %q; the launchagent watches are %s", name, strings.Join(agents, ", "))
}
```

In `(s *Spec) validate()`, replace `return validateItems(s.Menu, "menu", s.Watches, s.Window)` with `return s.validateItems(s.Menu, "menu")`.

In `internal/spec/quit.go`, `validateQuit`: replace `b.Action.validate(bp, s.Watches, s.Window)` with `s.validateAction(b.Action, bp, "")`.

Find any other caller: `grep -rn '\.validate(' internal/spec | grep -v 'w.validate\|s.validate\|Window.validate'` — replace each `x.validate(path, watches, window)` the same way.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/spec/`
Expected: PASS, including every case in `TestLaunchAgentRefusals`: its messages keep `"names no verb"`, `"not something perch can do"`, `"is a run watch"`, `"the launchagent watches are worker"` and `"declares no launchagent watch"`.

Run: `go test ./internal/backend/swiftappkit/ -run 'TestGolden|TestEmitted'`
Expected: PASS. The backend still looks up one-segment targets by name, which every existing golden uses.

- [ ] **Step 6: Commit**

```bash
git add internal/spec
git commit -m "let agent: name a use's watch, or self's inside a template"
```

---

### Task 5: Verb defaults

**Files:**
- Create: `internal/spec/verbs.go`
- Modify: `internal/spec/spec.go`
- Test: `internal/spec/verbs_test.go`
- Regenerate: `internal/backend/swiftappkit/testdata/launchagent/Render.swift`, and any other golden with an `agent:` item

**Interfaces:**
- Consumes: `Action.Agent` path (Task 4).
- Produces: after `ParseWith`, every `agent:` item has a `Text` and a `When` that includes its verb's condition.

- [ ] **Step 1: Write the failing tests**

`internal/spec/verbs_test.go`:

```go
package spec

import "testing"

func TestAnAgentItemGetsALabelAndAGuard(t *testing.T) {
	s := parseDoc(t, agentHead+`
watch:
  worker: {launchagent: dev.example.worker}
menu:
  - agent: worker.start
  - agent: worker.stop
  - {text: Bounce, agent: worker.restart}
  - {text: More, menu: [{agent: worker.stop, when: "worker.running"}]}
`)
	for i, want := range []struct{ text, when string }{
		{"Start", "worker.installed && !worker.loaded"},
		{"Stop", "worker.loaded"},
		{"Bounce", "worker.installed"},
	} {
		if got := s.Menu[i]; got.Text != want.text || got.When != want.when {
			t.Errorf("menu[%d] = %q when %q, want %q when %q", i, got.Text, got.When, want.text, want.when)
		}
	}
	sub := s.Menu[3].Menu[0]
	if sub.Text != "Stop" || sub.When != "(worker.running) && (worker.loaded)" {
		t.Errorf("submenu item = %q when %q; an author's when: combines with the verb's", sub.Text, sub.When)
	}
}

func TestATemplatesAgentItemIsGuardedThroughSelf(t *testing.T) {
	s := parseWith(t, useHead+"use:\n  daemon:\n    ctl: {label: dev.example.daemon}\nmenu: [outlet]\n", ctl)
	if got := s.Menu[0].When; got != "self.agent.installed && !self.agent.loaded" {
		t.Errorf("when = %q", got)
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/spec/ -run 'AnAgentItemGets|GuardedThroughSelf' -v`
Expected: FAIL — `Text` is empty and `When` is unchanged.

- [ ] **Step 3: Apply the defaults**

`internal/spec/verbs.go`:

```go
package spec

// verbLabel is what an agent: item says when it has no text:.
var verbLabel = map[AgentVerb]string{
	AgentStart:   "Start",
	AgentStop:    "Stop",
	AgentRestart: "Restart",
}

// verbGuard is when a verb can work, written against the watch the item acts
// on. start is bootstrap, which fails on a label launchd already holds; stop
// is bootout, which fails on one it does not; restart works from either.
func verbGuard(target string, v AgentVerb) string {
	switch v {
	case AgentStart:
		return target + ".installed && !" + target + ".loaded"
	case AgentStop:
		return target + ".loaded"
	}
	return target + ".installed"
}

// applyVerbDefaults labels every agent: item that has no text:, and combines
// its when: with its verb's guard, so a menu never offers a verb that fails.
func applyVerbDefaults(items []Item) {
	for i := range items {
		it := &items[i]
		applyVerbDefaults(it.Menu)
		if it.Action.Kind != ActionAgent {
			continue
		}
		if it.Text == "" {
			it.Text = verbLabel[it.Action.Verb]
		}
		guard := verbGuard(it.Action.Agent, it.Action.Verb)
		if it.When == "" {
			it.When = guard
		} else {
			it.When = "(" + it.When + ") && (" + guard + ")"
		}
	}
}
```

In `internal/spec/spec.go`, `ParseWith`, directly before `if err := s.validate(); err != nil {`:

```go
	applyVerbDefaults(s.Menu)
```

- [ ] **Step 4: Run the spec tests**

Run: `go test ./internal/spec/`
Expected: PASS. If a test asserts an `agent:` item's `When` verbatim, update its expectation to the combined form and say so in the commit body.

- [ ] **Step 5: Regenerate and read the goldens**

Run: `go test ./internal/backend/swiftappkit/ -run TestGolden -update && git diff --stat internal/backend/swiftappkit/testdata`
Expected: only goldens holding an `agent:` item change (`launchagent`, `quit`). `git diff internal/backend/swiftappkit/testdata` shows each `if` guarding an agent item gain `&& (results.worker.installed ...)`-style conjuncts and nothing else.

Run: `go test ./internal/backend/swiftappkit/ ./internal/preview/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/spec internal/backend/swiftappkit/testdata
git commit -m "label an agent: item and hide it when its verb would fail"
```

---

### Task 6: `self` and a use's name in expressions

**Files:**
- Modify: `internal/celswift/env.go`, `internal/celswift/lower.go`
- Test: `internal/celswift/use_test.go`

**Interfaces:**
- Consumes: `spec.Use` (Task 2).
- Produces: `(*Env).WithUses(uses []spec.Use) *Env`; `ForUse(u spec.Use, swift string) *Env`; `ForUseStates(u spec.Use, swift string) *Env`.

- [ ] **Step 1: Write the failing tests**

`internal/celswift/use_test.go`:

```go
package celswift

import (
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

func daemonUse() spec.Use {
	return spec.Use{
		Name:    "daemon",
		Watches: []spec.Watch{{Name: "agent", Kind: spec.WatchLaunchAgent, Label: "dev.example.daemon", Scope: "daemon"}},
		States:  []spec.State{{Name: "stopped", Cond: "!self.agent.loaded"}, {Name: "running"}},
	}
}

func TestSelfReachesAUsesWatchesAndStates(t *testing.T) {
	e := ForUse(daemonUse(), "daemon").Prefixed("results.")
	for src, want := range map[string]string{
		"self.agent.pid": "results.daemon.agent.pid",
		"self.running":   "results.daemon.running",
	} {
		if got, err := e.LowerExpr(src); err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q", src, got, err, want)
		}
	}
}

// A use's state condition sees its watches and not its states, for the reason
// a file's state condition sees no states: the ordering already decides.
func TestAUsesStateConditionSeesOnlyItsWatches(t *testing.T) {
	e := ForUseStates(daemonUse(), "self")
	if got, err := e.LowerCondition("!self.agent.loaded"); err != nil || got != "!(self.agent.loaded)" {
		t.Errorf("= %q, %v", got, err)
	}
	if got, err := e.LowerCondition("self.stopped"); err == nil {
		t.Errorf("self.stopped lowered to %q; want a refusal", got)
	}
}

func TestTheFileReachesAUseByName(t *testing.T) {
	e := NewEnv(nil).WithUses([]spec.Use{daemonUse()}).Prefixed("results.")
	for src, want := range map[string]string{
		"daemon.running":   "results.daemon.running",
		"daemon.agent.pid": "results.daemon.agent.pid",
	} {
		if got, err := e.LowerExpr(src); err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q", src, got, err, want)
		}
	}
}

func TestSelfOutsideATemplateSaysWhereItIsBound(t *testing.T) {
	_, err := NewEnv(nil).LowerExpr("self.agent.pid")
	if err == nil || !strings.Contains(err.Error(), "bound only inside a template") {
		t.Errorf("err = %v", err)
	}
}

func TestItAndSelfBothResolveInsideATemplatesEach(t *testing.T) {
	e := ForUse(daemonUse(), "daemon").Prefixed("results.").WithEach(&spec.Type{Kind: spec.TypeString}, "it1")
	if got, err := e.LowerText("it"); err != nil || got != "it1" {
		t.Errorf("it = %q, %v", got, err)
	}
	if got, err := e.LowerExpr("self.agent.pid"); err != nil || got != "results.daemon.agent.pid" {
		t.Errorf("self.agent.pid = %q, %v", got, err)
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/celswift/ -run 'Self|UseBy|UsesState' -v`
Expected: FAIL to compile — `ForUse` is undefined.

- [ ] **Step 3: Bind uses and self**

Append to `internal/celswift/env.go`:

```go
// WithUses returns a copy of e with each use bound to its object — its watches
// and its states — spelled in Swift by the use's own name.
func (e *Env) WithUses(uses []spec.Use) *Env {
	out := &Env{vars: append([]binding(nil), e.vars...)}
	for _, u := range uses {
		out.vars = append(out.vars, binding{name: u.Name, typ: useType(u, true), swift: u.Name})
	}
	return out
}

// ForUse is what a template's items and status rules lower against: self, and
// nothing of the file's. Prefixed reaches self through the results like any
// watch.
func ForUse(u spec.Use, swift string) *Env {
	return &Env{vars: []binding{{name: "self", typ: useType(u, true), swift: swift}}}
}

// ForUseStates is what a template's own state conditions lower against: self's
// watches without its states. swift is spelled as given; Prefixed leaves it.
func ForUseStates(u spec.Use, swift string) *Env {
	return &Env{vars: []binding{{name: "self", typ: useType(u, false), swift: swift, local: true}}}
}

func useType(u spec.Use, withStates bool) *spec.Type {
	t := obj()
	for _, w := range u.Watches {
		t.Fields = append(t.Fields, spec.Field{Name: w.Name, Type: watchType(w)})
	}
	if withStates {
		for _, st := range u.States {
			t.Fields = append(t.Fields, field(st.Name, spec.TypeBool))
		}
	}
	return t
}
```

In `internal/celswift/lower.go`, inside `case celast.IdentKind:`, directly after `if !ok {`:

```go
			if name == "self" {
				return value{}, fmt.Errorf("%q: self is bound only inside a template, where it is that use", src)
			}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/celswift/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/celswift
git commit -m "bind self inside a template and a use's name outside it"
```

---

### Task 7: Emit each use as a struct in `Results`

**Files:**
- Modify: `internal/spec/use.go`, `internal/backend/swiftappkit/shapes.go`, `render.go`, `main.go`, `preview.go`
- Test: `internal/backend/swiftappkit/use_test.go`, `golden_test.go`, `typecheck_test.go`
- Create: `internal/backend/swiftappkit/testdata/templates/` (by `-update`)

**Interfaces:**
- Consumes: `Spec.Uses`, `Item.Scope`, `StatusRule.Scope`, `(*Spec).AgentWatch`, `celswift.ForUse`, `ForUseStates`, `WithUses`.
- Produces: `(*spec.Spec).AllWatches() []spec.Watch`; `(spec.Watch).Key() string` (`name` or `use.name`), the key a preview state uses.

- [ ] **Step 1: Write the failing tests**

`internal/backend/swiftappkit/use_test.go`:

```go
package swiftappkit

import (
	"slices"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

const templatesDoc = `
app: {name: wall, id: dev.example.wall.menubar, icon: rectangle.stack, interval: 5s}
use:
  daemon:
    service: {label: dev.example.wall.daemon}
  client:
    service: {label: dev.example.wall.client, noun: Client}
watch:
  health: {http: "http://127.0.0.1:8787/api/health"}
state:
  - down: "!daemon.running"
  - wedged: "!health.ok"
  - up:
status:
  - outlet
  - {when: down, dim: true}
menu:
  - outlet
  - separator
  - {text: "pid {{daemon.agent.pid}}", when: up}
  - outlet: controls
  - separator
  - {text: Quit, quit: true}
`

func emitted(t *testing.T, doc string) map[string]string {
	t.Helper()
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	files, err := New().Emit(s)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	out := map[string]string{}
	for _, f := range files {
		out[f.Name] = string(f.Body)
	}
	return out
}

func TestAUseIsAStructInsideResults(t *testing.T) {
	files := emitted(t, templatesDoc)
	for file, wants := range map[string][]string{
		"Render.swift": {
			"struct DaemonUse {",
			"var agent = DaemonAgentResult()",
			"var daemon = DaemonUse()",
			"var client = ClientUse()",
			"extension DaemonUse {",
			"var uninstalled: Bool { (!(self.agent.installed)) }",
			"var stopped: Bool { !self.uninstalled && (!(self.agent.loaded)) }",
			"var state_down: Bool { (!(daemon.running)) }",
			"results.daemon.agent.pid",
			`label: "dev.example.wall.client"`,
		},
		"main.swift": {
			"next.daemon.agent = r",
			`DaemonAgentResult(Watcher.launchAgent(label: "dev.example.wall.daemon"`,
		},
	} {
		for _, want := range wants {
			if !strings.Contains(files[file], want) {
				t.Errorf("%s is missing %q", file, want)
			}
		}
	}
}

// fakeTemplates stands in for a repo's own templates.
type fakeTemplates map[string]string

func (f fakeTemplates) Template(name string) ([]byte, string, bool, error) {
	src, ok := f[name]
	return []byte(src), "templates/" + name + ".yaml", ok, nil
}

func (f fakeTemplates) Names() []string {
	var out []string
	for name := range f {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// A template sees only self, so it cannot come to depend on a file it was not
// written for.
func TestATemplateItemCannotReachTheFilesWatches(t *testing.T) {
	src := fakeTemplates{"peek": "watch:\n  own: {exists: /tmp}\nmenu:\n  default:\n    - {text: peek, when: \"fileWatch.ok\"}\n"}
	s, err := spec.ParseWith([]byte(`
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
use:
  p:
    peek:
watch:
  fileWatch: {exists: /tmp}
menu: [outlet]
`), src)
	if err != nil {
		t.Fatalf("spec.ParseWith: %v", err)
	}
	_, err = New().Emit(s)
	if err == nil || !strings.Contains(err.Error(), `unknown name "fileWatch"`) {
		t.Errorf("err = %v, want the file's watch refused inside the template", err)
	}
}
```

In `internal/backend/swiftappkit/golden_test.go`, add `"templates": templatesDoc,` to the map. In `typecheck_test.go`, add to the fixtures and to the map in `TestEmittedSwiftTypechecks`:

```go
// brainhouseOnService is brainhouse's menubar.yaml rewritten on service, as
// evidence the template covers a file that exists. brainhouse is not changed.
const brainhouseOnService = `
app: {name: brainhouse, id: com.brainhouse.menubar, icon: brain, interval: 5s}
use:
  server:
    service: {label: com.brainhouse}
watch:
  summary:
    http: http://localhost:8765/api/summary
    json: true
    shape: {live: int, awaiting_input: int}
state:
  - down: "!server.running"
  - wedged: "!summary.ok"
  - up:
status:
  - {when: wedged, icon: exclamationmark.triangle}
  - {when: down, dim: true}
  - badge: "summary.data.awaiting_input > 0 ? string(summary.data.awaiting_input) : ''"
menu:
  - text: "Server: running on :8765 — {{summary.data.awaiting_input}} awaiting input"
    when: "up && summary.data.awaiting_input > 0"
  - {text: "Server: not responding on :8765", when: wedged}
  - outlet
  - separator
  - {text: Open Dashboard, open: "http://localhost:8765/"}
  - separator
  - outlet: controls
  - {text: Open Logs, open: ~/Library/Logs/brainhouse}
  - separator
  - {text: Quit, quit: true}
`

// ontoOnService is the same for onto's local agent.
const ontoOnService = `
app: {name: onto, id: dev.onto.menubar, icon: circle, interval: 10s}
use:
  local:
    service: {label: dev.onto.agent, noun: This agent}
watch:
  fleet: {run: [onto, top, --json], json: true}
menu:
  - {when: "!fleet.ok", text: "onto did not answer"}
  - separator
  - outlet
  - outlet: controls
  - separator
  - {text: Quit, quit: true}
`
```

```go
		"templates":           templatesDoc,
		"brainhouseOnService": brainhouseOnService,
		"ontoOnService":       ontoOnService,
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/backend/swiftappkit/ -run 'AUseIsAStruct|TemplateItemCannot' -v`
Expected: FAIL — Render.swift has no `DaemonUse`.

- [ ] **Step 3: Every watch, and its key**

Append to `internal/spec/use.go`:

```go
// AllWatches is every watch polled: the file's, then each use's, in use: order.
func (s *Spec) AllWatches() []Watch {
	out := append([]Watch(nil), s.Watches...)
	for _, u := range s.Uses {
		out = append(out, u.Watches...)
	}
	return out
}

// Key names a watch across the file and its uses: its name, or use.name. It is
// what a preview state writes a use's watch under.
func (w Watch) Key() string {
	if w.Scope == "" {
		return w.Name
	}
	return w.Scope + "." + w.Name
}
```

- [ ] **Step 4: Names for a use's structs**

In `internal/backend/swiftappkit/shapes.go`, replace `structNames`, `nameStructs` and `resultTypeName` with:

```go
type structNames struct {
	shape  map[*spec.Type]string
	result map[string]string // Watch.Key() -> its result struct
	use    map[string]string // use name -> its struct
}

func nameStructs(s *spec.Spec) structNames {
	n := structNames{shape: map[*spec.Type]string{}, result: map[string]string{}, use: map[string]string{}}
	taken := map[string]bool{}
	for _, u := range s.Uses {
		n.use[u.Name] = unique(exported(u.Name)+"Use", taken)
	}
	// Result names are claimed before shapes, so a shape never takes one out
	// from under the watch it belongs to. Watch names differ but exported() can
	// flatten two of them together: a_b and aB both read as AB.
	for _, w := range s.AllWatches() {
		n.result[w.Key()] = unique(exported(w.Scope)+exported(w.Name)+"Result", taken)
	}
	for _, w := range s.AllWatches() {
		n.assign(w.Shape, exported(w.Scope)+exported(w.Name)+"Data", taken)
	}
	return n
}
```

```go
// resultTypeName is the struct holding one watch's poll outcome.
func (n structNames) resultTypeName(w spec.Watch) string { return n.result[w.Key()] }

// useTypeName is the struct holding one use's watches and states.
func (n structNames) useTypeName(u spec.Use) string { return n.use[u.Name] }

// resultPath is where a watch's result lives in Results: its own property, or
// one inside its use's.
func resultPath(w spec.Watch) string {
	if w.Scope == "" {
		return decl(w.Name)
	}
	return decl(w.Scope) + "." + decl(w.Name)
}
```

In `emitShapes`, change `for _, w := range s.Watches {` to `for _, w := range s.AllWatches() {`.

- [ ] **Step 5: Results, states and scoped lowering**

In `internal/backend/swiftappkit/render.go`:

Replace `emitRender` with:

```go
func emitRender(s *spec.Spec, n structNames) (string, error) {
	b := &buf{}
	e := renderEnv(s)
	scopes := scopeEnvs(s)

	b.line("%s", header)
	b.line("import Foundation")
	b.line("")

	emitResultTypes(b, s, n)
	emitResultInits(b, s, n)
	if err := emitUseStates(b, s, n); err != nil {
		return "", err
	}
	if err := emitStates(b, s); err != nil {
		return "", err
	}
	if err := emitFace(b, s, e, scopes); err != nil {
		return "", err
	}
	return emitMenu(b, s, e, scopes)
}
```

Replace `renderEnv` with:

```go
// renderEnv is the lowering environment for the file's own rules and items:
// watches, uses and states, reached through the results the functions take.
func renderEnv(s *spec.Spec) *celswift.Env {
	e := celswift.NewEnv(s.Watches).WithUses(s.Uses)
	for _, st := range s.States {
		e = e.WithState(st.Name, stateProp(st.Name))
	}
	return e.Prefixed("results.")
}

// scopeEnvs is what each use's rules and items lower against: self, reached
// through the results, and nothing of the file's.
func scopeEnvs(s *spec.Spec) map[string]*celswift.Env {
	out := map[string]*celswift.Env{}
	for _, u := range s.Uses {
		out[u.Name] = celswift.ForUse(u, u.Name).Prefixed("results.")
	}
	return out
}
```

In `emitStates`, change `e := celswift.NewEnv(s.Watches)` to `e := celswift.NewEnv(s.Watches).WithUses(s.Uses)`.

Add after `emitStates`:

```go
// emitUseStates writes each use's states as properties of its own struct, so
// a use's list is ordered against itself and nothing else.
func emitUseStates(b *buf, s *spec.Spec, n structNames) error {
	for _, u := range s.Uses {
		if len(u.States) == 0 {
			continue
		}
		e := celswift.ForUseStates(u, "self")
		b.line("extension %s {", n.useTypeName(u))
		b.in()
		for i, st := range u.States {
			parts := make([]string, 0, i+1)
			for _, earlier := range u.States[:i] {
				parts = append(parts, "!self."+earlier.Name)
			}
			if st.Cond == "" {
				if len(parts) == 0 {
					parts = append(parts, "true")
				}
			} else {
				cond, err := e.LowerCondition(st.Cond)
				if err != nil {
					return fmt.Errorf("use.%s: state[%d].%s: %w", u.Name, i, st.Name, err)
				}
				parts = append(parts, "("+cond+")")
			}
			b.line("var %s: Bool { %s }", decl(st.Name), strings.Join(parts, " && "))
		}
		b.out()
		b.line("}")
		b.line("")
	}
	return nil
}
```

In `emitResultTypes`: change the first loop to `for _, w := range s.AllWatches() {`. Directly before `b.line("struct Results {")` insert:

```go
	for _, u := range s.Uses {
		b.line("struct %s {", n.useTypeName(u))
		b.in()
		for _, w := range u.Watches {
			b.line("var %s = %s()", decl(w.Name), n.resultTypeName(w))
		}
		b.out()
		b.line("}")
		b.line("")
	}
```

and replace the body of the `Results` struct (between `b.in()` and `b.out()`) with:

```go
	for _, w := range s.Watches {
		b.line("var %s = %s()", decl(w.Name), n.resultTypeName(w))
	}
	for _, u := range s.Uses {
		b.line("var %s = %s()", decl(u.Name), n.useTypeName(u))
	}
	if len(s.Watches) == 0 && len(s.Uses) == 0 {
		b.line("// no watches declared")
	}
```

In `emitResultInits`, change the loop to `for _, w := range s.AllWatches() {`.

Change `emitFace`'s signature to `func emitFace(b *buf, s *spec.Spec, e *celswift.Env, scopes map[string]*celswift.Env) error`, and at the top of its rule loop body add

```go
		re := e
		if rule.Scope != "" {
			re = scopes[rule.Scope]
		}
```

then use `re` in place of `e` for `LowerCondition(rule.When)` and `LowerText(rule.Badge)`.

Replace `menuGen`, `emitMenu`'s head, `items`, and the agent case of `action`:

```go
// menuGen hands out unique local names so nested submenus and each: loops do
// not shadow one another.
type menuGen struct {
	b      *buf
	n      int
	spec   *spec.Spec
	file   *celswift.Env
	scopes map[string]*celswift.Env
}

func (g *menuGen) envFor(scope string) *celswift.Env {
	if scope == "" {
		return g.file
	}
	return g.scopes[scope]
}
```

```go
func emitMenu(b *buf, s *spec.Spec, e *celswift.Env, scopes map[string]*celswift.Env) (string, error) {
```

and inside it:

```go
	g := &menuGen{b: b, spec: s, file: e, scopes: scopes}
	if err := g.items(s.Menu, "menu", e, "menu", ""); err != nil {
		return "", err
	}
```

```go
// items lowers each item against the environment of the scope it came from:
// crossing into a template's items leaves the file's names, it included.
func (g *menuGen) items(items []spec.Item, into string, e *celswift.Env, path, scope string) error {
	for i, it := range items {
		ie := e
		if it.Scope != scope {
			ie = g.envFor(it.Scope)
		}
		if err := g.item(it, into, ie, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}
```

In `body`, change `g.items(it.Menu, sub, e, path+".menu")` to `g.items(it.Menu, sub, e, path+".menu", it.Scope)` and `g.action(it.Action, e, path)` to `g.action(it.Action, e, path, it.Scope)`.

Change `action`'s signature to `func (g *menuGen) action(a spec.Action, e *celswift.Env, path, scope string) (string, error)` and its agent case to:

```go
	case spec.ActionAgent:
		w, err := g.spec.AgentWatch(scope, a.Agent)
		if err != nil {
			return "", fmt.Errorf("%s.agent: %w", path, err)
		}
		return fmt.Sprintf(".agent(label: %s, plist: %s, verb: .%s)",
			celswift.SwiftString(w.Label), celswift.SwiftString(w.Plist), a.Verb), nil
```

In `emitQuit`, change `g.action(btn.Action, e, fmt.Sprintf("%s.buttons[%d]", path, j))` to `g.action(btn.Action, e, fmt.Sprintf("%s.buttons[%d]", path, j), "")`.

- [ ] **Step 6: Poll and preview every watch**

In `internal/backend/swiftappkit/main.go`, `emitPoll`: change `if len(s.Watches) == 0 {` to `if len(s.AllWatches()) == 0 {`, the loop to `for _, w := range s.AllWatches() {`, and `b.line("next.%s = r", decl(w.Name))` to `b.line("next.%s = r", resultPath(w))`.

In `internal/backend/swiftappkit/preview.go`, `emitDriver`: change `if len(s.Watches) == 0 {` to `if len(s.AllWatches()) == 0 {`, the loop to `for _, w := range s.AllWatches() {`, `celswift.SwiftString(w.Name)` to `celswift.SwiftString(w.Key())`, and `decl(w.Name)` to `resultPath(w)`.

- [ ] **Step 7: Run the tests, write the golden, typecheck**

Run: `go build ./internal/... && go vet ./internal/backend/swiftappkit/`
Expected: no output. A compile error names another caller of `emitFace`, `emitMenu`, `(*menuGen).items` or `(*menuGen).action` — give it the new arguments: the `scopes` map, and `""` for a scope outside any template.

Run: `go test ./internal/backend/swiftappkit/ -run 'AUseIsAStruct|TemplateItemCannot' -v`
Expected: PASS.

Run: `go test ./internal/backend/swiftappkit/ -run TestGolden -update && git status --short internal/backend/swiftappkit/testdata`
Expected: only `testdata/templates/` is new; no existing golden changes. Read `testdata/templates/Render.swift` once through: each use's rules and items reach `results.<use>`, and no template item names `health`.

Run: `go test ./internal/backend/swiftappkit/ ./internal/preview/`
Expected: PASS, including `TestEmittedSwiftTypechecks/templates`, `/brainhouseOnService` and `/ontoOnService` with no warnings.

- [ ] **Step 8: Commit**

```bash
git add internal/spec/use.go internal/backend/swiftappkit
git commit -m "emit each template use as a struct inside Results"
```

---

### Task 8: A preview state for a use's watches

**Files:**
- Modify: `internal/preview/preview.go`
- Test: `internal/preview/preview_test.go`

**Interfaces:**
- Consumes: `(spec.Watch).Key()`, `Spec.Uses` (Task 7).
- Produces: a state fence may say `daemon: {agent: {running: true}}`; `State.Watches` is keyed by `Watch.Key()`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/preview/preview_test.go`:

```go
const useDoc = `
app: {name: w, id: dev.example.w, icon: circle, interval: 10s}
use:
  daemon:
    service: {label: dev.example.daemon}
menu:
  - outlet
  - {text: Quit, quit: true}
`

func TestParseStateReadsAUsesWatches(t *testing.T) {
	s := parse(t, useDoc)
	st := state(t, s, "running", "daemon: {agent: {running: true, pid: 42}}")
	got, ok := st.Watches["daemon.agent"]
	if !ok || got.Running == nil || !*got.Running || got.Pid == nil || *got.Pid != 42 {
		t.Errorf("watches = %+v, want daemon.agent running with pid 42", st.Watches)
	}
}

func TestParseStateRefusesWhatAUseDoesNotHave(t *testing.T) {
	s := parse(t, useDoc)
	for src, want := range map[string]string{
		"daemon: {nope: {running: true}}": `use daemon has no watch named "nope"; it has agent`,
		"daemon: true":                    "want a mapping of its watches",
	} {
		_, err := ParseState("x", []byte(src), s)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to mention %q", src, err, want)
		}
	}
}

func TestRenderShowsAUsesReadout(t *testing.T) {
	s := parse(t, useDoc)
	frames := render(t, s, []State{state(t, s, "running", "daemon: {agent: {running: true, pid: 42}}")})
	var titles []string
	for _, n := range frames[0].Menu {
		titles = append(titles, n.Title)
	}
	if !slices.Contains(titles, "Service: running · pid 42") {
		t.Errorf("menu = %q", titles)
	}
}
```

Add `"slices"` to the test file's imports if it is not there.

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/preview/ -run 'Use' -v`
Expected: FAIL — `no watch named "daemon"`.

- [ ] **Step 3: Read a use's entry**

In `internal/preview/preview.go`, replace the loop in `ParseState` with:

```go
	for _, entry := range doc.entries {
		if u, ok := useNamed(s, entry.key); ok {
			fields, err := entry.value.fields()
			if err != nil {
				return st, fmt.Errorf("state %q: %s: want a mapping of its watches, %s", name, entry.key, strings.Join(namesOf(u.Watches), ", "))
			}
			for _, f := range fields {
				w, ok := watchIn(u.Watches, f.key)
				if !ok {
					return st, fmt.Errorf("state %q: use %s has no watch named %q; it has %s", name, u.Name, f.key, strings.Join(namesOf(u.Watches), ", "))
				}
				sample, err := parseSample(w, f.value)
				if err != nil {
					return st, fmt.Errorf("state %q: %s.%s: %w", name, u.Name, f.key, err)
				}
				st.Watches[w.Key()] = sample
			}
			continue
		}
		w, ok := watchIn(s.Watches, entry.key)
		if !ok {
			return st, fmt.Errorf("state %q: no watch or use named %q; this file declares %s",
				name, entry.key, strings.Join(watchNames(s), ", "))
		}
		sample, err := parseSample(w, entry.value)
		if err != nil {
			return st, fmt.Errorf("state %q: %s: %w", name, entry.key, err)
		}
		st.Watches[w.Key()] = sample
	}
```

Replace `watchNamed` and `watchNames` with:

```go
func watchIn(watches []spec.Watch, name string) (spec.Watch, bool) {
	for _, w := range watches {
		if w.Name == name {
			return w, true
		}
	}
	return spec.Watch{}, false
}

func useNamed(s *spec.Spec, name string) (spec.Use, bool) {
	for _, u := range s.Uses {
		if u.Name == name {
			return u, true
		}
	}
	return spec.Use{}, false
}

func namesOf(watches []spec.Watch) []string {
	out := make([]string, 0, len(watches))
	for _, w := range watches {
		out = append(out, w.Name)
	}
	return out
}

func watchNames(s *spec.Spec) []string {
	out := namesOf(s.Watches)
	for _, u := range s.Uses {
		out = append(out, u.Name)
	}
	if len(out) == 0 {
		return []string{"no watches"}
	}
	return out
}
```

`TestParseStateRefusesAWatchTheFileDoesNotDeclare` asserts on the old wording: run `grep -n 'no watch named' internal/preview/preview_test.go` and change its expected substring to `no watch or use named`.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/preview/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/preview
git commit -m "let a preview state say what a use's watches returned"
```

---

### Task 9: The schema knows `use:`, outlets, agent paths and template files

**Files:**
- Modify: `internal/schema/schema.go`, `internal/schema/drift_test.go`, `cmd/perch/main.go`
- Test: `internal/schema/drift_test.go`, `internal/schema/schema_test.go`

**Interfaces:**
- Produces: `schema.TemplateJSON() string`; `perch schema -template`.

`-template` is two syllables; no one-syllable word names a template.

- [ ] **Step 1: Write the failing tests**

Append to `internal/schema/drift_test.go`:

```go
func TestTemplateSchemaDeclaresExactlyTheKeysATemplateTakes(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(TemplateJSON()), &doc); err != nil {
		t.Fatalf("TemplateJSON is not JSON: %v", err)
	}
	got := keysAt(t, doc, "properties")
	if want := yamlTags(t)["rawTemplate"]; !reflect.DeepEqual(got, want) {
		t.Errorf("template schema %v\n parser %v", got, want)
	}
	if doc["additionalProperties"] != false {
		t.Error("the template schema accepts unknown keys")
	}
}
```

In the `sections` map, change `"rawStatusRule": "properties.status.items.properties",` to `"rawStatusRule": "properties.status.items.oneOf.0.properties",`.

Append to `internal/schema/schema_test.go`, adding `"encoding/json"` and `"regexp"` to its imports if they are absent:

```go
func TestAgentPatternTakesPaths(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(JSON()), &doc); err != nil {
		t.Fatal(err)
	}
	item := doc["definitions"].(map[string]any)["menu"].(map[string]any)["items"].(map[string]any)["oneOf"].([]any)[1]
	pattern := item.(map[string]any)["properties"].(map[string]any)["agent"].(map[string]any)["pattern"].(string)
	re := regexp.MustCompile(pattern)
	for v, want := range map[string]bool{
		"worker.start": true, "daemon.agent.stop": true, "self.agent.restart": true,
		"worker": false, "a.b.c.start": false, "worker.reload": false,
	} {
		if re.MatchString(v) != want {
			t.Errorf("%q: matches = %v, want %v", v, !want, want)
		}
	}
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./internal/schema/ -v`
Expected: FAIL — `TemplateJSON` is undefined.

- [ ] **Step 3: Update the schema**

In `internal/schema/schema.go`:

Replace the `"status"` property with:

```json
    "status": {
      "type": "array",
      "description": "First matching rule wins. - outlet places the uses' status rules.",
      "items": {
        "oneOf": [
          {
            "type": "object",
            "additionalProperties": false,
            "properties": {
              "when": {"type": "string", "description": "CEL condition; omit to always match."},
              "icon": {"description": "An SF Symbol name, or {asset: <name>} for a .png in menubar/Icons.", "oneOf": [{"type": "string"}, {"type": "object", "required": ["asset"], "additionalProperties": false, "properties": {"asset": {"type": "string", "pattern": "^[A-Za-z0-9_-]+$"}}}]},
              "dim": {"type": "boolean"},
              "badge": {"type": "string", "description": "CEL expression shown beside the icon."}
            }
          },
          {"$ref": "#/definitions/outletMark"},
          {"$ref": "#/definitions/namedOutlet"}
        ]
      }
    },
```

In `definitions.menu.items.oneOf`, change the first entry to `{"type": "string", "enum": ["separator", "outlet"]}` and append a third entry `{"$ref": "#/definitions/namedOutlet"}`. Add to `definitions`:

```json
    "outletMark": {"type": "string", "enum": ["outlet"], "description": "The default outlet: where uses' fragments land."},
    "namedOutlet": {
      "type": "object",
      "required": ["outlet"],
      "additionalProperties": false,
      "properties": {"outlet": {"type": ["string", "null"], "pattern": "^[A-Za-z_][A-Za-z0-9_]*$", "description": "A named outlet; a use's fragment of that name lands here."}}
    },
```

Replace both `"agent"` patterns (the menu item's and the quit button's) with:

```json
"pattern": "^([A-Za-z_][A-Za-z0-9_]*\\.){1,2}(start|stop|restart)$"
```

and the menu item's description with `"<watch>, <use>.<watch> or self.<watch>, then .start, .stop or .restart."`.

Append to `schema.go`:

```go
// TemplateJSON is the schema for a template file. It is cut from JSON rather
// than written again, so a watch, state, rule or item means the same in both.
func TemplateJSON() string {
	var doc map[string]any
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		panic("schema: " + err.Error())
	}
	props := doc["properties"].(map[string]any)
	tmpl := map[string]any{
		"$schema":              doc["$schema"],
		"title":                "perch template",
		"description":          "A template a menubar.yaml reaches under use:.",
		"type":                 "object",
		"additionalProperties": false,
		"definitions":          doc["definitions"],
		"properties": map[string]any{
			"params": map[string]any{
				"type":                 "object",
				"description":          "Parameter names and their defaults; ~ makes one required.",
				"propertyNames":        map[string]any{"pattern": "^[A-Za-z_][A-Za-z0-9_]*$"},
				"additionalProperties": map[string]any{"type": []string{"string", "null"}},
			},
			"watch": props["watch"],
			"state": props["state"],
			"status": map[string]any{
				"type":                 "object",
				"description":          "Outlet names to status rules; default is the default outlet. Every rule needs when:.",
				"additionalProperties": props["status"],
			},
			"menu": map[string]any{
				"type":                 "object",
				"description":          "Outlet names to menu items; default is the default outlet.",
				"additionalProperties": map[string]any{"$ref": "#/definitions/menu"},
			},
		},
	}
	out, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		panic("schema: " + err.Error())
	}
	return string(out) + "\n"
}
```

and add `import "encoding/json"` at the top of the file.

- [ ] **Step 4: The flag**

In `cmd/perch/main.go`, `runSchema`:

```go
	fs := newFlags("schema", e)
	out := fs.String("o", "", "write to a file instead of stdout")
	template := fs.Bool("template", false, "write the schema for a template file instead")
	if err := parse(fs, args); err != nil {
		return err
	}
	body := schema.JSON()
	if *template {
		body = schema.TemplateJSON()
	}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/schema/ ./cmd/perch/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/schema cmd/perch/main.go
git commit -m "describe use:, outlets and agent paths in the schema, and write one for templates"
```

---

### Task 10: Docs

**Files:**
- Modify: `docs/schema.md`, `docs/recipes/launchagent.md`, `internal/site/nav.go`, `internal/site/fences.go`, `docs/superpowers/specs/2026-09-12-launchagent-watch-design.md`
- Test: `internal/site/site_test.go` (existing), `internal/e2e/docs_test.go` (existing)

- [ ] **Step 1: Template-file fences highlight as YAML and are not built**

`internal/e2e/docs_test.go` builds every fence whose info is exactly `yaml`, so a template file is fenced ` ```yaml template `. In `internal/site/fences.go`, `extractFences`, add a case before `default:`:

```go
		case info == "yaml template":
			// A template file is not a menubar.yaml, so it has nothing to preview.
			b.lang, b.code = "yaml", body
```

- [ ] **Step 2: The `use` chapter**

In `internal/site/nav.go`, add after the `state` entry:

```go
			{Title: "`use`", URL: "menubar/use/", Source: "docs/schema.md", Schema: "use"},
```

In `docs/schema.md`, change the intro's key list to: ``A document has seven top-level keys — `app`, `watch`, `state`, `use`, `status`, `window` and `menu`.`` Then insert this chapter directly before `## Builtins`:

````markdown
## `use`

A mapping of names to templates. A template is a block of watches, states,
status rules and menu items written once; perch ships [`service`](#service),
and a repo's own live in `menubar/templates/<name>.yaml`.

```yaml
app: {name: wall, id: dev.example.wall.menubar, icon: rectangle.stack, interval: 5s}

use:
  daemon:
    service: {label: dev.example.wall.daemon, noun: Daemon}
  client:
    service: {label: dev.example.wall.client, noun: Client}

menu:
  - outlet
  - separator
  - outlet: controls
  - separator
  - {text: Quit, quit: true}
```

```state both running
daemon: {agent: {running: true, pid: 4821}}
client: {agent: {running: true, pid: 4822}}
```

```state client stopped
daemon: {agent: {running: true, pid: 4821}}
client: {agent: {installed: true}}
```

Each entry takes exactly one key, the template, and its arguments. A use's name
is an object in any expression: its watches and its states are fields —
`daemon.agent.pid`, `client.stopped`.

### Writing a template

A template file takes `params`, `watch`, `state`, `status` and `menu`, and
nothing else. Templates do not nest.

```yaml template
params:
  url: ~            # ~ is required
  noun: Server      # anything else is the default

watch:
  health:
    http: ${url}

state:
  - down: "!self.health.ok"
  - up:

menu:
  default:
    - {text: "${noun} is down", when: self.down}
```

| Key | Takes |
|---|---|
| `params` | A mapping of names to defaults. `~` makes one required. |
| `watch`, `state` | The same as in a `menubar.yaml`, but the use's own. |
| `status`, `menu` | A mapping of [outlet](#outlets) names to lists. `default` is the default outlet. |

`${name}` fills in a parameter when perch builds the app, on each string value
after the template is parsed, so a value holding `: ` stays one string. `$${`
writes a literal `${`, and any other `$` is left alone, so shell text like
`$HOME` passes through; a malformed `${` is a build error. Inside `[ ]` or
`{ }`, quote it: `{http: "${url}"}`. `{{ }}` is untouched.

Inside a template, `self` is that use, and the only name it sees: a template
cannot come to depend on a file it was not written for. Each use gets its own
state list, so exactly one of its states holds whatever the file's do, and the
file's `state:` may name a use's states.

A status rule in a template needs a `when:`: it would otherwise land ahead of
the file's rules and shadow every one of them.

### Outlets

`- outlet` in a file's `menu:` or `status:` is the default outlet;
`- outlet: controls` is a named one. Each use's fragments land at the matching
outlet, in `use:` order. A fragment whose outlet the file does not declare goes
to the default outlet, and with no default declared, that is the end of `menu:`
and the start of `status:`. A named outlet no use fills is refused, which is
what catches `control` written for `controls`.

### `service`

A LaunchAgent: whether it is installed, loaded and running, and the controls to
run it. It takes `label`, required, and `noun`, which defaults to `Service`.

| Part | Is |
|---|---|
| `self.agent` | A [`launchagent`](#launchagents) watch on `label`. |
| States | `uninstalled`, `stopped`, `idle`, `running`. |
| `status` default | A warning icon while uninstalled. |
| `menu` default | One readout line: `<noun>: running · pid 4821`. |
| `menu` controls | Start, Restart and Stop, each shown only when it can work. |

To change its labels, copy it into `menubar/templates/` under another name —
`perch schema -template -o menubar/templates/schema.json` gives an editor its
schema.
````

- [ ] **Step 3: `menu` and Builtins reflect paths and defaults**

In `docs/schema.md`'s `menu` chapter:
- ``The only bare item is `separator`.`` becomes ``The bare items are `separator` and [`outlet`](#outlets).``
- The `agent` row becomes: `` | `agent` | A string, `<watch>.<verb>` — or `<use>.<watch>.<verb>`, or `self.<watch>.<verb>` inside a template — where the watch is a `launchagent` watch and `<verb>` is `start`, `stop` or `restart`. | ``

In the Builtins chapter's `### Handled without asking` table, add a row:

```markdown
| An `agent:` item with no `text:` is labeled Start, Stop or Restart, and shows only when its verb can work. | [LaunchAgents](#launchagents) |
```

In `### LaunchAgents`, after the paragraph beginning ``An `agent:` also works as a button``, add:

```markdown
An `agent:` item shows only when its verb can work — Start while installed and
not loaded, Stop while loaded, Restart while installed — and a `when:` you write
is combined with that. With no `text:`, it is labeled Start, Stop or Restart.
The readout and all three controls are also the shipped [`service`](#service)
template.
```

- [ ] **Step 4: The recipe on `service`**

Replace `docs/recipes/launchagent.md` whole with:

````markdown
# Start and stop a LaunchAgent

A background job you own, with a light for what it is doing and the controls to
run it.

```yaml
app:
  name: worker
  id: dev.example.worker.menubar
  icon: gearshape
  interval: 10s

use:
  worker:
    service: {label: dev.example.worker, noun: Worker}

status:
  - outlet
  - {when: "worker.stopped || worker.idle", dim: true}

menu:
  - outlet
  - separator
  - outlet: controls
  - separator
  - {text: Quit, quit: true}
```

```state running
worker: {agent: {running: true, pid: 4821}}
```

```state loaded, not running
worker: {agent: {loaded: true}}
```

```state not loaded
worker: {agent: {installed: true}}
```

```state not installed
worker: {agent: {installed: false}}
```

The label is written once. [`service`](../schema.md#service) watches it, keeps
its own four states, and puts a readout at the default outlet and Start,
Restart and Stop at `controls` — each shown only when it can work, so the menu
never offers Start on a job launchd already holds.

The states are fields of the use: `worker.running`, `worker.stopped`. The watch
is `worker.agent`, and for a `launchctl` call perch does not write,
`worker.agent.target` is `gui/<your uid>/dev.example.worker`:

```
  - text: Why is it running?
    when: "worker.agent.loaded"
    run: [launchctl, blame, "{{worker.agent.target}}"]
```

**Loaded and running are different questions.** `launchctl print` succeeds for
any label launchd is holding, including a job that has already run and exited,
so a widget reading its exit status says Running about a job with no process.
[LaunchAgents](../schema.md#launchagents) has the rest.

Every action re-polls as soon as it finishes, which is what makes Start feel
like it did something: the menu that reopens says running.
````

- [ ] **Step 5: Point the LaunchAgent design at this**

In `docs/superpowers/specs/2026-09-12-launchagent-watch-design.md`, after the paragraph beginning ``The same reasoning retires `control:` ``, add:

```markdown
**Superseded in 2.0.** [Templates](2026-09-14-templates-design.md) answer these
objections: a template's guards name its own state list, a template is a file
an author copies to relabel, and outlets leave placement to the file. The
shipped `service` template is what `control:` was for.
```

- [ ] **Step 6: Run the docs checks**

Run: `go test ./internal/site/ ./internal/e2e/ -run 'Documented|Site|Build|Links'`
Expected: PASS — every `yaml` fence parses, emits and typechecks; every schema section is published; the site builds with its previews.

Run: `go run ./cmd/site -o _site && ls _site/menubar/use`
Expected: `index.html`.

- [ ] **Step 7: Commit**

```bash
git add docs internal/site
git commit -m "document use:, outlets and the service template"
```

---

### Task 11: Module path `/v2`, and the gate

**Files:**
- Modify: `go.mod`, every `.go` file importing the module, `README.md`, `docs/guide/install.md`, `docs/guide/overview.md`, `docs/superpowers/specs/2026-09-14-templates-design.md`, this plan

Go ignores a `v2.0.0` tag for `@latest` unless the module path ends in `/v2`.

- [ ] **Step 1: Look before rewriting**

Run: `grep -rn --include='*.go' 'orochi235/perch/' . | grep -v '"github.com/orochi235/perch/\(internal\|cmd\)/'`
Expected: no output. Anything listed is a URL or string that the rewrite below would wrongly change — handle it by hand first.

- [ ] **Step 2: Rewrite**

```bash
go mod edit -module github.com/orochi235/perch/v2
grep -rl --include='*.go' '"github.com/orochi235/perch/' . | xargs sed -i '' 's#"github.com/orochi235/perch/#"github.com/orochi235/perch/v2/#'
sed -i '' 's#go install github.com/orochi235/perch/cmd/perch@latest#go install github.com/orochi235/perch/v2/cmd/perch@latest#' README.md docs/guide/install.md docs/guide/overview.md
```

Run: `grep -rn --include='*.go' '"github.com/orochi235/perch/' . | grep -vc '/v2/'`
Expected: `0`.

- [ ] **Step 3: Build and vet**

Run: `go build ./... && go vet ./... && test -z "$(gofmt -l ./cmd ./internal)"`
Expected: no output, exit 0.

- [ ] **Step 4: Mark the spec and this plan built**

In `docs/superpowers/specs/2026-09-14-templates-design.md`, replace the opening status paragraph with:

```markdown
**Status: built.** `use:`, outlets, `self`, verb defaults and the shipped
`service` template land in perch 2.0. The second spec — code-backed built-ins,
and a `services` built-in combining several services' state and controls — is
not written.
```

In this plan, change `**Status: not started.**` to `**Status: built.**`.

- [ ] **Step 5: The full suite — the pre-push gate**

Check nothing else is running one: `ps aux | grep -E 'go test|\.test ' | grep -v grep` — expect no output.

Run: `go test ./... -timeout 10m 2>&1 | grep -vE '^ok|no test files'; echo "exit ${pipestatus[1]}"`
Expected: `exit 0` and nothing else printed. A timeout or OOM in a package this plan did not touch, on a loaded machine, is contention: say so rather than re-running for a clean number.

- [ ] **Step 6: Commit**

```bash
git add -A go.mod go.sum cmd internal README.md docs
git commit -m "move the module to /v2"
```

Tagging `v2.0.0` and pushing are the maintainer's call, not part of this plan.
