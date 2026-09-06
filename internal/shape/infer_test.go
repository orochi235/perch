package shape

import (
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
	"gopkg.in/yaml.v3"
)

func TestInferScalars(t *testing.T) {
	got, err := Infer([]byte(`{"n": 3, "d": 1.5, "s": "x", "b": true}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	want := "n: int\nd: double\ns: string\nb: bool\n"
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestInferKeepsDocumentOrder(t *testing.T) {
	got, err := Infer([]byte(`{"z": 1, "a": 2, "m": 3}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if got != "z: int\na: int\nm: int\n" {
		t.Errorf("got %q, want the order the command printed", got)
	}
}

// The design doc's own example, from what `onto top --once --json` would print.
func TestInferListOfObjectsUsesFlowStyle(t *testing.T) {
	got, err := Infer([]byte(`{
	  "nodes": [{"name": "studio", "up": true}],
	  "jobs": [{"id": "j1", "node": "studio", "cmd": "build"}]
	}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	want := "nodes: [{name: string, up: bool}]\n" +
		"jobs: [{id: string, node: string, cmd: string}]\n"
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestInferUnifiesDisagreeingElementsAsAny(t *testing.T) {
	got, err := Infer([]byte(`{"xs": [{"a": 1}, {"a": "one"}]}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if got != "xs: [{a: any}]\n" {
		t.Errorf("got %q, want the disagreeing field widened to any", got)
	}
}

func TestInferUnionsFieldsAcrossElements(t *testing.T) {
	got, err := Infer([]byte(`{"xs": [{"a": 1}, {"b": "two"}]}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if got != "xs: [{a: int, b: string}]\n" {
		t.Errorf("got %q, want fields from every element", got)
	}
}

func TestInferEmptyListIsAny(t *testing.T) {
	got, err := Infer([]byte(`{"xs": []}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if got != "xs: [any]\n" {
		t.Errorf("got %q", got)
	}
}

func TestInferNestedObjectUsesBlockStyle(t *testing.T) {
	got, err := Infer([]byte(`{"a": {"b": {"c": 1}}}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	want := "a:\n  b: {c: int}\n"
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestInferRejectsANonObjectDocument(t *testing.T) {
	if _, err := Infer([]byte(`[1, 2, 3]`)); err == nil {
		t.Fatal("want an error: a shape describes an object, got nil")
	}
}

func TestInferRejectsInvalidJSON(t *testing.T) {
	if _, err := Infer([]byte(`not json`)); err == nil {
		t.Fatal("want an error for invalid JSON, got nil")
	}
}

// An inferred shape is only useful if perch can read it back.
func TestInferredShapeParsesBackAsASpec(t *testing.T) {
	for _, doc := range []string{
		`{"nodes": [{"name": "studio", "up": true}], "jobs": [{"id": "j1", "cmd": "b"}]}`,
		`{"a": {"b": {"c": 1}}, "d": 1.5, "e": []}`,
		`{"xs": [{"a": 1}, {"b": "two"}]}`,
	} {
		body, err := Infer([]byte(doc))
		if err != nil {
			t.Fatalf("Infer(%s): %v", doc, err)
		}
		indented := "      " + strings.ReplaceAll(strings.TrimRight(body, "\n"), "\n", "\n      ")
		yaml := "app: {name: a, id: b, icon: circle, interval: 1s}\n" +
			"watch:\n  w:\n    run: [x]\n    json: true\n    shape:\n" + indented + "\n" +
			"menu: [{text: Quit, quit: true}]\n"
		s, err := spec.Parse([]byte(yaml))
		if err != nil {
			t.Errorf("inferred shape does not parse back:\n%s\n%v", yaml, err)
			continue
		}
		if s.Watches[0].Shape == nil {
			t.Errorf("shape was dropped for %s", doc)
		}
	}
}

// One healthy sample misses exactly the fields a widget most needs: the
// failure-path ones, omitted when nothing is wrong.
func TestInferAllUnionsFieldsAcrossSamples(t *testing.T) {
	got, err := InferAll([][]byte{
		[]byte(`{"jobs": [{"id": "j1"}]}`),
		[]byte(`{"jobs": [{"id": "j2", "error": "boom"}], "pruned": 3}`),
	})
	if err != nil {
		t.Fatalf("InferAll: %v", err)
	}
	want := "jobs: [{id: string, error: string}]\npruned: int\n"
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestInferAllWidensDisagreementAcrossSamples(t *testing.T) {
	got, err := InferAll([][]byte{
		[]byte(`{"n": 1}`),
		[]byte(`{"n": "one"}`),
	})
	if err != nil {
		t.Fatalf("InferAll: %v", err)
	}
	if got != "n: any\n" {
		t.Errorf("got %q, want the field widened to any", got)
	}
}

func TestInferAllNeedsAtLeastOneSample(t *testing.T) {
	if _, err := InferAll(nil); err == nil {
		t.Fatal("want an error for no samples, got nil")
	}
}

// A key perch cannot use as an identifier still has to come out as valid YAML,
// so perch build reports it against the line the author is looking at.
func TestInferQuotesKeysThatAreNotIdentifiers(t *testing.T) {
	got, err := Infer([]byte(`{"content-type": "json", "a: b": 1, "": 2, "ok": true}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	want := "\"content-type\": string\n\"a: b\": int\n\"\": int\nok: bool\n"
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	var into map[string]any
	if err := yaml.Unmarshal([]byte(got), &into); err != nil {
		t.Fatalf("inferred shape is not valid YAML: %v", err)
	}
}
