package schema

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The schema is written by hand beside the parser, in a different notation, and
// nothing at run time makes the two agree: a key added to one and not the other
// shows as a spurious editor error or as completion for a key perch refuses.
// These are the structs decodeStrict validates against, and where the schema
// states the same key set.
var sections = map[string]string{
	"rawSpec":       "properties",
	"rawApp":        "properties.app.properties",
	"rawWatch":      "properties.watch.additionalProperties.properties",
	"rawStatusRule": "properties.status.items.oneOf.0.properties",
	"rawWindow":     "properties.window.properties",
	"rawQuitRule":   "properties.app.properties.quit.items.properties",
	"rawZoom":       "properties.window.properties.zoom.properties",
	"itemFields":    "definitions.menu.items.oneOf.1.properties",
	"rawPost":       "definitions.menu.items.oneOf.1.properties.post.properties",
}

func TestSchemaDeclaresExactlyTheKeysTheParserAccepts(t *testing.T) {
	fromGo := yamlTags(t)
	doc := decoded(t)
	for structName, path := range sections {
		want, ok := fromGo[structName]
		if !ok {
			t.Errorf("internal/spec no longer declares %s; this table is stale", structName)
			continue
		}
		got := keysAt(t, doc, path)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s at %s\n schema %v\n parser %v", structName, path, got, want)
		}
	}
}

// yamlTags reads the yaml key of every field of every struct in internal/spec,
// which is the set decodeStrict accepts.
func yamlTags(t *testing.T) map[string][]string {
	t.Helper()
	pkgs, err := parser.ParseDir(token.NewFileSet(), "../spec", nil, 0)
	if err != nil {
		t.Fatalf("reading internal/spec: %v", err)
	}
	out := map[string][]string{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return true
				}
				var keys []string
				for _, f := range st.Fields.List {
					if f.Tag == nil {
						continue
					}
					tag, err := strconv.Unquote(f.Tag.Value)
					if err != nil {
						continue
					}
					if key := reflect.StructTag(tag).Get("yaml"); key != "" {
						keys = append(keys, strings.Split(key, ",")[0])
					}
				}
				if len(keys) > 0 {
					sort.Strings(keys)
					out[ts.Name.Name] = keys
				}
				return true
			})
		}
	}
	return out
}

// walk follows a dotted path; a numeric segment indexes a list.
func walk(t *testing.T, doc any, path string) any {
	t.Helper()
	node := doc
	for _, seg := range strings.Split(path, ".") {
		switch container := node.(type) {
		case map[string]any:
			next, ok := container[seg]
			if !ok {
				t.Fatalf("%s: the schema has no %q", path, seg)
			}
			node = next
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(container) {
				t.Fatalf("%s: %q does not index a list of %d", path, seg, len(container))
			}
			node = container[i]
		default:
			t.Fatalf("%s: %q has nothing under it", path, seg)
		}
	}
	return node
}

