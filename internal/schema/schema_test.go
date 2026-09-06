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
		m, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%v: not a schema object", path)
		}
		if node, ok = m[key]; !ok {
			t.Fatalf("%v: no %q", path, key)
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
	for _, name := range []string{"fleet", "my_fleet", "my-fleet", "9x", "it", "package", "size"} {
		agree(t, name, watchName, `
app: {name: a, id: dev.a, icon: circle, interval: 1s}
watch: {`+strconv.Quote(name)+`: {run: [x]}}
menu: [{text: Q, quit: true}]
`)
	}
}

func agree(t *testing.T, value string, allowed func(string) bool, doc string) {
	t.Helper()
	_, err := spec.Parse([]byte(doc))
	if takes, matches := err == nil, allowed(value); takes != matches {
		t.Errorf("%q: perch takes it = %v, the schema takes it = %v (%v)", value, takes, matches, err)
	}
}
