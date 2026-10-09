package schema

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/orochi235/perch/v2/internal/spec"
	"github.com/orochi235/perch/v2/internal/templates"
	"gopkg.in/yaml.v3"
)

func decoded(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(JSON()), &m); err != nil {
		t.Fatalf("emitted schema is not valid JSON: %v", err)
	}
	return m
}

func templateDecoded(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(TemplateJSON()), &m); err != nil {
		t.Fatalf("TemplateJSON is not valid JSON: %v", err)
	}
	return m
}

func TestSchemaIsValidJSON(t *testing.T) { decoded(t) }

func TestSchemaCoversEveryTopLevelKey(t *testing.T) {
	props, ok := decoded(t)["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema has no properties")
	}
	for _, key := range []string{"app", "watch", "status", "menu"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema does not describe %q", key)
		}
	}
}

func TestSchemaRefusesUnknownTopLevelKeys(t *testing.T) {
	if decoded(t)["additionalProperties"] != false {
		t.Error("schema allows unknown top-level keys; perch itself rejects them")
	}
}

// nameRule reads one of the schema's name constraints back out: a pattern, and
// the enum of words a "not" excludes.
func nameRule(t *testing.T, path ...string) func(string) bool {
	t.Helper()
	dotted := strings.Join(path, ".")
	sub, ok := walk(t, decoded(t), dotted).(map[string]any)
	if !ok {
		t.Fatalf("%v: not a schema object", path)
	}
	src, ok := sub["pattern"].(string)
	if !ok {
		t.Fatalf("%v: no pattern", path)
	}
	re, err := regexp.Compile(src)
	if err != nil {
		t.Fatalf("%v: %v", path, err)
	}
	denied := map[string]bool{}
	if not, ok := sub["not"].(map[string]any); ok {
		list, ok := not["enum"].([]any)
		if !ok {
			t.Fatalf("%v: a not without an enum", path)
		}
		for _, v := range list {
			denied[v.(string)] = true
		}
	}
	return func(v string) bool { return re.MatchString(v) && !denied[v] }
}

// The schema exists so an editor refuses what perch would. Names are where the
// two drift, because each states the rule in its own notation.
func TestSchemaPatternsAgreeWithTheParser(t *testing.T) {
	appName := nameRule(t, "properties", "app", "properties", "name")
	for _, name := range []string{"onto", "my app", "../x", ".hidden", "a/b"} {
		agree(t, name, appName, `
app: {name: `+strconv.Quote(name)+`, id: dev.a, icon: circle, interval: 1s}
menu: [{text: Q, quit: true}]
`)
	}

	appID := nameRule(t, "properties", "app", "properties", "id")
	for _, id := range []string{"dev.onto.menubar", "dev_a-b", "../evil", "dev/a", ".x"} {
		agree(t, id, appID, `
app: {name: a, id: `+strconv.Quote(id)+`, icon: circle, interval: 1s}
menu: [{text: Q, quit: true}]
`)
	}

	hotkey := nameRule(t, "properties", "app", "properties", "hotkey")
	for _, key := range []string{"cmd+shift+space", "ctrl+opt+k", "cmd+`", "cmd+\\", "cmd+]", "f13", "shift+f20", "f21", "f", "k", "shift+k", "cmd+shift", "Cmd+K", "cmd+enter"} {
		agree(t, key, hotkey, `
app: {name: a, id: dev.a, icon: circle, interval: 1s, hotkey: `+strconv.Quote(key)+`}
menu: [{text: Q, quit: true}]
`)
	}

	watchName := nameRule(t, "properties", "watch", "propertyNames")
	for _, name := range []string{"fleet", "my_fleet", "my-fleet", "9x", "it", "self", "package", "size", "init", "Type", "Protocol"} {
		agree(t, name, watchName, `
app: {name: a, id: dev.a, icon: circle, interval: 1s}
watch: {`+strconv.Quote(name)+`: {run: [x]}}
menu: [{text: Q, quit: true}]
`)
	}

	useName := nameRule(t, "properties", "use", "propertyNames")
	for _, name := range []string{"daemon", "my-daemon", "it", "self", "package", "init", "Type", "Protocol"} {
		agree(t, name, useName, `
app: {name: a, id: dev.a, icon: circle, interval: 1s}
use: {`+strconv.Quote(name)+`: {service: {label: dev.a.x}}}
menu: [{text: Q, quit: true}]
`)
	}

	stateName := nameRule(t, "properties", "state", "items", "propertyNames")
	for _, name := range []string{"init", "Type", "Protocol", "self", "package", "in", "widget"} {
		agree(t, name, stateName, `
app: {name: a, id: dev.a, icon: circle, interval: 1s}
state: [{`+strconv.Quote(name)+`: null}]
menu: [{text: Q, quit: true}]
`)
	}

	fieldName := nameRule(t, "definitions", "shape", "oneOf", "2", "propertyNames")
	for _, name := range []string{"init", "Type", "Protocol", "self", "package", "in", "it", "count"} {
		agree(t, name, fieldName, `
app: {name: a, id: dev.a, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {`+strconv.Quote(name)+`: string}}}
menu: [{text: Q, quit: true}]
`)
	}
}

