package shape

import (
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
// never trusted: a panic here is perch crashing on a command's output.
func FuzzInfer(f *testing.F) {
	for _, seed := range []string{
		`{}`, `{"a": 1}`, `{"a": [1, "x", null]}`, `[]`, `null`, `not json`,
		`{"a": {"b": {"c": [[[1]]]}}}`, `{"": 1}`, `{"a": 1e999}`,
		`{"a": 1} {"b": 2}`, `{"a": 1}]`, "\x00", `{"a": "\ud800"}`,
		`{"n": 9223372036854775808}`, `{"n": -0.0}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, doc string) {
		body, err := Infer([]byte(doc))
		if err != nil {
			return
		}
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(body), &node); err != nil {
			t.Fatalf("inferred a shape that is not YAML:\ninput %q\nshape %q\nerror %v", doc, body, err)
		}
		// A key perch cannot use as an identifier is quoted on purpose, so the
		// block still parses and perch build names the line. Those do not
		// round-trip, and are covered by TestInferQuotesKeysThatAreNotIdentifiers.
		if strings.Contains(body, `"`) {
			return
		}
		got, err := reparse(t, body)
		if err != nil {
			t.Fatalf("inferred a shape perch build rejects:\ninput  %q\nshape  %q\nerror  %v", doc, body, err)
		}
		if again := renderObject(got); again != body {
			t.Fatalf("shape does not round-trip:\ninput  %q\n first %q\nsecond %q", doc, body, again)
		}
	})
}
