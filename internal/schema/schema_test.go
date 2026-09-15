package schema

import (
	"encoding/json"
	"regexp"
	"strconv"
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

func decoded(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(JSON()), &m); err != nil {
		t.Fatalf("emitted schema is not valid JSON: %v", err)
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
	var node any = decoded(t)
	for _, key := range path {
		switch container := node.(type) {
		case map[string]any:
			next, ok := container[key]
			if !ok {
				t.Fatalf("%v: no %q", path, key)
			}
			node = next
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i >= len(container) {
				t.Fatalf("%v: %q does not index a list of %d", path, key, len(container))
			}
			node = container[i]
		default:
			t.Fatalf("%v: not a schema object", path)
		}
	}
	sub, ok := node.(map[string]any)
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
	for _, name := range []string{"init", "Type", "Protocol", "self", "widget"} {
		agree(t, name, stateName, `
app: {name: a, id: dev.a, icon: circle, interval: 1s}
state: [{`+strconv.Quote(name)+`: null}]
menu: [{text: Q, quit: true}]
`)
	}

	fieldName := nameRule(t, "definitions", "shape", "oneOf", "2", "propertyNames")
	for _, name := range []string{"init", "Type", "Protocol", "self", "count"} {
		agree(t, name, fieldName, `
app: {name: a, id: dev.a, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {`+strconv.Quote(name)+`: string}}}
menu: [{text: Q, quit: true}]
`)
	}
}

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

func agree(t *testing.T, value string, allowed func(string) bool, doc string) {
	t.Helper()
	_, err := spec.Parse([]byte(doc))
	if takes, matches := err == nil, allowed(value); takes != matches {
		t.Errorf("%q: perch takes it = %v, the schema takes it = %v (%v)", value, takes, matches, err)
	}
}
