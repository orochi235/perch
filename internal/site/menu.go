package site

import (
	"fmt"
	"html"
	"strings"

	"github.com/orochi235/perch/v2/internal/preview"
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
		menuTint, menuStyle := "", ""
		if f.Face.Tint != nil && f.Face.Tint.Menu {
			_, menuStyle = tintAttrs(f.Face.Tint)
			menuTint = " menu-tinted"
		}
		b.WriteString(fmt.Sprintf(`<div class="frame%s%s" data-frame="%d"%s>`, on(i == 0), menuTint, i, menuStyle))
		b.WriteString(barHTML(root, f))
		b.WriteString(menuHTML(root, f.Menu))
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
	tint, style := tintAttrs(f.Face.Tint)
	b.WriteString(`<div class="bar"><span class="statusitem` + dimmed(f.Face.Dim) + tint + `"` + style + `>`)
	wrapped := f.Face.Tint != nil && f.Face.Tint.Style == "chip" && f.Face.Tint.Wrap == "all"
	if wrapped {
		b.WriteString(`<span class="chip">`)
	}
	b.WriteString(iconHTML(root, f.Face.Icon))
	if f.Face.Badge != "" {
		b.WriteString(`<span class="badge">` + html.EscapeString(f.Face.Badge) + `</span>`)
	}
	if wrapped {
		b.WriteString(`</span>`)
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

// iconHTML draws artwork as itself, and an SF Symbol as its look-alike with
// the real name on hover.
func iconHTML(root string, icon preview.Icon) string {
	if icon.Asset != "" {
		return fmt.Sprintf(`<img class="icon art" src="%sicons/%s.png" alt="">`,
			root, html.EscapeString(icon.Asset))
	}
	svg, _ := symbolSVG(icon.Symbol)
	style := ""
	if icon.FG != "" || icon.BG != "" {
		style = colorStyle(icon.FG, icon.BG)
	}
	return fmt.Sprintf(`<span class="icon symbol"%s title="SF Symbol: %s">%s</span>`,
		style, html.EscapeString(icon.Symbol), svg)
}

// colorStyle carries a per-item color, which is data, so it cannot be a class.
func colorStyle(fg, bg string) string {
	ink, patch := inkColors(fg, bg)
	var decls []string
	if ink != "" {
		decls = append(decls, "color: "+ink)
	}
	if patch != "" {
		decls = append(decls, "background: "+patch)
	}
	if len(decls) == 0 {
		return ""
	}
	return ` style="` + html.EscapeString(strings.Join(decls, "; ")) + `"`
}

// withBadges sets the pills beside an escaped title, on the side each names.
func withBadges(title string, badges []preview.Badge) string {
	var before, after strings.Builder
	for _, b := range badges {
		class := "pill"
		if cssColor(b.BG) == "" {
			class += " outline"
		}
		glyph := ""
		if svg, ok := symbolSVG(b.Symbol); ok {
			glyph = `<span class="glyph" title="SF Symbol: ` + html.EscapeString(b.Symbol) + `">` + svg + `</span>`
		}
		pill := `<span class="` + class + `"` + colorStyle(b.FG, b.BG) + `>` + glyph + html.EscapeString(b.Text) + `</span>`
		if b.At == "start" {
			before.WriteString(pill)
		} else {
			after.WriteString(pill)
		}
	}
	return before.String() + title + after.String()
}

func menuHTML(root string, nodes []preview.Node) string {
	var b strings.Builder
	b.WriteString(`<div class="menu">`)
	for _, n := range nodes {
		b.WriteString(nodeHTML(root, n))
	}
	if len(nodes) == 0 {
		b.WriteString(`<div class="row label">no items</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func nodeHTML(root string, n preview.Node) string {
	if n.Separator {
		return `<div class="sep"></div>`
	}
	if n.Field != "" {
		tip := ""
		if n.Action != nil {
			tip = ` title="` + html.EscapeString(actionText(*n.Action)) + `"`
		}
		return `<div class="row field"` + tip + `><span class="input">` + html.EscapeString(n.Field) + `</span></div>`
	}
	title := withBadges(html.EscapeString(n.Title), n.Badges)
	if n.Icon != nil && (n.Icon.Symbol != "" || n.Icon.Asset != "") {
		title = iconHTML(root, *n.Icon) + title
	}
	if n.On != nil {
		mark := ""
		if *n.On {
			mark = "✓"
		}
		title = `<span class="check">` + mark + `</span>` + title
	}
	if n.Subtitle != "" {
		title += `<span class="subtitle">` + html.EscapeString(n.Subtitle) + `</span>`
	}
	if n.Key != "" {
		title += `<span class="key">` + html.EscapeString(n.Key) + `</span>`
	}
	look := lookAttrs(n)
	if len(n.Items) > 0 {
		var sub strings.Builder
		for _, item := range n.Items {
			sub.WriteString(nodeHTML(root, item))
		}
		return `<div class="row sub` + rowClass(n, true) + `" tabindex="0"` + look + `>` + title +
			`<span class="chev">›</span><div class="menu submenu">` + sub.String() + `</div></div>`
	}
	if n.Action == nil {
		return `<div class="row` + rowClass(n, false) + `"` + look + `>` + title + `</div>`
	}
	return `<div class="row` + rowClass(n, true) + `"` + look + ` title="` + html.EscapeString(actionText(*n.Action)) + `">` + title + `</div>`
}

// rowClass is how a row reads: a section header, live, or grayed out, which
// enabled: decides where it is set and whether the item acts where it is not.
func rowClass(n preview.Node, acts bool) string {
	if n.Header {
		return " header"
	}
	if n.Enabled != nil {
		acts = *n.Enabled
	}
	switch {
	case !acts:
		return " label"
	case len(n.Items) > 0:
		return ""
	}
	return " action"
}

// lookAttrs carries a row's size, weight and indent, which are per-item
// numbers and names that cannot each be a class.
func lookAttrs(n preview.Node) string {
	var decls []string
	if n.Size > 0 {
		decls = append(decls, fmt.Sprintf("font-size: %.3fem", n.Size/13))
	}
	if w, ok := cssWeights[n.Weight]; ok {
		decls = append(decls, "font-weight: "+w)
	}
	if n.Indent > 0 {
		decls = append(decls, fmt.Sprintf("padding-left: %dpx", 12+10*n.Indent))
	}
	if len(decls) == 0 {
		return ""
	}
	return ` style="` + html.EscapeString(strings.Join(decls, "; ")) + `"`
}

var cssWeights = map[string]string{
	"ultralight": "100", "thin": "200", "light": "300", "regular": "400", "medium": "500",
	"semibold": "600", "bold": "700", "heavy": "800", "black": "900",
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
	case a.Agent != nil:
		return a.Agent.Verb + "s " + a.Agent.Label
	default:
		return "runs " + strings.Join(a.Run, " ")
	}
}
