package celswift

import (
	"bytes"
	"encoding/json"
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
// every hole sits inside a JSON string. Each string is decoded before its holes
// are read: the expressions in it were written in YAML, not in JSON's escaping.
func (e *Env) LowerJSON(src string) (string, error) {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(src); {
		if src[i] != '"' {
			b.WriteString(escapeInterpolated(src[i : i+1]))
			i++
			continue
		}
		end := jsonStringEnd(src, i)
		var text string
		if end < 0 || json.Unmarshal([]byte(src[i:end]), &text) != nil {
			return "", fmt.Errorf("%q: not a JSON document", src)
		}
		inner, err := e.templateBody(text, jsonText, func(_, lowered string) string { return "Act.jsonEscaped(" + lowered + ")" })
		if err != nil {
			return "", err
		}
		b.WriteString(`\"` + inner + `\"`)
		i = end
	}
	b.WriteByte('"')
	return b.String(), nil
}

// jsonStringEnd is the offset just past the JSON string opening at s[i], or -1.
func jsonStringEnd(s string, i int) int {
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return -1
}

// jsonText is s as the inside of a JSON string, left readable: no \u003c for <.
func jsonText(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	out := strings.TrimSuffix(b.String(), "\n")
	return out[1 : len(out)-1]
}

func (e *Env) lowerTemplate(src string, wrap func(expr, lowered string) string) (string, error) {
	inner, err := e.templateBody(src, func(s string) string { return s }, wrap)
	if err != nil {
		return "", err
	}
	return `"` + inner + `"`, nil
}

// templateBody is the inside of the Swift string literal for src: lit prepares
// literal text before it is escaped for Swift, and wrap encloses each hole.
func (e *Env) templateBody(src string, lit func(string) string, wrap func(expr, lowered string) string) (string, error) {
	var b strings.Builder
	rest := src
	for {
		before, after, found := strings.Cut(rest, "{{")
		b.WriteString(escapeInterpolated(lit(before)))
		if !found {
			break
		}
		end := holeEnd(after)
		if end < 0 {
			return "", fmt.Errorf("%q: unclosed {{", src)
		}
		expr, tail := strings.TrimSpace(after[:end]), after[end+2:]
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

// holeEnd is the offset of the }} that closes a hole, skipping any inside a CEL
// string literal: "a}}b" is one string, not the end of the hole. -1 if none.
func holeEnd(s string) int {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' || c == '\'':
			raw := i > 0 && (s[i-1] == 'r' || s[i-1] == 'R')
			end := stringEnd(s, i, raw)
			if end < 0 {
				return -1
			}
			i = end - 1
		case c == '}' && i+1 < len(s) && s[i+1] == '}':
			return i
		}
	}
	return -1
}

// stringEnd is the offset just past the CEL string literal opening at s[i],
// single or triple quoted, or -1 if it never closes. A raw string has no escapes.
func stringEnd(s string, i int, raw bool) int {
	q := s[i : i+1]
	if strings.HasPrefix(s[i:], q+q+q) {
		q = q + q + q
	}
	for j := i + len(q); j < len(s); j++ {
		if !raw && s[j] == '\\' {
			j++
			continue
		}
		if strings.HasPrefix(s[j:], q) {
			return j + len(q)
		}
	}
	return -1
}
