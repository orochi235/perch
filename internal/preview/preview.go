// Package preview runs an emitted app's own decisions against stated outcomes,
// so the docs site can show the menu a menubar.yaml builds without a menu bar.
// It compiles the Swift perch emits rather than evaluating anything itself: the
// runtime coerces loosely, and a second evaluator would disagree with the app
// exactly where a doc page is most worth reading — the failing states.
package preview

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/orochi235/perch/v2/internal/backend"
	"github.com/orochi235/perch/v2/internal/backend/swiftappkit"
	"github.com/orochi235/perch/v2/internal/spec"
)

// Icon is what the status item draws: one of the two is set.
type Icon struct {
	Symbol string `json:"symbol,omitempty"`
	Asset  string `json:"asset,omitempty"`
}

// Face is the status item itself.
type Face struct {
	Icon  Icon   `json:"icon"`
	Dim   bool   `json:"dim"`
	Badge string `json:"badge"`
}

type Post struct {
	URL  string `json:"url"`
	Body string `json:"body"`
}

// Action is what activating an item does, with every expression already
// resolved against the state the frame was rendered for.
type Action struct {
	Run   []string `json:"run,omitempty"`
	Open  string   `json:"open,omitempty"`
	Post  *Post    `json:"post,omitempty"`
	Agent *Agent   `json:"agent,omitempty"`
	Quit  bool     `json:"quit,omitempty"`
}

// Agent is an agent: action: what it does, and to which LaunchAgent.
type Agent struct {
	Label string `json:"label"`
	Verb  string `json:"verb"`
}

// Node is one menu entry. An item with no action is a label, which macOS draws
// disabled; one with items is a submenu.
type Node struct {
	Separator bool    `json:"separator,omitempty"`
	Title     string  `json:"title,omitempty"`
	Action    *Action `json:"action,omitempty"`
	Items     []Node  `json:"items,omitempty"`
}

// Frame is what one state shows.
type Frame struct {
	State string `json:"state"`
	Face  Face   `json:"face"`
	Menu  []Node `json:"menu"`
}

// Sample is what one poll of one watch returned.
type Sample struct {
	Code      *int    `json:"code,omitempty"`
	Out       *string `json:"out,omitempty"`
	Err       *string `json:"err,omitempty"`
	Status    *int    `json:"status,omitempty"`
	Exists    *bool   `json:"exists,omitempty"`
	Installed *bool   `json:"installed,omitempty"`
	Loaded    *bool   `json:"loaded,omitempty"`
	Running   *bool   `json:"running,omitempty"`
	Pid       *int    `json:"pid,omitempty"`
}

// State is one named set of outcomes, as a docs page states them. A watch it
// leaves out is one that never answered.
type State struct {
	Name    string            `json:"name"`
	Watches map[string]Sample `json:"watches"`
}

// ParseState reads the YAML of one state fence against the spec it belongs to.
func ParseState(name string, src []byte, s *spec.Spec) (State, error) {
	st := State{Name: name, Watches: map[string]Sample{}}
	var doc node
	if err := unmarshal(src, &doc); err != nil {
		return st, fmt.Errorf("state %q: %w", name, err)
	}
	seen := map[string]bool{}
	record := func(key string, w spec.Watch, v value) error {
		if seen[w.Key()] {
			return fmt.Errorf("state %q: %s is stated twice", name, w.Key())
		}
		seen[w.Key()] = true
		sample, err := parseSample(w, v)
		if err != nil {
			return fmt.Errorf("state %q: %s: %w", name, key, err)
		}
		st.Watches[w.Key()] = sample
		return nil
	}
	for _, entry := range doc.entries {
		if u, ok := useNamed(s, entry.key); ok {
			fields, err := entry.value.fields()
			if err != nil {
				return st, fmt.Errorf("state %q: use %s: want a mapping of its watches (%s)", name, entry.key, joinOr(namesOf(u.Watches), "no watches"))
			}
			for _, f := range fields {
				w, ok := watchIn(u.Watches, f.key)
				if !ok {
					return st, fmt.Errorf("state %q: use %s: no watch named %q; it has %s", name, u.Name, f.key, joinOr(namesOf(u.Watches), "no watches"))
				}
				if err := record(u.Name+"."+f.key, w, f.value); err != nil {
					return st, err
				}
			}
			continue
		}
		if prefix, rest, ok := strings.Cut(entry.key, "."); ok && !strings.Contains(rest, ".") {
			if u, ok := useNamed(s, prefix); ok {
				if _, ok := watchIn(u.Watches, rest); ok {
					return st, fmt.Errorf("state %q: write %s as %s: {%s: …}", name, entry.key, prefix, rest)
				}
				return st, fmt.Errorf("state %q: use %s: no watch named %q; it has %s", name, u.Name, rest, joinOr(namesOf(u.Watches), "no watches"))
			}
		}
		w, ok := watchIn(s.Watches, entry.key)
		if !ok {
			return st, fmt.Errorf("state %q: no watch or use named %q; this file declares %s",
				name, entry.key, describeDeclared(s))
		}
		if err := record(entry.key, w, entry.value); err != nil {
			return st, err
		}
	}
	return st, nil
}

