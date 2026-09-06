package celswift

import (
	"fmt"
	"strings"
)

// LowerTemplate lowers text containing {{ }} holes to a Swift String
// expression. Literal text is escaped; each hole becomes an interpolation.
func (e *Env) LowerTemplate(src string) (string, error) {
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
		b.WriteString(`\(` + lowered + `)`)
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
