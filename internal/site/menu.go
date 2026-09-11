package site

import (
	"fmt"
	"html"
	"strings"

	"github.com/orochi235/perch/internal/preview"
)

// previewHTML draws what the emitted app decided: the status item in a menu
// bar, and the menu it opens, one tab per state the page declared.
func previewHTML(root string, yaml string, frames []preview.Frame) string {
	var b strings.Builder
	b.WriteString(`<figure class="preview">`)
	b.WriteString(`<div class="file">` + `<pre><code class="yaml">` + highlightYAML(yaml) + `</code></pre></div>`)
	b.WriteString(`<div class="built">`)

	if len(frames) > 1 {
		b.WriteString(`<div class="states">`)
		for i, f := range frames {
			b.WriteString(fmt.Sprintf(`<button type="button" class="state%s" data-frame="%d">%s</button>`,
				on(i == 0), i, html.EscapeString(f.State)))
		}
		b.WriteString(`</div>`)
	}

	for i, f := range frames {
		b.WriteString(fmt.Sprintf(`<div class="frame%s" data-frame="%d">`, on(i == 0), i))
		b.WriteString(barHTML(root, f))
		b.WriteString(menuHTML(f.Menu))
		b.WriteString(`</div>`)
	}

	b.WriteString(`</div>`)
	if len(frames) == 1 {
		b.WriteString(`<figcaption>` + html.EscapeString(frames[0].State) + `</figcaption>`)
	}
	b.WriteString(`</figure>`)
	return b.String()
}

func on(first bool) string {
	if first {
		return " is-on"
	}
	return ""
}

func barHTML(root string, f preview.Frame) string {
	var b strings.Builder
	b.WriteString(`<div class="bar"><span class="statusitem` + dimmed(f.Face.Dim) + `">`)
	b.WriteString(iconHTML(root, f.Face.Icon))
	if f.Face.Badge != "" {
		b.WriteString(`<span class="badge">` + html.EscapeString(f.Face.Badge) + `</span>`)
	}
	b.WriteString(`</span></div>`)
	return b.String()
}

func dimmed(dim bool) string {
	if dim {
		return " is-dim"
	}
	return ""
}

// iconHTML draws artwork as itself. An SF Symbol is Apple's to draw and is
// licensed for apps on Apple's platforms, so a web page names it instead.
func iconHTML(root string, icon preview.Icon) string {
	if icon.Asset != "" {
		return fmt.Sprintf(`<img class="icon art" src="%sicons/%s.png" alt="">`,
			root, html.EscapeString(icon.Asset))
	}
	return fmt.Sprintf(`<span class="icon symbol" title="SF Symbol: %s"></span>`,
		html.EscapeString(icon.Symbol))
}

func menuHTML(nodes []preview.Node) string {
	var b strings.Builder
	b.WriteString(`<div class="menu">`)
	for _, n := range nodes {
		b.WriteString(nodeHTML(n))
	}
	if len(nodes) == 0 {
		b.WriteString(`<div class="row label">no items</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func nodeHTML(n preview.Node) string {
	if n.Separator {
		return `<div class="sep"></div>`
	}
	title := html.EscapeString(n.Title)
	if len(n.Items) > 0 {
		var sub strings.Builder
		for _, item := range n.Items {
			sub.WriteString(nodeHTML(item))
		}
		return `<div class="row sub" tabindex="0">` + title +
			`<span class="chev">›</span><div class="menu submenu">` + sub.String() + `</div></div>`
	}
	if n.Action == nil {
		return `<div class="row label">` + title + `</div>`
	}
	return `<div class="row action" title="` + html.EscapeString(actionText(*n.Action)) + `">` + title + `</div>`
}

// actionText says what activating an item would do, which a picture of a menu
// otherwise cannot.
func actionText(a preview.Action) string {
	switch {
	case a.Quit:
		return "quits"
	case a.Open != "":
		return "opens " + a.Open
	case a.Post != nil:
		return "posts " + a.Post.Body + " to " + a.Post.URL
	default:
		return "runs " + strings.Join(a.Run, " ")
	}
}
