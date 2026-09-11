package site

import (
	"embed"
	"fmt"
	"html"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/orochi235/perch/internal/preview"
	"github.com/orochi235/perch/internal/spec"
)

//go:embed assets
var assets embed.FS

// Build renders the site from the repo at root into out.
func Build(root, out string) error {
	s, err := Load(root)
	if err != nil {
		return err
	}
	if err := s.render(root); err != nil {
		return err
	}
	if err := s.write(root, out); err != nil {
		return err
	}
	return checkLinks(out)
}

type previewJob struct {
	page  *Page
	block *block
}

func (s *Site) render(root string) error {
	var jobs []previewJob
	for _, p := range s.pages {
		for _, b := range p.blocks {
			switch {
			case b.yaml != "" && len(b.states) > 0:
				jobs = append(jobs, previewJob{page: p, block: b})
			case b.yaml != "":
				b.html = codeHTML("yaml", b.yaml)
			case b.help != "":
				text, err := s.help(root, b.help)
				if err != nil {
					return err
				}
				b.html = `<pre class="help"><code>` + html.EscapeString(text) + `</code></pre>`
			default:
				b.html = codeHTML(b.lang, b.code)
			}
		}
	}
	if err := runPreviews(jobs); err != nil {
		return err
	}
	for _, p := range s.pages {
		if err := s.renderMarkdown(p); err != nil {
			return err
		}
	}
	return nil
}

// runPreviews compiles and runs each page's previews. Every one is a swiftc
// run, so they go concurrently, and the cache means a rebuild after an edit to
// the prose compiles nothing at all.
func runPreviews(jobs []previewJob) error {
	if len(jobs) == 0 {
		return nil
	}
	r := preview.Renderer{CacheDir: cacheDir()}
	limit := max(runtime.NumCPU()/2, 1)
	tokens := make(chan struct{}, limit)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var failed error

	for _, job := range jobs {
		wg.Add(1)
		go func(job previewJob) {
			defer wg.Done()
			tokens <- struct{}{}
			defer func() { <-tokens }()
			if err := job.run(r); err != nil {
				mu.Lock()
				if failed == nil {
					failed = err
				}
				mu.Unlock()
			}
		}(job)
	}
	wg.Wait()
	return failed
}

func (j previewJob) run(r preview.Renderer) error {
	where := fmt.Sprintf("%s:%d", j.page.Source, j.block.line)
	s, err := spec.Parse([]byte(j.block.yaml))
	if err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	states := make([]preview.State, 0, len(j.block.states))
	for _, fence := range j.block.states {
		state, err := preview.ParseState(fence.name, []byte(fence.body), s)
		if err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		states = append(states, state)
	}
	frames, err := r.Render(s, states)
	if err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	j.block.html = previewHTML(j.page.root(), j.block.yaml, frames)
	return nil
}

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "perch", "preview")
}

// help is what `perch <command> -h` prints, so the flags a page shows are the
// binary's own.
func (s *Site) help(root, command string) (string, error) {
	bin, err := s.perch(root)
	if err != nil {
		return "", err
	}
	out, _ := exec.Command(bin, strings.Fields(command+" -h")...).CombinedOutput()
	if len(out) == 0 {
		return "", fmt.Errorf("perch %s -h printed nothing", command)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func (s *Site) perch(root string) (string, error) {
	if s.perchBin != "" {
		return s.perchBin, nil
	}
	dir, err := os.MkdirTemp("", "perch-site-")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "perch")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/perch")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("building perch for its own usage text: %w\n%s", err, out)
	}
	s.perchBin = bin
	return bin, nil
}

type pageData struct {
	Title    string
	Section  string
	Root     string
	Repo     string
	Content  template.HTML
	Sections []navSection
	Prev     *navLink
	Next     *navLink
}

type navSection struct {
	Title string
	Links []navLink
}

type navLink struct {
	Title   string
	Href    string
	Current bool
}

func (s *Site) write(root, out string) error {
	tmpl, err := template.ParseFS(assets, "assets/layout.html")
	if err != nil {
		return err
	}
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	for i, p := range s.pages {
		data := pageData{
			Title:    p.Title,
			Section:  p.Section,
			Root:     p.root(),
			Repo:     repo,
			Content:  template.HTML(p.html),
			Sections: s.navFor(p),
		}
		if i > 0 {
			data.Prev = &navLink{Title: s.pages[i-1].Title, Href: p.linkTo(s.pages[i-1])}
		}
		if i+1 < len(s.pages) {
			data.Next = &navLink{Title: s.pages[i+1].Title, Href: p.linkTo(s.pages[i+1])}
		}
		dir := filepath.Join(out, filepath.FromSlash(p.URL))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		var page strings.Builder
		if err := tmpl.Execute(&page, data); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(page.String()), 0o644); err != nil {
			return err
		}
	}
	return s.writeAssets(root, out)
}

func (s *Site) navFor(current *Page) []navSection {
	out := make([]navSection, 0, len(s.Sections))
	for _, section := range s.Sections {
		links := make([]navLink, 0, len(section.Pages))
		for _, p := range section.Pages {
			links = append(links, navLink{
				Title:   p.Title,
				Href:    current.linkTo(p),
				Current: p == current,
			})
		}
		out = append(out, navSection{Title: section.Title, Links: links})
	}
	return out
}

func (s *Site) writeAssets(root, out string) error {
	css, err := assets.ReadFile("assets/site.css")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "site.css"), css, 0o644); err != nil {
		return err
	}

	icons, err := filepath.Glob(filepath.Join(root, "docs", "recipes", "icons", "*.png"))
	if err != nil || len(icons) == 0 {
		return err
	}
	if err := os.MkdirAll(filepath.Join(out, "icons"), 0o755); err != nil {
		return err
	}
	for _, src := range icons {
		body, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, "icons", filepath.Base(src)), body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

var href = regexp.MustCompile(`(?:href|src)="([^"]+)"`)

// checkLinks reads the built site back and follows every link that stays on it.
// A page moved in the nav otherwise leaves a link that only a reader finds.
func checkLinks(out string) error {
	var broken []string
	err := filepath.Walk(out, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".html" {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range href.FindAllStringSubmatch(string(body), -1) {
			target, ok := resolve(out, path, m[1])
			if !ok {
				continue
			}
			if _, err := os.Stat(target); err != nil {
				rel, _ := filepath.Rel(out, path)
				broken = append(broken, fmt.Sprintf("%s → %s", rel, m[1]))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(broken) > 0 {
		return fmt.Errorf("the built site has links to nothing:\n  %s", strings.Join(broken, "\n  "))
	}
	return nil
}

// resolve turns one link into the file it wants, or reports that it leaves the
// site.
func resolve(out, from, link string) (string, bool) {
	if link == "" || strings.Contains(link, "://") || strings.HasPrefix(link, "#") || strings.HasPrefix(link, "mailto:") {
		return "", false
	}
	path, _, _ := strings.Cut(link, "#")
	if path == "" {
		return "", false
	}
	target := filepath.Join(filepath.Dir(from), filepath.FromSlash(path))
	if strings.HasSuffix(path, "/") || filepath.Ext(path) == "" {
		target = filepath.Join(target, "index.html")
	}
	return target, true
}
