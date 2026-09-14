package spec

import (
	"fmt"
	"net/url"

	"gopkg.in/yaml.v3"
)

// Window is the one WebKit window an app may declare. One per app, matching
// one status item per app, which is why nothing names it.
type Window struct {
	URL    string
	Title  string
	Width  int
	Height int
	// Zoom is nil when the author asked for no zoom controls, which is what
	// leaves Actual Size, Zoom In and Zoom Out out of the View menu.
	Zoom *Zoom
}

// Zoom bounds the page zoom. Clamped so a stray Cmd-+ cannot zoom a page into
// uselessness.
type Zoom struct {
	Min  float64
	Max  float64
	Step float64
}

const (
	defaultWindowWidth  = 1024
	defaultWindowHeight = 768
)

type rawWindow struct {
	URL   string   `yaml:"url"`
	Title string   `yaml:"title"`
	Size  []int    `yaml:"size"`
	Zoom  *rawZoom `yaml:"zoom"`
}

type rawZoom struct {
	Min  float64 `yaml:"min"`
	Max  float64 `yaml:"max"`
	Step float64 `yaml:"step"`
}

// parseWindow reads the window: block. appName is the title's default, so an
// app that wants its own name on the window says nothing.
func parseWindow(n *yaml.Node, appName string) (*Window, error) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	var raw rawWindow
	if err := decodeStrict(n, &raw, "window"); err != nil {
		return nil, err
	}
	w := &Window{
		URL:    raw.URL,
		Title:  raw.Title,
		Width:  defaultWindowWidth,
		Height: defaultWindowHeight,
	}
	if w.Title == "" {
		w.Title = appName
	}
	switch len(raw.Size) {
	case 0:
	case 2:
		w.Width, w.Height = raw.Size[0], raw.Size[1]
	default:
		return nil, fmt.Errorf("window.size: want [width, height], got %d value(s)", len(raw.Size))
	}
	if raw.Zoom != nil {
		w.Zoom = &Zoom{Min: raw.Zoom.Min, Max: raw.Zoom.Max, Step: raw.Zoom.Step}
	}
	return w, nil
}

func (w *Window) validate() error {
	if w.URL == "" {
		return fmt.Errorf("window.url: required (the page the window loads)")
	}
	u, err := url.Parse(w.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("window.url: %q is not a URL with a scheme and a host", w.URL)
	}
	if w.Width <= 0 || w.Height <= 0 {
		return fmt.Errorf("window.size: both values must be positive, got %dx%d", w.Width, w.Height)
	}
	if z := w.Zoom; z != nil {
		if z.Min <= 0 || z.Max <= 0 || z.Step <= 0 {
			return fmt.Errorf("window.zoom: min, max and step must all be positive, got %v/%v/%v", z.Min, z.Max, z.Step)
		}
		if z.Min >= z.Max {
			return fmt.Errorf("window.zoom: min %v is not below max %v, so there is nothing to zoom between", z.Min, z.Max)
		}
	}
	return nil
}