func watchIn(watches []spec.Watch, name string) (spec.Watch, bool) {
	for _, w := range watches {
		if w.Name == name {
			return w, true
		}
	}
	return spec.Watch{}, false
}

func useNamed(s *spec.Spec, name string) (spec.Use, bool) {
	for _, u := range s.Uses {
		if u.Name == name {
			return u, true
		}
	}
	return spec.Use{}, false
}

func namesOf(watches []spec.Watch) []string {
	out := make([]string, 0, len(watches))
	for _, w := range watches {
		out = append(out, w.Name)
	}
	return out
}

func useNames(s *spec.Spec) []string {
	out := make([]string, 0, len(s.Uses))
	for _, u := range s.Uses {
		out = append(out, u.Name)
	}
	return out
}

// joinOr lists names separated by ", ", or empty when there are none.
func joinOr(names []string, empty string) string {
	if len(names) == 0 {
		return empty
	}
	return strings.Join(names, ", ")
}

// describeDeclared lists what a state fence in this file may name, omitting
// whichever of watches and uses this file has none of.
func describeDeclared(s *spec.Spec) string {
	watches, uses := namesOf(s.Watches), useNames(s)
	switch {
	case len(watches) > 0 && len(uses) > 0:
		return fmt.Sprintf("watches %s and uses %s", strings.Join(watches, ", "), strings.Join(uses, ", "))
	case len(watches) > 0:
		return fmt.Sprintf("watches %s", strings.Join(watches, ", "))
	case len(uses) > 0:
		return fmt.Sprintf("uses %s", strings.Join(uses, ", "))
	default:
		return "no watches or uses"
	}
}

func parseSample(w spec.Watch, v value) (Sample, error) {
	var s Sample
	if w.Kind == spec.WatchExists {
		b, err := v.bool()
		if err != nil {
			return s, fmt.Errorf("an exists watch is stated as true or false")
		}
		s.Exists = &b
		return s, nil
	}
	fields, err := v.fields()
	if err != nil {
		return s, fmt.Errorf("want a mapping of %s", strings.Join(sampleKeys(w), ", "))
	}
	for _, f := range fields {
		switch {
		case f.key == "out":
			out, err := f.value.text()
			if err != nil {
				return s, fmt.Errorf("out: %w", err)
			}
			s.Out = &out
		case f.key == "err" && w.Kind == spec.WatchRun:
			text, err := f.value.text()
			if err != nil {
				return s, fmt.Errorf("err: %w", err)
			}
			s.Err = &text
		case f.key == "code" && w.Kind == spec.WatchRun:
			n, err := f.value.number()
			if err != nil {
				return s, fmt.Errorf("code: %w", err)
			}
			s.Code = &n
		case f.key == "status" && w.Kind == spec.WatchHTTP:
			n, err := f.value.number()
			if err != nil {
				return s, fmt.Errorf("status: %w", err)
			}
			s.Status = &n
		case f.key == "installed" && w.Kind == spec.WatchLaunchAgent:
			b, err := f.value.bool()
			if err != nil {
				return s, fmt.Errorf("installed: %w", err)
			}
			s.Installed = &b
		case f.key == "loaded" && w.Kind == spec.WatchLaunchAgent:
			b, err := f.value.bool()
			if err != nil {
				return s, fmt.Errorf("loaded: %w", err)
			}
			s.Loaded = &b
		case f.key == "running" && w.Kind == spec.WatchLaunchAgent:
			b, err := f.value.bool()
			if err != nil {
				return s, fmt.Errorf("running: %w", err)
			}
			s.Running = &b
		case f.key == "pid" && w.Kind == spec.WatchLaunchAgent:
			n, err := f.value.number()
			if err != nil {
				return s, fmt.Errorf("pid: %w", err)
			}
			s.Pid = &n
		default:
			return s, fmt.Errorf("a %s watch does not bind %q; it binds %s",
				w.Kind, f.key, strings.Join(sampleKeys(w), ", "))
		}
	}
	return s, nil
}

