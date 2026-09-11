package site

import (
	"fmt"
	"strings"
)

// A fenced block leaves the markdown before goldmark sees it, replaced by a
// comment the renderer swaps back for HTML. Code is highlighted here rather
// than by a library, a preview is run, and `help <command>` is the CLI's own
// usage. Rendering them as HTML in place is not an option: a markdown HTML
// block ends at the first blank line, and code has blank lines in it.
type block struct {
	id     int
	lang   string
	code   string
	help   string
	yaml   string
	line   int
	states []stateFence
	html   string
}

type stateFence struct {
	name string
	body string
}

func (b *block) placeholder() string { return fmt.Sprintf("<!--perch-block-%d-->", b.id) }

// extractFences rewrites src with every fenced block replaced by a placeholder.
// A yaml fence directly followed by state fences is one block: the file, and
// what it builds.
func extractFences(src string) (string, []*block) {
	lines := strings.Split(src, "\n")
	var out []string
	var blocks []*block

	for i := 0; i < len(lines); i++ {
		info, ok := fenceInfo(lines[i])
		if !ok {
			out = append(out, lines[i])
			continue
		}
		body, end := fenceBody(lines, i)
		b := &block{id: len(blocks), line: i + 2}
		switch {
		case info == "yaml":
			b.yaml = body
			end = collectStates(lines, end, b)
		case strings.HasPrefix(info, "state"):
			// A state fence with no file above it has nothing to render, so it
			// stays a code block rather than disappearing.
			b.lang, b.code = "yaml", body
		case strings.HasPrefix(info, "help "):
			b.help = strings.TrimSpace(strings.TrimPrefix(info, "help"))
		default:
			b.lang, b.code = info, body
		}
		blocks = append(blocks, b)
		out = append(out, b.placeholder())
		i = end
	}
	return strings.Join(out, "\n"), blocks
}

// collectStates takes the state fences following a yaml fence, across blank
// lines but nothing else.
func collectStates(lines []string, from int, b *block) int {
	end := from
	for i := from + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		info, ok := fenceInfo(lines[i])
		if !ok || !strings.HasPrefix(info, "state") {
			return end
		}
		body, stop := fenceBody(lines, i)
		b.states = append(b.states, stateFence{
			name: strings.TrimSpace(strings.TrimPrefix(info, "state")),
			body: body,
		})
		i, end = stop, stop
	}
	return end
}

func fenceInfo(line string) (string, bool) {
	if !strings.HasPrefix(line, "```") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "```")), true
}

// fenceBody returns what is inside the fence opening at start, and the line the
// fence closes on.
func fenceBody(lines []string, start int) (string, int) {
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			return strings.Join(lines[start+1:i], "\n"), i
		}
	}
	return strings.Join(lines[start+1:], "\n"), len(lines) - 1
}
