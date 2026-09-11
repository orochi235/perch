package site

import (
	"html"
	"strings"
)

// highlightYAML colors a menubar.yaml the way the docs read it: keys, strings,
// the values YAML gives meaning to, and comments. It is a few dozen lines
// because the alternative is a syntax-highlighting library, and this file's
// grammar is the subset perch accepts.
func highlightYAML(src string) string {
	var b strings.Builder
	for i, line := range strings.Split(src, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(highlightLine(line))
	}
	return b.String()
}

func highlightLine(line string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	rest := line[len(indent):]
	var b strings.Builder
	b.WriteString(indent)

	if strings.HasPrefix(rest, "#") {
		b.WriteString(span("c", rest))
		return b.String()
	}
	for strings.HasPrefix(rest, "- ") || rest == "-" {
		b.WriteString(span("p", "-"))
		if rest == "-" {
			return b.String()
		}
		b.WriteString(" ")
		rest = rest[2:]
	}
	if key, value, ok := splitKey(rest); ok {
		b.WriteString(span("k", key))
		b.WriteString(span("p", ":"))
		b.WriteString(highlightValue(value))
		return b.String()
	}
	b.WriteString(highlightValue(rest))
	return b.String()
}

// splitKey finds the `key:` a line binds, which is a plain scalar followed by a
// colon and then a space or nothing.
func splitKey(s string) (string, string, bool) {
	for i, r := range s {
		if r == ':' && (i+1 == len(s) || s[i+1] == ' ') {
			key := s[:i]
			if key == "" || strings.ContainsAny(key, `"'{}[]#`) {
				return "", "", false
			}
			return key, s[i+1:], true
		}
		if r == '#' || r == '{' || r == '[' || r == '"' || r == '\'' {
			return "", "", false
		}
	}
	return "", "", false
}

var literals = map[string]bool{"true": true, "false": true, "null": true, "separator": true}

// highlightValue scans what follows a key: strings, the flow mappings the docs
// use for short items, and an inline comment.
func highlightValue(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '"' || c == '\'':
			end := closingQuote(s, i)
			b.WriteString(span("s", s[i:end]))
			i = end
		case c == '#' && (i == 0 || s[i-1] == ' '):
			b.WriteString(span("c", s[i:]))
			return b.String()
		case c == '{' || c == '}' || c == '[' || c == ']' || c == ',' || c == ':':
			b.WriteString(span("p", string(c)))
			i++
		default:
			word, next := readWord(s, i)
			switch {
			case word == "":
				b.WriteString(html.EscapeString(string(c)))
				i++
				continue
			case next < len(s) && s[next] == ':':
				b.WriteString(span("k", word))
			case literals[word] || isNumber(word):
				b.WriteString(span("n", word))
			default:
				b.WriteString(html.EscapeString(word))
			}
			i = next
		}
	}
	return b.String()
}

func closingQuote(s string, start int) int {
	q := s[start]
	for i := start + 1; i < len(s); i++ {
		if s[i] == q {
			return i + 1
		}
	}
	return len(s)
}

func readWord(s string, start int) (string, int) {
	i := start
	for i < len(s) && !strings.ContainsRune(" ,{}[]:#\"'", rune(s[i])) {
		i++
	}
	return s[start:i], i
}

func isNumber(word string) bool {
	if word == "" {
		return false
	}
	for _, r := range word {
		if (r < '0' || r > '9') && r != '.' && r != '-' {
			return false
		}
	}
	return true
}

func span(class, text string) string {
	return `<span class="` + class + `">` + html.EscapeString(text) + `</span>`
}