// sampleKeys are what a state may say about a watch of this kind. They are the
// fields the watcher fills in, less the ok the kind works out for itself.
func sampleKeys(w spec.Watch) []string {
	switch w.Kind {
	case spec.WatchHTTP:
		return []string{"status", "out"}
	case spec.WatchExists:
		return []string{"true or false"}
	case spec.WatchLaunchAgent:
		return []string{"installed", "loaded", "running", "pid"}
	default:
		return []string{"code", "out", "err"}
	}
}

// Renderer runs previews. CacheDir, where set, keeps each compiled preview
// under the hash of the Swift it came from: a site is rebuilt far more often
// than its emitted Swift changes, and every compile is a swiftc run.
type Renderer struct {
	CacheDir string
}

// Render returns one frame per state, in the order given.
func (r Renderer) Render(s *spec.Spec, states []State) ([]Frame, error) {
	bin, cleanup, err := r.binary(s)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	in, err := json.Marshal(states)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin)
	cmd.Stdin = bytes.NewReader(in)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("the preview did not run: %w\n%s", err, stderr.String())
	}
	var frames []Frame
	if err := json.Unmarshal(out.Bytes(), &frames); err != nil {
		return nil, fmt.Errorf("the preview printed something other than frames: %w\n%s", err, out.String())
	}
	return frames, nil
}

func (r Renderer) binary(s *spec.Spec) (string, func(), error) {
	files, err := swiftappkit.New().EmitPreview(s)
	if err != nil {
		return "", func() {}, err
	}
	cached := ""
	if r.CacheDir != "" {
		cached = filepath.Join(r.CacheDir, sourceHash(files))
		if _, err := os.Stat(cached); err == nil {
			return cached, func() {}, nil
		}
	}

	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		return "", func() {}, fmt.Errorf("swiftc is not on PATH: a preview is the Swift perch emits, compiled")
	}
	dir, err := os.MkdirTemp("", "perch-preview-")
	if err != nil {
		return "", func() {}, err
	}
	clean := func() { os.RemoveAll(dir) }
	var paths []string
	for _, f := range files {
		p := filepath.Join(dir, f.Name)
		if err := os.WriteFile(p, f.Body, 0o644); err != nil {
			clean()
			return "", func() {}, err
		}
		paths = append(paths, p)
	}
	bin := filepath.Join(dir, "preview")
	if out, err := exec.Command(swiftc, append([]string{"-o", bin}, paths...)...).CombinedOutput(); err != nil {
		clean()
		return "", func() {}, fmt.Errorf("compiling the preview: %w\n%s", err, out)
	}
	if cached == "" {
		return bin, clean, nil
	}
	if err := os.MkdirAll(r.CacheDir, 0o755); err != nil {
		return bin, clean, nil
	}
	if err := os.Rename(bin, cached); err != nil {
		return bin, clean, nil
	}
	clean()
	return cached, func() {}, nil
}

func sourceHash(files []backend.File) string {
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%d\x00", f.Name, len(f.Body))
		h.Write(f.Body)
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func numberFrom(text string) (int, error) { return strconv.Atoi(text) }
