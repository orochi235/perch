package spec

import "testing"

const windowDoc = `
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
window:
  url: http://localhost:4747/
  title: reviewplex
  size: [1280, 860]
  zoom: {min: 0.5, max: 2.0, step: 0.1}
menu:
  - {text: Open, window: open}
  - {text: Quit, quit: true}
`

func TestWindowParses(t *testing.T) {
	s, err := Parse([]byte(windowDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	w := s.Window
	if w == nil {
		t.Fatal("Window is nil")
	}
	if w.URL != "http://localhost:4747/" || w.Title != "reviewplex" {
		t.Errorf("url/title = %q/%q", w.URL, w.Title)
	}
	if w.Width != 1280 || w.Height != 860 {
		t.Errorf("size = %dx%d, want 1280x860", w.Width, w.Height)
	}
	if w.Zoom == nil || w.Zoom.Min != 0.5 || w.Zoom.Max != 2.0 || w.Zoom.Step != 0.1 {
		t.Errorf("zoom = %+v", w.Zoom)
	}
	if s.Menu[0].Action.Kind != ActionWindow || s.Menu[0].Action.Window != WindowOpen {
		t.Errorf("action = %v/%v", s.Menu[0].Action.Kind, s.Menu[0].Action.Window)
	}
}

func TestWindowDefaults(t *testing.T) {
	s, err := Parse([]byte(`
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
window: {url: "http://localhost:1/"}
menu:
  - {text: Open, window: open}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Window.Title != "w" {
		t.Errorf("title = %q, want the app name", s.Window.Title)
	}
	if s.Window.Width != 1024 || s.Window.Height != 768 {
		t.Errorf("size = %dx%d, want the 1024x768 default", s.Window.Width, s.Window.Height)
	}
	if s.Window.Zoom != nil {
		t.Error("zoom is on without being asked for")
	}
}

func TestWindowRefusals(t *testing.T) {
	cases := map[string]string{
		"no url":            "window: {title: w}",
		"not a url":         `window: {url: "not a url"}`,
		"size of one":       `window: {url: "http://x/", size: [800]}`,
		"size of three":     `window: {url: "http://x/", size: [800, 600, 400]}`,
		"zero size":         `window: {url: "http://x/", size: [0, 600]}`,
		"zoom min over max": `window: {url: "http://x/", zoom: {min: 2.0, max: 0.5, step: 0.1}}`,
		"zoom step zero":    `window: {url: "http://x/", zoom: {min: 0.5, max: 2.0, step: 0}}`,
		"unknown key":       `window: {url: "http://x/", widht: 10}`,
	}
	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(
				"app: {name: w, id: dev.example.w, icon: gear, interval: 5s}\n" + block + "\n"))
			if err == nil {
				t.Fatal("accepted; want a refusal")
			}
		})
	}
}

func TestWindowActionNeedsTheBlock(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
menu:
  - {text: Open, window: open}
`))
	if err == nil {
		t.Fatal("accepted a window: action with no window: block")
	}
}

func TestWindowVerbRefused(t *testing.T) {
	_, err := Parse([]byte(`
app: {name: w, id: dev.example.w, icon: gear, interval: 5s}
window: {url: "http://x/"}
menu:
  - {text: Open, window: maximize}
`))
	if err == nil {
		t.Fatal("accepted an unknown window verb")
	}
}
