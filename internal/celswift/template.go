package celswift

import (
	"fmt"
	"strings"
)

// LowerTemplate lowers text containing {{ }} holes to a Swift String
// expression. Literal text is escaped; each hole becomes an interpolation.
func (e *Env) LowerTemplate(src string) (string, error) {
	return e.lowerTemplate(src, func(_, lowered string) string { return lowered })
}

// LowerURL is LowerTemplate for an open: target: a hole that is exactly query
// is percent-encoded, so the typed text arrives as one query parameter.
func (e *Env) LowerURL(src string) (string, error) {
	return e.lowerTemplate(src, func(expr, lowered string) string {
		if _, ok := e.Query(); ok && expr == "query" {
			return "Act.urlQuery(" + lowered + ")"
		}
		return lowered
	})
}

// LowerJSON is LowerTemplate for a post: body. The body is JSON perch wrote, so
// every hole sits inside a JSON string and is escaped for one.
func (e *Env) LowerJSON(src string) (string, error) {
	return e.lowerTemplate(src, func(_, lowered string) string { return "Act.jsonEscaped(" + lowered + ")" })
}

func (e *Env) lowerTemplate(src string, wrap func(expr, lowered string) string) (string, error) {
	var b strings.Builder
	b.WriteByte('"')
	rest := src
	for {
		before, after, found := strings.Cut(rest, "{{")
		b.WriteString(escapeInterpolated(before))
		if !found {
			break
		}
		expr, tail, closed := strings.Cut(after, "}}")
		if !closed {
			return "", fmt.Errorf("%q: unclosed {{", src)
		}
		expr = strings.TrimSpace(expr)
		if expr == "" {
			return "", fmt.Errorf("%q: empty {{ }}", src)
		}
		lowered, err := e.LowerText(expr)
		if err != nil {
			return "", err
		}
		b.WriteString(`\(` + wrap(expr, lowered) + `)`)
		rest = tail
	}
	b.WriteByte('"')
	return b.String(), nil
}

// escapeInterpolated escapes literal text for a Swift interpolated string
// literal. The backslash case must come first, and \( is what makes an
// interpolation, so a literal one has to be escaped too.
func escapeInterpolated(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\t", `\t`,
		"\r", `\r`,
	)
	return r.Replace(s)
}
