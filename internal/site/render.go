package site

import (
	"bytes"
	"html"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gohtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

const repo = "https://github.com/orochi235/perch"

func (s *Site) renderMarkdown(p *Page) error {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(&links{site: s, page: p}, 100)),
		),
		goldmark.WithRendererOptions(gohtml.WithUnsafe()),
	)
	var out bytes.Buffer
	if err := md.Convert([]byte(p.markdown), &out); err != nil {
		return err
	}
	rendered := out.String()
	for _, b := range p.blocks {
		rendered = strings.Replace(rendered, b.placeholder(), b.html, 1)
	}
	p.html = rendered
	return nil
}

// links sends every relative link somewhere that exists on the site, and
// everything else to the file on GitHub.
type links struct {
	site *Site
	page *Page
}

func (t *links) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if link, ok := n.(*ast.Link); ok {
			link.Destination = []byte(t.site.rewrite(t.page, string(link.Destination)))
		}
		return ast.WalkContinue, nil
	})
}

// rewrite resolves one link written for a reader of the markdown. An anchor
// into a schema.md section that is now its own page is the case that matters:
// the text is unchanged and the target moved.
func (s *Site) rewrite(p *Page, dest string) string {
	if dest == "" || strings.Contains(dest, "://") || strings.HasPrefix(dest, "mailto:") {
		return dest
	}
	path, frag, _ := strings.Cut(dest, "#")

	if path == "" {
		if target, ok := s.anchors[p.Source+"#"+frag]; ok && target != p {
			return p.linkTo(target) + "#" + frag
		}
		return dest
	}

	source := filepath.ToSlash(filepath.Join(filepath.Dir(p.Source), path))
	if frag != "" {
		if target, ok := s.anchors[source+"#"+frag]; ok {
			return p.linkTo(target) + "#" + frag
		}
	}
	if target := s.pageFor(source); target != nil {
		return p.linkTo(target)
	}
	return repo + "/blob/main/" + source
}

// pageFor is the page a whole file reads as: for schema.md, the one holding
// everything above its first section.
func (s *Site) pageFor(source string) *Page {
	for _, p := range s.pages {
		if p.Source != source {
			continue
		}
		if p.Schema == "" || p.Schema == introSection {
			return p
		}
	}
	return nil
}

func codeHTML(lang, code string) string {
	if lang == "yaml" {
		return `<pre><code class="yaml">` + highlightYAML(code) + `</code></pre>`
	}
	class := ""
	if lang != "" {
		class = ` class="` + html.EscapeString(lang) + `"`
	}
	return `<pre><code` + class + `>` + html.EscapeString(code) + `</code></pre>`
}
