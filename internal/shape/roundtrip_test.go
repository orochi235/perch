package shape

import (
	"fmt"
	"strings"
	"testing"

	"github.com/orochi235/perch/internal/spec"
	"gopkg.in/yaml.v3"
)

// reparse puts an inferred body back into a menubar.yaml and reads the shape
// out again. `perch shape` prints a block to paste in, so anything it prints
// that perch build then rejects is worse than no output at all.
func reparse(t *testing.T, body string) (*spec.Type, error) {
	t.Helper()
	indented := "      " + strings.ReplaceAll(strings.TrimRight(body, "\n"), "\n", "\n      ")
	doc := "app: {name: a, id: b, icon: circle, interval: 1s}\n" +
		"watch:\n  w:\n    run: [x]\n    json: true\n    shape:\n" + indented + "\n" +
		"menu: [{text: Quit, quit: true}]\n"
	s, err := spec.Parse([]byte(doc))
	if err != nil {
		return nil, err
	}
	return s.Watches[0].Shape, nil
}

// The shapes a nested payload produces are the ones a hand-written test is
// least likely to try, and the ones the block-style writer gets wrong.
func TestInferredShapeSurvivesARoundTrip(t *testing.T) {
	for _, doc := range []string{
		`{}`,
		`{"a": 1}`,
		`{"nodes": [{"name": "studio", "up": true}]}`,
		`{"a": {"b": {"c": 1}}}`,
		`{"grid": [[1]]}`,
		`{"grid": [[{"a": {"b": 1}}]]}`,
		`{"xs": [{"a": {"b": 1}, "c": 2}]}`,
		`{"xs": [{"a": [{"b": [1]}]}]}`,
		`{"xs": []}`,
		`{"deep": {"a": [{"b": {"c": [{"d": 1}]}}]}}`,
	} {
		body, err := Infer([]byte(doc))
		if err != nil {
			t.Errorf("%s: %v", doc, err)
			continue
		}
		got, err := reparse(t, body)
		if err != nil {
			t.Errorf("%s inferred:\n%s\nwhich perch build rejects: %v", doc, body, err)
			continue
		}
		if again := renderObject(got); again != body {
			t.Errorf("%s does not round-trip:\n first %q\nsecond %q", doc, body, again)
		}
	}
}

// perch shape runs a command and prints whatever comes back, so the input is
// never trusted. The block it prints has to be YAML that says exactly what was
// inferred: a nested list written as a bare `-`, a control character left raw,
// or a key repeated all read back as something else, several lines from the
// cause.
//
// Whether perch build then accepts the names is a separate question. It refuses
// _, a hyphen and CEL's reserved words, and shape prints those on purpose so the
// refusal lands on the line the author is looking at.
func FuzzInfer(f *testing.F) {
	for _, seed := range []string{
		`{}`, `{"a": 1}`, `{"a": [1, "x", null]}`, `[]`, `null`, `not json`,
		`{"a": {"b": {"c": [[[1]]]}}}`, `{"": 1}`, `{"_": ""}`, `{"a": 1e999}`,
		`{"a": 1} {"b": 2}`, `{"a": 1}]`, "\x00", `{"a": "\ud800"}`,
		`{"a": 1, "a": "x"}`, `{"content-type": [{"a-b": 1}]}`,
		`{"n": 9223372036854775808}`, `{"n": -0.0}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, doc string) {
		ty, err := parseDocument([]byte(doc))
		if err != nil || ty.Kind != spec.TypeObject {
			return
		}
		body := renderObject(ty)
		var back any
		if err := yaml.Unmarshal([]byte(body), &back); err != nil {
			t.Fatalf("inferred a shape that is not YAML:\ninput %q\nshape %q\nerror %v", doc, body, err)
		}
		sameShape(t, ty, back, doc, body, "shape")
	})
}

// sameShape checks that the YAML read back out says what was inferred.
func sameShape(t *testing.T, want *spec.Type, got any, doc, body, path string) {
	t.Helper()
	fail := func(why string, args ...any) {
		t.Fatalf("%s %s\ninput %q\nshape %q", path, fmt.Sprintf(why, args...), doc, body)
	}
	switch want.Kind {
	case spec.TypeObject:
		m, ok := got.(map[string]any)
		if !ok {
			fail("came back as %T, not a mapping", got)
		}
		if len(m) != len(want.Fields) {
			fail("came back with %d keys, want %d", len(m), len(want.Fields))
		}
		for _, f := range want.Fields {
			v, ok := m[f.Name]
			if !ok {
				fail("lost the key %q", f.Name)
			}
			sameShape(t, f.Type, v, doc, body, path+"."+f.Name)
		}
	case spec.TypeList:
		l, ok := got.([]any)
		if !ok {
			fail("came back as %T, not a list", got)
		}
		if len(l) != 1 {
			fail("came back with %d elements, want the one that names the type", len(l))
		}
		sameShape(t, want.Elem, l[0], doc, body, path+"[]")
	default:
		if got != want.Kind.String() {
			fail("came back as %#v, want %q", got, want.Kind.String())
		}
	}
}
