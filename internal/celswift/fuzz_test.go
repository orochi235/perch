package celswift

import (
	"testing"

	"github.com/orochi235/perch/internal/spec"
)

const fuzzSpec = `
app: {name: a, id: b, icon: circle, interval: 1s}
watch:
  raw: {run: [x], json: true}
  fleet:
    run: [y]
    json: true
    shape:
      count: int
      label: string
      jobs: [{id: string}]
      tags: [string]
menu: [{text: Quit, quit: true}]
`

func fuzzEnv(t *testing.T) *Env {
	t.Helper()
	s, err := spec.Parse([]byte(fuzzSpec))
	if err != nil {
		t.Fatalf("spec.Parse: %v", err)
	}
	return NewEnv(s.Watches)
}

// Anything an author can type reaches the lowering, so every entry point has to
// return an error rather than panic. A panic here is perch crashing on a typo.
func FuzzLower(f *testing.F) {
	for _, seed := range []string{
		"", " ", "fleet.ok", "!fleet.ok", "fleet.data.count == 0",
		"fleet.data.jobs[0].id", "size(fleet.data.tags)", `"a" in fleet.data.tags`,
		"fleet.ok ? 1 : 0", "has(raw.data.x)", "raw.data.a.b.c.d",
		"((((((((((", "fleet.", "1e309", "0x", `b"\x00"`, "null",
		"fleet.data.jobs.map(x, x)", "{'a': 1}", "[1,2]", "\x00",
	} {
		f.Add(seed)
	}
	e := fuzzEnv(&testing.T{})
	f.Fuzz(func(t *testing.T, src string) {
		_, _ = e.LowerExpr(src)
		_, _ = e.LowerCondition(src)
		_, _ = e.LowerText(src)
		_, _, _ = e.LowerList(src)
	})
}

// A lowered template is spliced straight into a Swift string literal, so a hole
// that closes one paren too few silently swallows the rest of the file.
func FuzzLowerTemplate(f *testing.F) {
	for _, seed := range []string{
		"", "plain text", "{{fleet.ok}}", "a {{fleet.data.label}} b",
		"{{", "}}", "{{}}", "{{{{}}}}", `say "hi"`, "tab\there",
		"back\\slash", "{{fleet.data.count}}{{fleet.data.count}}", "\x00",
		"unicode ü · ✓", "{{ fleet.ok }}", `{{""}}`, `{{")"}}`, `{{"\\"}}`,
	} {
		f.Add(seed)
	}
	e := fuzzEnv(&testing.T{})
	f.Fuzz(func(t *testing.T, src string) {
		got, err := e.LowerTemplate(src)
		if err != nil {
			return
		}
		if n, ok := scanLiteral(got, 0); !ok || n != len(got) {
			t.Fatalf("%q lowered to %q, which is not one well-formed Swift string literal", src, got)
		}
	})
}

// scanLiteral walks a Swift interpolated string literal starting at s[i] == '"'
// and returns the index just past its closing quote. A lowered template that
// does not scan as exactly one such literal would end early and swallow
// whatever the emitter splices in after it.
func scanLiteral(s string, i int) (int, bool) {
	if i >= len(s) || s[i] != '"' {
		return 0, false
	}
	for i++; i < len(s); {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == '(':
			n, ok := scanInterpolation(s, i)
			if !ok {
				return 0, false
			}
			i = n
		case s[i] == '\\':
			i += 2
		case s[i] == '"':
			return i + 1, true
		case s[i] == '\n' || s[i] == '\r':
			return 0, false // a raw newline does not close, it breaks the literal
		default:
			i++
		}
	}
	return 0, false
}

// scanInterpolation walks a \( … ) hole, including any string literals nested
// inside it, and returns the index just past its closing paren.
func scanInterpolation(s string, i int) (int, bool) {
	i += 2
	for depth := 1; i < len(s); {
		switch s[i] {
		case '"':
			n, ok := scanLiteral(s, i)
			if !ok {
				return 0, false
			}
			i = n
		case '(':
			depth++
			i++
		case ')':
			depth--
			i++
			if depth == 0 {
				return i, true
			}
		default:
			i++
		}
	}
	return 0, false
}
