package shape

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A list whose element does not fit on one line has to be written as a block
// sequence; flow style would have to nest a mapping inside a mapping inline.
func TestInferBlockSequenceForANestedElement(t *testing.T) {
	got, err := Infer([]byte(`{"xs": [{"a": {"b": 1}, "c": 2}]}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	want := "xs:\n  -\n    a: {b: int}\n    c: int\n"
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

func TestInferNestsListsInsideLists(t *testing.T) {
	got, err := Infer([]byte(`{"grid": [[{"a": {"b": 1}}]]}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	want := "grid:\n  -\n    -\n      a: {b: int}\n"
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

// A field seen as both an int and a double is a double, not any: widening to
// any would cost the author every arithmetic comparison on it.
func TestInferWidensIntAndDoubleToDouble(t *testing.T) {
	for _, doc := range []string{
		`{"xs": [1, 1.5]}`,
		`{"xs": [1.5, 1]}`,
	} {
		got, err := Infer([]byte(doc))
		if err != nil {
			t.Fatalf("Infer(%s): %v", doc, err)
		}
		if got != "xs: [double]\n" {
			t.Errorf("%s: got %q, want double", doc, got)
		}
	}
}

// null carries no type, and a field that is sometimes null is exactly the one a
// widget branches on, so it must widen rather than take the other sample's type.
func TestInferTreatsNullAsAny(t *testing.T) {
	got, err := InferAll([][]byte{
		[]byte(`{"err": null}`),
		[]byte(`{"err": "boom"}`),
	})
	if err != nil {
		t.Fatalf("InferAll: %v", err)
	}
	if got != "err: any\n" {
		t.Errorf("got %q, want any", got)
	}
}

func TestInferUnifiesNestedListElements(t *testing.T) {
	got, err := Infer([]byte(`{"xs": [[1], [1.5]]}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if got != "xs: [[double]]\n" {
		t.Errorf("got %q", got)
	}
}

// Two documents in one sample is a stream, not a document; taking the first and
// ignoring the rest would infer a shape from part of the output.
func TestInferRejectsTrailingContent(t *testing.T) {
	for _, doc := range []string{`{"a": 1} {"b": 2}`, `{"a": 1} trailing`, `{"a": 1}]`} {
		if got, err := Infer([]byte(doc)); err == nil {
			t.Errorf("%s: want an error, inferred %q", doc, got)
		}
	}
}

func TestInferRejectsTruncatedJSON(t *testing.T) {
	for _, doc := range []string{`{`, `{"a":`, `{"a": [1,`, `[`, ``} {
		if got, err := Infer([]byte(doc)); err == nil {
			t.Errorf("%q: want an error, inferred %q", doc, got)
		}
	}
}

// The document is what a command printed, so every non-object top level has to
// say so rather than emit a shape: block the author cannot use.
func TestInferRejectsEveryNonObjectDocument(t *testing.T) {
	for _, doc := range []string{`3`, `"x"`, `true`, `null`, `[{"a": 1}]`} {
		if got, err := Infer([]byte(doc)); err == nil {
			t.Errorf("%s: want an error, inferred %q", doc, got)
		}
	}
}

// Whitespace around the document is what a command's own trailing newline looks
// like, so it cannot be an error.
func TestInferIgnoresSurroundingWhitespace(t *testing.T) {
	got, err := Infer([]byte("\n\t {\"a\": 1}  \n\n"))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if got != "a: int\n" {
		t.Errorf("got %q", got)
	}
}

// YAML refuses a raw control character even inside quotes, so a command that
// prints one in a key must still yield a block that parses.
func TestInferEscapesControlCharactersInKeys(t *testing.T) {
	got, err := Infer([]byte("{\"a\u007fb\": 1, \"c\u0085d\": 2}"))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	want := "\"a\\x7fb\": int\n\"c\\x85d\": int\n"
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	var into map[string]string
	if err := yaml.Unmarshal([]byte(got), &into); err != nil {
		t.Fatalf("the escaped block is not valid YAML: %v", err)
	}
	if _, ok := into["a\u007fb"]; !ok {
		t.Errorf("the key did not survive escaping: %q", into)
	}
	// It parses, so perch build can refuse it against the line it came from.
	if _, err := reparse(t, got); err == nil {
		t.Error("want perch build to refuse a control character in a field name")
	}
}

// Past 1024 characters YAML needs the explicit ? key form, which a shape: block
// is not written in. Refusing names the key; emitting it would produce a block
// that fails to parse several lines away from the cause.
func TestInferRejectsAKeyTooLongForYAML(t *testing.T) {
	long := strings.Repeat("k", 1025)
	if got, err := Infer([]byte(`{"` + long + `": 1}`)); err == nil {
		t.Errorf("want an error, inferred %q", got)
	}
	if _, err := Infer([]byte(`{"` + strings.Repeat("k", 1024) + `": 1}`)); err != nil {
		t.Errorf("a 1024-character key is still usable: %v", err)
	}
}

// A command whose result set is empty prints {}, and the block it produces has
// to be one an author can paste.
func TestInferWritesAnEmptyObjectAsBraces(t *testing.T) {
	got, err := Infer([]byte(`{}`))
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	if got != "{}\n" {
		t.Errorf("got %q, want {}", got)
	}
	if _, err := reparse(t, got); err != nil {
		t.Errorf("the empty block does not parse back: %v", err)
	}
}

// Every escape class, because one that comes out wrong makes the whole block
// unparseable rather than just the line it is on.
func TestInferKeyEscapes(t *testing.T) {
	cases := []struct{ key, want string }{
		{"plain", "plain"},
		{"_a1", "_a1"},
		{"content-type", `"content-type"`},
		{"1leading", `"1leading"`},
		{"", `""`},
		{`say "hi"`, `"say \"hi\""`},
		{`back\slash`, `"back\\slash"`},
		{"tab\there", `"tab\there"`},
		{"nl\nhere", `"nl\nhere"`},
		{"cr\rhere", `"cr\rhere"`},
		{"nul\x00here", `"nul\x00here"`},
		{"del\u007f", `"del\x7f"`},
		{"c1\u009f", `"c1\x9f"`},
		{"noncharacter\ufffe", `"noncharacter\ufffe"`},
		{"emoji \U0001F600", "\"emoji \U0001F600\""},
	}
	for _, c := range cases {
		if got := yamlKey(c.key); got != c.want {
			t.Errorf("yamlKey(%q)\n got %s\nwant %s", c.key, got, c.want)
		}
	}
	// All of them together still have to parse back to the keys they came from.
	var b strings.Builder
	for _, c := range cases {
		b.WriteString(c.want + ": int\n")
	}
	var into map[string]string
	if err := yaml.Unmarshal([]byte(b.String()), &into); err != nil {
		t.Fatalf("escaped keys are not valid YAML: %v", err)
	}
	for _, c := range cases {
		if _, ok := into[c.key]; !ok {
			t.Errorf("key %q did not survive: %q", c.key, into)
		}
	}
}

// JSON allows a key twice. YAML's simple-key form does not, so the two
// occurrences unify into one field rather than being printed twice.
func TestInferUnifiesARepeatedKey(t *testing.T) {
	cases := []struct{ doc, want string }{
		{`{"a": 1, "a": 2}`, "a: int\n"},
		{`{"a": 1, "a": "x"}`, "a: any\n"},
		{`{"a": 1, "b": 2, "a": 1.5}`, "a: double\nb: int\n"},
		{`{"a": {"x": 1}, "a": {"y": 2}}`, "a: {x: int, y: int}\n"},
	}
	for _, c := range cases {
		got, err := Infer([]byte(c.doc))
		if err != nil {
			t.Errorf("%s: %v", c.doc, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s\n got %q\nwant %q", c.doc, got, c.want)
		}
	}
}

// A name perch build refuses is still printed, bare or quoted as the key
// requires, so the refusal lands on the line the author is looking at rather
// than on a field that silently went missing.
func TestInferPrintsNamesPerchBuildWillRefuse(t *testing.T) {
	for _, c := range []struct{ doc, want string }{
		{`{"_": 1}`, "_: int\n"},
		{`{"package": 1}`, "package: int\n"},
		{`{"in": 1}`, "in: int\n"},
		{`{"content-type": 1}`, `"content-type": int` + "\n"},
	} {
		got, err := Infer([]byte(c.doc))
		if err != nil {
			t.Errorf("%s: %v", c.doc, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s\n got %q\nwant %q", c.doc, got, c.want)
		}
		if _, err := reparse(t, got); err == nil {
			t.Errorf("%s: perch build accepted a name it should refuse", c.doc)
		}
	}
}