// keysAt walks to a properties object and returns the keys it declares.
func keysAt(t *testing.T, doc map[string]any, path string) []string {
	t.Helper()
	m, ok := walk(t, doc, path).(map[string]any)
	if !ok {
		t.Fatalf("%s: not a properties object", path)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Every mapping the schema describes is closed, because perch refuses an
// unknown key everywhere and an editor that accepts one is worse than silent.
func TestSchemaClosesEveryMappingTheParserCloses(t *testing.T) {
	doc := decoded(t)
	for structName, path := range sections {
		parent := strings.TrimSuffix(path, ".properties")
		if parent == path {
			continue // the root, checked by TestSchemaRefusesUnknownTopLevelKeys
		}
		m, ok := walk(t, doc, parent).(map[string]any)
		if !ok {
			t.Fatalf("%s: %s is not a schema object", structName, parent)
		}
		if m["additionalProperties"] != false {
			t.Errorf("%s at %s does not set additionalProperties: false", structName, parent)
		}
	}
}

// app.interval is a Go duration, which the schema restates as a pattern. The
// pattern can only check the spelling: perch additionally requires the duration
// to be positive, which no pattern of this shape can express.
func TestSchemaIntervalPatternMatchesWhatGoParsesAsADuration(t *testing.T) {
	rule := nameRule(t, "properties", "app", "properties", "interval")
	for _, v := range []string{"5s", "1m30s", "500ms", "1.5h", "1\u00b5s", "1us", "0s", "1ns", "2h45m", "5", "s", "5 s", "", "5sec", "1d"} {
		_, err := time.ParseDuration(v)
		if parses, matches := err == nil, rule(v); parses != matches {
			t.Errorf("%q: Go parses it = %v, the schema takes it = %v", v, parses, matches)
		}
	}
	// Negative durations spell correctly and are still refused, so the pattern
	// is deliberately narrower than time.ParseDuration here.
	if rule("-1s") {
		t.Error("the schema takes a negative interval")
	}
}

// The schema's shape enum and the parser's scalar table are the same list
// written twice.
func TestSchemaShapeScalarsAgreeWithTheParser(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(JSON()), &doc); err != nil {
		t.Fatal(err)
	}
	one := doc["definitions"].(map[string]any)["shape"].(map[string]any)["oneOf"].([]any)[0]
	var got []string
	for _, v := range one.(map[string]any)["enum"].([]any) {
		got = append(got, v.(string))
	}
	sort.Strings(got)
	want := []string{"any", "bool", "double", "int", "string"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("schema shape scalars = %v, want %v", got, want)
	}
	for _, name := range append(append([]string{}, want...), "float", "number", "object", "list") {
		allowed := func(v string) bool {
			for _, w := range want {
				if v == w {
					return true
				}
			}
			return false
		}
		agree(t, name, allowed, `
app: {name: a, id: dev.a, icon: circle, interval: 1s}
watch: {w: {run: [x], json: true, shape: {f: `+name+`}}}
menu: [{text: Q, quit: true}]
`)
	}
}

func TestTemplateSchemaDeclaresExactlyTheKeysATemplateTakes(t *testing.T) {
	doc := templateDecoded(t)
	got := keysAt(t, doc, "properties")
	if want := yamlTags(t)["rawTemplate"]; !reflect.DeepEqual(got, want) {
		t.Errorf("template schema %v\n parser %v", got, want)
	}
	if doc["additionalProperties"] != false {
		t.Error("the template schema accepts unknown keys")
	}
}

// A fragment's own status rules cannot place another use's fragment, so the
// item is just the rule object, with when: required rather than optional.
func TestTemplateStatusItemsHaveNoOutletAndRequireWhen(t *testing.T) {
	doc := templateDecoded(t)
	items, ok := walk(t, doc, "properties.status.additionalProperties.items").(map[string]any)
	if !ok {
		t.Fatal("properties.status.additionalProperties.items is not an object")
	}
	if _, has := items["oneOf"]; has {
		t.Error("template status items still offer an outlet mark")
	}
	if req, _ := items["required"].([]any); len(req) != 1 || req[0] != "when" {
		t.Errorf("template status items required = %v, want [\"when\"]", items["required"])
	}
}

// A fragment's own menu items cannot place another use's fragment either, so
// templateMenu's oneOf is just separator and the item object — no outlet ref.
func TestTemplateMenuHasNoTopLevelOutlet(t *testing.T) {
	doc := templateDecoded(t)
	oneOf, ok := walk(t, doc, "definitions.templateMenu.items.oneOf").([]any)
	if !ok || len(oneOf) != 2 {
		t.Fatalf("templateMenu.items.oneOf = %v, want [separator, item]", oneOf)
	}
	sep, ok := oneOf[0].(map[string]any)
	if !ok || !reflect.DeepEqual(sep["enum"], []any{"separator"}) {
		t.Errorf("templateMenu.items.oneOf[0] = %v, want the separator enum", oneOf[0])
	}
	item, ok := oneOf[1].(map[string]any)
	if !ok || item["properties"] == nil {
		t.Errorf("templateMenu.items.oneOf[1] = %v, want the menu item object", oneOf[1])
	}
	for _, v := range oneOf {
		if m, ok := v.(map[string]any); ok && m["$ref"] != nil {
			t.Errorf("templateMenu.items.oneOf offers %v; a fragment's top level takes no outlet mark", m["$ref"])
		}
	}
}

func TestTemplateParamsAllowScalarsAndNull(t *testing.T) {
	doc := templateDecoded(t)
	types, ok := walk(t, doc, "properties.params.additionalProperties.type").([]any)
	if !ok {
		t.Fatal("properties.params.additionalProperties.type is not a list")
	}
	got := make([]string, len(types))
	for i, v := range types {
		got[i] = v.(string)
	}
	sort.Strings(got)
	if want := []string{"boolean", "null", "number", "string"}; !reflect.DeepEqual(got, want) {
		t.Errorf("template params value types = %v, want %v", got, want)
	}
}

func TestTemplateOutletMapKeysArePlainNames(t *testing.T) {
	doc := templateDecoded(t)
	for _, path := range []string{"properties.status.propertyNames", "properties.menu.propertyNames"} {
		rule, ok := walk(t, doc, path).(map[string]any)
		if !ok {
			t.Fatalf("%s is not an object", path)
		}
		if rule["pattern"] != "^[A-Za-z_][A-Za-z0-9_]*$" {
			t.Errorf("%s pattern = %v", path, rule["pattern"])
		}
	}
}