func TestAgentPatternTakesPaths(t *testing.T) {
	doc := decoded(t)
	cases := map[string]bool{
		"worker.start": true, "daemon.agent.stop": true, "self.agent.restart": true,
		"worker": false, "a.b.c.start": false, "worker.reload": false,
	}
	for _, path := range []string{
		"definitions.menu.items.oneOf.1.properties.agent",
		"properties.app.properties.quit.items.properties.buttons.items.properties.agent",
	} {
		field, ok := walk(t, doc, path).(map[string]any)
		if !ok {
			t.Fatalf("%s: not a schema object", path)
		}
		re := regexp.MustCompile(field["pattern"].(string))
		for v, want := range cases {
			if re.MatchString(v) != want {
				t.Errorf("%s: %q: matches = %v, want %v", path, v, !want, want)
			}
		}
	}
}

// A template fills ${param} into every scalar before parsing, so its schema
// takes a hole wherever the menubar.yaml schema takes a scalar.
func TestTemplateScalarsAllowAParam(t *testing.T) {
	doc := templateDecoded(t)
	for path, values := range map[string]map[string]bool{
		"properties.watch.additionalProperties.properties.launchagent": {
			"${label}": true, "dev.x.${name}": true, "dev.example.worker": true, "dev example": false,
		},
		"properties.watch.additionalProperties.properties.json": {"${decode}": true, "yes": false},
		"definitions.tint": {"${color}": true, "teal": true, "plaid": false},
		"definitions.templateMenu.items.anyOf.0.oneOf.1.properties.window": {"${verb}": true, "open": true, "shut": false},
	} {
		for v, want := range values {
			if got := matchesString(t, doc, walk(t, doc, path), v); got != want {
				t.Errorf("%s: %q: matches = %v, want %v", path, v, got, want)
			}
		}
	}

	main := decoded(t)
	if matchesString(t, main, walk(t, main, "properties.watch.additionalProperties.properties.launchagent"), "${label}") {
		t.Error("JSON()'s launchagent takes a ${param}, which no launchd label holds")
	}
}

func TestTemplateLaunchAgentPatternTakesService(t *testing.T) {
	src, ok := templates.Source("service")
	if !ok {
		t.Fatal("perch ships no service template")
	}
	var doc struct {
		Watch struct {
			Agent struct {
				LaunchAgent string `yaml:"launchagent"`
			} `yaml:"agent"`
		} `yaml:"watch"`
	}
	if err := yaml.Unmarshal(src, &doc); err != nil {
		t.Fatal(err)
	}
	label := doc.Watch.Agent.LaunchAgent
	if label == "" {
		t.Fatal("service.yaml: watch.agent.launchagent is empty")
	}
	tmpl := templateDecoded(t)
	if !matchesString(t, tmpl, walk(t, tmpl, "properties.watch.additionalProperties.properties.launchagent"), label) {
		t.Errorf("the template schema refuses service.yaml's launchagent %q", label)
	}
}

// matchesString reports whether schema s takes the string v, for the keywords
// a string can meet in perch's schemas.
func matchesString(t *testing.T, doc map[string]any, s any, v string) bool {
	t.Helper()
	m, ok := s.(map[string]any)
	if !ok {
		t.Fatalf("schema %v is not an object", s)
	}
	if ref, ok := m["$ref"].(string); ok {
		return matchesString(t, doc, walk(t, doc, strings.ReplaceAll(strings.TrimPrefix(ref, "#/"), "/", ".")), v)
	}
	if branches, ok := m["anyOf"].([]any); ok {
		return slices.ContainsFunc(branches, func(b any) bool { return matchesString(t, doc, b, v) })
	}
	if branches, ok := m["oneOf"].([]any); ok {
		n := 0
		for _, b := range branches {
			if matchesString(t, doc, b, v) {
				n++
			}
		}
		return n == 1
	}
	switch ty := m["type"].(type) {
	case string:
		if ty != "string" {
			return false
		}
	case []any:
		if !slices.Contains(ty, any("string")) {
			return false
		}
	case nil:
		if m["enum"] == nil && m["pattern"] == nil {
			return false
		}
	}
	if enum, ok := m["enum"].([]any); ok && !slices.Contains(enum, any(v)) {
		return false
	}
	if p, ok := m["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(v) {
		return false
	}
	return true
}

func agree(t *testing.T, value string, allowed func(string) bool, doc string) {
	t.Helper()
	_, err := spec.Parse([]byte(doc))
	if takes, matches := err == nil, allowed(value); takes != matches {
		t.Errorf("%q: perch takes it = %v, the schema takes it = %v (%v)", value, takes, matches, err)
	}
}
