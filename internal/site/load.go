package site

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Site is every page, loaded and split, before anything is rendered.
type Site struct {
	Root     string
	Sections []*Section
	pages    []*Page
	anchors  map[string]*Page // "docs/schema.md#icon-assets" -> the page holding it
	perchBin string           // built once, for the usage text the commands page quotes
}

// Load reads every page named in the nav, splitting docs/schema.md into the
// pages that quote from it.
func Load(root string) (*Site, error) {
	s := &Site{Root: root, Sections: nav(), anchors: map[string]*Page{}}
	sources := map[string]string{}

	for _, section := range s.Sections {
		for _, p := range section.Pages {
			p.Section = section.Title
			src, ok := sources[p.Source]
			if !ok {
				body, err := os.ReadFile(filepath.Join(root, p.Source))
				if err != nil {
					return nil, err
				}
				src = string(body)
				sources[p.Source] = src
			}
			body, err := s.bodyFor(p, src)
			if err != nil {
				return nil, err
			}
			p.markdown, p.blocks = extractFences(body)
			p.anchors = headings(p.markdown)
			for _, a := range p.anchors {
				s.anchors[p.Source+"#"+a] = p
			}
			s.pages = append(s.pages, p)
		}
	}
	return s, s.checkSchemaIsPublished(sources["docs/schema.md"])
}

func (s *Site) bodyFor(p *Page, src string) (string, error) {
	if p.Schema == "" {
		return src, nil
	}
	section, ok := schemaSection(src, p.Schema)
	if !ok {
		return "", fmt.Errorf("%s has no section %q, which %s publishes", p.Source, p.Schema, p.URL)
	}
	return section, nil
}

// schemaSection returns one `##` section of schema.md as its own page, with its
// headings promoted so the page starts at a first-level heading.
func schemaSection(src, slug string) (string, bool) {
	lines := strings.Split(src, "\n")
	if slug == introSection {
		for i, line := range lines {
			if strings.HasPrefix(line, "## ") {
				return strings.Join(lines[:i], "\n"), true
			}
		}
		return src, true
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") || anchor(strings.TrimPrefix(line, "## ")) != slug {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "## ") {
				end = j
				break
			}
		}
		return promote(lines[i:end]), true
	}
	return "", false
}

// promote lifts every heading by one level, since the section's own heading is
// now the page's title and nothing may skip a level under it.
func promote(lines []string) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		if strings.HasPrefix(line, "##") && !inFence(lines, i) {
			out[i] = strings.TrimPrefix(line, "#")
			continue
		}
		out[i] = line
	}
	return strings.Join(out, "\n")
}

func inFence(lines []string, upto int) bool {
	open := false
	for i := 0; i < upto; i++ {
		if strings.HasPrefix(lines[i], "```") {
			open = !open
		}
	}
	return open
}

// checkSchemaIsPublished fails the build when schema.md grows a section the nav
// does not carry: the alternative is reference that silently never appears.
func (s *Site) checkSchemaIsPublished(src string) error {
	if src == "" {
		return nil
	}
	published := map[string]bool{}
	for _, p := range s.pages {
		if p.Schema != "" {
			published[p.Schema] = true
		}
	}
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") || inFence(lines, i) {
			continue
		}
		if slug := anchor(strings.TrimPrefix(line, "## ")); !published[slug] {
			return fmt.Errorf("docs/schema.md has a section %q that no page publishes; add it to internal/site/nav.go", slug)
		}
	}
	return nil
}

var headingLine = regexp.MustCompile(`(?m)^#{1,4} +(.+)$`)

func headings(md string) []string {
	var out []string
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if inFence(lines, i) {
			continue
		}
		if m := headingLine.FindStringSubmatch(line); m != nil {
			out = append(out, anchor(m[1]))
		}
	}
	return out
}

// anchor is goldmark's heading id for the same text, which is what a link
// written against GitHub's rendering also lands on.
func anchor(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}
