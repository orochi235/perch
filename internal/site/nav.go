// Package site builds perch's documentation site out of the markdown in docs/.
// A page's menu previews are run, not drawn: see internal/preview.
package site

import "strings"

// Section is one heading in the sidebar.
type Section struct {
	Title string
	Pages []*Page
}

// Page is one page of the site. Source is the markdown it comes from; Schema
// names the docs/schema.md section it was split out of, since that file stays
// whole for a reader on GitHub.
type Page struct {
	Title   string
	URL     string // "" is the front page; otherwise a directory path
	Source  string
	Schema  string
	Section string

	markdown string
	blocks   []*block
	anchors  []string
	html     string
}

// nav is the site's order and its sidebar labels. A docs/schema.md section
// missing from it fails the build rather than going unpublished.
func nav() []*Section {
	return []*Section{
		{Title: "Guide", Pages: []*Page{
			{Title: "Overview", URL: "", Source: "docs/guide/overview.md"},
			{Title: "Install", URL: "guide/install/", Source: "docs/guide/install.md"},
			{Title: "Your first app", URL: "guide/first-app/", Source: "docs/guide/first-app.md"},
			{Title: "Commands", URL: "guide/commands/", Source: "docs/guide/commands.md"},
		}},
		{Title: "menubar.yaml", Pages: []*Page{
			{Title: "The file", URL: "menubar/", Source: "docs/schema.md", Schema: introSection},
			{Title: "app", URL: "menubar/app/", Source: "docs/schema.md", Schema: "app"},
			{Title: "watch", URL: "menubar/watch/", Source: "docs/schema.md", Schema: "watch"},
			{Title: "status", URL: "menubar/status/", Source: "docs/schema.md", Schema: "status"},
			{Title: "menu", URL: "menubar/menu/", Source: "docs/schema.md", Schema: "menu"},
			{Title: "Expressions", URL: "menubar/expressions/", Source: "docs/schema.md", Schema: "expressions"},
			{Title: "Files in a repo", URL: "menubar/files/", Source: "docs/schema.md", Schema: "files-in-a-consuming-repo"},
			{Title: "Icon assets", URL: "menubar/icons/", Source: "docs/schema.md", Schema: "icon-assets"},
		}},
		{Title: "Recipes", Pages: []*Page{
			{Title: "Start and stop a LaunchAgent", URL: "recipes/launchagent/", Source: "docs/recipes/launchagent.md"},
			{Title: "Jobs, with a submenu each", URL: "recipes/jobs/", Source: "docs/recipes/jobs.md"},
			{Title: "Is my server up?", URL: "recipes/server/", Source: "docs/recipes/server.md"},
			{Title: "Homebrew updates", URL: "recipes/brew/", Source: "docs/recipes/brew.md"},
			{Title: "Uncommitted changes", URL: "recipes/git/", Source: "docs/recipes/git.md"},
			{Title: "Your own artwork", URL: "recipes/artwork/", Source: "docs/recipes/artwork.md"},
		}},
	}
}

// introSection stands for everything in schema.md above its first heading.
const introSection = "\x00intro"

// depth is how many directories down a page sits, which is how far its links
// have to climb: the site is published under /perch/, not at a root.
func (p *Page) depth() int { return strings.Count(strings.TrimSuffix(p.URL, "/"), "/") + boolToInt(p.URL != "") }

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (p *Page) root() string { return strings.Repeat("../", p.depth()) }

// linkTo writes one page's URL as seen from another.
func (p *Page) linkTo(other *Page) string {
	target := p.root() + other.URL
	if target == "" {
		return "./"
	}
	return target
}
